package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/tikman/olt-provisioning/internal/models"
)

type Data struct {
	UserID       uuid.UUID       `json:"user_id"`
	Role         models.UserRole `json:"role"`
	CreatedAt    time.Time       `json:"created_at"`
	LastActivity time.Time       `json:"last_activity"`
}

type Store struct {
	client *redis.Client
	ttl    time.Duration
	memory *MemoryStore
}

func NewStore(client *redis.Client, ttl time.Duration) *Store {
	return &Store{
		client: client,
		ttl:    ttl,
	}
}

// BackendStatus describes where sessions are actually stored, for health checks.
type BackendStatus string

const (
	// BackendUp means Redis answered a ping.
	BackendUp BackendStatus = "up"
	// BackendDown means Redis is configured but unreachable.
	BackendDown BackendStatus = "down"
	// BackendDisabled means no Redis was configured and sessions live in memory,
	// which is a deployment choice rather than a failure.
	BackendDisabled BackendStatus = "disabled"
)

// Backend reports whether the session backend is reachable.
func (s *Store) Backend(ctx context.Context) BackendStatus {
	if s.memory != nil || s.client == nil {
		return BackendDisabled
	}
	if err := s.client.Ping(ctx).Err(); err != nil {
		return BackendDown
	}
	return BackendUp
}

func (s *Store) Create(userID uuid.UUID, role models.UserRole) (string, error) {
	if s.memory != nil {
		return s.memory.create(userID, role)
	}

	token := uuid.New().String()
	now := time.Now().UTC()

	data := Data{
		UserID:       userID,
		Role:         role,
		CreatedAt:    now,
		LastActivity: now,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("failed to marshal session data: %w", err)
	}

	key := fmt.Sprintf("session:%s", token)
	ctx := context.Background()

	if err := s.client.Set(ctx, key, jsonData, s.ttl).Err(); err != nil {
		return "", fmt.Errorf("failed to store session: %w", err)
	}

	return token, nil
}

func (s *Store) Get(token string) (*Data, error) {
	if s.memory != nil {
		return s.memory.get(token)
	}

	key := fmt.Sprintf("session:%s", token)
	ctx := context.Background()

	jsonData, err := s.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, fmt.Errorf("session not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	var data Data
	if err := json.Unmarshal([]byte(jsonData), &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session data: %w", err)
	}

	return &data, nil
}

func (s *Store) Delete(token string) error {
	if s.memory != nil {
		return s.memory.delete(token)
	}

	key := fmt.Sprintf("session:%s", token)
	ctx := context.Background()

	if err := s.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	return nil
}

func (s *Store) Refresh(token string) error {
	if s.memory != nil {
		return s.memory.refresh(token)
	}

	data, err := s.Get(token)
	if err != nil {
		return err
	}

	data.LastActivity = time.Now().UTC()

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal session data: %w", err)
	}

	key := fmt.Sprintf("session:%s", token)
	ctx := context.Background()

	if err := s.client.Set(ctx, key, jsonData, s.ttl).Err(); err != nil {
		return fmt.Errorf("failed to refresh session: %w", err)
	}

	return nil
}
