package wa

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
)

// twoNumbers gives two CS numbers, each with its own customer thread.
func twoNumbers(t *testing.T) (
	*services.CSMessageService, *services.CSConversationService,
	*models.CSConversation, *models.CSConversation,
) {
	t.Helper()
	db, messages, conversations, first := drainSetup(t)

	second := models.WAAccount{Label: "CS Kedua", Status: models.WAAccountConnected}
	require.NoError(t, db.Create(&second).Error)

	other, err := conversations.FindOrCreate(services.IncomingPeer{
		WAAccountID: second.ID, JID: "628999@s.whatsapp.net",
		Phone: "628999888777", Name: "Siti",
	})
	require.NoError(t, err)
	return messages, conversations, first, other
}

// The whole point of scoping the outbox. A customer who wrote to one number
// must be answered from that number: a reply arriving from a number they have
// never messaged reads as a stranger, and they cannot reply to it in the
// thread they are looking at.
//
// Unscoped, both sessions claimed every queued row and whichever drained first
// sent it — from its own number, silently.
func TestASessionSendsOnlyTheRepliesForItsOwnNumber(t *testing.T) {
	messages, conversations, mine, theirs := twoNumbers(t)

	_, err := messages.Queue(mine.ID, uuid.New(), models.MessageKindText, "balasan nomor satu", nil, nil)
	require.NoError(t, err)
	_, err = messages.Queue(theirs.ID, uuid.New(), models.MessageKindText, "balasan nomor dua", nil, nil)
	require.NoError(t, err)

	sender := &fakeSender{}
	sent, err := NewDrainer(mine.WAAccountID, messages, conversations, sender, t.TempDir(), 0).
		Drain(context.Background(), 10)
	require.NoError(t, err)

	assert.Equal(t, 1, sent)
	assert.Equal(t, []string{"balasan nomor satu"}, sender.sent,
		"the other number's reply must be left for the session that holds it")
}

// And the other number's reply is not stranded: its own session still takes it.
func TestTheOtherNumbersSessionStillSendsItsOwnReply(t *testing.T) {
	messages, conversations, mine, theirs := twoNumbers(t)

	_, err := messages.Queue(mine.ID, uuid.New(), models.MessageKindText, "balasan nomor satu", nil, nil)
	require.NoError(t, err)
	_, err = messages.Queue(theirs.ID, uuid.New(), models.MessageKindText, "balasan nomor dua", nil, nil)
	require.NoError(t, err)

	sender := &fakeSender{}
	_, err = NewDrainer(theirs.WAAccountID, messages, conversations, sender, t.TempDir(), 0).
		Drain(context.Background(), 10)
	require.NoError(t, err)

	assert.Equal(t, []string{"balasan nomor dua"}, sender.sent)
}

// A session can only ask WhatsApp about people who wrote to the number it
// holds. Asking about another number's customers would put the question over
// the wrong connection, about somebody that connection has never met.
func TestAvatarSweepOnlyAsksAboutItsOwnNumbersCustomers(t *testing.T) {
	_, conversations, mine, theirs := twoNumbers(t)

	source := &fakePictures{answers: map[string]Picture{
		mine.CustomerJID:   {State: PictureNone},
		theirs.CustomerJID: {State: PictureNone},
	}}
	_, err := NewAvatarSweeper(mine.WAAccountID, conversations, source, t.TempDir(), 0, 0).
		Sweep(context.Background(), 10)
	require.NoError(t, err)

	assert.Equal(t, []string{mine.CustomerJID}, source.asked)
}
