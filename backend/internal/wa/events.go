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

// ControlChannel carries admin actions — pair a number, or give one up — to
// the wa process, the only thing that holds the WhatsApp connection.
const ControlChannel = "cs:control"

// Control actions carried on ControlChannel.
const (
	ControlConnect    = "connect"
	ControlDisconnect = "disconnect"
)

// ControlMessage is one admin action on ControlChannel. The API is the only
// publisher and the wa process the only subscriber — defined once here so
// the two sides cannot drift on the wire shape.
type ControlMessage struct {
	Action    string `json:"action"`
	AccountID string `json:"account_id"`
	Phone     string `json:"phone,omitempty"`
}

// Event is one inbox change worth waking a browser for.
type Event struct {
	Type           string `json:"type"`
	ConversationID string `json:"conversation_id,omitempty"`
	MessageID      string `json:"message_id,omitempty"`
	// WAAccountID names the number an account_status event is about. Without
	// it a browser watching several numbers applies whichever arrived last to
	// all of them.
	WAAccountID   string `json:"wa_account_id,omitempty"`
	AccountStatus string `json:"account_status,omitempty"`
	// PairingCode is the eight-character code an admin types into WhatsApp
	// under Linked Devices, set only while AccountStatus is "pairing".
	PairingCode string `json:"pairing_code,omitempty"`
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
