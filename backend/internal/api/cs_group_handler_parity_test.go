package api

import (
	"encoding/json"
	"net/http"
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

func (e *csGroupEnv) sent(t *testing.T, body string, at time.Time) *models.WAGroupMessage {
	t.Helper()
	msg, err := e.messages.Queue(services.GroupOutgoing{
		GroupID: e.group.ID, SenderUserID: e.cs, Kind: models.MessageKindText, Body: body,
	})
	require.NoError(t, err)
	require.NoError(t, e.messages.MarkSent(msg.ID, "W"+uuid.NewString()[:8], nil))
	require.NoError(t, e.db.Model(&models.WAGroupMessage{}).Where("id = ?", msg.ID).Update("wa_timestamp", at).Error)
	return msg
}

func (e *csGroupEnv) received(t *testing.T, body string) *models.WAGroupMessage {
	t.Helper()
	msg, _, err := e.messages.SaveArrived(services.GroupArrival{
		GroupID: e.group.ID, WAMessageID: "W" + uuid.NewString()[:8], Direction: models.MessageIn,
		SenderJID: "62811@s.whatsapp.net", SenderPhone: "62811", SenderPushName: "Budi",
		Kind: models.MessageKindText, Body: body, At: time.Now(),
	})
	require.NoError(t, err)
	return msg
}

func (e *csGroupEnv) messagePath(msg *models.WAGroupMessage, suffix string) string {
	return "/groups/" + e.group.ID.String() + "/messages/" + msg.ID.String() + suffix
}

func TestOnlyOurRecentTextCanBeEdited(t *testing.T) {
	env := setupCSGroups(t)
	fresh := env.sent(t, "jam 2", time.Now())
	stale := env.sent(t, "jam 2", time.Now().Add(-time.Hour))
	theirs := env.received(t, "jam 2")

	cases := map[string]struct {
		msg  *models.WAGroupMessage
		code int
		want string
	}{
		"ours, just sent":  {fresh, http.StatusAccepted, ""},
		"ours, an hour on": {stale, http.StatusConflict, "EDIT_WINDOW"},
		"someone else's":   {theirs, http.StatusConflict, "NOT_OURS"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			rec := env.do(t, env.cs, models.UserRoleCS, http.MethodPut, env.messagePath(tc.msg, ""), `{"body":"jam 3"}`)
			assert.Equal(t, tc.code, rec.Code, rec.Body.String())
			assert.Contains(t, rec.Body.String(), tc.want)
		})
	}
}

// Deleting someone else's message for everyone speaks for the number; it is
// an admin's call, and only where WhatsApp lets the number do it.
func TestDeletingSomeoneElsesMessageNeedsAnAdminInAGroupWeRun(t *testing.T) {
	env := setupCSGroups(t)
	theirs := env.received(t, "spam")
	path := env.messagePath(theirs, "")

	assert.Equal(t, http.StatusForbidden, env.do(t, env.cs, models.UserRoleCS, http.MethodDelete, path, "").Code)
	assert.Equal(t, http.StatusForbidden, env.do(t, env.admin, models.UserRoleAdmin, http.MethodDelete, path, "").Code,
		"the number is not an admin of this group")

	_, err := env.groups.Upsert(env.account.ID, services.GroupSnapshot{JID: env.group.JID, IsAdmin: true})
	require.NoError(t, err)
	assert.Equal(t, http.StatusAccepted, env.do(t, env.admin, models.UserRoleAdmin, http.MethodDelete, path, "").Code)

	mine := env.sent(t, "salah grup", time.Now())
	assert.Equal(t, http.StatusAccepted,
		env.do(t, env.cs, models.UserRoleCS, http.MethodDelete, env.messagePath(mine, ""), "").Code)
}

func TestAForwardCopiesTheAttachmentForEachGroup(t *testing.T) {
	env := setupCSGroups(t)
	other, err := env.groups.Upsert(env.account.ID, services.GroupSnapshot{JID: "120363000000000002@g.us", Name: "Lain"})
	require.NoError(t, err)
	rel := filepath.Join("2026", "10", "foto.jpg")
	require.NoError(t, os.MkdirAll(filepath.Join(env.mediaRoot, "2026", "10"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(env.mediaRoot, rel), []byte("jpeg"), 0o644))
	photo, _, err := env.messages.SaveArrived(services.GroupArrival{
		GroupID: env.group.ID, WAMessageID: "WPHOTO", Direction: models.MessageIn, Kind: models.MessageKindImage,
		Media: &services.MediaFile{Path: rel, Mime: "image/jpeg", Filename: "foto.jpg", Size: 4}, At: time.Now(),
	})
	require.NoError(t, err)

	rec := env.do(t, env.cs, models.UserRoleCS, http.MethodPost, env.messagePath(photo, "/forward"),
		`{"group_ids":["`+other.ID.String()+`"]}`)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var answer struct {
		Data []models.WAGroupMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &answer))
	require.Len(t, answer.Data, 1)
	copied, err := env.messages.Get(answer.Data[0].ID)
	require.NoError(t, err)
	assert.True(t, copied.Forwarded)
	assert.NotEqual(t, rel, copied.MediaPath, "each row owns its own file")
	_, err = os.Stat(filepath.Join(env.mediaRoot, copied.MediaPath))
	assert.NoError(t, err)
}

func TestReplyingPrivatelyOpensTheSendersThreadInTheInbox(t *testing.T) {
	env := setupCSGroups(t)
	theirs := env.received(t, "tolong dicek rumah saya")

	rec := env.do(t, env.cs, models.UserRoleCS, http.MethodPost, env.messagePath(theirs, "/reply-private"), "")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var answer struct {
		Data struct {
			ConversationID uuid.UUID `json:"conversation_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &answer))
	conv, err := env.conversations.Get(answer.Data.ConversationID)
	require.NoError(t, err)
	assert.Equal(t, "62811@s.whatsapp.net", conv.CustomerJID)
	assert.Equal(t, "Budi", conv.CustomerName)
}

// A CS who asked for a reaction must be able to learn it failed; an admin's
// requests, which can carry the invite link, stay theirs.
func TestAnActionCanBeReadBackByWhoeverAskedForIt(t *testing.T) {
	env := setupCSGroups(t)
	theirs := env.received(t, "mantap")
	rec := env.do(t, env.cs, models.UserRoleCS, http.MethodPost, env.messagePath(theirs, "/reaction"), `{"emoji":"👍"}`)
	require.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())
	var answer struct {
		Data models.WAGroupAction `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &answer))
	path := "/group-actions/" + answer.Data.ID.String()

	assert.Equal(t, http.StatusOK, env.do(t, env.cs, models.UserRoleCS, http.MethodGet, path, "").Code)
	assert.Equal(t, http.StatusForbidden, env.do(t, env.otherCS, models.UserRoleCS, http.MethodGet, path, "").Code)
}

func TestMessageInfoIsOnlyForOurOwnMessages(t *testing.T) {
	env := setupCSGroups(t)
	mine := env.sent(t, "rapat", time.Now())
	theirs := env.received(t, "oke")

	assert.Equal(t, http.StatusOK,
		env.do(t, env.cs, models.UserRoleCS, http.MethodGet, env.messagePath(mine, "/receipts"), "").Code)
	assert.Equal(t, http.StatusConflict,
		env.do(t, env.cs, models.UserRoleCS, http.MethodGet, env.messagePath(theirs, "/receipts"), "").Code)
}

func TestHistoryNeedsAMessageToCountBackFrom(t *testing.T) {
	env := setupCSGroups(t)
	path := "/groups/" + env.group.ID.String() + "/history"

	assert.Contains(t, env.do(t, env.cs, models.UserRoleCS, http.MethodPost, path, "").Body.String(), "NO_ANCHOR")
	env.received(t, "halo")
	assert.Equal(t, http.StatusAccepted, env.do(t, env.cs, models.UserRoleCS, http.MethodPost, path, "").Code)
}

// A message carries its writer's initials; a colleague editing or deleting it
// would be speaking in their name.
func TestAColleagueCannotEditOrDeleteWhatAnotherCSWrote(t *testing.T) {
	env := setupCSGroups(t)
	written := env.sent(t, "jadwal teknisi", time.Now())

	assert.Equal(t, http.StatusConflict,
		env.do(t, env.otherCS, models.UserRoleCS, http.MethodPut, env.messagePath(written, ""), `{"body":"diubah"}`).Code)
	assert.Equal(t, http.StatusForbidden,
		env.do(t, env.otherCS, models.UserRoleCS, http.MethodDelete, env.messagePath(written, ""), "").Code)
	assert.Equal(t, http.StatusConflict,
		env.do(t, env.admin, models.UserRoleAdmin, http.MethodPut, env.messagePath(written, ""), `{"body":"diubah"}`).Code,
		"not even an admin puts words in someone's name")
	assert.Equal(t, http.StatusAccepted,
		env.do(t, env.admin, models.UserRoleAdmin, http.MethodDelete, env.messagePath(written, ""), "").Code)
}
