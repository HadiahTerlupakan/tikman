package api

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

func adminChannel(t *testing.T, env *csHandlerEnv) models.WAChannel {
	t.Helper()
	row := models.WAChannel{
		WAAccountID: env.account.ID,
		JID:         "120363000000000001@newsletter",
		Name:        "Info Gangguan",
		Role:        models.ChannelRoleOwner,
	}
	require.NoError(t, env.db.Create(&row).Error)
	return row
}

// Broadcasting is open to everyone who can open the inbox — deliberately
// looser than quick replies and number management beside it. Viewer stays out
// because the whole /cs group is closed to that role.
func TestEveryInboxRoleMayPostAnUpdate(t *testing.T) {
	env := setupCSHandler(t)
	channel := adminChannel(t, env)

	for role, want := range map[models.UserRole]int{
		models.UserRoleAdmin:      http.StatusCreated,
		models.UserRoleCS:         http.StatusCreated,
		models.UserRoleTechnician: http.StatusCreated,
		models.UserRoleViewer:     http.StatusForbidden,
	} {
		body := `{"body":"Ada pemeliharaan malam ini","destinations":[` +
			`{"type":"channel","channel_id":"` + channel.ID.String() + `"}]}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/cs/broadcasts", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		env.asUser(env.cs, role).ServeHTTP(rec, req)
		assert.Equal(t, want, rec.Code, string(role))
	}
}

// A status accepts text, image and video — never a document. Refusing at the
// API is what makes the UI's disabled checkbox a guarantee rather than a
// suggestion, and it must refuse before the upload is stored so no orphaned
// file is left behind.
func TestADocumentIsRefusedForAStatusDestination(t *testing.T) {
	env := setupCSHandler(t)

	req := uploadRequest(t,
		"/api/v1/cs/broadcasts/media?status_account_id="+env.account.ID.String(),
		"application/pdf", 32)
	rec := httptest.NewRecorder()
	env.asUser(env.cs, models.UserRoleCS).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var stored []models.WABroadcastPost
	require.NoError(t, env.db.Find(&stored).Error)
	assert.Empty(t, stored)

	left, err := filepath.Glob(filepath.Join(env.mediaRoot, "*", "*", "*"))
	require.NoError(t, err)
	assert.Empty(t, left, "a refused upload must leave no file behind")
}

// An announcement with nowhere to go is a mistake, not an empty success.
func TestABroadcastWithNoDestinationIsRefused(t *testing.T) {
	env := setupCSHandler(t)

	body := `{"body":"Ada pemeliharaan","destinations":[]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cs/broadcasts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.asUser(env.cs, models.UserRoleCS).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// One request, two destinations, two rows — that is what lets a partial
// failure be readable afterwards.
func TestOneRequestToBothDestinationsWritesTwoRows(t *testing.T) {
	env := setupCSHandler(t)
	channel := adminChannel(t, env)

	body := `{"body":"Ada pemeliharaan malam ini","destinations":[` +
		`{"type":"channel","channel_id":"` + channel.ID.String() + `"},` +
		`{"type":"status","wa_account_id":"` + env.account.ID.String() + `"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cs/broadcasts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.asUser(env.cs, models.UserRoleCS).ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	var stored []models.WABroadcastPost
	require.NoError(t, env.db.Order("destination").Find(&stored).Error)
	require.Len(t, stored, 2)
	assert.Equal(t, models.DestinationChannel, stored[0].Destination)
	require.NotNil(t, stored[0].DestinationJID)
	assert.Equal(t, channel.JID, *stored[0].DestinationJID)
	assert.Equal(t, models.DestinationStatus, stored[1].Destination)
	assert.Nil(t, stored[1].DestinationJID)
}

// A channel the mirror no longer lists is refused before anything is queued —
// including the status row that shared the request.
func TestAnUnknownChannelRefusesTheWholeRequest(t *testing.T) {
	env := setupCSHandler(t)

	body := `{"body":"Ada pemeliharaan","destinations":[` +
		`{"type":"channel","channel_id":"` + uuid.New().String() + `"},` +
		`{"type":"status","wa_account_id":"` + env.account.ID.String() + `"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cs/broadcasts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.asUser(env.cs, models.UserRoleCS).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)

	var stored []models.WABroadcastPost
	require.NoError(t, env.db.Find(&stored).Error)
	assert.Empty(t, stored, "no row may be written when any destination is refused")
}
