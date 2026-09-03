package services

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// FakePushSender records what it was asked to send instead of reaching a
// real Firebase project, the same role FakePresence plays for Presence.
type FakePushSender struct {
	Tokens  []string
	Title   string
	Body    string
	Data    map[string]string
	Invalid []string
}

func (f *FakePushSender) SendEach(_ context.Context, tokens []string, title, body string, data map[string]string) ([]string, error) {
	f.Tokens = tokens
	f.Title = title
	f.Body = body
	f.Data = data
	return f.Invalid, nil
}

// pushTestSetup wires a notifier against a fake Sender and returns the db
// and account id too, since every test also needs to create its own
// conversation (via IncomingPeer.WAAccountID) and, in most cases, real users
// (via NewUserService(db)) sharing this same database.
func pushTestSetup(t *testing.T) (notifier *PushNotifierService, sender *FakePushSender, conversations *CSConversationService, messages *CSMessageService, pushService *PushService, accountID uuid.UUID, db *gorm.DB) {
	t.Helper()
	db = setupTestDB(t)
	account := models.WAAccount{Label: "CS Utama", Status: models.WAAccountConnected}
	require.NoError(t, db.Create(&account).Error)

	conversations = NewCSConversationService(db)
	messages = NewCSMessageService(db, conversations)
	pushService = NewPushService(db)
	sender = &FakePushSender{}
	notifier = NewPushNotifierService(sender, pushService, conversations, messages)
	return notifier, sender, conversations, messages, pushService, account.ID, db
}

func TestNotifyIncomingMessageSendsToEveryEligibleRole(t *testing.T) {
	notifier, sender, conversations, messages, pushService, accountID, db := pushTestSetup(t)
	users := NewUserService(db)

	conv, err := conversations.FindOrCreate(IncomingPeer{
		WAAccountID: accountID,
		JID:         "628111@s.whatsapp.net",
		Phone:       "628111222333",
		Name:        "Budi",
	})
	require.NoError(t, err)
	msg, _, err := messages.SaveInbound(InboundMessage{
		ConversationID: conv.ID,
		WAMessageID:    "wamid-1",
		Kind:           models.MessageKindText,
		Body:           "Internetnya mati sejak semalam",
	})
	require.NoError(t, err)

	admin, err := users.Create("admin1", "admin1@example.com", "password123", "", models.UserRoleAdmin)
	require.NoError(t, err)
	viewer, err := users.Create("viewer1", "viewer1@example.com", "password123", "", models.UserRoleViewer)
	require.NoError(t, err)
	require.NoError(t, pushService.Subscribe(admin.ID, "admin-token"))
	require.NoError(t, pushService.Subscribe(viewer.ID, "viewer-token"))

	require.NoError(t, notifier.NotifyIncomingMessage(context.Background(), conv.ID, msg.ID))

	assert.Equal(t, []string{"admin-token"}, sender.Tokens)
	assert.Equal(t, "Budi", sender.Title)
	assert.Equal(t, "Internetnya mati sejak semalam", sender.Body)
	assert.Equal(t, conv.ID.String(), sender.Data["conversation_id"])
}

func TestNotifyIncomingMessageFallsBackToPhoneWithNoName(t *testing.T) {
	notifier, sender, conversations, messages, pushService, accountID, db := pushTestSetup(t)
	users := NewUserService(db)
	admin, err := users.Create("admin1", "admin1@example.com", "password123", "", models.UserRoleAdmin)
	require.NoError(t, err)
	require.NoError(t, pushService.Subscribe(admin.ID, "admin-token"))

	conv, err := conversations.FindOrCreate(IncomingPeer{
		WAAccountID: accountID,
		JID:         "628222@s.whatsapp.net",
		Phone:       "628222333444",
		Name:        "",
	})
	require.NoError(t, err)
	msg, _, err := messages.SaveInbound(InboundMessage{
		ConversationID: conv.ID, WAMessageID: "wamid-2", Kind: models.MessageKindText, Body: "Halo",
	})
	require.NoError(t, err)

	require.NoError(t, notifier.NotifyIncomingMessage(context.Background(), conv.ID, msg.ID))
	assert.Equal(t, "628222333444", sender.Title)
}

func TestNotifyIncomingMessageTruncatesALongBody(t *testing.T) {
	notifier, sender, conversations, messages, pushService, accountID, db := pushTestSetup(t)
	users := NewUserService(db)
	admin, err := users.Create("admin1", "admin1@example.com", "password123", "", models.UserRoleAdmin)
	require.NoError(t, err)
	require.NoError(t, pushService.Subscribe(admin.ID, "admin-token"))

	conv, err := conversations.FindOrCreate(IncomingPeer{
		WAAccountID: accountID, JID: "628333@s.whatsapp.net", Phone: "628333444555",
	})
	require.NoError(t, err)
	longBody := strings.Repeat("a", 200)
	msg, _, err := messages.SaveInbound(InboundMessage{
		ConversationID: conv.ID, WAMessageID: "wamid-3", Kind: models.MessageKindText, Body: longBody,
	})
	require.NoError(t, err)

	require.NoError(t, notifier.NotifyIncomingMessage(context.Background(), conv.ID, msg.ID))
	assert.Equal(t, strings.Repeat("a", 120)+"…", sender.Body)
}

func TestNotifyIncomingMessageRemovesTokensTheSenderReportsInvalid(t *testing.T) {
	notifier, sender, conversations, messages, pushService, accountID, db := pushTestSetup(t)
	users := NewUserService(db)
	admin, err := users.Create("admin1", "admin1@example.com", "password123", "", models.UserRoleAdmin)
	require.NoError(t, err)
	require.NoError(t, pushService.Subscribe(admin.ID, "dead-token"))
	sender.Invalid = []string{"dead-token"}

	conv, err := conversations.FindOrCreate(IncomingPeer{
		WAAccountID: accountID, JID: "628444@s.whatsapp.net", Phone: "628444555666",
	})
	require.NoError(t, err)
	msg, _, err := messages.SaveInbound(InboundMessage{
		ConversationID: conv.ID, WAMessageID: "wamid-4", Kind: models.MessageKindText, Body: "Halo",
	})
	require.NoError(t, err)

	require.NoError(t, notifier.NotifyIncomingMessage(context.Background(), conv.ID, msg.ID))

	tokens, err := pushService.TokensForRoles(models.UserRoleAdmin)
	require.NoError(t, err)
	assert.Empty(t, tokens)
}

func TestNotifyIncomingMessageSendsNothingWithNoEligibleTokens(t *testing.T) {
	notifier, sender, conversations, messages, _, accountID, _ := pushTestSetup(t)
	conv, err := conversations.FindOrCreate(IncomingPeer{
		WAAccountID: accountID, JID: "628555@s.whatsapp.net", Phone: "628555666777",
	})
	require.NoError(t, err)
	msg, _, err := messages.SaveInbound(InboundMessage{
		ConversationID: conv.ID, WAMessageID: "wamid-5", Kind: models.MessageKindText, Body: "Halo",
	})
	require.NoError(t, err)

	require.NoError(t, notifier.NotifyIncomingMessage(context.Background(), conv.ID, msg.ID))
	assert.Nil(t, sender.Tokens)
}
