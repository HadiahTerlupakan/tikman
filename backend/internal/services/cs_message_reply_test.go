package services

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

// The quote is stored as a pointer to the other row. History resolves it, so a
// thread can draw the quoted block without the quoted message being on the page
// — which it very often is not, since a customer answers a message from hours
// ago and the page only holds the last fifty.
func TestHistoryResolvesTheMessageAReplyQuotes(t *testing.T) {
	messages, _, conv := messageSetup(t)

	asked, _, err := messages.SaveInbound(InboundMessage{
		ConversationID: conv.ID, WAMessageID: "3EB0AAA",
		Kind: models.MessageKindText, Body: "internet saya mati", At: time.Now(),
	})
	require.NoError(t, err)

	_, err = messages.Queue(conv.ID, uuid.New(), models.MessageKindText, "sudah kami cek", nil, &asked.ID)
	require.NoError(t, err)

	history, err := messages.History(conv.ID, 10, 0)
	require.NoError(t, err)
	require.Len(t, history, 2)

	reply := history[0]
	require.NotNil(t, reply.ReplyTo, "a reply must carry the message it quotes")
	assert.Equal(t, asked.ID, reply.ReplyTo.ID)
	assert.Equal(t, "internet saya mati", reply.ReplyTo.Body)
	assert.Equal(t, models.MessageIn, reply.ReplyTo.Direction)

	assert.Nil(t, history[1].ReplyTo, "a message that quotes nothing carries no quote")
}

// Retention sweeps old messages, and the reply to one of them must survive it:
// a CS's own words disappearing from the thread because the question they
// answered aged out would be a worse loss than the quote.
func TestHistoryKeepsAReplyWhoseQuotedMessageIsGone(t *testing.T) {
	messages, _, conv := messageSetup(t)

	asked, _, err := messages.SaveInbound(InboundMessage{
		ConversationID: conv.ID, WAMessageID: "3EB0BBB",
		Kind: models.MessageKindText, Body: "internet saya mati", At: time.Now(),
	})
	require.NoError(t, err)

	reply, err := messages.Queue(conv.ID, uuid.New(), models.MessageKindText, "sudah kami cek", nil, &asked.ID)
	require.NoError(t, err)

	require.NoError(t, messages.db.Delete(&models.CSMessage{}, "id = ?", asked.ID).Error)

	history, err := messages.History(conv.ID, 10, 0)
	require.NoError(t, err)
	require.Len(t, history, 1)
	assert.Equal(t, reply.ID, history[0].ID)
	assert.Nil(t, history[0].ReplyTo)
}

// A customer quoting one of our replies arrives naming the WhatsApp id of what
// they quoted. That id is what ties it back to a row we already hold.
func TestSaveInboundTiesAQuoteBackToTheMessageItAnswers(t *testing.T) {
	messages, _, conv := messageSetup(t)

	ours, err := messages.Queue(conv.ID, uuid.New(), models.MessageKindText, "sudah kami cek", nil, nil)
	require.NoError(t, err)
	require.NoError(t, messages.MarkSent(ours.ID, "3EB0OURS"))

	stored, created, err := messages.SaveInbound(InboundMessage{
		ConversationID: conv.ID, WAMessageID: "3EB0CCC", ReplyToWAID: "3EB0OURS",
		Kind: models.MessageKindText, Body: "masih mati juga", At: time.Now(),
	})
	require.NoError(t, err)
	require.True(t, created)
	require.NotNil(t, stored.ReplyToID)
	assert.Equal(t, ours.ID, *stored.ReplyToID)
}

// A customer can quote a message older than this inbox, or one sent from the
// phone itself. Nothing here holds it, and that is not a reason to drop their
// message — it arrives without a quote.
func TestSaveInboundStoresAQuoteOfAMessageItDoesNotHold(t *testing.T) {
	messages, _, conv := messageSetup(t)

	stored, created, err := messages.SaveInbound(InboundMessage{
		ConversationID: conv.ID, WAMessageID: "3EB0DDD", ReplyToWAID: "3EB0NEVERSEEN",
		Kind: models.MessageKindText, Body: "masih mati juga", At: time.Now(),
	})
	require.NoError(t, err)
	assert.True(t, created)
	assert.Nil(t, stored.ReplyToID)
}

// Quoting reaches across a thread, never between them: WhatsApp has no way to
// render a quote of a message from someone else's chat, and letting one be
// stored would put another customer's words in this customer's screen.
func TestQuoteTargetRefusesAMessageFromAnotherThread(t *testing.T) {
	messages, conversations, conv := messageSetup(t)

	other, err := conversations.FindOrCreate(IncomingPeer{
		WAAccountID: conv.WAAccountID, JID: "628999@s.whatsapp.net",
		Phone: "628999888777", Name: "Siti",
	})
	require.NoError(t, err)

	theirs, _, err := messages.SaveInbound(InboundMessage{
		ConversationID: other.ID, WAMessageID: "3EB0EEE",
		Kind: models.MessageKindText, Body: "punya orang lain", At: time.Now(),
	})
	require.NoError(t, err)

	_, err = messages.QuoteTarget(conv.ID, theirs.ID)
	assert.ErrorIs(t, err, ErrQuoteNotInThread)
}

// A reply still sitting in the outbox has never reached WhatsApp, so it has no
// id to quote. Sending it as a quote anyway would arrive at the customer's
// phone as an empty grey box.
func TestQuoteTargetRefusesAMessageThatHasNotReachedWhatsApp(t *testing.T) {
	messages, _, conv := messageSetup(t)

	waiting, err := messages.Queue(conv.ID, uuid.New(), models.MessageKindText, "menunggu", nil, nil)
	require.NoError(t, err)

	_, err = messages.QuoteTarget(conv.ID, waiting.ID)
	assert.ErrorIs(t, err, ErrQuoteNotSent)
}

func TestQuoteTargetAcceptsASentMessageInTheSameThread(t *testing.T) {
	messages, _, conv := messageSetup(t)

	asked, _, err := messages.SaveInbound(InboundMessage{
		ConversationID: conv.ID, WAMessageID: "3EB0FFF",
		Kind: models.MessageKindText, Body: "internet saya mati", At: time.Now(),
	})
	require.NoError(t, err)

	target, err := messages.QuoteTarget(conv.ID, asked.ID)
	require.NoError(t, err)
	assert.Equal(t, asked.ID, target.ID)
}
