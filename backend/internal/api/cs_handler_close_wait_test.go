package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
)

// Closing through the inbox is how a thread ends without a reply, and the
// report names whoever did it.
func TestClosingAThreadThroughTheInboxRecordsWhoClosedIt(t *testing.T) {
	env := setupCSHandler(t)
	conv, err := env.conversations.FindOrCreate(services.IncomingPeer{
		WAAccountID: env.account.ID, JID: "628123456789@s.whatsapp.net",
		Phone: "628123456789", Name: "Pak Budi",
	})
	require.NoError(t, err)
	_, _, err = env.messages.SaveInbound(services.InboundMessage{
		ConversationID: conv.ID, WAMessageID: "3EB0CLOSE",
		Kind: models.MessageKindText, Body: "halo", At: time.Now(),
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPut,
		"/api/v1/cs/conversations/"+conv.ID.String()+"/status",
		strings.NewReader(`{"status":"closed"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.asUser(env.cs, models.UserRoleCS).ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var wait models.CSWait
	require.NoError(t, env.db.First(&wait, "conversation_id = ?", conv.ID).Error)
	require.NotNil(t, wait.EndReason)
	assert.Equal(t, models.WaitClosed, *wait.EndReason)
	assert.Equal(t, &env.cs, wait.EndedBy)
}
