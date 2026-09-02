package services

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

func messageSetup(t *testing.T) (*CSMessageService, *CSConversationService, *models.CSConversation) {
	db := setupTestDB(t)
	conversations := NewCSConversationService(db)
	account := csAccount(t, db)
	conv, err := conversations.FindOrCreate(IncomingPeer{
		WAAccountID: account.ID, JID: "628111@s.whatsapp.net", Phone: "628111222333", Name: "Budi",
	})
	require.NoError(t, err)
	return NewCSMessageService(db, conversations), conversations, conv
}

// WhatsApp re-delivers events it is unsure about. Storing the second copy would
// show the customer's message twice and, worse, count it twice as unread.
func TestSaveInboundStoresARepeatedMessageOnlyOnce(t *testing.T) {
	messages, _, conv := messageSetup(t)

	in := InboundMessage{
		ConversationID: conv.ID,
		WAMessageID:    "3EB0ABC123",
		Kind:           models.MessageKindText,
		Body:           "internet saya mati",
		At:             time.Now(),
	}

	first, created, err := messages.SaveInbound(in)
	require.NoError(t, err)
	assert.True(t, created)

	second, created, err := messages.SaveInbound(in)
	require.NoError(t, err)
	assert.False(t, created, "the same WhatsApp message must not be stored twice")
	assert.Equal(t, first.ID, second.ID)

	history, err := messages.History(conv.ID, 10, 0)
	require.NoError(t, err)
	assert.Len(t, history, 1)
}

func TestSaveInboundMovesTheThreadToTheTopOfTheInbox(t *testing.T) {
	messages, conversations, conv := messageSetup(t)

	before, err := conversations.Get(conv.ID)
	require.NoError(t, err)

	later := time.Now().Add(time.Hour)
	_, _, err = messages.SaveInbound(InboundMessage{
		ConversationID: conv.ID, WAMessageID: "3EB0ABC124",
		Kind: models.MessageKindText, Body: "halo", At: later,
	})
	require.NoError(t, err)

	after, err := conversations.Get(conv.ID)
	require.NoError(t, err)
	assert.True(t, after.LastMessageAt.After(before.LastMessageAt))
	assert.Equal(t, 1, after.UnreadCount)
}

func TestQueuedMessageWaitsWithoutAWhatsAppID(t *testing.T) {
	messages, _, conv := messageSetup(t)
	sender := uuid.New()

	msg, err := messages.Queue(conv.ID, sender, models.MessageKindText, "sudah kami cek", nil)
	require.NoError(t, err)

	assert.Equal(t, models.MessageQueued, msg.Status)
	assert.Equal(t, models.MessageOut, msg.Direction)
	assert.Nil(t, msg.WAMessageID)
	require.NotNil(t, msg.SenderUserID)
	assert.Equal(t, sender, *msg.SenderUserID)
}

// This is the promise that no CS reply disappears: whatever was queued while
// the WhatsApp process was down is still here to be claimed when it returns.
func TestClaimQueuedReturnsOnlyWhatIsStillWaiting(t *testing.T) {
	messages, _, conv := messageSetup(t)
	sender := uuid.New()

	waiting, err := messages.Queue(conv.ID, sender, models.MessageKindText, "menunggu", nil)
	require.NoError(t, err)
	gone, err := messages.Queue(conv.ID, sender, models.MessageKindText, "terkirim", nil)
	require.NoError(t, err)
	require.NoError(t, messages.MarkSent(gone.ID, "3EB0SENT"))

	claimed, err := messages.ClaimQueued(10)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	assert.Equal(t, waiting.ID, claimed[0].ID)
}

func TestMarkFailedKeepsTheReasonWhereTheCSCanReadIt(t *testing.T) {
	messages, _, conv := messageSetup(t)

	msg, err := messages.Queue(conv.ID, uuid.New(), models.MessageKindText, "halo", nil)
	require.NoError(t, err)
	require.NoError(t, messages.MarkFailed(msg.ID, "nomor tidak terdaftar di WhatsApp"))

	history, err := messages.History(conv.ID, 10, 0)
	require.NoError(t, err)
	require.Len(t, history, 1)
	assert.Equal(t, models.MessageFailed, history[0].Status)
	assert.Equal(t, "nomor tidak terdaftar di WhatsApp", history[0].FailReason)
}

func TestApplyReceiptWalksAMessageForwardOnly(t *testing.T) {
	messages, _, conv := messageSetup(t)

	msg, err := messages.Queue(conv.ID, uuid.New(), models.MessageKindText, "halo", nil)
	require.NoError(t, err)
	require.NoError(t, messages.MarkSent(msg.ID, "3EB0AAA"))

	require.NoError(t, messages.ApplyReceipt("3EB0AAA", models.MessageRead))
	require.NoError(t, messages.ApplyReceipt("3EB0AAA", models.MessageDelivered))

	history, err := messages.History(conv.ID, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, models.MessageRead, history[0].Status,
		"a late delivered receipt must not undo a read one")
}
