package services

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

// What SQLite cannot check: migration 68's constraints, the upsert on its
// unique index, the outbox join, and deleting a number past the foreign keys.
func TestGroupsAgainstPostgres(t *testing.T) {
	db := setupPostgresTestDB(t)
	groups := NewWAGroupService(db)
	messages := NewWAGroupMessageService(db)
	actions := NewWAGroupActionService(db)
	account := csAccount(t, db)

	group, err := groups.Upsert(account.ID, groupSnap("1@g.us", "Teknisi", "62811"))
	require.NoError(t, err)
	again, err := groups.Upsert(account.ID, groupSnap("1@g.us", "Teknisi Baru", "62811", "62812"))
	require.NoError(t, err)
	assert.Equal(t, group.ID, again.ID, "the upsert lands on the row the unique index names")

	_, created, err := messages.SaveArrived(arrival(group.ID, "W1", "halo"))
	require.NoError(t, err)
	require.True(t, created)
	_, created, err = messages.SaveArrived(arrival(group.ID, "W1", "halo"))
	require.NoError(t, err)
	assert.False(t, created)

	queued, err := messages.Queue(GroupOutgoing{
		GroupID: group.ID, SenderUserID: uuid.New(), Kind: models.MessageKindText, Body: "OTW",
	})
	require.NoError(t, err)
	claimed, err := messages.ClaimQueued(account.ID, 10)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	assert.Equal(t, queued.ID, claimed[0].ID)
	assert.Equal(t, "1@g.us", claimed[0].GroupJID)

	listed, err := groups.List(GroupFilter{AccountID: &account.ID, Search: "baru"})
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, models.MessageOut, listed[0].LastMessageDirection)

	_, err = actions.Queue(GroupActionRequest{
		AccountID: account.ID, GroupID: &group.ID, Action: models.GroupActionLeave, UserID: uuid.New(),
	})
	require.NoError(t, err)

	require.NoError(t, NewCSPurgeService(db, t.TempDir()).DeleteAccount(account.ID))
	var left int64
	require.NoError(t, db.Model(&models.WAGroup{}).Count(&left).Error)
	assert.Zero(t, left)
}

// A row the migration's checks refuse must not be storable by any path that
// forgets the Go-side rules.
func TestGroupMigrationRefusesAnUnknownStatus(t *testing.T) {
	db := setupPostgresTestDB(t)
	account := csAccount(t, db)
	group, err := NewWAGroupService(db).Upsert(account.ID, groupSnap("1@g.us", "A"))
	require.NoError(t, err)

	err = db.Create(&models.WAGroupMessage{
		GroupID: group.ID, Direction: models.MessageIn, Kind: models.MessageKindText,
		Status: "lost", WATimestamp: time.Now(),
	}).Error
	assert.Error(t, err)
}

// The upserts and the per-page aggregates of reactions, receipts, votes and
// photos, which SQLite accepts more loosely than Postgres.
func TestGroupParityAgainstPostgres(t *testing.T) {
	db := setupPostgresTestDB(t)
	groups := NewWAGroupService(db)
	messages := NewWAGroupMessageService(db)
	avatars := NewWAAvatarService(db)
	account := csAccount(t, db)
	group, err := groups.Upsert(account.ID, groupSnap("1@g.us", "Teknisi", "62811", "62812"))
	require.NoError(t, err)

	queued, err := messages.Queue(GroupOutgoing{
		GroupID: group.ID, SenderUserID: uuid.New(), Kind: models.MessageKindText, Body: "OTW", MemberCount: 2,
	})
	require.NoError(t, err)
	require.NoError(t, messages.MarkSent(queued.ID, "WOUT", nil))
	for _, reader := range []string{"62811@s.whatsapp.net", "62811@s.whatsapp.net", "62812@s.whatsapp.net"} {
		_, err = messages.ApplyReceipt(group.ID, []string{"WOUT"}, reader, true, time.Now())
		require.NoError(t, err)
	}
	actor := models.GroupActor{JID: "62811@s.whatsapp.net", Name: "Budi"}
	require.NoError(t, messages.SetReaction(queued.ID, actor, "👍"))
	require.NoError(t, messages.SetReaction(queued.ID, actor, "❤️"))
	_, err = groups.SetPinned(group.ID, true)
	require.NoError(t, err)
	require.NoError(t, avatars.Track(account.ID, []string{"1@g.us", "62811@s.whatsapp.net"}))
	require.NoError(t, avatars.Track(account.ID, []string{"1@g.us"}))
	due, err := avatars.StaleSubjects(account.ID, 10, time.Now())
	require.NoError(t, err)
	require.Len(t, due, 2)
	assert.Equal(t, "1@g.us", due[0].JID, "a group's photo is looked at before its members'")

	history, err := messages.History(group.ID, 10, 0)
	require.NoError(t, err)
	require.Len(t, history, 1)
	assert.Equal(t, models.MessageRead, history[0].Status)
	require.NotNil(t, history[0].ReadCount)
	assert.Equal(t, 2, *history[0].ReadCount)
	require.Len(t, history[0].Reactions, 1)
	assert.Equal(t, "❤️", history[0].Reactions[0].Emoji)

	listed, err := groups.List(GroupFilter{})
	require.NoError(t, err)
	assert.True(t, listed[0].Pinned)
}
