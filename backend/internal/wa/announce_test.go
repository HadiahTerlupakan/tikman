package wa

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// recordingAnnouncer keeps what was said instead of saying it. The publisher
// the other tests use points at a dead Redis, which can neither fail nor be
// observed, so nothing there checks that the right thing was said about the
// right thread.
type recordingAnnouncer struct {
	events []Event
}

func (r *recordingAnnouncer) Publish(_ context.Context, event Event) error {
	r.events = append(r.events, event)
	return nil
}

func (r *recordingAnnouncer) conversationsNamed() []string {
	var named []string
	for _, event := range r.events {
		if event.Type == EventStatus && event.ConversationID != "" {
			named = append(named, event.ConversationID)
		}
	}
	return named
}

// A status event carrying no conversation only refreshes the inbox list. The
// ticks a CS is watching inside an open thread live under a different query
// key, so a receipt that names nothing leaves them frozen until the CS
// navigates away and back — which is the one thing a receipt exists to avoid.
func TestReceiptNamesTheThreadItMoved(t *testing.T) {
	_, messages, _, conv := drainSetup(t)
	recorder := &recordingAnnouncer{}
	receipts := &receiptHandler{messages: messages, publisher: recorder}

	ours, err := messages.Queue(conv.ID, uuid.New(), models.MessageKindText, "sudah kami cek", nil, nil)
	require.NoError(t, err)
	require.NoError(t, messages.MarkSent(ours.ID, "3EB0OURS"))

	require.NoError(t, receipts.handle(context.Background(), &events.Receipt{
		MessageIDs: []string{"3EB0OURS"},
		Type:       types.ReceiptTypeDelivered,
	}))

	assert.Equal(t, []string{conv.ID.String()}, recorder.conversationsNamed())
}

// WhatsApp reports several ids in one receipt, and a batch can span threads
// when a CS has answered more than one customer. Each thread the batch moved
// has to hear about it exactly once, however many of its messages were named.
func TestReceiptNamesEveryThreadOnceAcrossABatch(t *testing.T) {
	db, messages, conversations, first := drainSetup(t)

	var account models.WAAccount
	require.NoError(t, db.First(&account).Error)
	second, err := conversations.FindOrCreate(services.IncomingPeer{
		WAAccountID: account.ID, JID: "628222@s.whatsapp.net", Phone: "628222333444", Name: "Siti",
	})
	require.NoError(t, err)

	recorder := &recordingAnnouncer{}
	receipts := &receiptHandler{messages: messages, publisher: recorder}

	for _, spec := range []struct {
		conversationID uuid.UUID
		waID           string
	}{
		{first.ID, "3EB0A"},
		{first.ID, "3EB0B"},
		{second.ID, "3EB0C"},
	} {
		msg, err := messages.Queue(spec.conversationID, uuid.New(), models.MessageKindText, "halo", nil, nil)
		require.NoError(t, err)
		require.NoError(t, messages.MarkSent(msg.ID, spec.waID))
	}

	require.NoError(t, receipts.handle(context.Background(), &events.Receipt{
		MessageIDs: []string{"3EB0A", "3EB0B", "3EB0C"},
		Type:       types.ReceiptTypeRead,
	}))

	assert.ElementsMatch(t,
		[]string{first.ID.String(), second.ID.String()},
		recorder.conversationsNamed())
}

// WhatsApp does not send a receipt twice, and a receipt for a message we never
// stored changes nothing — announcing it would wake every open browser for a
// thread that did not move.
func TestReceiptSaysNothingWhenNothingMoved(t *testing.T) {
	_, messages, _, _ := drainSetup(t)
	recorder := &recordingAnnouncer{}
	receipts := &receiptHandler{messages: messages, publisher: recorder}

	require.NoError(t, receipts.handle(context.Background(), &events.Receipt{
		MessageIDs: []string{"never-stored"},
		Type:       types.ReceiptTypeRead,
	}))

	assert.Empty(t, recorder.conversationsNamed())
}

// The drain paces sends apart on purpose, so a reply that has actually gone
// must say so as it goes rather than at the end of the batch — otherwise the
// first reply of a busy queue shows a clock long after the customer has it.
func TestDrainAnnouncesEachReplyAsItGoes(t *testing.T) {
	_, messages, conversations, conv := drainSetup(t)
	recorder := &recordingAnnouncer{}
	sender := &fakeSender{}

	for _, body := range []string{"pertama", "kedua"} {
		_, err := messages.Queue(conv.ID, uuid.New(), models.MessageKindText, body, nil, nil)
		require.NoError(t, err)
	}

	n, err := NewDrainer(conv.WAAccountID, messages, conversations, sender,
		recorder, t.TempDir(), 0).Drain(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, 2, n)

	assert.Equal(t,
		[]string{conv.ID.String(), conv.ID.String()},
		recorder.conversationsNamed(),
		"one announcement per reply, each naming its thread")
}

// A refused send is a status change too: the reply flips to failed, and a CS
// watching the thread has to see that without reloading.
func TestDrainAnnouncesAFailedReply(t *testing.T) {
	_, messages, conversations, conv := drainSetup(t)
	recorder := &recordingAnnouncer{}
	sender := &fakeSender{err: errors.New("nomor tidak terdaftar di WhatsApp")}

	_, err := messages.Queue(conv.ID, uuid.New(), models.MessageKindText, "halo", nil, nil)
	require.NoError(t, err)

	n, err := NewDrainer(conv.WAAccountID, messages, conversations, sender,
		recorder, t.TempDir(), 0).Drain(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, 0, n)

	assert.Equal(t, []string{conv.ID.String()}, recorder.conversationsNamed())
}
