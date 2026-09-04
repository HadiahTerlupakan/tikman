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

// PresenceChannel carries "a CS is typing" the other way, to the wa process —
// the only thing holding a WhatsApp connection to tell the customer about it.
const PresenceChannel = "cs:presence"

// PresenceMessage is one CS typing state on PresenceChannel. It names the
// thread rather than the number: the wa process already knows which number a
// thread belongs to, and the browser that sends it does not need to care.
type PresenceMessage struct {
	ConversationID string `json:"conversation_id"`
	Typing         bool   `json:"typing"`
}

// ControlChannel carries admin actions — pair a number, or give one up — to
// the wa process, the only thing that holds the WhatsApp connection.
const ControlChannel = "cs:control"

// Control actions carried on ControlChannel.
const (
	ControlConnect    = "connect"
	ControlDisconnect = "disconnect"
	// ControlDelete is sent after the API has already removed the number and
	// everything on it. All it asks of this process is to let the pairing go,
	// so the phone stops listing a device for an inbox that no longer exists.
	ControlDelete = "delete"
	// ControlSyncChannels asks this process to re-read which channels a number
	// administers. The mirror refreshes hourly on its own; this is the button
	// for an admin who has just been given a channel and does not want to wait.
	ControlSyncChannels = "sync-channels"
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
	// Typing says whether the customer is writing, on a typing event. It is
	// absent when they have stopped, which reads as false.
	Typing bool `json:"typing,omitempty"`
}

// Event types.
const (
	EventMessage       = "message"
	EventAssignment    = "assignment"
	EventStatus        = "status"
	EventAccountStatus = "account_status"
	// EventTyping says a customer started or stopped writing. It carries no
	// stored change, so a browser answers it by showing a line rather than by
	// refetching anything.
	EventTyping = "typing"
	// EventBroadcastPost says one announcement changed status. It carries no
	// identifier: the history is one recent list across every destination, so
	// the browser refetches all of it and there is nothing to narrow by.
	EventBroadcastPost = "broadcast_post"
)

// announcer is what the handlers in this package need of a Publisher: a way to
// say something changed. Declared here, on the consuming side, so a test can
// watch what was announced — the alternative the tests used before was a
// Publisher aimed at a dead Redis, which cannot fail and cannot be observed,
// so nothing checked that the right thing was said about the right thread.
type announcer interface {
	Publish(ctx context.Context, event Event) error
}

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
