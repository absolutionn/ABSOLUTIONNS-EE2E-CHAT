package room

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync"
)

const (
	MaxParticipants = 2
	MaxQueueSize    = 64
	MaxSeenIDs      = 1024
)

var (
	ErrRoomNotFound   = errors.New("room not found")
	ErrRoomFull       = errors.New("room is full")
	ErrReplayDetected = errors.New("duplicate message / replay detected")
	ErrEmptyPayload   = errors.New("empty payload")
)

type Room struct {
	ID      string
	clients map[string]chan []byte
	seen    map[string]struct{}
}

type Manager struct {
	mu    sync.RWMutex
	rooms map[string]*Room
}

func NewManager() *Manager {
	return &Manager{rooms: make(map[string]*Room)}
}

func (m *Manager) CreateRoom() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	id := hex.EncodeToString(b)

	m.mu.Lock()
	defer m.mu.Unlock()
	m.rooms[id] = &Room{
		ID:      id,
		clients: make(map[string]chan []byte, MaxParticipants),
		seen:    make(map[string]struct{}),
	}
	return id, nil
}

func (m *Manager) Join(roomID, clientID string) (<-chan []byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	r, ok := m.rooms[roomID]
	if !ok {
		return nil, ErrRoomNotFound
	}
	if len(r.clients) >= MaxParticipants {
		return nil, ErrRoomFull
	}

	ch := make(chan []byte, MaxQueueSize)
	r.clients[clientID] = ch

	statusMsg, _ := json.Marshal(map[string]any{
		"type":  "status",
		"peers": len(r.clients),
	})
	for _, peerCh := range r.clients {
		select {
		case peerCh <- statusMsg:
		default:
		}
	}
	return ch, nil
}

func (m *Manager) Leave(roomID, clientID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	r, ok := m.rooms[roomID]
	if !ok {
		return
	}
	if ch, exists := r.clients[clientID]; exists {
		delete(r.clients, clientID)
		close(ch)
	}
	if len(r.clients) == 0 {
		delete(m.rooms, roomID)
		return
	}
	statusMsg, _ := json.Marshal(map[string]any{
		"type":  "status",
		"peers": len(r.clients),
	})
	for _, peerCh := range r.clients {
		select {
		case peerCh <- statusMsg:
		default:
		}
	}
}

func (m *Manager) Relay(roomID, senderID, msgID string, payload []byte) error {
	if len(payload) == 0 {
		return ErrEmptyPayload
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	r, ok := m.rooms[roomID]
	if !ok {
		return ErrRoomNotFound
	}
	if msgID != "" {
		if _, dup := r.seen[msgID]; dup {
			return ErrReplayDetected
		}
		if len(r.seen) >= MaxSeenIDs {
			r.seen = make(map[string]struct{})
		}
		r.seen[msgID] = struct{}{}
	}

	for cid, ch := range r.clients {
		if cid == senderID {
			continue
		}
		select {
		case ch <- payload:
		default:
		}
	}
	return nil
}

func (m *Manager) ActiveRooms() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.rooms)
}
