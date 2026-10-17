package wa

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
)

type adminCall struct {
	name         string
	jid          string
	participants []string
	change       models.GroupActionKind
}

type fakeGroupAdmin struct {
	calls     []adminCall
	refreshed []string
	err       error
	problems  string
	createJID string
}

func (f *fakeGroupAdmin) record(call adminCall) error {
	f.calls = append(f.calls, call)
	return f.err
}

func (f *fakeGroupAdmin) CreateGroup(_ context.Context, name string, participants []string) (string, string, error) {
	return f.createJID, f.problems, f.record(adminCall{name: "create:" + name, participants: participants})
}

func (f *fakeGroupAdmin) RenameGroup(_ context.Context, jid, name string) error {
	return f.record(adminCall{name: "rename:" + name, jid: jid})
}

func (f *fakeGroupAdmin) SetGroupTopic(_ context.Context, jid, _, topic string) error {
	return f.record(adminCall{name: "topic:" + topic, jid: jid})
}

func (f *fakeGroupAdmin) ChangeParticipants(
	_ context.Context, jid string, participants []string, change models.GroupActionKind,
) (string, error) {
	return f.problems, f.record(adminCall{name: "members", jid: jid, participants: participants, change: change})
}

func (f *fakeGroupAdmin) SetGroupAnnounce(_ context.Context, jid string, on bool) error {
	return f.record(adminCall{name: map[bool]string{true: "announce:on", false: "announce:off"}[on], jid: jid})
}

func (f *fakeGroupAdmin) SetGroupLocked(_ context.Context, jid string, _ bool) error {
	return f.record(adminCall{name: "locked", jid: jid})
}

func (f *fakeGroupAdmin) InviteLink(_ context.Context, jid string, reset bool) (string, error) {
	return "https://chat.whatsapp.com/abc", f.record(adminCall{name: map[bool]string{true: "reset", false: "link"}[reset], jid: jid})
}

func (f *fakeGroupAdmin) LeaveGroup(_ context.Context, jid string) error {
	return f.record(adminCall{name: "leave", jid: jid})
}

func (f *fakeGroupAdmin) RefreshGroupJID(_ context.Context, jid string) error {
	f.refreshed = append(f.refreshed, jid)
	return nil
}

func queueAction(t *testing.T, env groupDrainEnv, req services.GroupActionRequest) *models.WAGroupAction {
	t.Helper()
	req.AccountID = env.account.ID
	req.UserID = uuid.New()
	if req.Action != models.GroupActionCreate && req.GroupID == nil {
		req.GroupID = &env.group.ID
	}
	action, err := services.NewWAGroupActionService(env.db).Queue(req)
	require.NoError(t, err)
	return action
}

func runActions(t *testing.T, env groupDrainEnv, admin *fakeGroupAdmin, announced *recordingAnnouncer) {
	t.Helper()
	runner := NewGroupActionRunner(env.account.ID, services.NewWAGroupActionService(env.db), env.groups, admin, announced)
	_, err := runner.Run(context.Background(), 10)
	require.NoError(t, err)
}

func actionAfter(t *testing.T, env groupDrainEnv, id uuid.UUID) *models.WAGroupAction {
	t.Helper()
	action, err := services.NewWAGroupActionService(env.db).Get(id)
	require.NoError(t, err)
	return action
}

func TestAddingMembersReachesWhatsAppAndRecordsWhoWasRefused(t *testing.T) {
	env := groupDrainSetup(t)
	action := queueAction(t, env, services.GroupActionRequest{
		Action: models.GroupActionAdd, Args: services.GroupActionArgs{Participants: []string{"081234567890"}},
	})
	admin := &fakeGroupAdmin{problems: "6281234567890: sudah menjadi anggota"}
	announced := &recordingAnnouncer{}

	runActions(t, env, admin, announced)

	require.Len(t, admin.calls, 1)
	assert.Equal(t, teamGroup.String(), admin.calls[0].jid)
	assert.Equal(t, []string{"6281234567890@s.whatsapp.net"}, admin.calls[0].participants)
	assert.Equal(t, models.GroupActionAdd, admin.calls[0].change)
	assert.Equal(t, []string{teamGroup.String()}, admin.refreshed, "the member list is re-read so the admin sees the change")

	done := actionAfter(t, env, action.ID)
	assert.Equal(t, models.GroupActionDone, done.Status)
	assert.Equal(t, "6281234567890: sudah menjadi anggota", done.Result)
	require.NotEmpty(t, announced.events)
	assert.Equal(t, EventGroupAction, announced.events[0].Type)
	assert.Equal(t, action.ID.String(), announced.events[0].ActionID)
}

func TestARefusalIsRecordedWithWhatsAppsReason(t *testing.T) {
	env := groupDrainSetup(t)
	action := queueAction(t, env, services.GroupActionRequest{
		Action: models.GroupActionRename, Args: services.GroupActionArgs{Name: "Baru"},
	})

	runActions(t, env, &fakeGroupAdmin{err: errors.New("not-authorized")}, &recordingAnnouncer{})

	failed := actionAfter(t, env, action.ID)
	assert.Equal(t, models.GroupActionFailed, failed.Status)
	assert.Equal(t, "not-authorized", failed.FailReason)
}

func TestLeavingMarksTheGroupLeftAndKeepsItsHistory(t *testing.T) {
	env := groupDrainSetup(t)
	action := queueAction(t, env, services.GroupActionRequest{Action: models.GroupActionLeave})
	admin := &fakeGroupAdmin{}

	runActions(t, env, admin, &recordingAnnouncer{})

	assert.Equal(t, models.GroupActionDone, actionAfter(t, env, action.ID).Status)
	group, err := env.groups.Get(env.group.ID)
	require.NoError(t, err)
	assert.True(t, group.Left)
	assert.Empty(t, admin.refreshed, "a group we left cannot be re-read")
}

func TestNothingIsAskedOfWhatsAppForAGroupAlreadyLeft(t *testing.T) {
	env := groupDrainSetup(t)
	require.NoError(t, env.groups.MarkLeft(env.account.ID, teamGroup.String()))
	action := queueAction(t, env, services.GroupActionRequest{Action: models.GroupActionInviteLink})
	admin := &fakeGroupAdmin{}

	runActions(t, env, admin, &recordingAnnouncer{})

	assert.Empty(t, admin.calls)
	assert.Equal(t, models.GroupActionFailed, actionAfter(t, env, action.ID).Status)
}

func TestTheInviteLinkIsTheActionsResult(t *testing.T) {
	env := groupDrainSetup(t)
	action := queueAction(t, env, services.GroupActionRequest{Action: models.GroupActionResetInviteLink})
	admin := &fakeGroupAdmin{}

	runActions(t, env, admin, &recordingAnnouncer{})

	require.Len(t, admin.calls, 1)
	assert.Equal(t, "reset", admin.calls[0].name)
	assert.Equal(t, "https://chat.whatsapp.com/abc", actionAfter(t, env, action.ID).Result)
}

func TestACreatedGroupIsLinkedToTheActionThatMadeIt(t *testing.T) {
	env := groupDrainSetup(t)
	action := queueAction(t, env, services.GroupActionRequest{
		Action: models.GroupActionCreate,
		Args:   services.GroupActionArgs{Name: "Teknisi Baru", Participants: []string{"081234567890"}},
	})
	admin := &fakeGroupAdmin{createJID: "120363000000000099@g.us"}

	runActions(t, env, admin, &recordingAnnouncer{})

	done := actionAfter(t, env, action.ID)
	assert.Equal(t, models.GroupActionDone, done.Status)
	require.NotNil(t, done.GroupID)
	created, err := env.groups.Get(*done.GroupID)
	require.NoError(t, err)
	assert.Equal(t, "120363000000000099@g.us", created.JID)
}
