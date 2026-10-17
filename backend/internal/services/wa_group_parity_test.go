package services

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

// Push names are learnt one message at a time; the hourly sync must not
// forget them.
func TestAPushNameSurvivesTheSync(t *testing.T) {
	f := newGroupFixture(t)
	require.NoError(t, f.groups.NotePushName(f.group.ID, []string{"62811@s.whatsapp.net"}, "Budi Lapangan"))

	_, err := f.groups.Upsert(f.account.ID, groupSnap("1@g.us", "Teknisi", "62811"))
	require.NoError(t, err)

	members, err := f.groups.Participants(f.group.ID)
	require.NoError(t, err)
	require.Len(t, members, 1)
	assert.Equal(t, "Budi Lapangan", members[0].PushName)
}

func TestAPinnedGroupIsListedFirst(t *testing.T) {
	f := newGroupFixture(t)
	quiet, err := f.groups.Upsert(f.account.ID, groupSnap("2@g.us", "Sepi"))
	require.NoError(t, err)
	_, _, err = f.messages.SaveArrived(arrival(f.group.ID, "W1", "ramai"))
	require.NoError(t, err)

	pinned, err := f.groups.SetPinned(quiet.ID, true)
	require.NoError(t, err)
	assert.True(t, pinned.Pinned)

	groups, err := f.groups.List(GroupFilter{})
	require.NoError(t, err)
	assert.Equal(t, quiet.ID, groups[0].ID)
}

// History fetched from the phone is older than what the list shows. It must
// not become the "last message", nor count as unread.
func TestAnOlderMessageMovesNeitherTheSummaryNorTheUnreadCount(t *testing.T) {
	f := newGroupFixture(t)
	_, _, err := f.messages.SaveArrived(arrival(f.group.ID, "W2", "terbaru"))
	require.NoError(t, err)
	old := arrival(f.group.ID, "W1", "lama sekali")
	old.At = time.Now().Add(-48 * time.Hour)

	_, _, err = f.messages.SaveArrived(old)
	require.NoError(t, err)

	group := f.reload(t)
	assert.Equal(t, "terbaru", group.LastMessagePreview)
	assert.Equal(t, 1, group.UnreadCount)
}

func TestASystemNoticeIsNotUnread(t *testing.T) {
	f := newGroupFixture(t)
	notice := arrival(f.group.ID, "sys-1-0", "Budi menambahkan Rina")
	notice.Kind = models.MessageKindSystem

	_, _, err := f.messages.SaveArrived(notice)
	require.NoError(t, err)

	assert.Zero(t, f.reload(t).UnreadCount)
}

func TestReactionsAreOnePerMemberAndSummarisedByEmoji(t *testing.T) {
	f := newGroupFixture(t)
	msg, _, err := f.messages.SaveArrived(arrival(f.group.ID, "W1", "ODP aman"))
	require.NoError(t, err)
	budi := models.GroupActor{JID: "62811@s.whatsapp.net", Name: "Budi"}
	rina := models.GroupActor{JID: "62812@s.whatsapp.net", Name: "Rina"}

	require.NoError(t, f.messages.SetReaction(msg.ID, budi, "👍"))
	require.NoError(t, f.messages.SetReaction(msg.ID, rina, "👍"))
	require.NoError(t, f.messages.SetReaction(msg.ID, budi, "❤️"))
	require.NoError(t, f.messages.SetReaction(msg.ID, rina, ""))

	history, err := f.messages.History(f.group.ID, 10, 0)
	require.NoError(t, err)
	require.Len(t, history[0].Reactions, 1, "Budi changed his, Rina took hers away")
	assert.Equal(t, "❤️", history[0].Reactions[0].Emoji)
	assert.Equal(t, 1, history[0].Reactions[0].Count)
}

// Blue ticks in a group mean every member read it, and a late "delivered"
// must not undo them.
func TestAMessageIsReadOnceEveryMemberHasReadIt(t *testing.T) {
	f := newGroupFixture(t)
	queued, err := f.messages.Queue(GroupOutgoing{
		GroupID: f.group.ID, SenderUserID: uuid.New(), Kind: models.MessageKindText, Body: "rapat jam 3", MemberCount: 2,
	})
	require.NoError(t, err)
	require.NoError(t, f.messages.MarkSent(queued.ID, "WOUT", nil))
	at := time.Now()

	_, err = f.messages.ApplyReceipt(f.group.ID, []string{"WOUT"}, "62811@s.whatsapp.net", true, at)
	require.NoError(t, err)
	assert.Equal(t, models.MessageDelivered, mustGet(t, f, queued.ID).Status, "one of two read is not read")

	_, err = f.messages.ApplyReceipt(f.group.ID, []string{"WOUT"}, "62812@s.whatsapp.net", true, at)
	require.NoError(t, err)
	assert.Equal(t, models.MessageRead, mustGet(t, f, queued.ID).Status)

	_, err = f.messages.ApplyReceipt(f.group.ID, []string{"WOUT"}, "62812@s.whatsapp.net", false, at)
	require.NoError(t, err)
	assert.Equal(t, models.MessageRead, mustGet(t, f, queued.ID).Status)

	receipts, err := f.messages.Receipts(queued.ID)
	require.NoError(t, err)
	assert.Len(t, receipts, 2)
}

func TestADeletedMessageKeepsItsPlaceButNotItsContent(t *testing.T) {
	f := newGroupFixture(t)
	photo := arrival(f.group.ID, "W1", "foto ODP")
	photo.Media = &MediaFile{Path: "2026/10/a.jpg", Mime: "image/jpeg", Filename: "a.jpg", Size: 1}
	msg, _, err := f.messages.SaveArrived(photo)
	require.NoError(t, err)

	path, err := f.messages.MarkDeleted(msg.ID)
	require.NoError(t, err)

	assert.Equal(t, "2026/10/a.jpg", path, "the caller removes the file")
	deleted := mustGet(t, f, msg.ID)
	assert.True(t, deleted.Deleted)
	assert.Empty(t, deleted.Body)
	assert.Empty(t, deleted.MediaPath)
}

func TestAPollIsTalliedFromEachMembersLatestVote(t *testing.T) {
	f := newGroupFixture(t)
	extra, err := NewPollExtra("Rapat kapan?", []string{"Senin", "Selasa"}, 1)
	require.NoError(t, err)
	poll := arrival(f.group.ID, "WPOLL", "Polling: Rapat kapan?")
	poll.Kind, poll.Extra = models.MessageKindPoll, extra
	msg, _, err := f.messages.SaveArrived(poll)
	require.NoError(t, err)
	budi := models.GroupActor{JID: "62811@s.whatsapp.net", Name: "Budi"}

	require.NoError(t, f.messages.SetVote(msg.ID, budi, []string{"Senin"}))
	require.NoError(t, f.messages.SetVote(msg.ID, budi, []string{"Selasa"}))

	history, err := f.messages.History(f.group.ID, 10, 0)
	require.NoError(t, err)
	assert.JSONEq(t, `{"question":"Rapat kapan?","selectable_count":1,"options":[
		{"name":"Senin","voters":[]},
		{"name":"Selasa","voters":[{"jid":"62811@s.whatsapp.net","name":"Budi","is_self":false}]}]}`,
		string(history[0].ExtraData))
}

func TestNewPollExtraRefusesWhatWhatsAppWouldRefuse(t *testing.T) {
	for name, options := range map[string][]string{
		"one option":        {"Ya"},
		"duplicate options": {"Ya", "Ya"},
		"thirteen options":  {"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12", "13"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := NewPollExtra("Setuju?", options, 1)
			assert.True(t, errors.Is(err, ErrValidation))
		})
	}
}

func TestStarredMessagesAreListedNewestStarFirst(t *testing.T) {
	f := newGroupFixture(t)
	first, _, err := f.messages.SaveArrived(arrival(f.group.ID, "W1", "satu"))
	require.NoError(t, err)
	second, _, err := f.messages.SaveArrived(arrival(f.group.ID, "W2", "dua"))
	require.NoError(t, err)

	_, err = f.messages.SetStarred(second.ID, true)
	require.NoError(t, err)
	starred, err := f.messages.SetStarred(first.ID, true)
	require.NoError(t, err)
	assert.True(t, starred.Starred)

	list, err := f.messages.Starred(f.group.ID)
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, first.ID, list[0].ID)
}

// The phone counts back from a message it knows; a notice is ours, not one.
func TestTheHistoryAnchorIsTheOldestRealMessage(t *testing.T) {
	f := newGroupFixture(t)
	_, err := f.messages.OldestAnchor(f.group.ID)
	assert.ErrorIs(t, err, ErrNoHistoryAnchor)

	notice := arrival(f.group.ID, "sys-1-0", "Budi keluar")
	notice.Kind, notice.At = models.MessageKindSystem, time.Now().Add(-time.Hour)
	_, _, err = f.messages.SaveArrived(notice)
	require.NoError(t, err)
	_, _, err = f.messages.SaveArrived(arrival(f.group.ID, "W1", "halo"))
	require.NoError(t, err)

	anchor, err := f.messages.OldestAnchor(f.group.ID)
	require.NoError(t, err)
	assert.Equal(t, "W1", *anchor.WAMessageID)
}

func mustGet(t *testing.T, f groupFixture, id uuid.UUID) *models.WAGroupMessage {
	t.Helper()
	msg, err := f.messages.Get(id)
	require.NoError(t, err)
	return msg
}
