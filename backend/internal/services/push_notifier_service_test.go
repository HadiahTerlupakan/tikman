package services

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// FakePushSender records what it was asked to send instead of reaching a
// real Firebase project, the same role FakePresence plays for Presence.
type FakePushSender struct {
	FIDs    []string
	Title   string
	Body    string
	Data    map[string]string
	Invalid []string
}

func (f *FakePushSender) SendEach(_ context.Context, fids []string, title, body string, data map[string]string) ([]string, error) {
	f.FIDs = fids
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
	require.NoError(t, pushService.Subscribe(admin.ID, "admin-fid"))
	require.NoError(t, pushService.Subscribe(viewer.ID, "viewer-fid"))

	require.NoError(t, notifier.NotifyIncomingMessage(context.Background(), conv.ID, msg.ID))

	assert.Equal(t, []string{"admin-fid"}, sender.FIDs)
	assert.Equal(t, "Budi", sender.Title)
	assert.Equal(t, "Internetnya mati sejak semalam", sender.Body)
	assert.Equal(t, conv.ID.String(), sender.Data["conversation_id"])
}

func TestNotifyIncomingMessageFallsBackToPhoneWithNoName(t *testing.T) {
	notifier, sender, conversations, messages, pushService, accountID, db := pushTestSetup(t)
	users := NewUserService(db)
	admin, err := users.Create("admin1", "admin1@example.com", "password123", "", models.UserRoleAdmin)
	require.NoError(t, err)
	require.NoError(t, pushService.Subscribe(admin.ID, "admin-fid"))

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
	require.NoError(t, pushService.Subscribe(admin.ID, "admin-fid"))

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

func TestNotifyIncomingMessageRemovesFIDsTheSenderReportsInvalid(t *testing.T) {
	notifier, sender, conversations, messages, pushService, accountID, db := pushTestSetup(t)
	users := NewUserService(db)
	admin, err := users.Create("admin1", "admin1@example.com", "password123", "", models.UserRoleAdmin)
	require.NoError(t, err)
	require.NoError(t, pushService.Subscribe(admin.ID, "dead-fid"))
	sender.Invalid = []string{"dead-fid"}

	conv, err := conversations.FindOrCreate(IncomingPeer{
		WAAccountID: accountID, JID: "628444@s.whatsapp.net", Phone: "628444555666",
	})
	require.NoError(t, err)
	msg, _, err := messages.SaveInbound(InboundMessage{
		ConversationID: conv.ID, WAMessageID: "wamid-4", Kind: models.MessageKindText, Body: "Halo",
	})
	require.NoError(t, err)

	require.NoError(t, notifier.NotifyIncomingMessage(context.Background(), conv.ID, msg.ID))

	fids, err := pushService.FIDsForRoles(models.UserRoleAdmin)
	require.NoError(t, err)
	assert.Empty(t, fids)
}

func TestNotifyIncomingMessageSendsNothingWithNoEligibleFIDs(t *testing.T) {
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
	assert.Nil(t, sender.FIDs)
}

// cs:events announces a CS's own reply with the same EventMessage an inbound
// message uses, so without a direction check the sender is pushed their own
// words under the customer's name.
func TestNotifyIncomingMessageIgnoresAnOutboundReply(t *testing.T) {
	notifier, sender, conversations, messages, pushService, accountID, db := pushTestSetup(t)
	users := NewUserService(db)
	admin, err := users.Create("admin1", "admin1@example.com", "password123", "", models.UserRoleAdmin)
	require.NoError(t, err)
	require.NoError(t, pushService.Subscribe(admin.ID, "admin-fid"))

	conv, err := conversations.FindOrCreate(IncomingPeer{
		WAAccountID: accountID, JID: "628666@s.whatsapp.net", Phone: "628666777888",
	})
	require.NoError(t, err)
	reply, err := messages.Queue(conv.ID, admin.ID, models.MessageKindText, "Baik pak, kami cek dulu", nil, nil)
	require.NoError(t, err)

	require.NoError(t, notifier.NotifyIncomingMessage(context.Background(), conv.ID, reply.ID))
	assert.Nil(t, sender.FIDs)
}

// A photo or a document usually arrives with no caption at all, and a
// notification showing only the customer's name over a blank line gives a CS
// nothing to judge urgency by.
func TestNotifyIncomingMessageNamesTheMediaKindWhenThereIsNoCaption(t *testing.T) {
	cases := []struct {
		kind models.MessageKind
		body string
		want string
	}{
		{models.MessageKindImage, "", "📷 Foto"},
		{models.MessageKindDocument, "", "📄 Dokumen"},
		{models.MessageKindAudio, "", "🎤 Pesan suara"},
		{models.MessageKindVideo, "", "🎬 Video"},
		{models.MessageKindImage, "Ini foto ONT-nya", "Ini foto ONT-nya"},
	}

	for i, tc := range cases {
		notifier, sender, conversations, messages, pushService, accountID, db := pushTestSetup(t)
		users := NewUserService(db)
		admin, err := users.Create("admin1", "admin1@example.com", "password123", "", models.UserRoleAdmin)
		require.NoError(t, err)
		require.NoError(t, pushService.Subscribe(admin.ID, "admin-fid"))

		conv, err := conversations.FindOrCreate(IncomingPeer{
			WAAccountID: accountID, JID: "628777@s.whatsapp.net", Phone: "628777888999",
		})
		require.NoError(t, err)
		msg, _, err := messages.SaveInbound(InboundMessage{
			ConversationID: conv.ID,
			WAMessageID:    fmt.Sprintf("wamid-media-%d", i),
			Kind:           tc.kind,
			Body:           tc.body,
		})
		require.NoError(t, err)

		require.NoError(t, notifier.NotifyIncomingMessage(context.Background(), conv.ID, msg.ID))
		assert.Equal(t, tc.want, sender.Body, "kind %s", tc.kind)
	}
}

// The cut counts runes, not bytes: an Indonesian body or an emoji sliced at
// byte 120 would reach the device as a replacement character.
func TestNotifyIncomingMessageTruncatesMultiByteBodiesByRune(t *testing.T) {
	notifier, sender, conversations, messages, pushService, accountID, db := pushTestSetup(t)
	users := NewUserService(db)
	admin, err := users.Create("admin1", "admin1@example.com", "password123", "", models.UserRoleAdmin)
	require.NoError(t, err)
	require.NoError(t, pushService.Subscribe(admin.ID, "admin-fid"))

	conv, err := conversations.FindOrCreate(IncomingPeer{
		WAAccountID: accountID, JID: "628888@s.whatsapp.net", Phone: "628888999000",
	})
	require.NoError(t, err)
	// 130 runes, every one of them multi-byte, so the 120-rune cut lands well
	// inside what a byte-based slice would have split.
	longBody := strings.Repeat("😀", 130)
	msg, _, err := messages.SaveInbound(InboundMessage{
		ConversationID: conv.ID, WAMessageID: "wamid-emoji", Kind: models.MessageKindText, Body: longBody,
	})
	require.NoError(t, err)

	require.NoError(t, notifier.NotifyIncomingMessage(context.Background(), conv.ID, msg.ID))
	assert.Equal(t, strings.Repeat("😀", 120)+"…", sender.Body)
	assert.True(t, utf8.ValidString(sender.Body))
}

// The boundary is the one branch of previewOf a body-length test cannot reach
// by accident: at exactly the limit nothing is cut and no ellipsis is added,
// one rune over and both happen.
func TestPreviewOfCutsOnlyPastTheLimit(t *testing.T) {
	atLimit := strings.Repeat("a", pushPreviewRunes)
	assert.Equal(t, atLimit, previewOf(atLimit), "a body exactly at the limit is untouched")
	assert.Equal(t, strings.Repeat("a", pushPreviewRunes-1), previewOf(strings.Repeat("a", pushPreviewRunes-1)))

	overLimit := strings.Repeat("a", pushPreviewRunes+1)
	assert.Equal(t, atLimit+"…", previewOf(overLimit), "one rune over is cut to the limit and marked")
}
