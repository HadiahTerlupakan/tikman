package wa

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"go.mau.fi/whatsmeow/types"
)

var (
	ourPhone = types.JID{User: "628999000111", Server: types.DefaultUserServer}
	ourLID   = types.JID{User: "77700011122233", Server: types.HiddenUserServer}
)

// In a LID-addressed group our number appears only under its LID. Matching on
// the phone number alone would say we are not admin of a group we run.
func TestGroupSnapshotRecognisesOurNumberByItsLID(t *testing.T) {
	info := &types.GroupInfo{
		JID:       teamGroup,
		GroupName: types.GroupName{Name: "Teknisi"},
		Participants: []types.GroupParticipant{
			{JID: ourLID, LID: ourLID, IsAdmin: true},
			{JID: types.JID{User: "55511122233344", Server: types.HiddenUserServer},
				PhoneNumber: types.JID{User: "628133334444", Server: types.DefaultUserServer}},
		},
	}

	snap := groupSnapshot(info, selfAddresses{phone: ourPhone, lid: ourLID}, nil)

	assert.True(t, snap.IsAdmin)
	require.Len(t, snap.Participants, 2)
	assert.True(t, snap.Participants[0].IsSelf)
	assert.Equal(t, models.GroupRoleAdmin, snap.Participants[0].Role)
	assert.False(t, snap.Participants[1].IsSelf)
	assert.Equal(t, "628133334444", snap.Participants[1].Phone)
}

func TestGroupSnapshotNamesMembersFromTheAddressBook(t *testing.T) {
	member := types.JID{User: "628133334444", Server: types.DefaultUserServer}
	info := &types.GroupInfo{
		JID:          teamGroup,
		Participants: []types.GroupParticipant{{JID: member, PhoneNumber: member, IsSuperAdmin: true}},
	}

	snap := groupSnapshot(info, selfAddresses{phone: ourPhone}, map[types.JID]string{member: "Pak Joko"})

	require.Len(t, snap.Participants, 1)
	assert.Equal(t, "Pak Joko", snap.Participants[0].Name)
	assert.Equal(t, models.GroupRoleSuperAdmin, snap.Participants[0].Role)
	assert.False(t, snap.IsAdmin, "a group where someone else is admin is not ours to manage")
}

func TestDescribeProblemsNamesOnlyTheMembersWhatsAppRefused(t *testing.T) {
	problems := describeProblems([]types.GroupParticipant{
		{JID: types.JID{User: "628111", Server: types.DefaultUserServer}},
		{JID: types.JID{User: "628222", Server: types.DefaultUserServer}, Error: 409},
		{JID: types.JID{User: "628333", Server: types.DefaultUserServer}, Error: 999},
	})

	assert.Equal(t, "628222: sudah menjadi anggota\n628333: ditolak WhatsApp (kode 999)", problems)
}
