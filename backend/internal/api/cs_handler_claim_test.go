package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

// Opening a thread is not taking it: the first CS to answer is the one who
// serves the customer, and the handover is audited like any other.
func TestSendToAThreadNobodyHoldsMakesTheSenderItsHolder(t *testing.T) {
	env := setupCSHandler(t)
	conv := env.conversation(t, "628111@s.whatsapp.net", "628111222333")

	rec := sendReply(t, env, conv, `{"body":"selamat pagi, ada yang bisa dibantu?"}`)

	require.Equal(t, http.StatusCreated, rec.Code)
	after, err := env.conversations.Get(conv.ID)
	require.NoError(t, err)
	require.NotNil(t, after.AssignedUserID)
	assert.Equal(t, env.cs, *after.AssignedUserID)
	assert.Equal(t, models.ConversationOpen, after.Status)

	var handover models.AuditLog
	require.NoError(t, env.db.Where("resource_id = ? AND action = ?", conv.ID, "assign").First(&handover).Error)
	assert.Contains(t, string(handover.NewValue), env.cs.String())
}

func TestSendMediaToAThreadNobodyHoldsMakesTheSenderItsHolder(t *testing.T) {
	env := setupCSHandler(t)
	conv := env.conversation(t, "628111@s.whatsapp.net", "628111222333")

	req := uploadRequest(t, "/api/v1/cs/conversations/"+conv.ID.String()+"/media", "image/jpeg", 1024)
	rec := httptest.NewRecorder()
	env.asUser(env.cs, models.UserRoleCS).ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
	after, err := env.conversations.Get(conv.ID)
	require.NoError(t, err)
	require.NotNil(t, after.AssignedUserID)
	assert.Equal(t, env.cs, *after.AssignedUserID)
}
