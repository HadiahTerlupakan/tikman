package services

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

type groupFixture struct {
	db       *gorm.DB
	groups   *WAGroupService
	messages *WAGroupMessageService
	account  models.WAAccount
	group    *models.WAGroup
}

func newGroupFixture(t *testing.T) groupFixture {
	t.Helper()
	db := setupTestDB(t)
	groups := NewWAGroupService(db)
	account := csAccount(t, db)
	group, err := groups.Upsert(account.ID, groupSnap("1@g.us", "Teknisi", "62811"))
	require.NoError(t, err)
	return groupFixture{db: db, groups: groups, messages: NewWAGroupMessageService(db), account: account, group: group}
}

func (f groupFixture) reload(t *testing.T) *models.WAGroup {
	t.Helper()
	group, err := f.groups.Get(f.group.ID)
	require.NoError(t, err)
	return group
}

// WhatsApp re-delivers on every reconnect; a message must show and count once.
func TestSaveArrivedStoresAGroupMessageOnce(t *testing.T) {
	f := newGroupFixture(t)

	first, created, err := f.messages.SaveArrived(arrival(f.group.ID, "W1", "lampu LOS"))
	require.NoError(t, err)
	assert.True(t, created)
	again, created, err := f.messages.SaveArrived(arrival(f.group.ID, "W1", "lampu LOS"))
	require.NoError(t, err)
	assert.False(t, created)
	assert.Equal(t, first.ID, again.ID)

	history, err := f.messages.History(f.group.ID, 10, 0)
	require.NoError(t, err)
	require.Len(t, history, 1)
	assert.Equal(t, "Budi", history[0].SenderName)

	group := f.reload(t)
	assert.Equal(t, 1, group.UnreadCount)
	assert.Equal(t, "Budi", group.LastMessageSender)
}

// The same WhatsApp id in two groups is two messages: ids are only unique
// within a chat.
func TestTheSameWhatsAppIDInTwoGroupsIsTwoMessages(t *testing.T) {
	f := newGroupFixture(t)
	other, err := f.groups.Upsert(f.account.ID, groupSnap("2@g.us", "Lain"))
	require.NoError(t, err)

	_, created, err := f.messages.SaveArrived(arrival(f.group.ID, "W1", "a"))
	require.NoError(t, err)
	require.True(t, created)
	_, created, err = f.messages.SaveArrived(arrival(other.ID, "W1", "b"))
	require.NoError(t, err)
	assert.True(t, created)
}

// What the phone holding the number writes is ours, not something waiting to
// be read.
func TestAMessageFromOurOwnPhoneDoesNotCountAsUnread(t *testing.T) {
	f := newGroupFixture(t)
	mine := arrival(f.group.ID, "W1", "siap")
	mine.Direction = models.MessageOut

	msg, _, err := f.messages.SaveArrived(mine)
	require.NoError(t, err)

	assert.Equal(t, models.MessageSent, msg.Status)
	assert.Equal(t, 0, f.reload(t).UnreadCount)
	assert.Equal(t, "siap", f.reload(t).LastMessagePreview)
}

func TestAQuotedGroupMessageIsResolvedInTheHistory(t *testing.T) {
	f := newGroupFixture(t)
	_, _, err := f.messages.SaveArrived(arrival(f.group.ID, "W1", "ONT mati"))
	require.NoError(t, err)
	reply := arrival(f.group.ID, "W2", "sudah dicek")
	reply.ReplyToWAID = "W1"
	reply.SenderName = "Rina"
	_, _, err = f.messages.SaveArrived(reply)
	require.NoError(t, err)

	history, err := f.messages.History(f.group.ID, 10, 0)
	require.NoError(t, err)
	var found bool
	for _, row := range history {
		if row.Body == "sudah dicek" {
			require.NotNil(t, row.ReplyTo)
			assert.Equal(t, "ONT mati", row.ReplyTo.Body)
			assert.Equal(t, "Budi", row.ReplyTo.SenderName, "a group quote names who wrote it")
			found = true
		}
	}
	assert.True(t, found)
}

func TestQuoteTargetRefusesAnotherGroupsMessageAndAnUnsentOne(t *testing.T) {
	f := newGroupFixture(t)
	other, err := f.groups.Upsert(f.account.ID, groupSnap("2@g.us", "Lain"))
	require.NoError(t, err)
	elsewhere, _, err := f.messages.SaveArrived(arrival(other.ID, "W9", "rahasia"))
	require.NoError(t, err)
	queued, err := f.messages.Queue(GroupOutgoing{
		GroupID: f.group.ID, SenderUserID: uuid.New(), Kind: models.MessageKindText, Body: "antre",
	})
	require.NoError(t, err)

	_, err = f.messages.QuoteTarget(f.group.ID, elsewhere.ID)
	assert.True(t, errors.Is(err, ErrValidation))
	_, err = f.messages.QuoteTarget(f.group.ID, queued.ID)
	assert.True(t, errors.Is(err, ErrValidation))
}

// Each number sends only its own groups' messages, and needs the group's JID
// to do it.
func TestClaimQueuedAnswersOnlyThisNumbersGroupMessages(t *testing.T) {
	f := newGroupFixture(t)
	otherAccount := models.WAAccount{Label: "CS Dua", Status: models.WAAccountConnected}
	require.NoError(t, f.db.Create(&otherAccount).Error)
	otherGroup, err := f.groups.Upsert(otherAccount.ID, groupSnap("5@g.us", "Lain"))
	require.NoError(t, err)

	mine, err := f.messages.Queue(GroupOutgoing{GroupID: f.group.ID, SenderUserID: uuid.New(), Kind: models.MessageKindText, Body: "a"})
	require.NoError(t, err)
	_, err = f.messages.Queue(GroupOutgoing{GroupID: otherGroup.ID, SenderUserID: uuid.New(), Kind: models.MessageKindText, Body: "b"})
	require.NoError(t, err)

	claimed, err := f.messages.ClaimQueued(f.account.ID, 10)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	assert.Equal(t, mine.ID, claimed[0].ID)
	assert.Equal(t, "1@g.us", claimed[0].GroupJID)
	assert.Equal(t, 0, f.reload(t).UnreadCount, "what we send is not unread")

	require.NoError(t, f.messages.MarkSent(mine.ID, "WSENT", nil))
	claimed, err = f.messages.ClaimQueued(f.account.ID, 10)
	require.NoError(t, err)
	assert.Empty(t, claimed)
}
