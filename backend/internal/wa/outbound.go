package wa

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
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
	accountID     uuid.UUID
	messages      *services.CSMessageService
	conversations *services.CSConversationService
	sender        Sender
	publisher     announcer
	mediaRoot     string
	pace          time.Duration
}

// NewDrainer constructs a Drainer. pace is the gap left between two sends:
// emptying the queue as fast as the connection allows is the pattern that gets
// an unofficial number flagged fastest, so the queue is drained deliberately
// slowly.
func NewDrainer(
	accountID uuid.UUID,
	messages *services.CSMessageService,
	conversations *services.CSConversationService,
	sender Sender,
	publisher announcer,
	mediaRoot string,
	pace time.Duration,
) *Drainer {
	return &Drainer{
		accountID:     accountID,
		messages:      messages,
		conversations: conversations,
		sender:        sender,
		publisher:     publisher,
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

	waiting, err := d.messages.ClaimQueued(d.accountID, limit)
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
			d.announce(ctx, msg.ConversationID)
			continue
		}
		if err := d.messages.MarkSent(msg.ID, waID); err != nil {
			return sent, err
		}
		d.announce(ctx, msg.ConversationID)
		d.acknowledgeRead(ctx, msg.ConversationID)
		sent++
	}
	return sent, nil
}

// announce tells the browsers holding a thread open that one of its replies
// moved. Published per message rather than once at the end of the drain: the
// pace between sends is measured in seconds, so a batch would leave the first
// reply showing a clock long after it had actually gone.
//
// Failing to announce is logged nowhere and returned to nobody, deliberately —
// the reply is already sent, and Redis carries no truth here. The browser's
// next refetch closes the gap.
func (d *Drainer) announce(ctx context.Context, conversationID uuid.UUID) {
	if d.publisher == nil {
		return
	}
	_ = d.publisher.Publish(ctx, Event{
		Type:           EventStatus,
		ConversationID: conversationID.String(),
	})
}

// acknowledgeRead turns the customer's ticks blue, but only once a CS has
// actually answered them. Marking on read of the inbox instead would tell every
// customer their message had been seen the moment it landed in a queue, which
// is a promise the team has not made yet.
//
// A CS answering in three short messages sends one receipt, not three: the
// first marks the thread read and the next two find nothing left waiting.
//
// A failure here costs the blue ticks and nothing else — the reply has already
// gone — so it is logged nowhere and stops nothing. The local status stays
// unchanged, which means the next reply in the thread tries again.
func (d *Drainer) acknowledgeRead(ctx context.Context, conversationID uuid.UUID) {
	conv, err := d.conversations.Get(conversationID)
	if err != nil {
		return
	}
	waiting, err := d.messages.InboundAwaitingRead(conversationID)
	if err != nil || len(waiting) == 0 {
		return
	}

	ids := make([]string, 0, len(waiting))
	rows := make([]uuid.UUID, 0, len(waiting))
	for _, msg := range waiting {
		ids = append(ids, *msg.WAMessageID)
		rows = append(rows, msg.ID)
	}
	if err := d.sender.MarkRead(ctx, conv.CustomerJID, ids, time.Now()); err != nil {
		return
	}
	_ = d.messages.MarkInboundRead(rows)
}

func (d *Drainer) send(ctx context.Context, msg models.CSMessage) (string, error) {
	conv, err := d.conversations.Get(msg.ConversationID)
	if err != nil {
		return "", fmt.Errorf("percakapan tidak ditemukan: %w", err)
	}

	quote := d.quote(msg)
	if msg.Kind == models.MessageKindText {
		return d.sender.SendText(ctx, conv.CustomerJID, msg.Body, quote)
	}
	return d.sender.SendMedia(
		ctx, conv.CustomerJID, msg.Kind,
		filepath.Join(d.mediaRoot, msg.MediaPath),
		msg.MediaMime, msg.MediaFilename, msg.Body, quote,
	)
}

// quote loads the message a reply answers, answering nil when there is nothing
// to quote any more.
//
// A quote that no longer resolves costs the grey block, not the reply. The row
// was checked when the CS wrote it; by the time it drains, retention may have
// swept the message it answered — and holding the reply back over that would
// keep an answer from a customer who is waiting for it.
func (d *Drainer) quote(msg models.CSMessage) *Quote {
	if msg.ReplyToID == nil {
		return nil
	}
	target, err := d.messages.Get(*msg.ReplyToID)
	if err != nil || target.WAMessageID == nil {
		return nil
	}
	return &Quote{
		StanzaID: *target.WAMessageID,
		FromMe:   target.Direction == models.MessageOut,
		Body:     target.Body,
		Kind:     target.Kind,
	}
}
