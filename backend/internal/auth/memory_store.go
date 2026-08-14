package auth

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
)

type MemoryStore struct {
	sessions map[string]*Data
	ttl      time.Duration
	mu       sync.RWMutex
}

func NewMemoryStore(ttl time.Duration) *Store {
	ms := &MemoryStore{
		sessions: make(map[string]*Data),
		ttl:      ttl,
	}

	// Return as Store interface
	return &Store{
		client: nil,
		ttl:    ttl,
		memory: ms,
	}
}

func (ms *MemoryStore) create(userID uuid.UUID, role models.UserRole) (string, error) {
	token := uuid.New().String()
	now := time.Now().UTC()

	data := &Data{
		UserID:       userID,
		Role:         role,
		CreatedAt:    now,
		LastActivity: now,
	}

	ms.mu.Lock()
	ms.sessions[token] = data
	ms.mu.Unlock()

	return token, nil
}

func (ms *MemoryStore) get(token string) (*Data, error) {
	ms.mu.RLock()
	data, exists := ms.sessions[token]
	ms.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("session not found")
	}

	// Check expiration
	if time.Since(data.LastActivity) > ms.ttl {
		ms.mu.Lock()
		delete(ms.sessions, token)
		ms.mu.Unlock()
		return nil, fmt.Errorf("session expired")
	}

	return data, nil
}

func (ms *MemoryStore) delete(token string) error {
	ms.mu.Lock()
	delete(ms.sessions, token)
	ms.mu.Unlock()
	return nil
}

func (ms *MemoryStore) refresh(token string) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	data, exists := ms.sessions[token]
	if !exists {
		return fmt.Errorf("session not found")
	}

	data.LastActivity = time.Now().UTC()
	return nil
}
