package services

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

func awaitingSetup(t *testing.T) (*CSMessageService, *CSConversationService, models.WAAccount) {
	t.Helper()
	db := setupTestDB(t)
	conversations := NewCSConversationService(db)
	return NewCSMessageService(db, conversations), conversations, csAccount(t, db)
}

func thread(t *testing.T, s *CSConversationService, acc models.WAAccount, phone string) *models.CSConversation {
	t.Helper()
	conv, err := s.FindOrCreate(IncomingPeer{
		WAAccountID: acc.ID, JID: phone + "@s.whatsapp.net", Phone: phone, Name: "Pelanggan",
	})
	require.NoError(t, err)
	return conv
}

func customerWrote(t *testing.T, m *CSMessageService, convID uuid.UUID, waID string) {
	t.Helper()
	_, _, err := m.SaveInbound(InboundMessage{
		ConversationID: convID, WAMessageID: waID,
		Kind: models.MessageKindText, Body: "internet saya mati", At: time.Now(),
	})
	require.NoError(t, err)
}

// The view is "waiting on us", not "nobody owns it". Whoever holds the thread,
// a customer's message that has had no answer is work outstanding.
func TestAwaitingReplyIsEveryThreadTheCustomerSpokeLastIn(t *testing.T) {
	messages, conversations, acc := awaitingSetup(t)

	unanswered := thread(t, conversations, acc, "628111222333")
	answered := thread(t, conversations, acc, "628222333444")
	customerWrote(t, messages, unanswered.ID, "3EB0A")
	customerWrote(t, messages, answered.ID, "3EB0B")
	_, err := messages.Queue(answered.ID, uuid.New(), models.MessageKindText, "sudah kami cek", nil, nil)
	require.NoError(t, err)

	waiting, err := conversations.List(ConversationFilter{AwaitingReply: true})
	require.NoError(t, err)
	require.Len(t, waiting, 1)
	assert.Equal(t, unanswered.ID, waiting[0].ID)
}

// A thread somebody already holds still needs answering. Under the old rule it
// vanished from this view the moment it was assigned, which is exactly when
// somebody became responsible for it.
func TestAwaitingReplyKeepsAThreadThatIsAlreadyHeld(t *testing.T) {
	messages, conversations, acc := awaitingSetup(t)

	conv := thread(t, conversations, acc, "628111222333")
	customerWrote(t, messages, conv.ID, "3EB0A")
	require.NoError(t, conversations.Assign(conv.ID, uuid.New()))

	waiting, err := conversations.List(ConversationFilter{AwaitingReply: true})
	require.NoError(t, err)
	assert.Len(t, waiting, 1)
}

// The case that has no other home: a customer writes again after their thread
// was closed. It is not new, and nobody is waiting on it under any status rule.
func TestAwaitingReplyCatchesACustomerWritingAfterTheThreadWasClosed(t *testing.T) {
	messages, conversations, acc := awaitingSetup(t)

	conv := thread(t, conversations, acc, "628111222333")
	customerWrote(t, messages, conv.ID, "3EB0A")
	_, err := messages.Queue(conv.ID, uuid.New(), models.MessageKindText, "sudah kami cek", nil, nil)
	require.NoError(t, err)
	require.NoError(t, conversations.Close(conv.ID))

	waiting, err := conversations.List(ConversationFilter{AwaitingReply: true})
	require.NoError(t, err)
	require.Empty(t, waiting, "a closed thread with our answer last is not waiting on anyone")

	customerWrote(t, messages, conv.ID, "3EB0B")

	waiting, err = conversations.List(ConversationFilter{AwaitingReply: true})
	require.NoError(t, err)
	require.Len(t, waiting, 1)
	assert.Equal(t, conv.ID, waiting[0].ID)
}

// The direction is recorded when the reply is queued, not when WhatsApp takes
// it. Otherwise a thread stays in this view until the outbox drains, and the
// next CS to look answers a customer who has already been answered.
func TestAwaitingReplyClearsAsSoonAsAReplyIsQueued(t *testing.T) {
	messages, conversations, acc := awaitingSetup(t)

	conv := thread(t, conversations, acc, "628111222333")
	customerWrote(t, messages, conv.ID, "3EB0A")

	msg, err := messages.Queue(conv.ID, uuid.New(), models.MessageKindText, "sudah kami cek", nil, nil)
	require.NoError(t, err)
	assert.Equal(t, models.MessageQueued, msg.Status, "still waiting to be sent")

	waiting, err := conversations.List(ConversationFilter{AwaitingReply: true})
	require.NoError(t, err)
	assert.Empty(t, waiting)
}
