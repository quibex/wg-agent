package wireguard

import (
	"sync"
)

// PeerInfo хранит информацию о пире
type PeerInfo struct {
	PublicKey string
	PeerID    string
	Enabled   bool
	AllowedIP string
}

// PeerStore хранит состояние пиров в памяти
type PeerStore struct {
	mu    sync.RWMutex
	peers map[string]*PeerInfo // ключ - публичный ключ
}

// NewPeerStore создает новый PeerStore
func NewPeerStore() *PeerStore {
	return &PeerStore{
		peers: make(map[string]*PeerInfo),
	}
}

// AddPeer добавляет пира в хранилище
func (ps *PeerStore) AddPeer(publicKey, peerID, allowedIP string) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	ps.peers[publicKey] = &PeerInfo{
		PublicKey: publicKey,
		PeerID:    peerID,
		Enabled:   true,
		AllowedIP: allowedIP,
	}
}

// RemovePeer удаляет пира из хранилища
func (ps *PeerStore) RemovePeer(publicKey string) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	delete(ps.peers, publicKey)
}

// SetPeerEnabled устанавливает статус пира
func (ps *PeerStore) SetPeerEnabled(publicKey string, enabled bool) bool {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if peer, exists := ps.peers[publicKey]; exists {
		peer.Enabled = enabled
		return true
	}
	return false
}

// GetPeer возвращает информацию о пире
func (ps *PeerStore) GetPeer(publicKey string) (*PeerInfo, bool) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	peer, exists := ps.peers[publicKey]
	if !exists {
		return nil, false
	}

	// Возвращаем копию
	return &PeerInfo{
		PublicKey: peer.PublicKey,
		PeerID:    peer.PeerID,
		Enabled:   peer.Enabled,
		AllowedIP: peer.AllowedIP,
	}, true
}

// ListPeers возвращает список всех пиров
func (ps *PeerStore) ListPeers() []*PeerInfo {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	peers := make([]*PeerInfo, 0, len(ps.peers))
	for _, peer := range ps.peers {
		peers = append(peers, &PeerInfo{
			PublicKey: peer.PublicKey,
			PeerID:    peer.PeerID,
			Enabled:   peer.Enabled,
			AllowedIP: peer.AllowedIP,
		})
	}
	return peers
}
