package services

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"go.uber.org/zap"
)

// Event type constants to avoid circular import of wa package.
const (
	eventMessageType = "message"
	eventStatusType  = "status"
)

// Event mimics wa.Event without importing wa to avoid circular dependency.
type testEvent struct {
	Type           string `json:"type"`
	ConversationID string `json:"conversation_id,omitempty"`
	MessageID      string `json:"message_id,omitempty"`
}

func TestHandlePayloadNotifiesOnAnIncomingMessageEvent(t *testing.T) {
	notifier, sender, conversations, messages, pushService, accountID, db := pushTestSetup(t)
	users := NewUserService(db)
	admin, err := users.Create("admin1", "admin1@example.com", "password123", "", models.UserRoleAdmin)
	require.NoError(t, err)
	require.NoError(t, pushService.Subscribe(admin.ID, "admin-token"))

	conv, err := conversations.FindOrCreate(IncomingPeer{
		WAAccountID: accountID, JID: "628666@s.whatsapp.net", Phone: "628666777888",
	})
	require.NoError(t, err)
	msg, _, err := messages.SaveInbound(InboundMessage{
		ConversationID: conv.ID, WAMessageID: "wamid-6", Kind: models.MessageKindText, Body: "Halo",
	})
	require.NoError(t, err)

	listener := NewPushEventListener(nil, notifier, zap.NewNop())
	payload, err := json.Marshal(testEvent{
		Type:           eventMessageType,
		ConversationID: conv.ID.String(),
		MessageID:      msg.ID.String(),
	})
	require.NoError(t, err)

	listener.HandlePayload(context.Background(), string(payload))
	assert.Equal(t, []string{"admin-token"}, sender.Tokens)
}

func TestHandlePayloadIgnoresEveryEventTypeButMessage(t *testing.T) {
	notifier, sender, _, _, _, _, _ := pushTestSetup(t)
	listener := NewPushEventListener(nil, notifier, zap.NewNop())

	payload, err := json.Marshal(testEvent{Type: eventStatusType})
	require.NoError(t, err)

	listener.HandlePayload(context.Background(), string(payload))
	assert.Nil(t, sender.Tokens, "a non-message event must never trigger a send")
}

func TestHandlePayloadIgnoresUndecodablePayloads(t *testing.T) {
	notifier, sender, _, _, _, _, _ := pushTestSetup(t)
	listener := NewPushEventListener(nil, notifier, zap.NewNop())

	listener.HandlePayload(context.Background(), "not json")
	assert.Nil(t, sender.Tokens)
}

func TestHandlePayloadIgnoresAMessageEventWithAnUnparsableID(t *testing.T) {
	notifier, sender, _, _, _, _, _ := pushTestSetup(t)
	listener := NewPushEventListener(nil, notifier, zap.NewNop())

	payload, err := json.Marshal(testEvent{
		Type: eventMessageType, ConversationID: "not-a-uuid", MessageID: uuid.New().String(),
	})
	require.NoError(t, err)

	listener.HandlePayload(context.Background(), string(payload))
	assert.Nil(t, sender.Tokens)
}
