package wa

import (
	"context"
	"fmt"
	"strings"

	"github.com/tikman/olt-provisioning/internal/models"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

// participantProblems is what WhatsApp's per-member error codes mean, in the
// words an admin can act on. Only the codes whose meaning is settled are
// named; any other is shown as its number rather than guessed at.
var participantProblems = map[int]string{
	403: "tidak bisa ditambahkan langsung karena pengaturan privasinya; kirim tautan undangan",
	408: "baru saja keluar dari grup dan belum bisa ditambahkan lagi",
	409: "sudah menjadi anggota",
}

var participantChanges = map[models.GroupActionKind]whatsmeow.ParticipantChange{
	models.GroupActionAdd:     whatsmeow.ParticipantChangeAdd,
	models.GroupActionRemove:  whatsmeow.ParticipantChangeRemove,
	models.GroupActionPromote: whatsmeow.ParticipantChangePromote,
	models.GroupActionDemote:  whatsmeow.ParticipantChangeDemote,
}

// CreateGroup makes a group and answers its JID with the members WhatsApp
// would not add.
func (c *Client) CreateGroup(ctx context.Context, name string, participants []string) (string, string, error) {
	jids, err := parseJIDs(participants)
	if err != nil {
		return "", "", err
	}
	info, err := c.wa.CreateGroup(ctx, whatsmeow.ReqCreateGroup{Name: name, Participants: jids})
	if err != nil {
		return "", "", err
	}
	return info.JID.ToNonAD().String(), describeProblems(info.Participants), nil
}

// RenameGroup changes a group's name.
func (c *Client) RenameGroup(ctx context.Context, jid, name string) error {
	group, err := types.ParseJID(jid)
	if err != nil {
		return err
	}
	return c.wa.SetGroupName(ctx, group, name)
}

// SetGroupTopic changes a group's description. The previous id is passed when
// the mirror has one, which spares whatsmeow a round trip to look it up.
func (c *Client) SetGroupTopic(ctx context.Context, jid, previousTopicID, topic string) error {
	group, err := types.ParseJID(jid)
	if err != nil {
		return err
	}
	return c.wa.SetGroupTopic(ctx, group, previousTopicID, "", topic)
}

// ChangeParticipants adds, removes, promotes or demotes members, answering
// the ones WhatsApp refused and why.
func (c *Client) ChangeParticipants(
	ctx context.Context, jid string, participants []string, change models.GroupActionKind,
) (string, error) {
	group, err := types.ParseJID(jid)
	if err != nil {
		return "", err
	}
	verb, ok := participantChanges[change]
	if !ok {
		return "", fmt.Errorf("perubahan anggota tidak dikenal %q", change)
	}
	jids, err := parseJIDs(participants)
	if err != nil {
		return "", err
	}
	answered, err := c.wa.UpdateGroupParticipants(ctx, group, jids, verb)
	if err != nil {
		return "", err
	}
	return describeProblems(answered), nil
}

// SetGroupAnnounce sets whether only admins may send.
func (c *Client) SetGroupAnnounce(ctx context.Context, jid string, on bool) error {
	group, err := types.ParseJID(jid)
	if err != nil {
		return err
	}
	return c.wa.SetGroupAnnounce(ctx, group, on)
}

// SetGroupLocked sets whether only admins may edit the group's details.
func (c *Client) SetGroupLocked(ctx context.Context, jid string, on bool) error {
	group, err := types.ParseJID(jid)
	if err != nil {
		return err
	}
	return c.wa.SetGroupLocked(ctx, group, on)
}

// InviteLink answers the group's invite link, revoking the old one first when
// reset is set.
func (c *Client) InviteLink(ctx context.Context, jid string, reset bool) (string, error) {
	group, err := types.ParseJID(jid)
	if err != nil {
		return "", err
	}
	return c.wa.GetGroupInviteLink(ctx, group, reset)
}

// LeaveGroup takes the number out of a group.
func (c *Client) LeaveGroup(ctx context.Context, jid string) error {
	group, err := types.ParseJID(jid)
	if err != nil {
		return err
	}
	return c.wa.LeaveGroup(ctx, group)
}

// RefreshGroupJID re-reads one group into the mirror.
func (c *Client) RefreshGroupJID(ctx context.Context, jid string) error {
	group, err := types.ParseJID(jid)
	if err != nil {
		return err
	}
	_, err = c.RefreshGroup(ctx, group)
	return err
}

func parseJIDs(raw []string) ([]types.JID, error) {
	jids := make([]types.JID, 0, len(raw))
	for _, s := range raw {
		jid, err := types.ParseJID(s)
		if err != nil {
			return nil, fmt.Errorf("anggota tidak valid %q: %w", s, err)
		}
		jids = append(jids, jid)
	}
	return jids, nil
}

// describeProblems lists the members WhatsApp answered with an error code,
// one per line, empty when everyone went through.
func describeProblems(answered []types.GroupParticipant) string {
	var lines []string
	for _, p := range answered {
		if p.Error == 0 {
			continue
		}
		who := p.JID.User
		if !p.PhoneNumber.IsEmpty() {
			who = p.PhoneNumber.User
		}
		reason, known := participantProblems[p.Error]
		if !known {
			reason = fmt.Sprintf("ditolak WhatsApp (kode %d)", p.Error)
		}
		lines = append(lines, who+": "+reason)
	}
	return strings.Join(lines, "\n")
}
