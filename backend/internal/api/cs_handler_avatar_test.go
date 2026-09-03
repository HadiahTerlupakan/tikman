package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
)

func avatarRequest(t *testing.T, env *csHandlerEnv, convID string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/cs/conversations/"+convID+"/avatar", nil)
	rec := httptest.NewRecorder()
	env.asUser(env.cs, models.UserRoleCS).ServeHTTP(rec, req)
	return rec
}

func TestServeAvatarAnswersThePhotoAsAnImage(t *testing.T) {
	env := setupCSHandler(t)
	conv := env.conversation(t, "628111@s.whatsapp.net", "628111222333")

	rel := filepath.Join("avatars", "face.jpg")
	require.NoError(t, os.MkdirAll(filepath.Join(env.handler.mediaRoot, "avatars"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(env.handler.mediaRoot, rel), []byte("jpegbytes"), 0o640))
	_, err := env.conversations.SetAvatar(conv.ID, "PIC1", rel)
	require.NoError(t, err)

	rec := avatarRequest(t, env, conv.ID.String())

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "image/jpeg", rec.Header().Get("Content-Type"))
	assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "jpegbytes", rec.Body.String())
	// Drawn as a face in a list, so no attachment disposition — but then the
	// sandbox is what stands between a stored file and the API's own origin.
	assert.Contains(t, rec.Header().Get("Content-Security-Policy"), "sandbox")
}

func TestServeAvatarAnswers404WhenThereIsNoPhoto(t *testing.T) {
	env := setupCSHandler(t)
	conv := env.conversation(t, "628111@s.whatsapp.net", "628111222333")

	rec := avatarRequest(t, env, conv.ID.String())

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "AVATAR_NOT_FOUND")
}

// AvatarPath comes out of a database row and is never validated on the way in,
// so a corrupted or tampered value must still not become a read outside the
// media root.
func TestServeAvatarRefusesAPathThatEscapesTheMediaRoot(t *testing.T) {
	env := setupCSHandler(t)
	conv := env.conversation(t, "628111@s.whatsapp.net", "628111222333")

	_, err := env.conversations.SetAvatar(conv.ID, "PIC1", "../../../../../../../../etc/hosts")
	require.NoError(t, err)

	rec := avatarRequest(t, env, conv.ID.String())

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.NotContains(t, rec.Body.String(), "localhost")
}

// The extension is written by the sweeper from a narrow allowlist, so one that
// is not on it means the row is not something this inbox stored.
func TestServeAvatarRefusesAStoredPathWithATypeItDoesNotServe(t *testing.T) {
	env := setupCSHandler(t)
	conv := env.conversation(t, "628111@s.whatsapp.net", "628111222333")

	rel := filepath.Join("avatars", "face.svg")
	require.NoError(t, os.MkdirAll(filepath.Join(env.handler.mediaRoot, "avatars"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(env.handler.mediaRoot, rel), []byte("<svg onload='alert(1)'/>"), 0o640))
	_, err := env.conversations.SetAvatar(conv.ID, "PIC1", rel)
	require.NoError(t, err)

	rec := avatarRequest(t, env, conv.ID.String())

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.NotContains(t, rec.Body.String(), "<svg")
}

// The list tells a browser which rows have a photo, so it does not point every
// avatar at an endpoint that will 404 on most of them.
func TestConversationListSaysWhichThreadsHaveAPhoto(t *testing.T) {
	env := setupCSHandler(t)
	withPhoto := env.conversation(t, "628111@s.whatsapp.net", "628111222333")
	env.conversation(t, "628999@s.whatsapp.net", "628999888777")

	_, err := env.conversations.SetAvatar(withPhoto.ID, "PIC1", filepath.Join("avatars", "face.jpg"))
	require.NoError(t, err)

	rows, err := env.conversations.List(services.ConversationFilter{})
	require.NoError(t, err)
	require.Len(t, rows, 2)

	found := map[string]bool{}
	for _, row := range rows {
		found[row.CustomerPhone] = row.HasAvatar
	}
	assert.True(t, found["628111222333"])
	assert.False(t, found["628999888777"])
}
