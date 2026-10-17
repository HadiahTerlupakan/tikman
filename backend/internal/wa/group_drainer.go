package wa

import (
	"context"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"github.com/tikman/olt-provisioning/internal/wa/linkpreview"
)

// groupSender is the part of the connection a group send needs. It differs
// from Sender in carrying mentions, which only a group has.
type groupSender interface {
	SendGroupText(ctx context.Context, jid, body string, quote *Quote, mentions []string, preview *linkpreview.Preview) (string, error)
	SendGroupMedia(
		ctx context.Context, jid string, kind models.MessageKind, path, mime, filename, caption string,
		quote *Quote, mentions []string,
	) (string, error)
}

// GroupDrainer sends the messages waiting in the group outbox. It is the
// customer Drainer's twin and takes the same care: one drain at a time,
// because ClaimQueued does not lock, and a deliberate pace between sends.
type GroupDrainer struct {
	mu        sync.Mutex
	accountID uuid.UUID
	messages  *services.WAGroupMessageService
	sender    groupSender
	publisher announcer
	mediaRoot string
	pace      time.Duration
}

// NewGroupDrainer constructs a GroupDrainer.
func NewGroupDrainer(
	accountID uuid.UUID,
	messages *services.WAGroupMessageService,
	sender groupSender,
	publisher announcer,
	mediaRoot string,
	pace time.Duration,
) *GroupDrainer {
	return &GroupDrainer{
		accountID: accountID, messages: messages, sender: sender,
		publisher: publisher, mediaRoot: mediaRoot, pace: pace,
	}
}

// Drain sends what is waiting and answers how many reached WhatsApp. A refused
// message is recorded with its reason and the drain moves on.
func (d *GroupDrainer) Drain(ctx context.Context, limit int) (int, error) {
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

		waID, preview, err := d.send(ctx, msg)
		if err != nil {
			if markErr := d.messages.MarkFailed(msg.ID, err.Error()); markErr != nil {
				return sent, markErr
			}
			d.announce(ctx, msg.WAGroupMessage)
			continue
		}
		if err := d.messages.MarkSent(msg.ID, waID, preview); err != nil {
			return sent, err
		}
		d.announce(ctx, msg.WAGroupMessage)
		sent++
	}
	return sent, nil
}

func (d *GroupDrainer) send(ctx context.Context, msg services.QueuedGroupMessage) (string, *linkpreview.Preview, error) {
	quote := d.quote(msg.WAGroupMessage)
	mentions := services.Mentions(&msg.WAGroupMessage)
	if msg.Kind == models.MessageKindText {
		preview := linkpreview.Resolve(ctx, msg.Body)
		waID, err := d.sender.SendGroupText(ctx, msg.GroupJID, msg.Body, quote, mentions, preview)
		return waID, preview, err
	}
	waID, err := d.sender.SendGroupMedia(
		ctx, msg.GroupJID, msg.Kind, filepath.Join(d.mediaRoot, msg.MediaPath),
		msg.MediaMime, msg.MediaFilename, msg.Body, quote, mentions,
	)
	return waID, nil, err
}

// quote names who wrote the quoted message: in a group the chat is not the
// author, and a quote naming the wrong one arrives as an empty box.
func (d *GroupDrainer) quote(msg models.WAGroupMessage) *Quote {
	if msg.ReplyToID == nil {
		return nil
	}
	target, err := d.messages.Get(*msg.ReplyToID)
	if err != nil || target.WAMessageID == nil {
		return nil
	}
	return &Quote{
		StanzaID:    *target.WAMessageID,
		FromMe:      target.Direction == models.MessageOut,
		Participant: target.SenderJID,
		Body:        target.Body,
		Kind:        target.Kind,
	}
}

func (d *GroupDrainer) announce(ctx context.Context, msg models.WAGroupMessage) {
	if d.publisher == nil {
		return
	}
	_ = d.publisher.Publish(ctx, Event{
		Type: EventGroupMessage, GroupID: msg.GroupID.String(), MessageID: msg.ID.String(),
	})
}
