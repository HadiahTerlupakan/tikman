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
		GroupID: f.group.ID, SenderUserID: uuid.New(), Kind: models.MessageKindText,
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

func TestCheckMentionsRefusesSomeoneOutsideTheGroup(t *testing.T) {
	f := newGroupFixture(t)
	withJoko(t, f)

	require.NoError(t, f.groups.CheckMentions(f.group.ID, []string{jokoLID}))
	err := f.groups.CheckMentions(f.group.ID, []string{jokoLID, "628999@s.whatsapp.net"})
	assert.True(t, errors.Is(err, ErrValidation), "got %v", err)
}
