package api

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"github.com/tikman/olt-provisioning/internal/wa"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// localFakePushSender records tokens it was asked to send instead of reaching
// a real Firebase project. Defined locally since services.FakePushSender lives
// in a test file in a different package.
type localFakePushSender struct {
	Tokens  []string
	Title   string
	Body    string
	Data    map[string]string
	Invalid []string
}

func (f *localFakePushSender) SendEach(_ context.Context, tokens []string, title, body string, data map[string]string) ([]string, error) {
	f.Tokens = tokens
	f.Title = title
	f.Body = body
	f.Data = data
	return f.Invalid, nil
}

// pushListenerTestSetup builds the notifier and fake sender needed by tests.
func pushListenerTestSetup(t *testing.T) (
	notifier *services.PushNotifierService,
	sender *localFakePushSender,
	conversations *services.CSConversationService,
	messages *services.CSMessageService,
	pushService *services.PushService,
	accountID uuid.UUID,
	db *gorm.DB,
) {
	t.Helper()
	db = TestDB(t)
	account := models.WAAccount{Label: "CS Utama", Status: models.WAAccountConnected}
	require.NoError(t, db.Create(&account).Error)

	conversations = services.NewCSConversationService(db)
	messages = services.NewCSMessageService(db, conversations)
	pushService = services.NewPushService(db)
	sender = &localFakePushSender{}
	notifier = services.NewPushNotifierService(sender, pushService, conversations, messages)
	return notifier, sender, conversations, messages, pushService, account.ID, db
}

func TestHandlePayloadNotifiesOnAnIncomingMessageEvent(t *testing.T) {
	notifier, sender, conversations, messages, pushService, accountID, db := pushListenerTestSetup(t)
	users := services.NewUserService(db)
	admin, err := users.Create("admin1", "admin1@example.com", "password123", "", models.UserRoleAdmin)
	require.NoError(t, err)
	require.NoError(t, pushService.Subscribe(admin.ID, "admin-token"))

	conv, err := conversations.FindOrCreate(services.IncomingPeer{
		WAAccountID: accountID, JID: "628666@s.whatsapp.net", Phone: "628666777888",
	})
	require.NoError(t, err)
	msg, _, err := messages.SaveInbound(services.InboundMessage{
		ConversationID: conv.ID, WAMessageID: "wamid-6", Kind: models.MessageKindText, Body: "Halo",
	})
	require.NoError(t, err)

	listener := NewPushEventListener(nil, notifier, zap.NewNop())
	payload, err := json.Marshal(wa.Event{
		Type:           wa.EventMessage,
		ConversationID: conv.ID.String(),
		MessageID:      msg.ID.String(),
	})
	require.NoError(t, err)

	listener.HandlePayload(context.Background(), string(payload))
	assert.Equal(t, []string{"admin-token"}, sender.Tokens)
}

func TestHandlePayloadIgnoresEveryEventTypeButMessage(t *testing.T) {
	notifier, sender, _, _, _, _, _ := pushListenerTestSetup(t)
	listener := NewPushEventListener(nil, notifier, zap.NewNop())

	payload, err := json.Marshal(wa.Event{Type: wa.EventStatus})
	require.NoError(t, err)

	listener.HandlePayload(context.Background(), string(payload))
	assert.Nil(t, sender.Tokens, "a non-message event must never trigger a send")
}

func TestHandlePayloadIgnoresUndecodablePayloads(t *testing.T) {
	notifier, sender, _, _, _, _, _ := pushListenerTestSetup(t)
	listener := NewPushEventListener(nil, notifier, zap.NewNop())

	listener.HandlePayload(context.Background(), "not json")
	assert.Nil(t, sender.Tokens)
}

func TestHandlePayloadIgnoresAMessageEventWithAnUnparsableID(t *testing.T) {
	notifier, sender, _, _, _, _, _ := pushListenerTestSetup(t)
	listener := NewPushEventListener(nil, notifier, zap.NewNop())

	payload, err := json.Marshal(wa.Event{
		Type: wa.EventMessage, ConversationID: "not-a-uuid", MessageID: uuid.New().String(),
	})
	require.NoError(t, err)

	listener.HandlePayload(context.Background(), string(payload))
	assert.Nil(t, sender.Tokens)
}
