package wa

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"go.uber.org/zap"
)

// isGroupChat says whether a message belongs to a group. Asked of the chat's
// server rather than IsGroup: whatsmeow sets IsGroup for the status and
// broadcast-list server too, and those are not groups anyone can read here.
func isGroupChat(info types.MessageInfo) bool {
	return info.Chat.Server == types.GroupServer
}

// handleGroup stores one message from a group, whoever sent it — a member, or
// the phone holding this number. Every group is kept: there is no customer to
// match it to, so unlike handleFromPhone nothing is filtered out.
//
// No read receipt is sent. Marking a group read tells every sender their
// message was seen, which would be the team speaking for itself without
// having looked.
func (h *inboundHandler) handleGroup(ctx context.Context, evt *events.Message) error {
	att, keep := h.readable(evt)
	if !keep {
		return nil
	}

	group, created, err := h.groups.FindOrCreate(h.accountID, evt.Info.Chat.ToNonAD().String())
	if err != nil {
		return err
	}
	if created {
		// A group the sync has not listed yet has no name to show. Off this
		// goroutine: it is a round trip, and whatsmeow holds every other event
		// for this number while a handler runs.
		go h.groupSeen(evt.Info.Chat.ToNonAD())
	}

	names := h.memberNames(group.ID)
	body, media := h.fetch(ctx, evt, att)
	msg, stored, err := h.groupMessages.SaveArrived(services.GroupArrival{
		GroupID:     group.ID,
		WAMessageID: evt.Info.ID,
		Direction:   groupDirection(evt.Info),
		SenderJID:   evt.Info.Sender.ToNonAD().String(),
		SenderPhone: senderPhoneJID(evt.Info.MessageSource).User,
		SenderName:  senderName(evt.Info, names),
		Kind:        att.kind,
		Body:        resolveMentions(body, mentionedJIDs(evt.Message), names),
		Media:       media,
		ReplyToWAID: quotedStanzaID(evt.Message),
		Preview:     inboundPreview(evt.Message),
		At:          evt.Info.Timestamp,
	})
	if err != nil || !stored {
		// Same reasoning as store: a failed save committed nothing, and a
		// re-delivery's row already names its own copy of the file.
		h.discard(evt, media)
		return err
	}

	h.announceGroup(ctx, Event{Type: EventGroupMessage, GroupID: group.ID.String(), MessageID: msg.ID.String()})
	return nil
}

func groupDirection(info types.MessageInfo) models.MessageDirection {
	if info.IsFromMe {
		return models.MessageOut
	}
	return models.MessageIn
}

// memberNames answers what each member is called, by every address WhatsApp
// may use for them. A failure costs the names, not the message.
func (h *inboundHandler) memberNames(groupID uuid.UUID) map[string]string {
	members, err := h.groups.Participants(groupID)
	if err != nil {
		h.logger.Warn("Could not read a group's members to name a sender", zap.Error(err))
		return nil
	}
	return namesByAddress(members)
}

// namesByAddress indexes members by the user part of each of their addresses,
// which is also what a mention writes after the "@".
func namesByAddress(members []models.WAGroupParticipant) map[string]string {
	names := make(map[string]string, len(members)*2)
	for _, member := range members {
		label := member.Name
		if label == "" {
			label = member.Phone
		}
		if label == "" {
			continue
		}
		for _, address := range []string{member.JID, member.LID} {
			if user := jidUser(address); user != "" {
				names[user] = label
			}
		}
		if member.Phone != "" {
			names[member.Phone] = label
		}
	}
	return names
}

func jidUser(address string) string {
	user, _, found := strings.Cut(address, "@")
	if !found {
		return ""
	}
	return user
}

// senderName is the push name the sender's phone attached, falling back to
// what the group's member list knows them as.
func senderName(info types.MessageInfo, names map[string]string) string {
	if info.PushName != "" {
		return info.PushName
	}
	for _, jid := range []types.JID{info.Sender, info.SenderAlt} {
		if name, ok := names[jid.User]; ok && jid.User != "" {
			return name
		}
	}
	return ""
}

// mentionedJIDs answers who a message tags, read from every shape that can
// carry a mention.
func mentionedJIDs(msg *waE2E.Message) []string {
	for _, ctx := range []*waE2E.ContextInfo{
		msg.GetExtendedTextMessage().GetContextInfo(),
		msg.GetImageMessage().GetContextInfo(),
		msg.GetVideoMessage().GetContextInfo(),
		msg.GetDocumentMessage().GetContextInfo(),
	} {
		if mentioned := ctx.GetMentionedJID(); len(mentioned) > 0 {
			return mentioned
		}
	}
	return nil
}

// resolveMentions rewrites "@628123…" or "@<LID>" into "@Name". A mention in a
// LID-addressed group is a long number that is nobody's phone, and read as is
// it tells the team nothing about who was asked.
func resolveMentions(body string, mentioned []string, names map[string]string) string {
	for _, jid := range mentioned {
		user := jidUser(jid)
		if name, ok := names[user]; ok && user != "" {
			body = strings.ReplaceAll(body, "@"+user, "@"+name)
		}
	}
	return body
}

// announceGroup tells the browsers about group traffic. Like announce, a
// failure is logged and nothing more: the next refetch closes the gap.
func (h *inboundHandler) announceGroup(ctx context.Context, event Event) {
	if err := h.publisher.Publish(ctx, event); err != nil {
		h.logger.Warn("Could not announce a group change",
			zap.String("group_id", event.GroupID), zap.Error(err))
	}
}
