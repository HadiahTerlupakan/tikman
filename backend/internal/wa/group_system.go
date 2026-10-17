package wa

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"go.uber.org/zap"
)

// selfLabel is how the paired number is named in a notice, as a phone names
// its own owner.
const selfLabel = "Anda"

// labeller names a member in a notice.
type labeller func(types.JID) string

// systemNotices words what one group change did, the sentences a phone puts
// in the middle of the chat. The actor is who made the change; a change
// WhatsApp reports without one is worded without a subject.
func systemNotices(evt *events.GroupInfo, label labeller) []string {
	who := "Seseorang"
	if evt.Sender != nil {
		who = label(*evt.Sender)
	}
	var notices []string
	notices = append(notices, membershipNotices(evt, who, label)...)
	if evt.Name != nil {
		notices = append(notices, fmt.Sprintf("%s mengubah nama grup menjadi \"%s\"", who, evt.Name.Name))
	}
	if evt.Topic != nil {
		verb := "mengubah deskripsi grup"
		if evt.Topic.TopicDeleted {
			verb = "menghapus deskripsi grup"
		}
		notices = append(notices, who+" "+verb)
	}
	notices = append(notices, settingNotices(evt, who)...)
	return notices
}

func membershipNotices(evt *events.GroupInfo, who string, label labeller) []string {
	var notices []string
	if joined := othersThan(evt.Join, evt.Sender); len(joined) > 0 {
		notices = append(notices, fmt.Sprintf("%s menambahkan %s", who, names(joined, label)))
	}
	if len(evt.Join) > 0 && evt.Sender != nil && contains(evt.Join, *evt.Sender) {
		via := "bergabung"
		if evt.JoinReason == "invite" {
			via = "bergabung lewat tautan undangan"
		}
		notices = append(notices, who+" "+via)
	}
	if removed := othersThan(evt.Leave, evt.Sender); len(removed) > 0 {
		notices = append(notices, fmt.Sprintf("%s mengeluarkan %s", who, names(removed, label)))
	}
	if len(evt.Leave) > 0 && evt.Sender != nil && contains(evt.Leave, *evt.Sender) {
		notices = append(notices, who+" keluar")
	}
	if len(evt.Promote) > 0 {
		notices = append(notices, fmt.Sprintf("%s menjadikan %s admin", who, names(evt.Promote, label)))
	}
	if len(evt.Demote) > 0 {
		notices = append(notices, fmt.Sprintf("%s mencabut status admin %s", who, names(evt.Demote, label)))
	}
	return notices
}

func settingNotices(evt *events.GroupInfo, who string) []string {
	var notices []string
	if evt.Announce != nil {
		setting := "semua peserta dapat mengirim pesan"
		if evt.Announce.IsAnnounce {
			setting = "hanya admin yang dapat mengirim pesan"
		}
		notices = append(notices, fmt.Sprintf("%s mengubah setelan grup: %s", who, setting))
	}
	if evt.Locked != nil {
		setting := "semua peserta dapat mengedit info grup"
		if evt.Locked.IsLocked {
			setting = "hanya admin yang dapat mengedit info grup"
		}
		notices = append(notices, fmt.Sprintf("%s mengubah setelan grup: %s", who, setting))
	}
	if evt.Ephemeral != nil {
		state := "mematikan pesan sementara"
		if evt.Ephemeral.IsEphemeral {
			state = "menyalakan pesan sementara"
		}
		notices = append(notices, who+" "+state)
	}
	if evt.NewInviteLink != nil {
		notices = append(notices, who+" mengatur ulang tautan undangan grup")
	}
	return notices
}

func othersThan(jids []types.JID, sender *types.JID) []types.JID {
	if sender == nil {
		return jids
	}
	out := make([]types.JID, 0, len(jids))
	for _, jid := range jids {
		if jid.User != sender.User {
			out = append(out, jid)
		}
	}
	return out
}

func contains(jids []types.JID, target types.JID) bool {
	for _, jid := range jids {
		if jid.User == target.User {
			return true
		}
	}
	return false
}

func names(jids []types.JID, label labeller) string {
	parts := make([]string, len(jids))
	for i, jid := range jids {
		parts[i] = label(jid)
	}
	return strings.Join(parts, ", ")
}

// memberLabeller names members from the member list as it stood before the
// change, so someone who just left is still named. The number itself is
// "Anda"; a stranger is their phone number, or their LID when that is all
// WhatsApp gave.
func memberLabeller(members []models.WAGroupParticipant, self selfAddresses) labeller {
	return func(jid types.JID) string {
		if self.is(jid) {
			return selfLabel
		}
		for _, member := range members {
			if !memberIs(member, jid) {
				continue
			}
			for _, name := range []string{member.Name, member.PushName} {
				if name != "" {
					return name
				}
			}
			if member.Phone != "" {
				return "+" + member.Phone
			}
		}
		return "+" + jid.User
	}
}

// recordNotices stores the sentences as system messages. Their ids are made
// from the change's own time so a notice WhatsApp delivers twice is stored
// once.
func (c *Client) recordNotices(ctx context.Context, groupID uuid.UUID, notices []string, at time.Time) {
	for i, notice := range notices {
		msg, stored, err := c.inbound.groupMessages.SaveArrived(services.GroupArrival{
			GroupID:     groupID,
			WAMessageID: fmt.Sprintf("sys-%d-%d", at.UnixNano(), i),
			Direction:   models.MessageIn,
			Kind:        models.MessageKindSystem,
			Body:        notice,
			At:          at,
		})
		if err != nil {
			c.logger.Warn("Could not record a group notice", zap.Error(err))
			continue
		}
		if stored {
			// Best effort, like every announcement: the next refetch closes a gap.
			_ = c.publisher.Publish(ctx, Event{Type: EventGroupMessage, GroupID: groupID.String(), MessageID: msg.ID.String()})
		}
	}
}
