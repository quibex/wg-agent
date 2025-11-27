package server

import (
	"context"
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
	iface          string // WireGuard интерфейс (wg0)
	clients        *wireguard.ClientStore
	subnet         string // подсеть для IP (10.8.0.0/24)
	serverEndpoint string // endpoint для клиентов (vpn.example.com:51820)
}

func newAgentService(log *slog.Logger, wgClient wireguard.Client, iface, subnet, serverEndpoint string) *agentService {
	return &agentService{
		log:            log,
		wgClient:       wgClient,
		iface:          iface,
		clients:        wireguard.NewClientStore(),
		subnet:         subnet,
		serverEndpoint: serverEndpoint,
	}
}

// CreateClient создаёт нового VPN клиента.
func (s *agentService) CreateClient(ctx context.Context, req *proto.CreateClientRequest) (*proto.CreateClientResponse, error) {
	userID := req.UserId
	if userID == "" {
		return nil, fmt.Errorf("user_id is required")
	}

	// Проверяем, что клиент ещё не существует
	if s.clients.Exists(userID) {
		return nil, fmt.Errorf("client %s already exists", userID)
	}

	// Проверяем, что сервер настроен
	if s.serverEndpoint == "" {
		return nil, fmt.Errorf("server not configured: SERVER_PUBLIC_IP is required")
	}

	// Генерируем ключи
	privateKey, publicKey, err := wireguard.GenerateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate keys: %w", err)
	}

	// Выделяем IP адрес
	usedIPs := s.clients.GetUsedIPs()
	clientIP, err := wireguard.AllocateIP(s.subnet, usedIPs)
	if err != nil {
		return nil, fmt.Errorf("failed to allocate IP: %w", err)
	}

	// Получаем публичный ключ сервера
	device, err := s.wgClient.Device(s.iface)
	if err != nil {
		return nil, fmt.Errorf("failed to get WireGuard device: %w", err)
	}
	serverPublicKey := device.PublicKey.String()

	// Добавляем пира в WireGuard
	key, _ := wgtypes.ParseKey(publicKey)
	_, ipNet, _ := net.ParseCIDR(clientIP)
	keepalive := 25 * time.Second

	cfg := wgtypes.Config{
		Peers: []wgtypes.PeerConfig{{
			PublicKey:                   key,
			AllowedIPs:                  []net.IPNet{*ipNet},
			ReplaceAllowedIPs:           true,
			PersistentKeepaliveInterval: &keepalive,
		}},
	}

	if err := s.wgClient.ConfigureDevice(s.iface, cfg); err != nil {
		return nil, fmt.Errorf("failed to add peer to WireGuard: %w", err)
	}

	// Сохраняем клиента
	s.clients.Add(&wireguard.ClientData{
		UserID:     userID,
		PublicKey:  publicKey,
		PrivateKey: privateKey,
		AllowedIP:  clientIP,
		Enabled:    true,
	})

	// Генерируем конфиг
	configFile := wireguard.GenerateClientConfig(
		privateKey,
		serverPublicKey,
		s.serverEndpoint,
		"0.0.0.0/0",       // весь трафик через VPN
		"1.1.1.1, 1.0.0.1", // Cloudflare DNS
		clientIP,
	)

	// Генерируем QR код
	qrCode, err := wireguard.GenerateQRCode(configFile)
	if err != nil {
		s.log.Warn("failed to generate QR code", "error", err)
		qrCode = ""
	}

	// Генерируем deep link для автоимпорта
	deepLink := wireguard.GenerateWireGuardLink(configFile)

	s.log.Info("client created", "user_id", userID, "client_ip", clientIP)

	return &proto.CreateClientResponse{
		ConfigFile:   configFile,
		QrCodeBase64: qrCode,
		DeepLink:     deepLink,
		ClientIp:     clientIP,
	}, nil
}

// DisableClient отключает клиента.
func (s *agentService) DisableClient(ctx context.Context, req *proto.DisableClientRequest) (*proto.DisableClientResponse, error) {
	userID := req.UserId
	if userID == "" {
		return &proto.DisableClientResponse{Success: false, Message: "user_id is required"}, nil
	}

	client, exists := s.clients.Get(userID)
	if !exists {
		return &proto.DisableClientResponse{Success: false, Message: "client not found"}, nil
	}

	if !client.Enabled {
		return &proto.DisableClientResponse{Success: true, Message: "already disabled"}, nil
	}

	// Удаляем из WireGuard
	key, _ := wgtypes.ParseKey(client.PublicKey)
	cfg := wgtypes.Config{
		Peers: []wgtypes.PeerConfig{{
			PublicKey: key,
			Remove:    true,
		}},
	}

	if err := s.wgClient.ConfigureDevice(s.iface, cfg); err != nil {
		return &proto.DisableClientResponse{Success: false, Message: err.Error()}, nil
	}

	s.clients.SetEnabled(userID, false)
	s.log.Info("client disabled", "user_id", userID)

	return &proto.DisableClientResponse{Success: true, Message: "disabled"}, nil
}

// EnableClient включает клиента.
func (s *agentService) EnableClient(ctx context.Context, req *proto.EnableClientRequest) (*proto.EnableClientResponse, error) {
	userID := req.UserId
	if userID == "" {
		return &proto.EnableClientResponse{Success: false, Message: "user_id is required"}, nil
	}

	client, exists := s.clients.Get(userID)
	if !exists {
		return &proto.EnableClientResponse{Success: false, Message: "client not found"}, nil
	}

	if client.Enabled {
		return &proto.EnableClientResponse{Success: true, Message: "already enabled"}, nil
	}

	// Добавляем обратно в WireGuard
	key, _ := wgtypes.ParseKey(client.PublicKey)
	_, ipNet, _ := net.ParseCIDR(client.AllowedIP)
	keepalive := 25 * time.Second

	cfg := wgtypes.Config{
		Peers: []wgtypes.PeerConfig{{
			PublicKey:                   key,
			AllowedIPs:                  []net.IPNet{*ipNet},
			ReplaceAllowedIPs:           true,
			PersistentKeepaliveInterval: &keepalive,
		}},
	}

	if err := s.wgClient.ConfigureDevice(s.iface, cfg); err != nil {
		return &proto.EnableClientResponse{Success: false, Message: err.Error()}, nil
	}

	s.clients.SetEnabled(userID, true)
	s.log.Info("client enabled", "user_id", userID)

	return &proto.EnableClientResponse{Success: true, Message: "enabled"}, nil
}

// DeleteClient полностью удаляет клиента.
func (s *agentService) DeleteClient(ctx context.Context, req *proto.DeleteClientRequest) (*emptypb.Empty, error) {
	userID := req.UserId
	if userID == "" {
		return nil, fmt.Errorf("user_id is required")
	}

	client, exists := s.clients.Get(userID)
	if !exists {
		return nil, fmt.Errorf("client not found")
	}

	// Удаляем из WireGuard (если включен)
	if client.Enabled {
		key, _ := wgtypes.ParseKey(client.PublicKey)
		cfg := wgtypes.Config{
			Peers: []wgtypes.PeerConfig{{
				PublicKey: key,
				Remove:    true,
			}},
		}
		if err := s.wgClient.ConfigureDevice(s.iface, cfg); err != nil {
			s.log.Warn("failed to remove peer from WireGuard", "error", err)
		}
	}

	s.clients.Delete(userID)
	s.log.Info("client deleted", "user_id", userID)

	return &emptypb.Empty{}, nil
}

// GetClient возвращает информацию о клиенте.
func (s *agentService) GetClient(ctx context.Context, req *proto.GetClientRequest) (*proto.GetClientResponse, error) {
	userID := req.UserId
	if userID == "" {
		return nil, fmt.Errorf("user_id is required")
	}

	client, exists := s.clients.Get(userID)
	if !exists {
		return nil, fmt.Errorf("client not found")
	}

	resp := &proto.GetClientResponse{
		UserId:   client.UserID,
		ClientIp: client.AllowedIP,
		Enabled:  client.Enabled,
	}

	// Получаем статистику из WireGuard если клиент включен
	if client.Enabled {
		device, err := s.wgClient.Device(s.iface)
		if err == nil {
			for _, peer := range device.Peers {
				if peer.PublicKey.String() == client.PublicKey {
					resp.RxBytes = peer.ReceiveBytes
					resp.TxBytes = peer.TransmitBytes
					resp.LastHandshake = peer.LastHandshakeTime.Unix()
					break
				}
			}
		}
	}

	return resp, nil
}

// ListClients возвращает список всех клиентов.
func (s *agentService) ListClients(ctx context.Context, req *proto.ListClientsRequest) (*proto.ListClientsResponse, error) {
	clients := s.clients.List()

	// Получаем статистику из WireGuard
	var device *wgtypes.Device
	device, _ = s.wgClient.Device(s.iface)

	peerStats := make(map[string]wgtypes.Peer)
	if device != nil {
		for _, peer := range device.Peers {
			peerStats[peer.PublicKey.String()] = peer
		}
	}

	result := make([]*proto.ClientInfo, 0, len(clients))
	for _, c := range clients {
		info := &proto.ClientInfo{
			UserId:   c.UserID,
			ClientIp: c.AllowedIP,
			Enabled:  c.Enabled,
		}

		if peer, ok := peerStats[c.PublicKey]; ok {
			info.LastHandshake = peer.LastHandshakeTime.Unix()
		}

		result = append(result, info)
	}

	return &proto.ListClientsResponse{Clients: result}, nil
}
