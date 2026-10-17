package services

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

func groupSnap(jid, name string, members ...string) GroupSnapshot {
	snap := GroupSnapshot{JID: jid, Name: name}
	for _, phone := range members {
		snap.Participants = append(snap.Participants, models.WAGroupParticipant{
			JID: phone + "@s.whatsapp.net", Phone: phone, Role: models.GroupRoleMember,
		})
	}
	return snap
}

func TestSyncAllMirrorsGroupsAndTheirMembers(t *testing.T) {
	db := setupTestDB(t)
	svc := NewWAGroupService(db)
	account := csAccount(t, db)

	require.NoError(t, svc.SyncAll(account.ID, []GroupSnapshot{
		groupSnap("1@g.us", "Teknisi Cariu", "62811", "62812"),
	}))

	groups, err := svc.List(GroupFilter{})
	require.NoError(t, err)
	require.Len(t, groups, 1)
	assert.Equal(t, "Teknisi Cariu", groups[0].Name)
	assert.Equal(t, 2, groups[0].ParticipantCount)

	members, err := svc.Participants(groups[0].ID)
	require.NoError(t, err)
	assert.Len(t, members, 2)
}

// A group the number left must stay readable, so it is marked rather than
// deleted; and one it rejoins must stop being marked.
func TestSyncAllMarksAGroupNoLongerListedAsLeftAndClearsItOnReturn(t *testing.T) {
	db := setupTestDB(t)
	svc := NewWAGroupService(db)
	account := csAccount(t, db)
	require.NoError(t, svc.SyncAll(account.ID, []GroupSnapshot{groupSnap("1@g.us", "A"), groupSnap("2@g.us", "B")}))

	require.NoError(t, svc.SyncAll(account.ID, []GroupSnapshot{groupSnap("1@g.us", "A")}))
	left := listedGroup(t, svc, "2@g.us")
	assert.True(t, left.Left)
	assert.False(t, listedGroup(t, svc, "1@g.us").Left)

	require.NoError(t, svc.SyncAll(account.ID, []GroupSnapshot{groupSnap("1@g.us", "A"), groupSnap("2@g.us", "B")}))
	assert.False(t, listedGroup(t, svc, "2@g.us").Left)
}

// The sync rewrites WhatsApp's metadata; the unread count and last message are
// ours and must survive it.
func TestSyncKeepsTheUnreadCountAndLastMessage(t *testing.T) {
	db := setupTestDB(t)
	svc := NewWAGroupService(db)
	messages := NewWAGroupMessageService(db)
	account := csAccount(t, db)
	group, err := svc.Upsert(account.ID, groupSnap("1@g.us", "A"))
	require.NoError(t, err)
	_, _, err = messages.SaveArrived(arrival(group.ID, "W1", "halo"))
	require.NoError(t, err)

	require.NoError(t, svc.SyncAll(account.ID, []GroupSnapshot{groupSnap("1@g.us", "A baru")}))

	after := listedGroup(t, svc, "1@g.us")
	assert.Equal(t, "A baru", after.Name)
	assert.Equal(t, 1, after.UnreadCount)
	assert.Equal(t, "halo", after.LastMessagePreview)
}

func TestFindOrCreateAnswersTheSameGroupAndSaysWhenItIsNew(t *testing.T) {
	db := setupTestDB(t)
	svc := NewWAGroupService(db)
	account := csAccount(t, db)

	first, created, err := svc.FindOrCreate(account.ID, "9@g.us")
	require.NoError(t, err)
	assert.True(t, created)
	second, created, err := svc.FindOrCreate(account.ID, "9@g.us")
	require.NoError(t, err)
	assert.False(t, created)
	assert.Equal(t, first.ID, second.ID)
}

func TestListPutsTheMostRecentlyActiveGroupFirstAndFiltersByName(t *testing.T) {
	db := setupTestDB(t)
	svc := NewWAGroupService(db)
	messages := NewWAGroupMessageService(db)
	account := csAccount(t, db)
	quiet, err := svc.Upsert(account.ID, groupSnap("1@g.us", "Grup Sepi"))
	require.NoError(t, err)
	busy, err := svc.Upsert(account.ID, groupSnap("2@g.us", "Grup Ramai"))
	require.NoError(t, err)
	_, err = svc.Upsert(account.ID, groupSnap("3@g.us", "Tanpa Pesan"))
	require.NoError(t, err)

	older := arrival(quiet.ID, "W1", "lama")
	older.At = time.Now().Add(-time.Hour)
	_, _, err = messages.SaveArrived(older)
	require.NoError(t, err)
	_, _, err = messages.SaveArrived(arrival(busy.ID, "W2", "baru"))
	require.NoError(t, err)

	groups, err := svc.List(GroupFilter{})
	require.NoError(t, err)
	require.Len(t, groups, 3)
	assert.Equal(t, []string{"Grup Ramai", "Grup Sepi", "Tanpa Pesan"},
		[]string{groups[0].Name, groups[1].Name, groups[2].Name})

	found, err := svc.List(GroupFilter{Search: "ramai"})
	require.NoError(t, err)
	require.Len(t, found, 1)
	assert.Equal(t, busy.ID, found[0].ID)
}

func TestMarkReadClearsTheUnreadCountOnce(t *testing.T) {
	db := setupTestDB(t)
	svc := NewWAGroupService(db)
	messages := NewWAGroupMessageService(db)
	account := csAccount(t, db)
	group, err := svc.Upsert(account.ID, groupSnap("1@g.us", "A"))
	require.NoError(t, err)
	_, _, err = messages.SaveArrived(arrival(group.ID, "W1", "halo"))
	require.NoError(t, err)

	cleared, err := svc.MarkRead(group.ID)
	require.NoError(t, err)
	assert.True(t, cleared)
	cleared, err = svc.MarkRead(group.ID)
	require.NoError(t, err)
	assert.False(t, cleared, "a second read changes nothing and must say so")
}

func listedGroup(t *testing.T, svc *WAGroupService, jid string) models.WAGroup {
	t.Helper()
	groups, err := svc.List(GroupFilter{})
	require.NoError(t, err)
	for _, g := range groups {
		if g.JID == jid {
			return g
		}
	}
	t.Fatalf("group %s not found", jid)
	return models.WAGroup{}
}

func arrival(groupID uuid.UUID, waID, body string) GroupArrival {
	return GroupArrival{
		GroupID: groupID, WAMessageID: waID, Direction: models.MessageIn,
		SenderJID: "62811@s.whatsapp.net", SenderPhone: "62811", SenderName: "Budi",
		Kind: models.MessageKindText, Body: body, At: time.Now(),
	}
}
