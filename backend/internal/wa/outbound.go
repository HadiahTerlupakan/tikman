package wa

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
)

// Drainer sends the replies waiting in the outbox.
//
// Only one drain runs at a time. Two things call it — the periodic sweep and
// the announcement that a CS just hit send — and ClaimQueued reads rows without
// locking them, so overlapping drains would hand both the same row and the
// customer would receive the reply twice.
//
// This still leaves the outbox at-least-once: a crash after WhatsApp accepts a
// message but before MarkSent records it will resend on restart. That trade is
// deliberate. Closing it would mean an in-flight status plus a reaper for rows
// stranded in it, and a stranded row is a reply that silently never arrives —
// trading a duplicate the customer can see for a loss nobody can.
type Drainer struct {
	mu            sync.Mutex
	messages      *services.CSMessageService
	conversations *services.CSConversationService
	sender        Sender
	mediaRoot     string
	pace          time.Duration
}

// NewDrainer constructs a Drainer. pace is the gap left between two sends:
// emptying the queue as fast as the connection allows is the pattern that gets
// an unofficial number flagged fastest, so the queue is drained deliberately
// slowly.
func NewDrainer(
	messages *services.CSMessageService,
	conversations *services.CSConversationService,
	sender Sender,
	mediaRoot string,
	pace time.Duration,
) *Drainer {
	return &Drainer{
		messages:      messages,
		conversations: conversations,
		sender:        sender,
		mediaRoot:     mediaRoot,
		pace:          pace,
	}
}

// Drain sends what is waiting and answers how many reached WhatsApp. A message
// WhatsApp refuses is recorded with its reason and the drain continues: one bad
// number must not hold up every other customer's reply.
func (d *Drainer) Drain(ctx context.Context, limit int) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	waiting, err := d.messages.ClaimQueued(limit)
	if err != nil {
		return 0, err
	}

	sent := 0
	for i, msg := range waiting {
		if i > 0 && d.pace > 0 {
			select {
			case <-ctx.Done():
				return sent, ctx.Err()
			case <-time.After(d.pace):
			}
		}

		waID, err := d.send(ctx, msg)
		if err != nil {
			if markErr := d.messages.MarkFailed(msg.ID, err.Error()); markErr != nil {
				return sent, markErr
			}
			continue
		}
		if err := d.messages.MarkSent(msg.ID, waID); err != nil {
			return sent, err
		}
		sent++
	}
	return sent, nil
}

func (d *Drainer) send(ctx context.Context, msg models.CSMessage) (string, error) {
	conv, err := d.conversations.Get(msg.ConversationID)
	if err != nil {
		return "", fmt.Errorf("percakapan tidak ditemukan: %w", err)
	}

	if msg.Kind == models.MessageKindText {
		return d.sender.SendText(ctx, conv.CustomerJID, msg.Body)
	}
	return d.sender.SendMedia(
		ctx, conv.CustomerJID, msg.Kind,
		filepath.Join(d.mediaRoot, msg.MediaPath),
		msg.MediaMime, msg.MediaFilename, msg.Body,
	)
}
