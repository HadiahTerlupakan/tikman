package api

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

// uploadRequest builds a multipart POST carrying one file of the given size,
// declared with the given content type.
func uploadRequest(t *testing.T, path, contentType string, size int) *http.Request {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", `form-data; name="file"; filename="foto.jpg"`)
	header.Set("Content-Type", contentType)
	part, err := writer.CreatePart(header)
	require.NoError(t, err)
	_, err = part.Write(bytes.Repeat([]byte("x"), size))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, path, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

// Nothing bounded an upload before this: storeUpload copied whatever arrived
// to disk and wa.Client.SendMedia read it back whole into memory. The refusal
// has to name the size, because "a file is required" for a photo the CS can
// see they attached reads as a broken page.
func TestSendMediaRefusesAnUploadPastTheSizeCap(t *testing.T) {
	env := setupCSHandler(t)
	conv := env.conversation(t, "628111@s.whatsapp.net", "628111222333")
	require.NoError(t, env.conversations.Assign(conv.ID, env.cs))

	req := uploadRequest(t, "/api/v1/cs/conversations/"+conv.ID.String()+"/media",
		"image/jpeg", maxUploadBytes+1)
	rec := httptest.NewRecorder()
	env.asUser(env.cs, models.UserRoleCS).ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "MEDIA_TOO_LARGE")

	var stored int64
	require.NoError(t, env.db.Model(&models.CSMessage{}).Count(&stored).Error)
	assert.Zero(t, stored, "an upload that was refused must leave no message behind")
}

// The cap must not have closed the door on ordinary attachments, which is the
// whole point of the endpoint.
func TestSendMediaAcceptsAnUploadUnderTheSizeCap(t *testing.T) {
	env := setupCSHandler(t)
	conv := env.conversation(t, "628111@s.whatsapp.net", "628111222333")
	require.NoError(t, env.conversations.Assign(conv.ID, env.cs))

	req := uploadRequest(t, "/api/v1/cs/conversations/"+conv.ID.String()+"/media",
		"image/jpeg", 1024)
	rec := httptest.NewRecorder()
	env.asUser(env.cs, models.UserRoleCS).ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)

	var stored models.CSMessage
	require.NoError(t, env.db.Where("conversation_id = ?", conv.ID).First(&stored).Error)
	assert.Equal(t, models.MessageKindImage, stored.Kind)
	assert.Equal(t, int64(1024), stored.MediaSize)
}
