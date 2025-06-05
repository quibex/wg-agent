package server

import (
	"context"
	"errors"
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
	log      *slog.Logger
	wgClient wireguard.Client
	defIface string
}

func newAgentService(log *slog.Logger, wgClient wireguard.Client, defIface string) *agentService {
	return &agentService{
		log:      log,
		wgClient: wgClient,
		defIface: defIface,
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

	return &proto.AddPeerResponse{ListenPort: int32(device.ListenPort)}, nil
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
	return &emptypb.Empty{}, nil
}

func (a *agentService) ListPeers(ctx context.Context, req *proto.ListPeersRequest) (*proto.ListPeersResponse, error) {
	iface := req.Interface
	if iface == "" {
		iface = a.defIface
	}
	device, err := a.wgClient.Device(iface)
	if err != nil {
		return nil, err
	}
	pubKeys := make([]string, 0, len(device.Peers))
	for _, p := range device.Peers {
		pubKeys = append(pubKeys, p.PublicKey.String())
	}
	return &proto.ListPeersResponse{PublicKeys: pubKeys}, nil
}
