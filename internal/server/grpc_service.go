package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/quibex/wg-agent/internal/wireguard"
	proto "github.com/quibex/wg-agent/pkg/api/proto"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"google.golang.org/protobuf/types/known/emptypb"
)

// agentService реализует WireGuardAgentServer
type agentService struct {
	proto.UnimplementedWireGuardAgentServer
	log            *slog.Logger
	wgClient       wireguard.Client
	defIface       string
	peerStore      *wireguard.PeerStore
	subnet         string // подсеть для выделения IP (например "10.8.0.0/24")
	serverEndpoint string // endpoint сервера для клиентов (например "vpn.example.com:51820")
}

func newAgentService(log *slog.Logger, wgClient wireguard.Client, defIface, subnet, serverEndpoint string) *agentService {
	return &agentService{
		log:            log,
		wgClient:       wgClient,
		defIface:       defIface,
		peerStore:      wireguard.NewPeerStore(),
		subnet:         subnet,
		serverEndpoint: serverEndpoint,
	}
}

// AddPeer добавляет пира в WireGuard интерфейс.
//
// Используйте когда у вас уже есть публичный ключ клиента.
// НЕ генерирует конфиг/QR - для этого используйте GeneratePeerConfig().
func (a *agentService) AddPeer(ctx context.Context, req *proto.AddPeerRequest) (*proto.AddPeerResponse, error) {
	iface := req.Interface
	if iface == "" {
		iface = a.defIface
	}

	// Валидация входных данных
	if err := wireguard.ValidatePublicKey(req.PublicKey); err != nil {
		return nil, errors.New("invalid public_key: must be base64-encoded 32 bytes")
	}
	if err := wireguard.ValidateAllowedIP(req.AllowedIp); err != nil {
		return nil, errors.New("invalid allowed_ip: must be in CIDR format (e.g., 10.8.0.10/32)")
	}

	key, _ := wgtypes.ParseKey(req.PublicKey)
	_, ipNet, _ := net.ParseCIDR(req.AllowedIp)

	// Настройка keepalive (по умолчанию 25 секунд)
	keepalive := time.Duration(req.KeepaliveS) * time.Second
	if req.KeepaliveS == 0 {
		keepalive = 25 * time.Second
	}

	cfg := wgtypes.Config{
		Peers: []wgtypes.PeerConfig{
			{
				PublicKey:                   key,
				AllowedIPs:                  []net.IPNet{*ipNet},
				UpdateOnly:                  false,
				ReplaceAllowedIPs:           true,
				PersistentKeepaliveInterval: &keepalive,
			},
		},
	}

	if err := a.wgClient.ConfigureDevice(iface, cfg); err != nil {
		return nil, fmt.Errorf("failed to configure device: %w", err)
	}

	device, err := a.wgClient.Device(iface)
	if err != nil {
		a.log.Error("failed to get device after add", "error", err)
		return nil, fmt.Errorf("failed to get device info: %w", err)
	}

	// Добавляем в store
	a.peerStore.AddPeer(req.PublicKey, req.PeerId, req.AllowedIp)

	a.log.Info("peer added",
		"public_key", req.PublicKey,
		"peer_id", req.PeerId,
		"allowed_ip", req.AllowedIp,
	)

	// Возвращаем информацию о сервере для ручной настройки клиента
	return &proto.AddPeerResponse{
		ListenPort:      int32(device.ListenPort),
		ServerPublicKey: device.PublicKey.String(),
		ServerEndpoint:  a.serverEndpoint,
	}, nil
}

func (a *agentService) RemovePeer(ctx context.Context, req *proto.RemovePeerRequest) (*emptypb.Empty, error) {
	iface := req.Interface
	if iface == "" {
		iface = a.defIface
	}
	if err := wireguard.ValidatePublicKey(req.PublicKey); err != nil {
		return nil, errors.New("invalid public_key")
	}

	key, _ := wgtypes.ParseKey(req.PublicKey)
	cfg := wgtypes.Config{
		Peers: []wgtypes.PeerConfig{{
			PublicKey: key,
			Remove:    true,
		}},
	}

	if err := a.wgClient.ConfigureDevice(iface, cfg); err != nil {
		return nil, err
	}

	// Удаляем из store
	a.peerStore.RemovePeer(req.PublicKey)

	a.log.Info("peer removed", "public_key", req.PublicKey)
	return &emptypb.Empty{}, nil
}

func (a *agentService) ListPeers(ctx context.Context, req *proto.ListPeersRequest) (*proto.ListPeersResponse, error) {
	iface := req.Interface
	if iface == "" {
		iface = a.defIface
	}

	// Получаем информацию из store
	storePeers := a.peerStore.ListPeers()
	peers := make([]*proto.PeerInfo, 0, len(storePeers))

	for _, peer := range storePeers {
		peers = append(peers, &proto.PeerInfo{
			PublicKey: peer.PublicKey,
			AllowedIp: peer.AllowedIP,
			Enabled:   peer.Enabled,
			PeerId:    peer.PeerID,
		})
	}

	return &proto.ListPeersResponse{Peers: peers}, nil
}

// DisablePeer временно отключает пира (блокирует трафик)
func (a *agentService) DisablePeer(ctx context.Context, req *proto.DisablePeerRequest) (*emptypb.Empty, error) {
	iface := req.Interface
	if iface == "" {
		iface = a.defIface
	}

	if err := wireguard.ValidatePublicKey(req.PublicKey); err != nil {
		return nil, errors.New("invalid public_key")
	}

	// Обновляем состояние в store
	if !a.peerStore.SetPeerEnabled(req.PublicKey, false) {
		return nil, errors.New("peer not found")
	}

	// Удаляем пира из WireGuard (временно)
	key, _ := wgtypes.ParseKey(req.PublicKey)
	cfg := wgtypes.Config{
		Peers: []wgtypes.PeerConfig{{
			PublicKey: key,
			Remove:    true,
		}},
	}

	if err := a.wgClient.ConfigureDevice(iface, cfg); err != nil {
		// Возвращаем состояние обратно при ошибке
		a.peerStore.SetPeerEnabled(req.PublicKey, true)
		return nil, err
	}

	a.log.Info("peer disabled", "public_key", req.PublicKey)
	return &emptypb.Empty{}, nil
}

// EnablePeer включает ранее отключенного пира
func (a *agentService) EnablePeer(ctx context.Context, req *proto.EnablePeerRequest) (*emptypb.Empty, error) {
	iface := req.Interface
	if iface == "" {
		iface = a.defIface
	}

	if err := wireguard.ValidatePublicKey(req.PublicKey); err != nil {
		return nil, errors.New("invalid public_key")
	}

	// Получаем информацию о пире из store
	peerInfo, exists := a.peerStore.GetPeer(req.PublicKey)
	if !exists {
		return nil, errors.New("peer not found")
	}

	if peerInfo.Enabled {
		return &emptypb.Empty{}, nil // уже включен
	}

	// Восстанавливаем пира в WireGuard
	key, _ := wgtypes.ParseKey(req.PublicKey)
	_, ipNet, _ := net.ParseCIDR(peerInfo.AllowedIP)

	cfg := wgtypes.Config{
		Peers: []wgtypes.PeerConfig{
			{
				PublicKey:                   key,
				AllowedIPs:                  []net.IPNet{*ipNet},
				UpdateOnly:                  false,
				ReplaceAllowedIPs:           true,
				PersistentKeepaliveInterval: func() *time.Duration { d := 25 * time.Second; return &d }(),
			},
		},
	}

	if err := a.wgClient.ConfigureDevice(iface, cfg); err != nil {
		return nil, err
	}

	// Обновляем состояние в store
	a.peerStore.SetPeerEnabled(req.PublicKey, true)

	a.log.Info("peer enabled", "public_key", req.PublicKey)
	return &emptypb.Empty{}, nil
}

// GetPeerInfo возвращает детальную информацию о пире
func (a *agentService) GetPeerInfo(ctx context.Context, req *proto.GetPeerInfoRequest) (*proto.GetPeerInfoResponse, error) {
	iface := req.Interface
	if iface == "" {
		iface = a.defIface
	}

	if err := wireguard.ValidatePublicKey(req.PublicKey); err != nil {
		return nil, errors.New("invalid public_key")
	}

	// Получаем информацию из store
	peerInfo, exists := a.peerStore.GetPeer(req.PublicKey)
	if !exists {
		return nil, errors.New("peer not found")
	}

	response := &proto.GetPeerInfoResponse{
		PublicKey: peerInfo.PublicKey,
		AllowedIp: peerInfo.AllowedIP,
		Enabled:   peerInfo.Enabled,
		PeerId:    peerInfo.PeerID,
	}

	// Если пир включен, получаем статистику из WireGuard
	if peerInfo.Enabled {
		device, err := a.wgClient.Device(iface)
		if err != nil {
			a.log.Warn("failed to get device info", "error", err)
		} else {
			key, _ := wgtypes.ParseKey(req.PublicKey)
			for _, peer := range device.Peers {
				if peer.PublicKey.String() == key.String() {
					response.LastHandshakeUnix = peer.LastHandshakeTime.Unix()
					response.RxBytes = peer.ReceiveBytes
					response.TxBytes = peer.TransmitBytes
					break
				}
			}
		}
	}

	return response, nil
}

// GeneratePeerConfig генерирует полную конфигурацию для нового клиента.
//
// Что делает:
// 1. Генерирует пару ключей (приватный + публичный)
// 2. Выделяет свободный IP из подсети сервера
// 3. Создает готовый конфиг для клиента WireGuard
// 4. Генерирует QR-код для мобильных приложений
//
// ВАЖНО: После вызова нужно добавить пира через AddPeer()!
func (a *agentService) GeneratePeerConfig(ctx context.Context, req *proto.GeneratePeerConfigRequest) (*proto.GeneratePeerConfigResponse, error) {
	iface := req.Interface
	if iface == "" {
		iface = a.defIface
	}

	// Проверяем, что serverEndpoint настроен
	if a.serverEndpoint == "" {
		return nil, errors.New("server endpoint not configured: set SERVER_PUBLIC_IP environment variable")
	}

	// Генерируем пару ключей
	privateKey, publicKey, err := wireguard.GenerateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate key pair: %w", err)
	}

	// Получаем информацию о сервере
	device, err := a.wgClient.Device(iface)
	if err != nil {
		return nil, fmt.Errorf("failed to get device info: %w", err)
	}

	// Выделяем IP адрес
	usedIPs := wireguard.GetUsedIPs(device)
	clientIP, err := wireguard.AllocateIP(a.subnet, usedIPs)
	if err != nil {
		return nil, fmt.Errorf("failed to allocate IP: %w", err)
	}

	// Параметры конфигурации с дефолтами
	serverPublicKey := device.PublicKey.String()
	allowedIPs := req.AllowedIps
	if allowedIPs == "" {
		allowedIPs = "0.0.0.0/0" // весь трафик через VPN
	}
	dnsServers := req.DnsServers
	if dnsServers == "" {
		dnsServers = "1.1.1.1, 1.0.0.1" // Cloudflare DNS
	}

	// Генерируем конфигурацию клиента
	config := wireguard.GenerateClientConfig(
		privateKey,
		serverPublicKey,
		a.serverEndpoint, // используем endpoint из конфигурации сервера
		allowedIPs,
		dnsServers,
		clientIP,
	)

	// Генерируем QR код
	qrCode, err := wireguard.GenerateQRCode(config)
	if err != nil {
		a.log.Warn("failed to generate QR code", "error", err)
		qrCode = "" // не критичная ошибка
	}

	a.log.Info("peer config generated",
		"public_key", publicKey,
		"client_ip", clientIP,
	)

	return &proto.GeneratePeerConfigResponse{
		PrivateKey: privateKey,
		PublicKey:  publicKey,
		Config:     config,
		QrCode:     qrCode,
		AllowedIp:  clientIP,
	}, nil
}
