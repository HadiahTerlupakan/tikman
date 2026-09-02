package services

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// presenceTTL outlives the fifteen-second SSE heartbeat several times over, so
// one slow network moment does not drop a CS out of the rotation mid-shift.
const presenceTTL = 60 * time.Second

const (
	presenceKeyPrefix = "cs:online:"
	turnKey           = "cs:rr:pointer"
)

// Presence is who currently has the CS inbox open. It sits behind an interface
// because assignment is the logic worth testing, and Redis is not.
type Presence interface {
	MarkOnline(ctx context.Context, userID uuid.UUID) error
	Online(ctx context.Context) ([]uuid.UUID, error)
	NextTurn(ctx context.Context) (uint64, error)
}

// RedisPresence keeps the online set as keys that expire on their own, so a CS
// whose browser died simply stops being counted; nothing has to clean up.
type RedisPresence struct {
	client *redis.Client
}

// NewRedisPresence constructs a RedisPresence.
func NewRedisPresence(client *redis.Client) *RedisPresence {
	return &RedisPresence{client: client}
}

// MarkOnline records that a CS still has the inbox open.
func (p *RedisPresence) MarkOnline(ctx context.Context, userID uuid.UUID) error {
	return p.client.Set(ctx, presenceKeyPrefix+userID.String(), "1", presenceTTL).Err()
}

// Online lists the CS currently at their desks, sorted so that the rotation is
// the same order for every caller.
func (p *RedisPresence) Online(ctx context.Context) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	iter := p.client.Scan(ctx, 0, presenceKeyPrefix+"*", 100).Iterator()
	for iter.Next(ctx) {
		id, err := uuid.Parse(strings.TrimPrefix(iter.Val(), presenceKeyPrefix))
		if err != nil {
			continue
		}
		ids = append(ids, id)
	}
	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("scan presence: %w", err)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
	return ids, nil
}

// NextTurn hands out the next position in the rotation.
func (p *RedisPresence) NextTurn(ctx context.Context) (uint64, error) {
	n, err := p.client.Incr(ctx, turnKey).Result()
	if err != nil {
		return 0, fmt.Errorf("advance rotation: %w", err)
	}
	return uint64(n), nil
}

// FakePresence is the in-memory stand-in the assignment tests run against.
type FakePresence struct {
	mu     sync.Mutex
	online []uuid.UUID
	turn   uint64
}

// NewFakePresence constructs a FakePresence with the given users online.
func NewFakePresence(ids ...uuid.UUID) *FakePresence {
	return &FakePresence{online: ids}
}

// SetOnline replaces who is online, which is how a test moves a shift along.
func (p *FakePresence) SetOnline(ids ...uuid.UUID) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.online = ids
}

// MarkOnline adds a user to the online set.
func (p *FakePresence) MarkOnline(_ context.Context, userID uuid.UUID) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, id := range p.online {
		if id == userID {
			return nil
		}
	}
	p.online = append(p.online, userID)
	return nil
}

// Online lists who is at their desk.
func (p *FakePresence) Online(_ context.Context) ([]uuid.UUID, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	ids := make([]uuid.UUID, len(p.online))
	copy(ids, p.online)
	sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
	return ids, nil
}

// NextTurn hands out the next position in the rotation.
func (p *FakePresence) NextTurn(_ context.Context) (uint64, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.turn++
	return p.turn, nil
}
