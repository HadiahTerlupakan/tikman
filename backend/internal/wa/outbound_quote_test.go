package wa

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
)

// The drain is where a stored quote becomes something WhatsApp can read: the
// row holds one of our ids, and the customer's phone only knows WhatsApp ids.
func TestDrainSendsTheWhatsAppIDOfTheQuotedMessage(t *testing.T) {
	_, messages, conversations, conv := drainSetup(t)

	asked, _, err := messages.SaveInbound(services.InboundMessage{
		ConversationID: conv.ID, WAMessageID: "3EB0ASKED",
		Kind: models.MessageKindText, Body: "internet saya mati", At: time.Now(),
	})
	require.NoError(t, err)

	_, err = messages.Queue(conv.ID, uuid.New(), models.MessageKindText, "sudah kami cek", nil, &asked.ID)
	require.NoError(t, err)

	sender := &fakeSender{}
	sent, err := NewDrainer(conv.WAAccountID, messages, conversations, sender, nil, t.TempDir(), 0).Drain(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, 1, sent)

	require.Len(t, sender.quotes, 1)
	quote := sender.quotes[0]
	require.NotNil(t, quote)
	assert.Equal(t, "3EB0ASKED", quote.StanzaID)
	assert.False(t, quote.FromMe, "the customer wrote the quoted message")
	assert.Equal(t, "internet saya mati", quote.Body)
}

// Quoting our own earlier reply is the other half, and it is the half that
// breaks quietly: FromMe is what decides which number WhatsApp is told to look
// the message up under.
func TestDrainMarksAQuoteOfOurOwnReplyAsOurs(t *testing.T) {
	_, messages, conversations, conv := drainSetup(t)

	ours, err := messages.Queue(conv.ID, uuid.New(), models.MessageKindText, "sudah kami cek", nil, nil)
	require.NoError(t, err)
	require.NoError(t, messages.MarkSent(ours.ID, "3EB0OURS"))

	_, err = messages.Queue(conv.ID, uuid.New(), models.MessageKindText, "masih dicek ya", nil, &ours.ID)
	require.NoError(t, err)

	sender := &fakeSender{}
	_, err = NewDrainer(conv.WAAccountID, messages, conversations, sender, nil, t.TempDir(), 0).Drain(context.Background(), 10)
	require.NoError(t, err)

	require.Len(t, sender.quotes, 1)
	require.NotNil(t, sender.quotes[0])
	assert.True(t, sender.quotes[0].FromMe)
}

// Retention runs between a CS writing a reply and the outbox draining it. The
// answer must still go out: a customer waiting on it does not care that the
// grey block above it could not be drawn.
func TestDrainStillSendsAReplyWhoseQuotedMessageIsGone(t *testing.T) {
	db, messages, conversations, conv := drainSetup(t)

	asked, _, err := messages.SaveInbound(services.InboundMessage{
		ConversationID: conv.ID, WAMessageID: "3EB0GONE",
		Kind: models.MessageKindText, Body: "internet saya mati", At: time.Now(),
	})
	require.NoError(t, err)

	_, err = messages.Queue(conv.ID, uuid.New(), models.MessageKindText, "sudah kami cek", nil, &asked.ID)
	require.NoError(t, err)
	require.NoError(t, db.Delete(&models.CSMessage{}, "id = ?", asked.ID).Error)

	sender := &fakeSender{}
	sent, err := NewDrainer(conv.WAAccountID, messages, conversations, sender, nil, t.TempDir(), 0).Drain(context.Background(), 10)
	require.NoError(t, err)

	assert.Equal(t, 1, sent)
	assert.Equal(t, []string{"sudah kami cek"}, sender.sent)
	assert.Nil(t, sender.quotes[0])
}

func TestDrainSendsNoQuoteForAnOrdinaryReply(t *testing.T) {
	_, messages, conversations, conv := drainSetup(t)

	_, err := messages.Queue(conv.ID, uuid.New(), models.MessageKindText, "halo", nil, nil)
	require.NoError(t, err)

	sender := &fakeSender{}
	_, err = NewDrainer(conv.WAAccountID, messages, conversations, sender, nil, t.TempDir(), 0).Drain(context.Background(), 10)
	require.NoError(t, err)

	require.Len(t, sender.quotes, 1)
	assert.Nil(t, sender.quotes[0])
}
