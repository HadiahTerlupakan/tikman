package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
)

// unreadThread returns one conversation carrying a single unread inbound
// message, which is the only thing in the module that raises the count.
func unreadThread(t *testing.T, env *csHandlerEnv, waMessageID string) *models.CSConversation {
	t.Helper()

	conv := env.conversation(t, "628111@s.whatsapp.net", "628111222333")
	_, created, err := env.messages.SaveInbound(services.InboundMessage{
		ConversationID: conv.ID,
		WAMessageID:    waMessageID,
		Kind:           models.MessageKindText,
		Body:           "halo",
		At:             time.Now(),
	})
	require.NoError(t, err)
	require.True(t, created)

	before, err := env.conversations.Get(conv.ID)
	require.NoError(t, err)
	require.Equal(t, 1, before.UnreadCount)
	return conv
}

func (e *csHandlerEnv) openThread(t *testing.T, conv *models.CSConversation) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/cs/conversations/"+conv.ID.String()+"/messages", nil)
	rec := httptest.NewRecorder()
	e.asUser(e.cs, models.UserRoleCS).ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
}

// Every inbound message raises unread_count and nothing used to lower it, so
// a long-running thread showed a lifetime counter and the badge stopped
// meaning anything.
func TestOpeningAThreadClearsItsUnreadBadge(t *testing.T) {
	env := setupCSHandler(t)
	conv := unreadThread(t, env, "wamid.unread.1")

	env.openThread(t, conv)

	after, err := env.conversations.Get(conv.ID)
	require.NoError(t, err)
	assert.Zero(t, after.UnreadCount)
}

// A message that arrives after the CS read the thread has to raise the badge
// again — clearing must not be a one-way switch on the row.
func TestAMessageAfterAReadRaisesTheBadgeAgain(t *testing.T) {
	env := setupCSHandler(t)
	conv := unreadThread(t, env, "wamid.unread.2")
	env.openThread(t, conv)

	_, created, err := env.messages.SaveInbound(services.InboundMessage{
		ConversationID: conv.ID,
		WAMessageID:    "wamid.unread.3",
		Kind:           models.MessageKindText,
		Body:           "halo lagi",
		At:             time.Now(),
	})
	require.NoError(t, err)
	require.True(t, created)

	after, err := env.conversations.Get(conv.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, after.UnreadCount)
}
