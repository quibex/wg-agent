package server

import (
	"context"
	"io"
	"testing"

	proto "github.com/our-org/wg-project/api"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"log/slog"
)

// errClient is a wireguard.Client that returns an error from Device
// but succeeds for ConfigureDevice.
type errClient struct{}

func (errClient) Device(name string) (*wgtypes.Device, error) {
	return nil, io.EOF
}

func (errClient) ConfigureDevice(name string, cfg wgtypes.Config) error {
	return nil
}

func (errClient) Close() error { return nil }

func TestAddPeer_DeviceError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := newAgentService(logger, errClient{}, "wg0")

	_, err := svc.AddPeer(context.Background(), &proto.AddPeerRequest{
		PublicKey:  "jNQKmw+IF/llmxOlGwrMxaHiPiG5xQyBq3/OmfEpuQM=",
		AllowedIp:  "10.8.0.10/32",
		KeepaliveS: 0,
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}
