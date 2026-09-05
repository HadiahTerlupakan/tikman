package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// mirrorInterval is how stale the round-robin's view of presence can be.
//
// It is a poll rather than a subscription because the Go Admin SDK has no
// listener for the Realtime Database: db.Ref offers Get, GetShallow, Set,
// Delete and Transaction over REST and nothing that streams. The browser has a
// real listener, so the team panel updates instantly and this does not.
const mirrorInterval = 15 * time.Second

// PresenceSnapshot reads who the Realtime Database currently holds.
type PresenceSnapshot interface {
	Present(ctx context.Context) ([]uuid.UUID, error)
}

// OnlinePresence is what the mirror writes into: bring an agent online when
// the snapshot still holds them, drop them the moment it does not.
//
// It is deliberately narrower than Presence rather than an addition to it.
// Presence is also CSAssignmentService's dependency, and wa's tests supply
// their own minimal fake for it; widening Presence itself would force that
// fake — in a package this change must not touch — to grow a method it never
// calls.
type OnlinePresence interface {
	MarkOnline(ctx context.Context, userID uuid.UUID) error
	MarkOffline(ctx context.Context, userID uuid.UUID) error
}

// PresenceMirror projects the Realtime Database's presence into the Redis keys
// the round-robin already reads, so the wa process — which runs assignment and
// holds the WhatsApp sessions — needs no Firebase configuration and no restart.
type PresenceMirror struct {
	snapshot PresenceSnapshot
	presence OnlinePresence
	logger   *zap.Logger
	// mirrored is the previous pass's set, so a departure can be deleted
	// rather than waited out.
	mirrored map[uuid.UUID]struct{}
}

// NewPresenceMirror constructs a PresenceMirror.
func NewPresenceMirror(snapshot PresenceSnapshot, presence OnlinePresence, logger *zap.Logger) *PresenceMirror {
	return &PresenceMirror{
		snapshot: snapshot,
		presence: presence,
		logger:   logger,
		mirrored: make(map[uuid.UUID]struct{}),
	}
}

// Sync brings Redis into line with one reading of the Realtime Database.
//
// A failed reading changes nothing. One dropped request is a blip, and
// emptying the rotation over it would stop assignment for the whole team; a
// real outage is covered by the keys' own TTL expiring while nothing refreshes
// them.
func (m *PresenceMirror) Sync(ctx context.Context) error {
	present, err := m.snapshot.Present(ctx)
	if err != nil {
		return fmt.Errorf("read presence snapshot: %w", err)
	}

	now := make(map[uuid.UUID]struct{}, len(present))
	for _, id := range present {
		now[id] = struct{}{}
		if err := m.presence.MarkOnline(ctx, id); err != nil {
			return fmt.Errorf("mirror online %s: %w", id, err)
		}
	}

	for id := range m.mirrored {
		if _, still := now[id]; still {
			continue
		}
		if err := m.presence.MarkOffline(ctx, id); err != nil {
			return fmt.Errorf("mirror offline %s: %w", id, err)
		}
	}

	m.mirrored = now
	return nil
}

// Run mirrors until the context is cancelled.
func (m *PresenceMirror) Run(ctx context.Context) {
	ticker := time.NewTicker(mirrorInterval)
	defer ticker.Stop()

	for {
		if err := m.Sync(ctx); err != nil {
			m.logger.Warn("mirror CS presence", zap.Error(err))
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
