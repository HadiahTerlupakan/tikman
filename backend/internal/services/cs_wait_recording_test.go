package services

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

func recordingSetup(t *testing.T) (*gorm.DB, *CSMessageService, *CSConversationService, *models.CSConversation) {
	t.Helper()
	db := setupTestDB(t)
	conversations := NewCSConversationService(db)
	conv, err := conversations.FindOrCreate(peer(csAccount(t, db).ID))
	require.NoError(t, err)
	return db, NewCSMessageService(db, conversations), conversations, conv
}

func customerSent(conversationID uuid.UUID, waID string, sent time.Time) InboundMessage {
	return InboundMessage{
		ConversationID: conversationID, WAMessageID: waID,
		Kind: models.MessageKindText, Body: "internet mati", At: sent,
	}
}

func storedWaits(t *testing.T, db *gorm.DB) []models.CSWait {
	t.Helper()
	var waits []models.CSWait
	require.NoError(t, db.Order("started_at ASC").Find(&waits).Error)
	return waits
}

func TestStoringACustomerMessageStartsAWait(t *testing.T) {
	db, messages, _, conv := recordingSetup(t)

	_, _, err := messages.SaveInbound(customerSent(conv.ID, "3EB0A", time.Now()))
	require.NoError(t, err)

	waits := storedWaits(t, db)
	require.Len(t, waits, 1)
	assert.Equal(t, conv.ID, waits[0].ConversationID)
	assert.Nil(t, waits[0].EndedAt)
}

// WhatsApp re-delivers on every reconnect, and a duplicate is not more waiting.
func TestACustomerMessageDeliveredTwiceTouchesTheWaitOnce(t *testing.T) {
	db, messages, _, conv := recordingSetup(t)
	in := customerSent(conv.ID, "3EB0A", time.Now())
	_, _, err := messages.SaveInbound(in)
	require.NoError(t, err)

	_, created, err := messages.SaveInbound(in)
	require.NoError(t, err)

	assert.False(t, created)
	assert.Len(t, storedWaits(t, db), 1)
}

func TestStoringAPhoneReplyEndsTheWait(t *testing.T) {
	db, messages, _, conv := recordingSetup(t)
	sent := time.Now()
	_, _, err := messages.SaveInbound(customerSent(conv.ID, "3EB0A", sent))
	require.NoError(t, err)

	_, _, err = messages.SaveFromPhone(InboundMessage{
		ConversationID: conv.ID, WAMessageID: "3EB0PHONE",
		Kind: models.MessageKindText, Body: "sudah kami cek", At: sent.Add(time.Minute),
	})
	require.NoError(t, err)

	waits := storedWaits(t, db)
	require.Len(t, waits, 1)
	require.NotNil(t, waits[0].EndReason)
	assert.Equal(t, models.WaitPhone, *waits[0].EndReason)
}

// The figures must never be why a customer's message is lost: with the waits
// table gone, the message and the thread still land as before.
func TestAMessageIsStoredEvenWhenItsWaitCannotBeRecorded(t *testing.T) {
	db, messages, conversations, conv := recordingSetup(t)
	require.NoError(t, db.Migrator().DropTable(&models.CSWait{}))

	_, created, err := messages.SaveInbound(customerSent(conv.ID, "3EB0A", time.Now()))

	require.NoError(t, err)
	assert.True(t, created)
	waiting, err := conversations.List(ConversationFilter{AwaitingReply: true})
	require.NoError(t, err)
	assert.Len(t, waiting, 1)
}

func TestAPhoneReplyIsStoredEvenWhenItsWaitCannotBeRecorded(t *testing.T) {
	db, messages, conversations, conv := recordingSetup(t)
	sent := time.Now()
	_, _, err := messages.SaveInbound(customerSent(conv.ID, "3EB0A", sent))
	require.NoError(t, err)
	require.NoError(t, db.Migrator().DropTable(&models.CSWait{}))

	_, created, err := messages.SaveFromPhone(InboundMessage{
		ConversationID: conv.ID, WAMessageID: "3EB0PHONE",
		Kind: models.MessageKindText, Body: "sudah", At: sent.Add(time.Minute),
	})

	require.NoError(t, err)
	assert.True(t, created)
	waiting, err := conversations.List(ConversationFilter{AwaitingReply: true})
	require.NoError(t, err)
	assert.Empty(t, waiting)
}
