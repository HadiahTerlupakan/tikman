package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

func setTyping(t *testing.T, env *csHandlerEnv, conv *models.CSConversation, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost,
		"/api/v1/cs/conversations/"+conv.ID.String()+"/typing", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.asUser(env.cs, models.UserRoleCS).ServeHTTP(rec, req)
	return rec
}

func TestTheHolderCanRaiseAndClearTheTypingLine(t *testing.T) {
	env := setupCSHandler(t)
	conv := env.conversation(t, "628111@s.whatsapp.net", "628111222333")
	require.NoError(t, env.conversations.Assign(conv.ID, env.cs))

	for _, body := range []string{`{"typing":true}`, `{"typing":false}`} {
		rec := setTyping(t, env, conv, body)
		assert.Equal(t, http.StatusNoContent, rec.Code, body)
	}
}

// Anyone with the inbox open can read along. A customer watching two colleagues
// browse past would see somebody typing who was never going to answer.
func TestOnlyTheHolderMayShowAsTyping(t *testing.T) {
	env := setupCSHandler(t)
	conv := env.conversation(t, "628111@s.whatsapp.net", "628111222333")

	rec := setTyping(t, env, conv, `{"typing":true}`)
	assert.Equal(t, http.StatusConflict, rec.Code)
}
