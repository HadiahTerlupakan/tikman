package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
)

func (e *csHandlerEnv) deleteAs(id uuid.UUID, role models.UserRole, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/cs"+path, nil)
	rec := httptest.NewRecorder()
	e.asUser(id, role).ServeHTTP(rec, req)
	return rec
}

// storedPhoto writes an attachment under the handler's own media root and
// stores the message that points at it.
func (e *csHandlerEnv) storedPhoto(t *testing.T, conv uuid.UUID, waID string) (*models.CSMessage, string) {
	t.Helper()
	rel := filepath.Join("2026", "09", waID+".jpg")
	require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(e.mediaRoot, rel)), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(e.mediaRoot, rel), []byte("x"), 0o644))

	msg, _, err := e.messages.SaveInbound(services.InboundMessage{
		ConversationID: conv, WAMessageID: waID, Kind: models.MessageKindImage,
		Media: &services.MediaFile{Path: rel, Mime: "image/jpeg", Filename: "p.jpg", Size: 1},
		At:    time.Now(),
	})
	require.NoError(t, err)
	return msg, rel
}

func (e *csHandlerEnv) onDisk(rel string) bool {
	_, err := os.Stat(filepath.Join(e.mediaRoot, rel))
	return err == nil
}

func TestTheHolderCanDeleteAMessageAndItsAttachment(t *testing.T) {
	env := setupCSHandler(t)
	conv := env.conversation(t, "628111@s.whatsapp.net", "628111222333")
	require.NoError(t, env.conversations.Assign(conv.ID, env.cs))
	msg, rel := env.storedPhoto(t, conv.ID, "3EB0A")

	rec := env.deleteAs(env.cs, models.UserRoleCS, "/messages/"+msg.ID.String())
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	assert.False(t, env.onDisk(rel), "the attachment left the disk with the row")
	history, err := env.messages.History(conv.ID, 10, 0)
	require.NoError(t, err)
	assert.Empty(t, history)
}

// The same gate as replying. A CS who is not working the thread cannot tell a
// mistyped message from the customer's own words.
func TestACSWhoDoesNotHoldTheThreadCannotDeleteFromIt(t *testing.T) {
	env := setupCSHandler(t)
	conv := env.conversation(t, "628111@s.whatsapp.net", "628111222333")
	require.NoError(t, env.conversations.Assign(conv.ID, env.cs))
	msg, rel := env.storedPhoto(t, conv.ID, "3EB0A")

	rec := env.deleteAs(env.otherCS, models.UserRoleCS, "/messages/"+msg.ID.String())
	assert.Equal(t, http.StatusConflict, rec.Code)

	assert.True(t, env.onDisk(rel), "nothing was removed")
	history, err := env.messages.History(conv.ID, 10, 0)
	require.NoError(t, err)
	assert.Len(t, history, 1)
}

func TestAnAdminCanDeleteFromAThreadSomebodyElseHolds(t *testing.T) {
	env := setupCSHandler(t)
	conv := env.conversation(t, "628111@s.whatsapp.net", "628111222333")
	require.NoError(t, env.conversations.Assign(conv.ID, env.cs))
	msg, _ := env.storedPhoto(t, conv.ID, "3EB0A")

	rec := env.deleteAs(uuid.New(), models.UserRoleAdmin, "/messages/"+msg.ID.String())
	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

func TestClearingAThreadEmptiesItWithoutRemovingIt(t *testing.T) {
	env := setupCSHandler(t)
	conv := env.conversation(t, "628111@s.whatsapp.net", "628111222333")
	require.NoError(t, env.conversations.Assign(conv.ID, env.cs))
	env.storedPhoto(t, conv.ID, "3EB0A")
	env.storedPhoto(t, conv.ID, "3EB0B")

	rec := env.deleteAs(env.cs, models.UserRoleCS, "/conversations/"+conv.ID.String()+"/messages")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	history, err := env.messages.History(conv.ID, 10, 0)
	require.NoError(t, err)
	assert.Empty(t, history)
	_, err = env.conversations.Get(conv.ID)
	assert.NoError(t, err, "the thread stays in the inbox")
}

// Emptying every thread on every number has no thread holder to gate it, so
// nothing but the admin role stands between a CS and the whole history.
func TestOnlyAnAdminCanClearTheWholeInbox(t *testing.T) {
	env := setupCSHandler(t)
	conv := env.conversation(t, "628111@s.whatsapp.net", "628111222333")
	env.storedPhoto(t, conv.ID, "3EB0A")

	refused := env.deleteAs(env.cs, models.UserRoleCS, "/messages")
	assert.Equal(t, http.StatusForbidden, refused.Code)

	history, err := env.messages.History(conv.ID, 10, 0)
	require.NoError(t, err)
	require.Len(t, history, 1, "the refusal really did leave it there")

	allowed := env.deleteAs(uuid.New(), models.UserRoleAdmin, "/messages")
	require.Equal(t, http.StatusOK, allowed.Code, allowed.Body.String())

	history, err = env.messages.History(conv.ID, 10, 0)
	require.NoError(t, err)
	assert.Empty(t, history)
}

func TestOnlyAnAdminCanDeleteANumber(t *testing.T) {
	env := setupCSHandler(t)

	refused := env.deleteAs(env.cs, models.UserRoleCS, "/wa-accounts/"+env.account.ID.String())
	assert.Equal(t, http.StatusForbidden, refused.Code)

	_, err := services.NewCSAccountService(env.db).Get(env.account.ID)
	assert.NoError(t, err, "the number is still there")
}

func TestDeletingANumberTakesItsThreadsMessagesAndFilesWithIt(t *testing.T) {
	env := setupCSHandler(t)
	conv := env.conversation(t, "628111@s.whatsapp.net", "628111222333")
	_, rel := env.storedPhoto(t, conv.ID, "3EB0A")

	rec := env.deleteAs(uuid.New(), models.UserRoleAdmin, "/wa-accounts/"+env.account.ID.String())
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	assert.False(t, env.onDisk(rel))
	_, err := env.conversations.Get(conv.ID)
	assert.Error(t, err, "the thread went with the number")

	accounts, err := services.NewCSAccountService(env.db).List()
	require.NoError(t, err)
	assert.Empty(t, accounts)
}

// Clearing a number is not deleting it: the number keeps answering, with a
// clean history behind it.
func TestClearingANumberKeepsTheNumberAndItsThreads(t *testing.T) {
	env := setupCSHandler(t)
	conv := env.conversation(t, "628111@s.whatsapp.net", "628111222333")
	env.storedPhoto(t, conv.ID, "3EB0A")

	rec := env.deleteAs(uuid.New(), models.UserRoleAdmin,
		"/wa-accounts/"+env.account.ID.String()+"/messages")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	_, err := services.NewCSAccountService(env.db).Get(env.account.ID)
	assert.NoError(t, err)
	_, err = env.conversations.Get(conv.ID)
	assert.NoError(t, err)

	history, err := env.messages.History(conv.ID, 10, 0)
	require.NoError(t, err)
	assert.Empty(t, history)
}

func TestDeletingAMessageThatIsNotThereIsANotFound(t *testing.T) {
	env := setupCSHandler(t)
	rec := env.deleteAs(uuid.New(), models.UserRoleAdmin, "/messages/"+uuid.New().String())
	assert.Equal(t, http.StatusNotFound, rec.Code)
}
