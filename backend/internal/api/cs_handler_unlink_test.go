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

func linkONT(t *testing.T, env *csHandlerEnv, convID, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPut,
		"/api/v1/cs/conversations/"+convID+"/ont", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.asUser(env.cs, models.UserRoleCS).ServeHTTP(rec, req)
	return rec
}

// Linking writes the customer's number onto the ONT. A CS who linked the wrong
// one must be able to take it back, or every later chat from that customer
// matches the same wrong ONT and the correction only looks done.
func TestUnlinkingTakesTheCustomersNumberBackOffTheONT(t *testing.T) {
	env := setupCSHandler(t)
	conv := env.conversation(t, "628111@s.whatsapp.net", "628111222333")
	ont := env.ont(t, "")

	require.Equal(t, http.StatusOK,
		linkONT(t, env, conv.ID.String(), `{"ont_id":"`+ont.ID.String()+`"}`).Code)

	linked, err := env.onts.GetByID(ont.ID)
	require.NoError(t, err)
	require.Equal(t, "628111222333", linked.Phone)

	require.Equal(t, http.StatusOK, linkONT(t, env, conv.ID.String(), `{"ont_id":null}`).Code)

	after, err := env.onts.GetByID(ont.ID)
	require.NoError(t, err)
	assert.Empty(t, after.Phone)

	conversation, err := env.conversations.Get(conv.ID)
	require.NoError(t, err)
	assert.Nil(t, conversation.ONTID)
}

// Moving a link is the same correction in one step. The number has to follow,
// or it is left on the ONT the CS just decided was wrong.
func TestRelinkingMovesTheNumberToTheNewONT(t *testing.T) {
	env := setupCSHandler(t)
	conv := env.conversation(t, "628111@s.whatsapp.net", "628111222333")
	wrong := env.ont(t, "")
	right := env.ont(t, "")

	require.Equal(t, http.StatusOK,
		linkONT(t, env, conv.ID.String(), `{"ont_id":"`+wrong.ID.String()+`"}`).Code)
	require.Equal(t, http.StatusOK,
		linkONT(t, env, conv.ID.String(), `{"ont_id":"`+right.ID.String()+`"}`).Code)

	left, err := env.onts.GetByID(wrong.ID)
	require.NoError(t, err)
	assert.Empty(t, left.Phone, "the number must not stay on the ONT that was corrected away")

	moved, err := env.onts.GetByID(right.ID)
	require.NoError(t, err)
	assert.Equal(t, "628111222333", moved.Phone)
}

// A number the operator entered for a different subscriber is theirs, not this
// thread's to erase.
func TestUnlinkingLeavesANumberThatBelongsToSomebodyElse(t *testing.T) {
	env := setupCSHandler(t)
	conv := env.conversation(t, "628111@s.whatsapp.net", "628111222333")
	ont := env.ont(t, "628999888777")

	require.Equal(t, http.StatusOK,
		linkONT(t, env, conv.ID.String(), `{"ont_id":"`+ont.ID.String()+`"}`).Code)
	require.Equal(t, http.StatusOK, linkONT(t, env, conv.ID.String(), `{"ont_id":null}`).Code)

	after, err := env.onts.GetByID(ont.ID)
	require.NoError(t, err)
	assert.Equal(t, "628999888777", after.Phone)
}
