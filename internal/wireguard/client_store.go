package wireguard

import (
	"sync"
)

// ClientData хранит информацию о клиенте VPN
type ClientData struct {
	UserID     string // ID пользователя из внешней системы
	PublicKey  string // публичный ключ клиента
	PrivateKey string // приватный ключ клиента (для генерации конфига)
	AllowedIP  string // выделенный IP (например "10.8.0.10/32")
	Enabled    bool   // включен/отключен
}

// ClientStore хранит состояние клиентов в памяти
// TODO: в будущем можно сделать персистентное хранилище
type ClientStore struct {
	mu      sync.RWMutex
	clients map[string]*ClientData // ключ - user_id
}

// NewClientStore создает новый ClientStore
func NewClientStore() *ClientStore {
	return &ClientStore{
		clients: make(map[string]*ClientData),
	}
}

// Add добавляет клиента
func (cs *ClientStore) Add(client *ClientData) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.clients[client.UserID] = client
}

// Get возвращает клиента по user_id
func (cs *ClientStore) Get(userID string) (*ClientData, bool) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	client, exists := cs.clients[userID]
	if !exists {
		return nil, false
	}

	// Возвращаем копию
	return &ClientData{
		UserID:     client.UserID,
		PublicKey:  client.PublicKey,
		PrivateKey: client.PrivateKey,
		AllowedIP:  client.AllowedIP,
		Enabled:    client.Enabled,
	}, true
}

// Delete удаляет клиента
func (cs *ClientStore) Delete(userID string) bool {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if _, exists := cs.clients[userID]; exists {
		delete(cs.clients, userID)
		return true
	}
	return false
}

// SetEnabled включает/отключает клиента
func (cs *ClientStore) SetEnabled(userID string, enabled bool) bool {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if client, exists := cs.clients[userID]; exists {
		client.Enabled = enabled
		return true
	}
	return false
}

// List возвращает список всех клиентов
func (cs *ClientStore) List() []*ClientData {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	clients := make([]*ClientData, 0, len(cs.clients))
	for _, c := range cs.clients {
		clients = append(clients, &ClientData{
			UserID:     c.UserID,
			PublicKey:  c.PublicKey,
			PrivateKey: c.PrivateKey,
			AllowedIP:  c.AllowedIP,
			Enabled:    c.Enabled,
		})
	}
	return clients
}

// Exists проверяет существует ли клиент
func (cs *ClientStore) Exists(userID string) bool {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	_, exists := cs.clients[userID]
	return exists
}

// GetUsedIPs возвращает список занятых IP
func (cs *ClientStore) GetUsedIPs() []string {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	ips := make([]string, 0, len(cs.clients))
	for _, c := range cs.clients {
		// Убираем маску для сравнения
		ip := c.AllowedIP
		if len(ip) > 3 && ip[len(ip)-3:] == "/32" {
			ip = ip[:len(ip)-3]
		}
		ips = append(ips, ip)
	}
	return ips
}
