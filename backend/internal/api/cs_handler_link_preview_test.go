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

func previewFor(t *testing.T, env *csHandlerEnv, text string, role models.UserRole) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/cs/link-preview?text="+text, nil)
	rec := httptest.NewRecorder()
	env.asUser(env.cs, role).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		return rec.Code, nil
	}
	var body struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return rec.Code, body.Data
}

// Typing has no link in it most of the time, and that must cost nothing and
// reach no network.
func TestLinkPreviewAnswersEmptyForTextWithoutALink(t *testing.T) {
	env := setupCSHandler(t)

	code, data := previewFor(t, env, "halo+pak+sudah+dicek", models.UserRoleCS)

	assert.Equal(t, http.StatusOK, code)
	assert.Nil(t, data)
}

// api carries wg0 and can reach every OLT. A CS typing a plant address must
// get nothing back rather than a probe of their own network.
func TestLinkPreviewRefusesTheOperatorsOwnNetwork(t *testing.T) {
	env := setupCSHandler(t)

	for _, target := range []string{
		"http%3A%2F%2F172.30.30.3%2F",
		"http%3A%2F%2F192.168.220.22%2F",
		"http%3A%2F%2F10.88.0.1%2F",
		"http%3A%2F%2F169.254.169.254%2Flatest%2Fmeta-data%2F",
		"http%3A%2F%2F127.0.0.1%3A8080%2Fadmin",
	} {
		code, data := previewFor(t, env, "cek+"+target, models.UserRoleCS)
		assert.Equal(t, http.StatusOK, code, target)
		assert.Nil(t, data, "%s must resolve to nothing", target)
	}
}

// The same gate as the rest of the inbox: a Viewer cannot open CS, so it may
// not make the server fetch URLs either.
func TestLinkPreviewIsClosedToRolesThatCannotOpenTheInbox(t *testing.T) {
	env := setupCSHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cs/link-preview?text=x", nil)
	rec := httptest.NewRecorder()
	env.asUser(uuid.New(), models.UserRoleViewer).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}
