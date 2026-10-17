package services

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

func TestQueueingACreateStoresTheMembersAsJIDsOnceEach(t *testing.T) {
	f := newGroupFixture(t)
	actions := NewWAGroupActionService(f.db)

	action, err := actions.Queue(GroupActionRequest{
		AccountID: f.account.ID,
		Action:    models.GroupActionCreate,
		Args: GroupActionArgs{
			Name:         "Teknisi Baru",
			Participants: []string{"0812 3456 7890", "+62 812-3456-7890", "111222333@lid"},
		},
		UserID: uuid.New(),
	})
	require.NoError(t, err)
	assert.Equal(t, models.GroupActionQueued, action.Status)

	args, err := actions.Args(action)
	require.NoError(t, err)
	assert.Equal(t, []string{"6281234567890@s.whatsapp.net", "111222333@lid"}, args.Participants)
}

func TestQueueRefusesARequestWhatsAppWouldRefuse(t *testing.T) {
	f := newGroupFixture(t)
	actions := NewWAGroupActionService(f.db)
	groupID := f.group.ID

	cases := []struct {
		name string
		req  GroupActionRequest
	}{
		{"create without a name", GroupActionRequest{Action: models.GroupActionCreate,
			Args: GroupActionArgs{Participants: []string{"081234567890"}}}},
		{"a name past 25 characters", GroupActionRequest{GroupID: &groupID, Action: models.GroupActionRename,
			Args: GroupActionArgs{Name: "Grup Teknisi Wilayah Cariu Bogor"}}},
		{"adding nobody", GroupActionRequest{GroupID: &groupID, Action: models.GroupActionAdd}},
		{"a number that is not Indonesian", GroupActionRequest{GroupID: &groupID, Action: models.GroupActionRemove,
			Args: GroupActionArgs{Participants: []string{"12025550123"}}}},
		{"an action that names no group", GroupActionRequest{Action: models.GroupActionLeave}},
		{"an action nobody knows", GroupActionRequest{GroupID: &groupID, Action: "delete_group"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.req.AccountID = f.account.ID
			tc.req.UserID = uuid.New()
			_, err := actions.Queue(tc.req)
			assert.True(t, errors.Is(err, ErrValidation), "got %v", err)
		})
	}

	var queued int64
	require.NoError(t, f.db.Model(&models.WAGroupAction{}).Count(&queued).Error)
	assert.Zero(t, queued, "a refused request must not reach the wa process")
}

func TestAnActionIsClaimedUntilItIsAnswered(t *testing.T) {
	f := newGroupFixture(t)
	actions := NewWAGroupActionService(f.db)
	groupID := f.group.ID
	action, err := actions.Queue(GroupActionRequest{
		AccountID: f.account.ID, GroupID: &groupID, Action: models.GroupActionInviteLink, UserID: uuid.New(),
	})
	require.NoError(t, err)

	claimed, err := actions.ClaimQueued(f.account.ID, 10)
	require.NoError(t, err)
	require.Len(t, claimed, 1)

	require.NoError(t, actions.MarkDone(action.ID, "https://chat.whatsapp.com/abc", nil))
	claimed, err = actions.ClaimQueued(f.account.ID, 10)
	require.NoError(t, err)
	assert.Empty(t, claimed)

	done, err := actions.Get(action.ID)
	require.NoError(t, err)
	assert.Equal(t, models.GroupActionDone, done.Status)
	assert.Equal(t, "https://chat.whatsapp.com/abc", done.Result)
	assert.NotNil(t, done.DoneAt)
}
