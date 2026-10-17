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
