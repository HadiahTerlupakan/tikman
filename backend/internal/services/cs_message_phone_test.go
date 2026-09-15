package services

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

func phoneMessage(convID uuid.UUID, waID string, at time.Time) InboundMessage {
	return InboundMessage{
		ConversationID: convID, WAMessageID: waID,
		Kind: models.MessageKindText, Body: "sudah kami cek dari hp", At: at,
	}
}

// Nothing marks a phone reply but its shape: outbound, with no CS on it. Stored
// with a sender or as inbound, it would read as a TikMan reply or as the
// customer talking.
func TestAPhoneReplyIsStoredAsOutboundWithNoSender(t *testing.T) {
	messages, conversations, acc := awaitingSetup(t)
	conv := thread(t, conversations, acc, "628111222333")

	msg, created, err := messages.SaveFromPhone(phoneMessage(conv.ID, "3EB0PHONE", time.Now()))

	require.NoError(t, err)
	assert.True(t, created)
	assert.Equal(t, models.MessageOut, msg.Direction)
	assert.Nil(t, msg.SenderUserID)
	assert.Equal(t, models.MessageSent, msg.Status)
}

// The customer has been answered, so the thread must stop asking for an answer
// and stop badging messages somebody already read on the phone.
func TestAPhoneReplyTakesTheThreadOutOfAwaitingReply(t *testing.T) {
	messages, conversations, acc := awaitingSetup(t)
	conv := thread(t, conversations, acc, "628111222333")
	storeCustomerMessage(t, messages, conv.ID, "3EB0A")

	_, _, err := messages.SaveFromPhone(phoneMessage(conv.ID, "3EB0PHONE", time.Now()))
	require.NoError(t, err)

	waiting, err := conversations.List(ConversationFilter{AwaitingReply: true})
	require.NoError(t, err)
	assert.Empty(t, waiting)
	got, err := conversations.Get(conv.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, got.UnreadCount)
}

// A phone reply WhatsApp hands over late, after the customer wrote again, did
// not answer that newer message. Treating it as the last word would hide a
// customer who is still waiting.
func TestALatePhoneReplyLeavesANewerCustomerMessageWaiting(t *testing.T) {
	messages, conversations, acc := awaitingSetup(t)
	conv := thread(t, conversations, acc, "628111222333")
	storeCustomerMessage(t, messages, conv.ID, "3EB0A")

	earlier := time.Now().Add(-time.Minute)
	_, _, err := messages.SaveFromPhone(phoneMessage(conv.ID, "3EB0PHONE", earlier))
	require.NoError(t, err)

	waiting, err := conversations.List(ConversationFilter{AwaitingReply: true})
	require.NoError(t, err)
	require.Len(t, waiting, 1)
	got, err := conversations.Get(conv.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, got.UnreadCount)
}

// Nobody is known to have sent it, so it can neither claim the thread nor
// reopen one a CS decided was finished.
func TestAPhoneReplyLeavesTheHolderAndAClosedThreadAlone(t *testing.T) {
	messages, conversations, acc := awaitingSetup(t)
	conv := thread(t, conversations, acc, "628111222333")
	storeCustomerMessage(t, messages, conv.ID, "3EB0A")
	holder := uuid.New()
	require.NoError(t, conversations.Assign(conv.ID, holder))
	require.NoError(t, conversations.Close(conv.ID))

	_, _, err := messages.SaveFromPhone(phoneMessage(conv.ID, "3EB0PHONE", time.Now()))
	require.NoError(t, err)

	got, err := conversations.Get(conv.ID)
	require.NoError(t, err)
	assert.Equal(t, models.ConversationClosed, got.Status)
	require.NotNil(t, got.AssignedUserID)
	assert.Equal(t, holder, *got.AssignedUserID)
}

// WhatsApp re-delivers events on every reconnect. A second copy would show the
// reply twice in the thread.
func TestTheSamePhoneReplyDeliveredTwiceIsStoredOnce(t *testing.T) {
	messages, conversations, acc := awaitingSetup(t)
	conv := thread(t, conversations, acc, "628111222333")
	in := phoneMessage(conv.ID, "3EB0PHONE", time.Now())

	first, created, err := messages.SaveFromPhone(in)
	require.NoError(t, err)
	require.True(t, created)
	second, created, err := messages.SaveFromPhone(in)
	require.NoError(t, err)

	assert.False(t, created)
	assert.Equal(t, first.ID, second.ID)
	history, err := messages.History(conv.ID, 10, 0)
	require.NoError(t, err)
	assert.Len(t, history, 1)
}
