package api

import (
	"encoding/json"
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

func sendReply(t *testing.T, env *csHandlerEnv, conv *models.CSConversation, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost,
		"/api/v1/cs/conversations/"+conv.ID.String()+"/messages", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.asUser(env.cs, models.UserRoleCS).ServeHTTP(rec, req)
	return rec
}

func TestSendStoresTheMessageAReplyQuotes(t *testing.T) {
	env := setupCSHandler(t)

	conv := env.conversation(t, "628111@s.whatsapp.net", "628111222333")
	require.NoError(t, env.conversations.Assign(conv.ID, env.cs))

	asked, _, err := env.messages.SaveInbound(services.InboundMessage{
		ConversationID: conv.ID, WAMessageID: "3EB0ASKED",
		Kind: models.MessageKindText, Body: "internet saya mati", At: time.Now(),
	})
	require.NoError(t, err)

	rec := sendReply(t, env, conv, `{"body":"sudah kami cek","reply_to_id":"`+asked.ID.String()+`"}`)
	require.Equal(t, http.StatusCreated, rec.Code)

	var answer struct {
		Data models.CSMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &answer))
	require.NotNil(t, answer.Data.ReplyToID)
	assert.Equal(t, asked.ID, *answer.Data.ReplyToID)

	// The quoted block comes back on the reply itself, so the thread can draw
	// it without waiting for the next history fetch.
	require.NotNil(t, answer.Data.ReplyTo)
	assert.Equal(t, "internet saya mati", answer.Data.ReplyTo.Body)
}

// Quoting reaches across a thread, never between them. Without this check a CS
// could put another customer's words on this customer's screen.
func TestSendRefusesAQuoteFromAnotherThread(t *testing.T) {
	env := setupCSHandler(t)

	mine := env.conversation(t, "628111@s.whatsapp.net", "628111222333")
	require.NoError(t, env.conversations.Assign(mine.ID, env.cs))
	other := env.conversation(t, "628999@s.whatsapp.net", "628999888777")

	theirs, _, err := env.messages.SaveInbound(services.InboundMessage{
		ConversationID: other.ID, WAMessageID: "3EB0THEIRS",
		Kind: models.MessageKindText, Body: "punya orang lain", At: time.Now(),
	})
	require.NoError(t, err)

	rec := sendReply(t, env, mine, `{"body":"sudah kami cek","reply_to_id":"`+theirs.ID.String()+`"}`)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "INVALID_REPLY_TO")

	history, err := env.messages.History(mine.ID, 10, 0)
	require.NoError(t, err)
	assert.Empty(t, history, "a refused reply must not be queued")
}

// A reply still in the outbox has no WhatsApp id, so quoting it would reach the
// customer's phone as an empty grey box.
func TestSendRefusesAQuoteOfAReplyThatHasNotBeenSent(t *testing.T) {
	env := setupCSHandler(t)

	conv := env.conversation(t, "628111@s.whatsapp.net", "628111222333")
	require.NoError(t, env.conversations.Assign(conv.ID, env.cs))

	waiting, err := env.messages.Queue(conv.ID, env.cs, models.MessageKindText, "menunggu", nil, nil)
	require.NoError(t, err)

	rec := sendReply(t, env, conv, `{"body":"sudah kami cek","reply_to_id":"`+waiting.ID.String()+`"}`)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "belum terkirim")
}

func TestSendRefusesAReplyToIdThatIsNotAnID(t *testing.T) {
	env := setupCSHandler(t)

	conv := env.conversation(t, "628111@s.whatsapp.net", "628111222333")
	require.NoError(t, env.conversations.Assign(conv.ID, env.cs))

	rec := sendReply(t, env, conv, `{"body":"sudah kami cek","reply_to_id":"bukan-uuid"}`)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "INVALID_REPLY_TO_ID")
}

func TestSendWithoutAQuoteStoresNone(t *testing.T) {
	env := setupCSHandler(t)

	conv := env.conversation(t, "628111@s.whatsapp.net", "628111222333")
	require.NoError(t, env.conversations.Assign(conv.ID, env.cs))

	rec := sendReply(t, env, conv, `{"body":"halo"}`)
	require.Equal(t, http.StatusCreated, rec.Code)

	var answer struct {
		Data models.CSMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &answer))
	assert.Nil(t, answer.Data.ReplyToID)
	assert.Nil(t, answer.Data.ReplyTo)
}
