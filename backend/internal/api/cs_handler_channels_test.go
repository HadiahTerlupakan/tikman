package api

import (
	"net/http"
	"net/http/httptest"
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
		body := `{"channel_id":"` + channel.ID.String() + `","body":"Ada pemeliharaan malam ini"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/cs/channel-posts", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		env.asUser(env.cs, role).ServeHTTP(rec, req)
		assert.Equal(t, want, rec.Code, string(role))
	}
}

// A channel the mirror no longer lists is a channel this number may no longer
// post to. Refusing at the API means nothing is queued that could only ever
// fail later.
func TestPostingToAnUnknownChannelIsRefused(t *testing.T) {
	env := setupCSHandler(t)

	body := `{"channel_id":"` + uuid.New().String() + `","body":"Ada pemeliharaan"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cs/channel-posts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.asUser(env.cs, models.UserRoleCS).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// An empty update is nothing to announce, and WhatsApp would refuse it several
// seconds later where the sender can no longer see the form.
func TestAnEmptyUpdateIsRefused(t *testing.T) {
	env := setupCSHandler(t)
	channel := adminChannel(t, env)

	body := `{"channel_id":"` + channel.ID.String() + `","body":"   "}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cs/channel-posts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.asUser(env.cs, models.UserRoleCS).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// The queued row must carry the JID and the number, not the picked row id: the
// mirror row it came from is deleted and recreated on every sync.
func TestAQueuedUpdateCarriesTheChannelAndItsNumber(t *testing.T) {
	env := setupCSHandler(t)
	channel := adminChannel(t, env)

	body := `{"channel_id":"` + channel.ID.String() + `","body":"Ada pemeliharaan malam ini"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cs/channel-posts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.asUser(env.cs, models.UserRoleCS).ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	var stored []models.WAChannelPost
	require.NoError(t, env.db.Find(&stored).Error)
	require.Len(t, stored, 1)
	assert.Equal(t, channel.JID, stored[0].ChannelJID)
	assert.Equal(t, env.account.ID, stored[0].WAAccountID)
	assert.Equal(t, models.ChannelPostQueued, stored[0].Status)
}

// The upload allowlist is the same one chat attachments go through, and it
// deliberately excludes the types ServeMedia would hand back from the API's
// own origin.
func TestAnUnacceptedAttachmentTypeIsRefused(t *testing.T) {
	env := setupCSHandler(t)
	channel := adminChannel(t, env)

	req := uploadRequest(t,
		"/api/v1/cs/channel-posts/media?channel_id="+channel.ID.String(), "text/html", 32)
	rec := httptest.NewRecorder()
	env.asUser(env.cs, models.UserRoleCS).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
