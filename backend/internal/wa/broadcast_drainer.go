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

// BroadcastSender is the part of whatsmeow that posting an announcement needs.
// It is its own interface rather than an addition to Sender so the chat outbox
// and its tests are untouched by broadcast work.
type BroadcastSender interface {
	SendChannelText(ctx context.Context, channelJID, body string) (waMessageID string, err error)
	SendChannelMedia(
		ctx context.Context, channelJID string, kind models.MessageKind,
		path, mime, filename, caption string,
	) (waMessageID string, err error)
	SendStatusText(ctx context.Context, body string) (waMessageID string, err error)
	SendStatusMedia(
		ctx context.Context, kind models.MessageKind,
		path, mime, filename, caption string,
	) (waMessageID string, err error)
}

// BroadcastDrainer posts the updates waiting in the channel outbox.
//
// Only one drain runs at a time, for the reason the message Drainer records:
// ClaimQueued reads rows without locking them, so overlapping drains would
// hand both the same row — and a duplicate here reaches every subscriber.
type BroadcastDrainer struct {
	mu        sync.Mutex
	accountID uuid.UUID
	posts     *services.CSBroadcastPostService
	sender    BroadcastSender
	publisher announcer
	mediaRoot string
	pace      time.Duration
}

// NewBroadcastDrainer constructs a BroadcastDrainer. pace is the gap left
// between two posts, for the same reason the message drainer has one:
// emptying a queue as fast as the connection allows is what gets an
// unofficial number flagged.
func NewBroadcastDrainer(
	accountID uuid.UUID,
	posts *services.CSBroadcastPostService,
	sender BroadcastSender,
	publisher announcer,
	mediaRoot string,
	pace time.Duration,
) *BroadcastDrainer {
	return &BroadcastDrainer{
		accountID: accountID,
		posts:     posts,
		sender:    sender,
		publisher: publisher,
		mediaRoot: mediaRoot,
		pace:      pace,
	}
}

// Drain posts what is waiting and answers how many reached WhatsApp. An update
// WhatsApp refuses is recorded with its reason and the drain continues: one
// channel refusing must not hold up an announcement to another.
func (d *BroadcastDrainer) Drain(ctx context.Context, limit int) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	waiting, err := d.posts.ClaimQueued(d.accountID, limit)
	if err != nil {
		return 0, err
	}

	sent := 0
	for i, post := range waiting {
		if i > 0 && d.pace > 0 {
			select {
			case <-ctx.Done():
				return sent, ctx.Err()
			case <-time.After(d.pace):
			}
		}

		waID, err := d.send(ctx, post)
		if err != nil {
			if markErr := d.posts.MarkFailed(post.ID, err.Error()); markErr != nil {
				return sent, markErr
			}
			d.announce(ctx)
			continue
		}
		if err := d.posts.MarkSent(post.ID, waID); err != nil {
			return sent, err
		}
		d.announce(ctx)
		sent++
	}
	return sent, nil
}

func (d *BroadcastDrainer) send(ctx context.Context, post models.WABroadcastPost) (string, error) {
	switch post.Destination {
	case models.DestinationStatus:
		return d.sendStatus(ctx, post)
	case models.DestinationChannel:
		return d.sendChannel(ctx, post)
	default:
		return "", fmt.Errorf("tujuan tidak dikenal %q", post.Destination)
	}
}

func (d *BroadcastDrainer) sendStatus(ctx context.Context, post models.WABroadcastPost) (string, error) {
	if post.Kind == models.MessageKindText {
		return d.sender.SendStatusText(ctx, post.Body)
	}
	return d.sender.SendStatusMedia(
		ctx, post.Kind, filepath.Join(d.mediaRoot, post.MediaPath),
		post.MediaMime, post.MediaFilename, post.Body,
	)
}

func (d *BroadcastDrainer) sendChannel(ctx context.Context, post models.WABroadcastPost) (string, error) {
	if post.DestinationJID == nil {
		return "", fmt.Errorf("kiriman saluran tanpa saluran")
	}
	jid := *post.DestinationJID
	if post.Kind == models.MessageKindText {
		return d.sender.SendChannelText(ctx, jid, post.Body)
	}
	return d.sender.SendChannelMedia(
		ctx, jid, post.Kind, filepath.Join(d.mediaRoot, post.MediaPath),
		post.MediaMime, post.MediaFilename, post.Body,
	)
}

// announce tells the browsers with a broadcast history open that one
// announcement moved. Published per post rather than once at the end, for the
// same reason as the message drainer: the pace between sends is measured in
// seconds, so a batched announcement would leave the first update showing a
// clock long after it had gone.
//
// A failure to publish is returned to nobody: the update is already sent, and
// the browser's next refetch closes the gap.
func (d *BroadcastDrainer) announce(ctx context.Context) {
	if d.publisher == nil {
		return
	}
	_ = d.publisher.Publish(ctx, Event{Type: EventBroadcastPost})
}
