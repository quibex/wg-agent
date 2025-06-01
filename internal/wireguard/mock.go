package wireguard

import (
	"fmt"
	"sync"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// MockClient мок для WireGuard клиента
type MockClient struct {
	mu      sync.RWMutex
	devices map[string]*wgtypes.Device
	closed  bool
}

// NewMockClient создает новый мок клиент
func NewMockClient() *MockClient {
	return &MockClient{
		devices: make(map[string]*wgtypes.Device),
	}
}

// Device возвращает информацию об устройстве
func (m *MockClient) Device(name string) (*wgtypes.Device, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.closed {
		return nil, fmt.Errorf("client closed")
	}

	device, exists := m.devices[name]
	if !exists {
		return nil, fmt.Errorf("device %s not found", name)
	}

	return device, nil
}

// ConfigureDevice конфигурирует устройство
func (m *MockClient) ConfigureDevice(name string, cfg wgtypes.Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return fmt.Errorf("client closed")
	}

	device, exists := m.devices[name]
	if !exists {
		// Создаем новое устройство если его нет
		device = &wgtypes.Device{
			Name:       name,
			Type:       wgtypes.LinuxKernel,
			PublicKey:  wgtypes.Key{},
			ListenPort: 51820,
			Peers:      []wgtypes.Peer{},
		}
		m.devices[name] = device
	}

	// Применяем конфигурацию
	if cfg.ListenPort != nil {
		device.ListenPort = *cfg.ListenPort
	}

	// Обрабатываем peers
	for _, peerCfg := range cfg.Peers {
		if peerCfg.Remove {
			// Удаляем peer
			for i, peer := range device.Peers {
				if peer.PublicKey.String() == peerCfg.PublicKey.String() {
					device.Peers = append(device.Peers[:i], device.Peers[i+1:]...)
					break
				}
			}
		} else {
			// Добавляем или обновляем peer
			found := false
			for i, peer := range device.Peers {
				if peer.PublicKey.String() == peerCfg.PublicKey.String() {
					// Обновляем существующий peer
					if peerCfg.AllowedIPs != nil {
						device.Peers[i].AllowedIPs = peerCfg.AllowedIPs
					}
					if peerCfg.PersistentKeepaliveInterval != nil {
						device.Peers[i].PersistentKeepaliveInterval = *peerCfg.PersistentKeepaliveInterval
					}
					found = true
					break
				}
			}
			if !found {
				// Добавляем новый peer
				newPeer := wgtypes.Peer{
					PublicKey: peerCfg.PublicKey,
				}
				if peerCfg.AllowedIPs != nil {
					newPeer.AllowedIPs = peerCfg.AllowedIPs
				}
				if peerCfg.PersistentKeepaliveInterval != nil {
					newPeer.PersistentKeepaliveInterval = *peerCfg.PersistentKeepaliveInterval
				}
				device.Peers = append(device.Peers, newPeer)
			}
		}
	}

	return nil
}

// Close закрывает клиент
func (m *MockClient) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

// AddMockDevice добавляет мок устройство для тестирования
func (m *MockClient) AddMockDevice(name string, listenPort int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.devices[name] = &wgtypes.Device{
		Name:       name,
		Type:       wgtypes.LinuxKernel,
		PublicKey:  wgtypes.Key{},
		ListenPort: listenPort,
		Peers:      []wgtypes.Peer{},
	}
}
