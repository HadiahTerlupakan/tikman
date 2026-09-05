package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

func getOnline(t *testing.T, env *csHandlerEnv, as uuid.UUID, role models.UserRole) (int, []uuid.UUID) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/cs/online", nil)
	rec := httptest.NewRecorder()
	env.asUser(as, role).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		return rec.Code, nil
	}
	var body struct {
		Data []uuid.UUID `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return rec.Code, body.Data
}

// Presence was written on every heartbeat and read only by the round-robin.
// Nothing could tell a CS who else was at their desk.
func TestOnlineListsTheAgentsPresenceHolds(t *testing.T) {
	env := setupCSHandler(t)
	first, second := uuid.New(), uuid.New()
	require.NoError(t, env.presence.MarkOnline(t.Context(), first))
	require.NoError(t, env.presence.MarkOnline(t.Context(), second))

	code, online := getOnline(t, env, env.cs, models.UserRoleCS)

	require.Equal(t, http.StatusOK, code)
	assert.ElementsMatch(t, []uuid.UUID{first, second}, online)
}

// An empty inbox is a normal state, not an error, and the browser renders a
// list either way — so it has to be [] and never null.
func TestOnlineAnswersAnEmptyListWhenNobodyIsWatching(t *testing.T) {
	env := setupCSHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cs/online", nil)
	rec := httptest.NewRecorder()
	env.asUser(env.cs, models.UserRoleCS).ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"data":[]}`, rec.Body.String())
}

// The same gate as the rest of the inbox: a Viewer cannot open CS at all, so
// it may not learn who is staffing it either.
func TestOnlineIsClosedToRolesThatCannotOpenTheInbox(t *testing.T) {
	env := setupCSHandler(t)

	code, _ := getOnline(t, env, uuid.New(), models.UserRoleViewer)

	assert.Equal(t, http.StatusForbidden, code)
}
