package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"time"

	proto "github.com/our-org/wg-project/api"
	"github.com/our-org/wg-project/internal/wireguard"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"google.golang.org/protobuf/types/known/emptypb"
)

// agentService реализует WireGuardAgentServer
type agentService struct {
	proto.UnimplementedWireGuardAgentServer
	log       *slog.Logger
	wgClient  wireguard.Client
	defIface  string
	peerStore *wireguard.PeerStore
	subnet    string // подсеть для выделения IP (например "10.8.0.0/24")
}

func newAgentService(log *slog.Logger, wgClient wireguard.Client, defIface, subnet string) *agentService {
	return &agentService{
		log:       log,
		wgClient:  wgClient,
		defIface:  defIface,
		peerStore: wireguard.NewPeerStore(),
		subnet:    subnet,
	}
}

// AddPeer добавляет пира без перезапуска интерфейса
func (a *agentService) AddPeer(ctx context.Context, req *proto.AddPeerRequest) (*proto.AddPeerResponse, error) {
	iface := req.Interface
	if iface == "" {
		iface = a.defIface
	}

	// Валидация входных данных
	if err := wireguard.ValidatePublicKey(req.PublicKey); err != nil {
		return nil, errors.New("invalid public_key")
	}
	if err := wireguard.ValidateAllowedIP(req.AllowedIp); err != nil {
		return nil, errors.New("invalid allowed_ip")
	}

	key, _ := wgtypes.ParseKey(req.PublicKey)
	_, ipNet, _ := net.ParseCIDR(req.AllowedIp)

	cfg := wgtypes.Config{
		Peers: []wgtypes.PeerConfig{
			{
				PublicKey:                   key,
				AllowedIPs:                  []net.IPNet{*ipNet},
				UpdateOnly:                  false,
				ReplaceAllowedIPs:           true,
				PersistentKeepaliveInterval: func() *time.Duration { d := time.Duration(req.KeepaliveS) * time.Second; return &d }(),
			},
		},
	}

	if err := a.wgClient.ConfigureDevice(iface, cfg); err != nil {
		return nil, err
	}

	device, err := a.wgClient.Device(iface)
	if err != nil {
		a.log.Error("failed to get device after add", "error", err)
		return nil, err
	}

	// Добавляем в store
	a.peerStore.AddPeer(req.PublicKey, req.PeerId, req.AllowedIp)

	// Генерируем конфигурацию и QR код для клиента
	serverPublicKey := device.PublicKey.String()
	serverEndpoint := fmt.Sprintf("YOUR_SERVER_IP:%d", device.ListenPort)

	config := wireguard.GenerateClientConfig(
		"CLIENT_PRIVATE_KEY", // будет заменено lime-bot на фактический ключ
		serverPublicKey,
		serverEndpoint,
		"0.0.0.0/0",
		"1.1.1.1, 1.0.0.1",
		req.AllowedIp,
	)

	// Генерируем QR код
	qrCode, err := wireguard.GenerateQRCode(config)
	if err != nil {
		a.log.Warn("failed to generate QR code", "error", err)
		qrCode = "" // не критичная ошибка
	}

	a.log.Info("peer added", "public_key", req.PublicKey, "peer_id", req.PeerId)

	return &proto.AddPeerResponse{
		ListenPort: int32(device.ListenPort),
		Config:     config,
		QrCode:     qrCode,
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

// GeneratePeerConfig генерирует новую пару ключей и конфигурацию
func (a *agentService) GeneratePeerConfig(ctx context.Context, req *proto.GeneratePeerConfigRequest) (*proto.GeneratePeerConfigResponse, error) {
	iface := req.Interface
	if iface == "" {
		iface = a.defIface
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

	// Создаем конфигурацию
	serverPublicKey := device.PublicKey.String()
	allowedIPs := req.AllowedIps
	if allowedIPs == "" {
		allowedIPs = "0.0.0.0/0"
	}
	dnsServers := req.DnsServers
	if dnsServers == "" {
		dnsServers = "1.1.1.1, 1.0.0.1"
	}

	config := wireguard.GenerateClientConfig(
		privateKey,
		serverPublicKey,
		req.ServerEndpoint,
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

	return &proto.GeneratePeerConfigResponse{
		PrivateKey: privateKey,
		PublicKey:  publicKey,
		Config:     config,
		QrCode:     qrCode,
		AllowedIp:  clientIP,
	}, nil
}
