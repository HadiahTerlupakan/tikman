package services

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

func channelSetup(t *testing.T) (*CSChannelService, models.WAAccount) {
	t.Helper()
	db := setupTestDB(t)
	return NewCSChannelService(db), csAccount(t, db)
}

func channel(jid, name string, role models.ChannelRole) models.WAChannel {
	return models.WAChannel{JID: jid, Name: name, Role: role, SubscriberCount: 12}
}

// Replacing rather than merging is the whole reason a revoked admin right
// needs no removal path: the next sync simply does not mention that channel.
func TestReplaceDropsAChannelTheNumberNoLongerAdmins(t *testing.T) {
	channels, account := channelSetup(t)

	require.NoError(t, channels.Replace(account.ID, []models.WAChannel{
		channel("120363000000000001@newsletter", "Info Gangguan", models.ChannelRoleOwner),
		channel("120363000000000002@newsletter", "Promo", models.ChannelRoleAdmin),
	}))
	require.NoError(t, channels.Replace(account.ID, []models.WAChannel{
		channel("120363000000000001@newsletter", "Info Gangguan", models.ChannelRoleOwner),
	}))

	rows, err := channels.List()
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "120363000000000001@newsletter", rows[0].JID)
}

// One number's sync must not empty another's picker.
func TestReplaceLeavesAnotherNumbersChannelsAlone(t *testing.T) {
	db := setupTestDB(t)
	channels := NewCSChannelService(db)
	first := csAccount(t, db)
	second := models.WAAccount{Label: "CS Kedua", Status: models.WAAccountConnected}
	require.NoError(t, db.Create(&second).Error)

	require.NoError(t, channels.Replace(first.ID, []models.WAChannel{
		channel("120363000000000001@newsletter", "Info Gangguan", models.ChannelRoleOwner),
	}))
	require.NoError(t, channels.Replace(second.ID, []models.WAChannel{
		channel("120363000000000009@newsletter", "Kanal Kedua", models.ChannelRoleAdmin),
	}))
	require.NoError(t, channels.Replace(second.ID, nil))

	rows, err := channels.List()
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, first.ID, rows[0].WAAccountID)
}

// Replace stamps the account and a fresh sync time onto rows the caller did
// not fill in, so the sync code never has to remember to.
func TestReplaceStampsAccountAndSyncTime(t *testing.T) {
	channels, account := channelSetup(t)

	require.NoError(t, channels.Replace(account.ID, []models.WAChannel{
		channel("120363000000000003@newsletter", "Pemeliharaan", models.ChannelRoleAdmin),
	}))

	rows, err := channels.List()
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, account.ID, rows[0].WAAccountID)
	assert.False(t, rows[0].SyncedAt.IsZero())
	assert.NotEqual(t, uuid.Nil, rows[0].ID)
}

// Get is how a post request turns a picked id into the JID and the number it
// must be sent through. An id that is not in the mirror must not resolve.
func TestGetRefusesAnUnknownChannel(t *testing.T) {
	channels, _ := channelSetup(t)

	_, err := channels.Get(uuid.New())
	assert.Error(t, err)
}
