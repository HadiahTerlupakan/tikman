package wa

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
)

// GroupAdmin is the part of the WhatsApp connection that manages groups. Its
// own interface so the runner — which request maps to which call, what is
// recorded afterwards — can be tested without a connection.
type GroupAdmin interface {
	CreateGroup(ctx context.Context, name string, participants []string) (jid, problems string, err error)
	RenameGroup(ctx context.Context, jid, name string) error
	SetGroupTopic(ctx context.Context, jid, previousTopicID, topic string) error
	ChangeParticipants(ctx context.Context, jid string, participants []string, change models.GroupActionKind) (problems string, err error)
	SetGroupAnnounce(ctx context.Context, jid string, on bool) error
	SetGroupLocked(ctx context.Context, jid string, on bool) error
	InviteLink(ctx context.Context, jid string, reset bool) (string, error)
	LeaveGroup(ctx context.Context, jid string) error
	// RefreshGroupJID re-reads a group into the mirror after it changed.
	RefreshGroupJID(ctx context.Context, jid string) error
	MessageAdmin
}

// errGroupLeft is an action on a group the number is no longer in.
var errGroupLeft = errors.New("nomor ini sudah tidak menjadi anggota grup")

// GroupActionRunner carries out the management requests waiting on one
// number. One run at a time, for the reason the drainers give: the claim does
// not lock, and a request carried out twice adds or removes someone twice.
type GroupActionRunner struct {
	mu        sync.Mutex
	accountID uuid.UUID
	actions   *services.WAGroupActionService
	groups    *services.WAGroupService
	messages  *services.WAGroupMessageService
	admin     GroupAdmin
	publisher announcer
	mediaRoot string
}

// NewGroupActionRunner constructs a GroupActionRunner.
func NewGroupActionRunner(
	accountID uuid.UUID,
	actions *services.WAGroupActionService,
	groups *services.WAGroupService,
	messages *services.WAGroupMessageService,
	admin GroupAdmin,
	publisher announcer,
	mediaRoot string,
) *GroupActionRunner {
	return &GroupActionRunner{
		accountID: accountID, actions: actions, groups: groups, messages: messages,
		admin: admin, publisher: publisher, mediaRoot: mediaRoot,
	}
}

// Run carries out what is waiting and answers how many WhatsApp accepted. A
// refusal is recorded with WhatsApp's reason and the next request still runs.
func (r *GroupActionRunner) Run(ctx context.Context, limit int) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	waiting, err := r.actions.ClaimQueued(r.accountID, limit)
	if err != nil {
		return 0, err
	}
	done := 0
	for i := range waiting {
		action := &waiting[i]
		result, groupID, err := r.carryOut(ctx, action)
		if err != nil {
			if markErr := r.actions.MarkFailed(action.ID, err.Error()); markErr != nil {
				return done, markErr
			}
		} else {
			if markErr := r.actions.MarkDone(action.ID, result, groupID); markErr != nil {
				return done, markErr
			}
			done++
		}
		r.announce(ctx, action, groupID)
	}
	return done, nil
}

// carryOut runs one request, answering what to show the admin and, for a
// create, the group it made.
func (r *GroupActionRunner) carryOut(ctx context.Context, action *models.WAGroupAction) (string, *uuid.UUID, error) {
	args, err := r.actions.Args(action)
	if err != nil {
		return "", nil, err
	}
	if action.Action == models.GroupActionCreate {
		return r.create(ctx, args)
	}
	if action.GroupID == nil {
		return "", nil, services.ErrGroupActionNeedsGroup
	}
	group, err := r.groups.Get(*action.GroupID)
	if err != nil {
		return "", nil, fmt.Errorf("grup tidak ditemukan: %w", err)
	}
	if group.Left {
		return "", nil, errGroupLeft
	}

	result, err := r.apply(ctx, action.Action, group, args)
	if err != nil {
		return "", nil, err
	}
	r.afterwards(ctx, action.Action, group)
	return result, nil, nil
}

func (r *GroupActionRunner) create(ctx context.Context, args services.GroupActionArgs) (string, *uuid.UUID, error) {
	jid, problems, err := r.admin.CreateGroup(ctx, args.Name, args.Participants)
	if err != nil {
		return "", nil, err
	}
	// WhatsApp also announces the new group as a JoinedGroup event, which may
	// already have stored it; asking for the row covers either order. The
	// group exists whatever happens next, so failing to read it back costs the
	// link from this action to it, not the action.
	if err := r.admin.RefreshGroupJID(ctx, jid); err != nil {
		return problems, nil, nil
	}
	group, _, err := r.groups.FindOrCreate(r.accountID, jid)
	if err != nil {
		return problems, nil, nil
	}
	return problems, &group.ID, nil
}

func (r *GroupActionRunner) apply(
	ctx context.Context, kind models.GroupActionKind, group *models.WAGroup, args services.GroupActionArgs,
) (string, error) {
	switch kind {
	case models.GroupActionRename:
		return "", r.admin.RenameGroup(ctx, group.JID, args.Name)
	case models.GroupActionSetTopic:
		return "", r.admin.SetGroupTopic(ctx, group.JID, group.TopicID, args.Topic)
	case models.GroupActionAdd, models.GroupActionRemove, models.GroupActionPromote, models.GroupActionDemote:
		return r.admin.ChangeParticipants(ctx, group.JID, args.Participants, kind)
	case models.GroupActionSetAnnounce:
		return "", r.admin.SetGroupAnnounce(ctx, group.JID, args.Enabled)
	case models.GroupActionSetLocked:
		return "", r.admin.SetGroupLocked(ctx, group.JID, args.Enabled)
	case models.GroupActionInviteLink, models.GroupActionResetInviteLink:
		return r.admin.InviteLink(ctx, group.JID, kind == models.GroupActionResetInviteLink)
	case models.GroupActionLeave:
		return "", r.admin.LeaveGroup(ctx, group.JID)
	default:
		return "", r.applyToMessage(ctx, kind, group, args)
	}
}

// afterwards brings the mirror in line with what was just changed, so the
// admin sees it without waiting for the hourly sync. A failure here costs
// freshness only; the action itself already succeeded.
func (r *GroupActionRunner) afterwards(ctx context.Context, kind models.GroupActionKind, group *models.WAGroup) {
	switch kind {
	case models.GroupActionLeave:
		_ = r.groups.MarkLeft(r.accountID, group.JID)
	case models.GroupActionInviteLink, models.GroupActionResetInviteLink,
		models.GroupActionReact, models.GroupActionRevoke, models.GroupActionEdit,
		models.GroupActionVote, models.GroupActionHistory:
	default:
		_ = r.admin.RefreshGroupJID(ctx, group.JID)
	}
}

func (r *GroupActionRunner) announce(ctx context.Context, action *models.WAGroupAction, created *uuid.UUID) {
	if r.publisher == nil {
		return
	}
	event := Event{Type: EventGroupAction, ActionID: action.ID.String()}
	switch {
	case created != nil:
		event.GroupID = created.String()
	case action.GroupID != nil:
		event.GroupID = action.GroupID.String()
	}
	_ = r.publisher.Publish(ctx, event)
	// A message action changed the thread; its browsers refetch on this.
	switch action.Action {
	case models.GroupActionReact, models.GroupActionRevoke, models.GroupActionEdit, models.GroupActionVote:
		_ = r.publisher.Publish(ctx, Event{Type: EventGroupMessage, GroupID: event.GroupID})
	}
	if action.Action == models.GroupActionLeave {
		_ = r.publisher.Publish(ctx, Event{Type: EventGroup, GroupID: event.GroupID})
	}
}
