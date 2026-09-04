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

// An empty update is nothing to announce, and WhatsApp would refuse it several
// seconds later where the sender can no longer see the form.
func TestAnEmptyUpdateIsRefused(t *testing.T) {
	env := setupCSHandler(t)
	channel := adminChannel(t, env)

	body := `{"body":"   ","destinations":[` +
		`{"type":"channel","channel_id":"` + channel.ID.String() + `"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cs/broadcasts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.asUser(env.cs, models.UserRoleCS).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// A database that will not answer is not "that channel is gone". Reporting it
// as 404 tells a sender their admin right was revoked and sends them chasing
// one they still have, while the real fault goes unlogged.
func TestAChannelLookupThatFailsIsNotReportedAsMissing(t *testing.T) {
	env := setupCSHandler(t)
	channel := adminChannel(t, env)
	require.NoError(t, env.db.Migrator().DropTable(&models.WAChannel{}))

	body := `{"body":"Ada pemeliharaan","destinations":[` +
		`{"type":"channel","channel_id":"` + channel.ID.String() + `"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cs/broadcasts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.asUser(env.cs, models.UserRoleCS).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// The upload allowlist is the same one chat attachments go through, and it
// deliberately excludes the types ServeMedia would hand back from the API's
// own origin. This is a different refusal from the status-only "no
// documents" rule TestADocumentIsRefusedForAStatusDestination covers: this
// one applies to every destination, and rejects far more than documents.
func TestAnUnacceptedAttachmentTypeIsRefused(t *testing.T) {
	env := setupCSHandler(t)
	channel := adminChannel(t, env)

	req := uploadRequest(t,
		"/api/v1/cs/broadcasts/media?channel_id="+channel.ID.String(), "text/html", 32)
	rec := httptest.NewRecorder()
	env.asUser(env.cs, models.UserRoleCS).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// The media handler's success path: only its MIME refusal was ever exercised,
// and this is where the file on disk and the row that names it have to agree —
// the drainer reads exactly what storeUpload wrote, or the post fails on the
// wa side with nothing the sender can act on.
func TestAQueuedMediaUpdateKeepsTheFileItNames(t *testing.T) {
	env := setupCSHandler(t)
	channel := adminChannel(t, env)

	req := uploadRequest(t,
		"/api/v1/cs/broadcasts/media?channel_id="+channel.ID.String(), "image/jpeg", 32)
	rec := httptest.NewRecorder()
	env.asUser(env.cs, models.UserRoleCS).ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	var stored []models.WABroadcastPost
	require.NoError(t, env.db.Find(&stored).Error)
	require.Len(t, stored, 1)
	assert.Equal(t, models.DestinationChannel, stored[0].Destination)
	require.NotNil(t, stored[0].DestinationJID)
	assert.Equal(t, channel.JID, *stored[0].DestinationJID)
	assert.Equal(t, env.account.ID, stored[0].WAAccountID)
	assert.Equal(t, models.BroadcastQueued, stored[0].Status)
	assert.Equal(t, models.MessageKindImage, stored[0].Kind)
	assert.Equal(t, "image/jpeg", stored[0].MediaMime)
	assert.Equal(t, "foto.jpg", stored[0].MediaFilename)
	assert.Equal(t, int64(32), stored[0].MediaSize)

	require.NotEmpty(t, stored[0].MediaPath)
	assert.FileExists(t, filepath.Join(env.mediaRoot, stored[0].MediaPath),
		"the row points at the file that was actually written")
}
