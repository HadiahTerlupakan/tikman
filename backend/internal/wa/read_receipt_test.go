package wa

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
)

// customerSaid stores one incoming message in a thread, as the inbound handler
// would have stored it.
func customerSaid(t *testing.T, messages *services.CSMessageService, conversationID uuid.UUID, waID, body string) {
	t.Helper()
	_, created, err := messages.SaveInbound(services.InboundMessage{
		ConversationID: conversationID,
		WAMessageID:    waID,
		Kind:           models.MessageKindText,
		Body:           body,
		At:             time.Now(),
	})
	require.NoError(t, err)
	require.True(t, created)
}

// The customer is left watching two grey ticks until somebody answers them.
// Marking read on the reply is the point chosen deliberately over marking on
// the inbox opening: blue ticks then mean a CS has answered, not that a message
// reached a queue.
func TestReplyingTurnsTheCustomersTicksBlue(t *testing.T) {
	_, messages, conversations, conv := drainSetup(t)
	sender := &fakeSender{}

	customerSaid(t, messages, conv.ID, "3EB0THEIRS1", "pak internet saya mati")
	customerSaid(t, messages, conv.ID, "3EB0THEIRS2", "sudah dari tadi pagi")
	_, err := messages.Queue(conv.ID, uuid.New(), models.MessageKindText, "kami cek dulu ya", nil, nil)
	require.NoError(t, err)

	_, err = NewDrainer(conv.WAAccountID, messages, conversations, sender,
		nil, t.TempDir(), 0).Drain(context.Background(), 10)
	require.NoError(t, err)

	require.Len(t, sender.reads, 1)
	assert.Equal(t, conv.CustomerJID, sender.reads[0].chatJID)
	assert.Equal(t, []string{"3EB0THEIRS1", "3EB0THEIRS2"}, sender.reads[0].ids)
}

// A CS who answers in two short messages has read the customer once. A receipt
// per reply would put the same acknowledgement on the wire twice, which is
// exactly the traffic pattern that gets an unofficial number flagged.
func TestOneThreadIsAcknowledgedOncePerDrain(t *testing.T) {
	_, messages, conversations, conv := drainSetup(t)
	sender := &fakeSender{}

	customerSaid(t, messages, conv.ID, "3EB0THEIRS", "halo")
	for _, body := range []string{"halo pak", "ada yang bisa dibantu?"} {
		_, err := messages.Queue(conv.ID, uuid.New(), models.MessageKindText, body, nil, nil)
		require.NoError(t, err)
	}

	_, err := NewDrainer(conv.WAAccountID, messages, conversations, sender,
		nil, t.TempDir(), 0).Drain(context.Background(), 10)
	require.NoError(t, err)

	assert.Len(t, sender.reads, 1)
}

// The acknowledgement is recorded locally so a later reply does not walk back
// over the whole history. Without it every answer in a long-running thread
// would re-acknowledge months of messages.
func TestAlreadyAcknowledgedMessagesAreNotSentAgain(t *testing.T) {
	_, messages, conversations, conv := drainSetup(t)
	sender := &fakeSender{}
	drainer := NewDrainer(conv.WAAccountID, messages, conversations, sender, nil, t.TempDir(), 0)

	customerSaid(t, messages, conv.ID, "3EB0FIRST", "halo")
	_, err := messages.Queue(conv.ID, uuid.New(), models.MessageKindText, "halo pak", nil, nil)
	require.NoError(t, err)
	_, err = drainer.Drain(context.Background(), 10)
	require.NoError(t, err)

	customerSaid(t, messages, conv.ID, "3EB0SECOND", "sudah bisa")
	_, err = messages.Queue(conv.ID, uuid.New(), models.MessageKindText, "siap pak", nil, nil)
	require.NoError(t, err)
	_, err = drainer.Drain(context.Background(), 10)
	require.NoError(t, err)

	require.Len(t, sender.reads, 2)
	assert.Equal(t, []string{"3EB0FIRST"}, sender.reads[0].ids)
	assert.Equal(t, []string{"3EB0SECOND"}, sender.reads[1].ids,
		"the second drain acknowledges only what arrived since the first")
}

// A reply WhatsApp refused is not an answer. Telling the customer their message
// was read while the reply sits failed in front of the CS would be a lie the
// customer can see and the team cannot.
func TestARefusedReplyLeavesTheTicksGrey(t *testing.T) {
	_, messages, conversations, conv := drainSetup(t)
	sender := &fakeSender{err: errors.New("nomor tidak terdaftar di WhatsApp")}

	customerSaid(t, messages, conv.ID, "3EB0THEIRS", "halo")
	_, err := messages.Queue(conv.ID, uuid.New(), models.MessageKindText, "halo pak", nil, nil)
	require.NoError(t, err)

	_, err = NewDrainer(conv.WAAccountID, messages, conversations, sender,
		nil, t.TempDir(), 0).Drain(context.Background(), 10)
	require.NoError(t, err)

	assert.Empty(t, sender.reads)
}

// A receipt WhatsApp would not take must not be recorded as taken, or the
// customer's ticks stay grey forever while every later reply skips them.
func TestAReceiptWhatsAppRefusesIsRetriedOnTheNextReply(t *testing.T) {
	_, messages, conversations, conv := drainSetup(t)
	sender := &refusingReader{}
	drainer := NewDrainer(conv.WAAccountID, messages, conversations, sender, nil, t.TempDir(), 0)

	customerSaid(t, messages, conv.ID, "3EB0THEIRS", "halo")
	for range 2 {
		_, err := messages.Queue(conv.ID, uuid.New(), models.MessageKindText, "halo pak", nil, nil)
		require.NoError(t, err)
		_, err = drainer.Drain(context.Background(), 10)
		require.NoError(t, err)
	}

	assert.Equal(t, 2, sender.attempts, "the unacknowledged message is offered again")
}

// refusingReader sends replies happily but never accepts a read receipt.
type refusingReader struct {
	fakeSender
	attempts int
}

func (r *refusingReader) MarkRead(_ context.Context, _ string, _ []string, _ time.Time) error {
	r.attempts++
	return errors.New("receipt ditolak")
}
