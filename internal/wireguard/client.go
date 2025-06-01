package wireguard

import (
	"net"

	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// Client интерфейс для работы с WireGuard
type Client interface {
	Device(name string) (*wgtypes.Device, error)
	ConfigureDevice(name string, cfg wgtypes.Config) error
	Close() error
}

// WGClient реальная реализация WireGuard клиента
type WGClient struct {
	client *wgctrl.Client
}

// NewClient создает новый WireGuard клиент
func NewClient() (*WGClient, error) {
	client, err := wgctrl.New()
	if err != nil {
		return nil, err
	}
	return &WGClient{client: client}, nil
}

// Device возвращает информацию об устройстве
func (c *WGClient) Device(name string) (*wgtypes.Device, error) {
	return c.client.Device(name)
}

// ConfigureDevice конфигурирует устройство
func (c *WGClient) ConfigureDevice(name string, cfg wgtypes.Config) error {
	return c.client.ConfigureDevice(name, cfg)
}

// Close закрывает клиент
func (c *WGClient) Close() error {
	return c.client.Close()
}

// ValidatePublicKey проверяет корректность публичного ключа
func ValidatePublicKey(keyStr string) error {
	_, err := wgtypes.ParseKey(keyStr)
	return err
}

// ValidateAllowedIP проверяет корректность allowed IP
func ValidateAllowedIP(ipStr string) error {
	_, _, err := net.ParseCIDR(ipStr)
	return err
}
