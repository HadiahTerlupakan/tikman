package wa

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

func customerTyping(chat string, state types.ChatPresence) *events.ChatPresence {
	jid := types.JID{User: chat, Server: types.DefaultUserServer}
	return &events.ChatPresence{
		MessageSource: types.MessageSource{Chat: jid, Sender: jid},
		State:         state,
	}
}

func presenceSetup(t *testing.T) (*presenceHandler, *recordingAnnouncer, *models.CSConversation) {
	t.Helper()
	db, _, conversations, conv := drainSetup(t)

	var account models.WAAccount
	require.NoError(t, db.First(&account).Error)

	recorder := &recordingAnnouncer{}
	return &presenceHandler{
		accountID:     account.ID,
		conversations: conversations,
		publisher:     recorder,
	}, recorder, conv
}

func TestACustomerWritingIsAnnouncedAgainstTheirThread(t *testing.T) {
	handler, recorder, conv := presenceSetup(t)

	require.NoError(t, handler.handle(context.Background(),
		customerTyping("628111", types.ChatPresenceComposing)))

	require.Len(t, recorder.events, 1)
	assert.Equal(t, EventTyping, recorder.events[0].Type)
	assert.Equal(t, conv.ID.String(), recorder.events[0].ConversationID)
	assert.True(t, recorder.events[0].Typing)
}

// Stopping has to travel too, or the line stays up until a timer somewhere
// happens to clear it.
func TestACustomerWhoStopsWritingIsAnnouncedAsWell(t *testing.T) {
	handler, recorder, conv := presenceSetup(t)

	require.NoError(t, handler.handle(context.Background(),
		customerTyping("628111", types.ChatPresencePaused)))

	require.Len(t, recorder.events, 1)
	assert.Equal(t, conv.ID.String(), recorder.events[0].ConversationID)
	assert.False(t, recorder.events[0].Typing)
}

// Presence arrives for people who are typing, not for people who have written.
// Creating a thread here would fill the inbox with rows holding nothing to
// answer — one per stranger who opened the chat and thought better of it.
func TestAStrangerTypingCreatesNoThread(t *testing.T) {
	handler, recorder, _ := presenceSetup(t)

	require.NoError(t, handler.handle(context.Background(),
		customerTyping("628999", types.ChatPresenceComposing)))

	assert.Empty(t, recorder.events)

	conv, err := handler.conversations.FindByPeer(handler.accountID, "628999@s.whatsapp.net")
	require.NoError(t, err)
	assert.Nil(t, conv, "no thread was invented for a customer who has said nothing")
}

// The echo of our own typing, and any group this number is in, are not a
// customer writing to the inbox.
func TestOurOwnTypingIsNotAnnounced(t *testing.T) {
	handler, recorder, _ := presenceSetup(t)

	evt := customerTyping("628111", types.ChatPresenceComposing)
	evt.IsFromMe = true
	require.NoError(t, handler.handle(context.Background(), evt))

	assert.Empty(t, recorder.events)
}
