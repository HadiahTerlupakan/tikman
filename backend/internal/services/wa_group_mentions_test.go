package services

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

const jokoLID = "55511122233344@lid"

func withJoko(t *testing.T, f groupFixture) {
	t.Helper()
	_, err := f.groups.Upsert(f.account.ID, GroupSnapshot{
		JID: "1@g.us", Name: "Teknisi",
		Participants: []models.WAGroupParticipant{{
			JID: jokoLID, LID: jokoLID, Phone: "628133334444", Name: "Pak Joko", Role: models.GroupRoleMember,
		}},
	})
	require.NoError(t, err)
}

// What goes to WhatsApp has to stay "@<address>"; what the team reads should
// say who was tagged.
func TestAMentionIsStoredForWhatsAppAndShownByName(t *testing.T) {
	f := newGroupFixture(t)
	withJoko(t, f)

	queued, err := f.messages.Queue(GroupOutgoing{
		GroupID: f.group.ID, SenderUserID: new(uuid.New()), Kind: models.MessageKindText,
		Body: "@55511122233344 tolong cek ODP 3", Mentions: []string{jokoLID, jokoLID},
	})
	require.NoError(t, err)

	assert.Equal(t, "@55511122233344 tolong cek ODP 3", queued.Body)
	assert.Equal(t, []string{jokoLID}, Mentions(queued), "a member tagged twice is listed once")

	history, err := f.messages.History(f.group.ID, 10, 0)
	require.NoError(t, err)
	require.Len(t, history, 1)
	assert.Equal(t, "@Pak Joko tolong cek ODP 3", history[0].Body)
	assert.Equal(t, "@Pak Joko tolong cek ODP 3", f.reload(t).LastMessagePreview)
}

// "@semua" tags members the group has no name for yet, such as a LID-only
// member who never posted; the bubble still has to know someone was tagged.
func TestAMessageTaggingOnlyUnnamedMembersIsMarkedTagged(t *testing.T) {
	f := newGroupFixture(t)
	const unnamed = "77788899900011@lid"
	_, err := f.groups.Upsert(f.account.ID, GroupSnapshot{
		JID: "1@g.us", Name: "Teknisi",
		Participants: []models.WAGroupParticipant{{JID: unnamed, LID: unnamed, Role: models.GroupRoleMember}},
	})
	require.NoError(t, err)
	_, err = f.messages.Queue(GroupOutgoing{
		GroupID: f.group.ID, SenderUserID: new(uuid.New()), Kind: models.MessageKindText,
		Body: "@semua rapat jam 3", Mentions: []string{unnamed},
	})
	require.NoError(t, err)
	_, err = f.messages.Queue(GroupOutgoing{
		GroupID: f.group.ID, SenderUserID: new(uuid.New()), Kind: models.MessageKindText, Body: "tanpa tag",
	})
	require.NoError(t, err)

	history, err := f.messages.History(f.group.ID, 10, 0)
	require.NoError(t, err)
	require.Len(t, history, 2)
	tagged := map[string]bool{}
	for _, m := range history {
		assert.Empty(t, m.MentionNames)
		tagged[m.Body] = m.Tagged
	}
	assert.Equal(t, map[string]bool{"@semua rapat jam 3": true, "tanpa tag": false}, tagged)
}

func TestCheckMentionsRefusesSomeoneOutsideTheGroup(t *testing.T) {
	f := newGroupFixture(t)
	withJoko(t, f)

	require.NoError(t, f.groups.CheckMentions(f.group.ID, []string{jokoLID}))
	err := f.groups.CheckMentions(f.group.ID, []string{jokoLID, "628999@s.whatsapp.net"})
	assert.True(t, errors.Is(err, ErrValidation), "got %v", err)
}
