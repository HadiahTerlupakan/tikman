package wa

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// EventsChannel carries inbox changes to whichever API processes have CS
// browsers attached.
const EventsChannel = "cs:events"

// OutboxChannel carries the announcement that a reply is waiting, so the wa
// process drains it in seconds instead of waiting for its next sweep.
const OutboxChannel = "cs:outbox"

// Event is one inbox change worth waking a browser for.
type Event struct {
	Type           string `json:"type"`
	ConversationID string `json:"conversation_id,omitempty"`
	MessageID      string `json:"message_id,omitempty"`
	AccountStatus  string `json:"account_status,omitempty"`
}

// Event types.
const (
	EventMessage       = "message"
	EventAssignment    = "assignment"
	EventStatus        = "status"
	EventAccountStatus = "account_status"
)

// Publisher announces inbox changes. Redis carries no truth here — it only
// saves the browser from waiting for its next poll, so a failure to publish is
// worth logging and nothing more.
type Publisher struct {
	client *redis.Client
}

// NewPublisher constructs a Publisher.
func NewPublisher(client *redis.Client) *Publisher {
	return &Publisher{client: client}
}

// Publish announces one change.
func (p *Publisher) Publish(ctx context.Context, event Event) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode event: %w", err)
	}
	return p.client.Publish(ctx, EventsChannel, payload).Err()
}
