package wa

import (
	"context"

	"github.com/redis/go-redis/v9"
	"github.com/tikman/olt-provisioning/internal/models"
	"go.uber.org/zap"
)

// GroupQueueNotifier announces group messages queued outside a request, such
// as the daily graph report: the browsers refetch, and the wa process drains
// now rather than on its next sweep, which stays the net behind a lost notice.
type GroupQueueNotifier struct {
	client    *redis.Client
	publisher *Publisher
	logger    *zap.Logger
}

// NewGroupQueueNotifier constructs a GroupQueueNotifier.
func NewGroupQueueNotifier(client *redis.Client, logger *zap.Logger) *GroupQueueNotifier {
	return &GroupQueueNotifier{client: client, publisher: NewPublisher(client), logger: logger}
}

// GroupMessagesQueued announces each message, then wakes the drain once.
func (n *GroupQueueNotifier) GroupMessagesQueued(ctx context.Context, msgs []models.WAGroupMessage) {
	for _, msg := range msgs {
		err := n.publisher.Publish(ctx, Event{
			Type: EventGroupMessage, GroupID: msg.GroupID.String(), MessageID: msg.ID.String(),
		})
		if err != nil {
			n.logger.Warn("publish group message event failed", zap.Error(err))
		}
	}
	if err := n.client.Publish(ctx, OutboxChannel, "").Err(); err != nil {
		n.logger.Warn("publish group outbox notice failed", zap.Error(err))
	}
}
