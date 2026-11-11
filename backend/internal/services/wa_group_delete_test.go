package services

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// deletableGroup is a group with something in every table that hangs off it,
// and one file on disk for each kind the delete has to find.
type deletableGroup struct {
	groupFixture
	root       string
	other      *models.WAGroup
	photo      string
	groupFace  string
	soloFace   string
	sharedFace string
}

func newDeletableGroup(t *testing.T) deletableGroup {
	t.Helper()
	f := newGroupFixture(t)
	d := deletableGroup{groupFixture: f, root: t.TempDir()}
	d.photo = filepath.Join("2026", "11", "foto.jpg")
	d.groupFace = filepath.Join("avatars", "grup.jpg")
	d.soloFace = filepath.Join("avatars", "budi.jpg")
	d.sharedFace = filepath.Join("avatars", "sari.jpg")
	for _, rel := range []string{d.photo, d.groupFace, d.soloFace, d.sharedFace} {
		writeMedia(t, d.root, rel)
	}

	group, err := f.groups.Upsert(f.account.ID, groupSnap("1@g.us", "Teknisi", "62811", "62899"))
	require.NoError(t, err)
	require.Equal(t, f.group.ID, group.ID)
	d.other, err = f.groups.Upsert(f.account.ID, groupSnap("2@g.us", "Lain", "62899"))
	require.NoError(t, err)

	msg, _, err := f.messages.SaveArrived(groupPhoto(f, "W1", d.photo, time.Now()))
	require.NoError(t, err)
	require.NoError(t, f.db.Create(&models.WAGroupReaction{MessageID: msg.ID, ReactorJID: "a", Emoji: "👍"}).Error)
	require.NoError(t, f.db.Create(&models.WAGroupReceipt{MessageID: msg.ID, ReaderJID: "a"}).Error)
	require.NoError(t, f.db.Create(&models.WAGroupPollVote{MessageID: msg.ID, VoterJID: "a", Options: "[]"}).Error)

	report := models.GraphReport{WAGroupID: &group.ID, CustomerName: "PT Contoh"}
	require.NoError(t, f.db.Create(&report).Error)
	require.NoError(t, f.db.Create(&models.GraphReportPort{
		ReportID: report.ID, DeviceID: 1, DeviceName: "r1", IfName: "ether1", Label: "Uplink",
	}).Error)

	for jid, rel := range map[string]string{
		"1@g.us": d.groupFace, "62811@s.whatsapp.net": d.soloFace, "62899@s.whatsapp.net": d.sharedFace,
	} {
		require.NoError(t, f.db.Create(&models.WAAvatar{WAAccountID: f.account.ID, JID: jid, Path: rel}).Error)
	}
	require.NoError(t, f.db.Create(&models.WAGroupAction{
		WAAccountID: f.account.ID, GroupID: &group.ID, Action: models.GroupActionLeave,
		Payload: "{}", Status: models.GroupActionDone, RequestedBy: uuid.New(),
	}).Error)
	return d
}

func (d deletableGroup) leave(t *testing.T) {
	t.Helper()
	require.NoError(t, d.groups.MarkLeft(d.account.ID, "1@g.us"))
}

func countRows(t *testing.T, db *gorm.DB, model any) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Model(model).Count(&n).Error)
	return n
}

func exists(root, rel string) bool {
	_, err := os.Stat(filepath.Join(root, rel))
	return err == nil
}

func TestDeleteLeftRemovesTheGroupWithEverythingItKept(t *testing.T) {
	d := newDeletableGroup(t)
	d.leave(t)

	require.NoError(t, d.groups.DeleteLeft(d.group.ID, d.root))

	_, err := d.groups.Get(d.group.ID)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	for _, model := range []any{
		&models.WAGroupMessage{}, &models.WAGroupReaction{}, &models.WAGroupReceipt{},
		&models.WAGroupPollVote{}, &models.GraphReport{}, &models.GraphReportPort{},
	} {
		assert.Zero(t, countRows(t, d.db, model), "%T", model)
	}
	var members int64
	require.NoError(t, d.db.Model(&models.WAGroupParticipant{}).Where("group_id = ?", d.group.ID).Count(&members).Error)
	assert.Zero(t, members)

	assert.False(t, exists(d.root, d.photo), "attachment")
	assert.False(t, exists(d.root, d.groupFace), "group photo")
	assert.False(t, exists(d.root, d.soloFace), "photo of a member seen nowhere else")
	assert.True(t, exists(d.root, d.sharedFace), "a member still in another group keeps their photo")
	assert.Equal(t, int64(1), countRows(t, d.db, &models.WAAvatar{}))

	var action models.WAGroupAction
	require.NoError(t, d.db.First(&action).Error)
	assert.Nil(t, action.GroupID, "the request outlives the group, naming none")
	_, err = d.groups.Get(d.other.ID)
	assert.NoError(t, err, "other groups are untouched")
}

// A member who left before the number did is no longer in the member list,
// but the photo tracked while they were stays until the group goes with it.
func TestDeleteLeftRemovesTheAvatarOfAFormerMemberWhoWroteThere(t *testing.T) {
	d := newDeletableGroup(t)
	formerFace := filepath.Join("avatars", "dedi.jpg")
	writeMedia(t, d.root, formerFace)
	former := arrival(d.group.ID, "W2", "pamit")
	former.SenderJID, former.SenderPhone = "62822@s.whatsapp.net", "62822"
	_, _, err := d.messages.SaveArrived(former)
	require.NoError(t, err)
	require.NoError(t, d.db.Create(&models.WAAvatar{
		WAAccountID: d.account.ID, JID: former.SenderJID, Path: formerFace,
	}).Error)
	d.leave(t)

	require.NoError(t, d.groups.DeleteLeft(d.group.ID, d.root))

	assert.False(t, exists(d.root, formerFace))
	var left int64
	require.NoError(t, d.db.Model(&models.WAAvatar{}).Where("jid = ?", former.SenderJID).Count(&left).Error)
	assert.Zero(t, left)
	assert.True(t, exists(d.root, d.sharedFace), "a member still in another group keeps their photo")
}

// The number is still in it: the next sync would only bring it back empty.
func TestDeleteLeftRefusesAGroupStillJoinedAndRemovesNothing(t *testing.T) {
	d := newDeletableGroup(t)

	err := d.groups.DeleteLeft(d.group.ID, d.root)

	assert.ErrorIs(t, err, ErrGroupNotLeft)
	_, err = d.groups.Get(d.group.ID)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), countRows(t, d.db, &models.WAGroupMessage{}))
	assert.Equal(t, int64(1), countRows(t, d.db, &models.GraphReport{}))
	assert.Equal(t, int64(3), countRows(t, d.db, &models.WAAvatar{}))
	assert.True(t, exists(d.root, d.photo))
	assert.True(t, exists(d.root, d.groupFace))
}

func TestDeleteLeftAnswersNotFoundForAMissingGroup(t *testing.T) {
	f := newGroupFixture(t)
	assert.ErrorIs(t, f.groups.DeleteLeft(uuid.New(), t.TempDir()), gorm.ErrRecordNotFound)
}

// Paths come from rows written from what WhatsApp sent; one that climbs out
// of the media root must not take a file with it.
func TestDeleteLeftDoesNotFollowAPathOutOfTheMediaRoot(t *testing.T) {
	d := newDeletableGroup(t)
	outside := filepath.Join(filepath.Dir(d.root), "di-luar-"+uuid.NewString()+".txt")
	require.NoError(t, os.WriteFile(outside, []byte("x"), 0o644))
	t.Cleanup(func() { _ = os.Remove(outside) })
	escape := filepath.Join("..", filepath.Base(outside))
	require.NoError(t, d.db.Model(&models.WAGroupMessage{}).Where("group_id = ?", d.group.ID).
		Update("media_path", escape).Error)
	d.leave(t)

	err := d.groups.DeleteLeft(d.group.ID, d.root)

	assert.ErrorIs(t, err, ErrGroupMediaLeftBehind)
	_, statErr := os.Stat(outside)
	assert.NoError(t, statErr, "a file outside the root stays")
	_, err = d.groups.Get(d.group.ID)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound, "the rows still go")
}

// What SQLite cannot check: the delete runs past migrations 68, 70 and 71's
// foreign keys, and the orphan-avatar subquery binds on Postgres.
func TestDeleteLeftAgainstPostgres(t *testing.T) {
	db := setupPostgresTestDB(t)
	groups := NewWAGroupService(db)
	account := csAccount(t, db)
	group, err := groups.Upsert(account.ID, groupSnap("1@g.us", "Teknisi", "62811"))
	require.NoError(t, err)
	f := groupFixture{db: db, groups: groups, messages: NewWAGroupMessageService(db), account: account, group: group}
	root := t.TempDir()
	rel := filepath.Join("2026", "11", "foto.jpg")
	writeMedia(t, root, rel)
	msg, _, err := f.messages.SaveArrived(groupPhoto(f, "W1", rel, time.Now()))
	require.NoError(t, err)
	require.NoError(t, db.Create(&models.WAGroupReaction{MessageID: msg.ID, ReactorJID: "a", Emoji: "👍"}).Error)
	report := models.GraphReport{WAGroupID: &group.ID, CustomerName: "PT Contoh"}
	require.NoError(t, db.Create(&report).Error)
	require.NoError(t, db.Create(&models.WAAvatar{WAAccountID: account.ID, JID: "62811@s.whatsapp.net"}).Error)

	assert.ErrorIs(t, groups.DeleteLeft(group.ID, root), ErrGroupNotLeft)
	require.NoError(t, groups.MarkLeft(account.ID, "1@g.us"))
	require.NoError(t, groups.DeleteLeft(group.ID, root))

	for _, model := range []any{&models.WAGroup{}, &models.WAGroupMessage{}, &models.WAGroupReaction{},
		&models.GraphReport{}, &models.WAAvatar{}} {
		assert.Zero(t, countRows(t, db, model), "%T", model)
	}
	assert.False(t, exists(root, rel))
}
