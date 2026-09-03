package api

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/tikman/olt-provisioning/internal/services"
	"github.com/tikman/olt-provisioning/internal/wa"
	"go.uber.org/zap"
)

// PushEventListener relays incoming-message events from the same cs:events
// channel the SSE stream reads, turning each into a push notification.
type PushEventListener struct {
	redis    *redis.Client
	notifier *services.PushNotifierService
	logger   *zap.Logger
}

func NewPushEventListener(redisClient *redis.Client, notifier *services.PushNotifierService, logger *zap.Logger) *PushEventListener {
	return &PushEventListener{redis: redisClient, notifier: notifier, logger: logger}
}

// Run subscribes to wa.EventsChannel and pushes a notification for every
// incoming customer message, until ctx is done or the connection drops — the
// same run-until-stopped shape as cs_handler_stream.go's Stream handler.
func (l *PushEventListener) Run(ctx context.Context) {
	sub := l.redis.Subscribe(ctx, wa.EventsChannel)
	defer func() { _ = sub.Close() }()
	incoming := sub.Channel()

	for {
		select {
		case <-ctx.Done():
			return
		case msg, open := <-incoming:
			if !open {
				return
			}
			l.HandlePayload(ctx, msg.Payload)
		}
	}
}

// HandlePayload decodes one cs:events message and, if it announces an
// incoming customer message, triggers a push notification. Kept separate
// from Run so it is testable without a real Redis connection.
func (l *PushEventListener) HandlePayload(ctx context.Context, payload string) {
	var event wa.Event
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		l.logger.Warn("Could not decode a cs:events payload", zap.Error(err))
		return
	}
	if event.Type != wa.EventMessage {
		return
	}

	convID, err := uuid.Parse(event.ConversationID)
	if err != nil {
		l.logger.Warn("cs:events message carried an invalid conversation id", zap.Error(err))
		return
	}
	msgID, err := uuid.Parse(event.MessageID)
	if err != nil {
		l.logger.Warn("cs:events message carried an invalid message id", zap.Error(err))
		return
	}

	if err := l.notifier.NotifyIncomingMessage(ctx, convID, msgID); err != nil {
		l.logger.Warn("Could not send push notifications for an incoming message", zap.Error(err))
	}
}
