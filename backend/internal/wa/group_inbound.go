package wa

import (
	"context"

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

	members := h.members(group.ID)
	h.notePushName(group.ID, evt.Info)
	if handled, err := h.groupChange(ctx, evt, group.ID, members); handled {
		return err
	}
	att, keep := h.groupShape(evt)
	if !keep {
		return nil
	}
	return h.storeGroupMessage(ctx, evt, group.ID, att, members)
}

func (h *inboundHandler) storeGroupMessage(
	ctx context.Context, evt *events.Message, groupID uuid.UUID, att attachment, members []models.WAGroupParticipant,
) error {
	names := services.MemberNames(members)
	mentions := mentionedJIDs(evt.Message)
	body, media := h.fetch(ctx, evt, att)
	msg, stored, err := h.groupMessages.SaveArrived(services.GroupArrival{
		GroupID:        groupID,
		WAMessageID:    evt.Info.ID,
		Direction:      groupDirection(evt.Info),
		SenderJID:      evt.Info.Sender.ToNonAD().String(),
		SenderPhone:    senderPhoneJID(evt.Info.MessageSource).User,
		SenderName:     bookName(evt.Info, members),
		SenderPushName: evt.Info.PushName,
		Kind:           att.kind,
		Body:           services.ResolveMentions(body, mentions, names),
		Media:          media,
		ReplyToWAID:    quotedStanzaID(evt.Message),
		Preview:        inboundPreview(evt.Message),
		At:             evt.Info.Timestamp,
		Mentions:       mentions,
		Extra:          att.extra,
		Forwarded:      isForwarded(evt.Message),
	})
	if err != nil || !stored {
		// Same reasoning as store: a failed save committed nothing, and a
		// re-delivery's row already names its own copy of the file.
		h.discard(evt, media)
		return err
	}

	h.announceGroup(ctx, Event{Type: EventGroupMessage, GroupID: groupID.String(), MessageID: msg.ID.String()})
	return nil
}

func groupDirection(info types.MessageInfo) models.MessageDirection {
	if info.IsFromMe {
		return models.MessageOut
	}
	return models.MessageIn
}

// members answers the group's member list. A failure costs the names, not
// the message.
func (h *inboundHandler) members(groupID uuid.UUID) []models.WAGroupParticipant {
	members, err := h.groups.Participants(groupID)
	if err != nil {
		h.logger.Warn("Could not read a group's members to name a sender", zap.Error(err))
		return nil
	}
	return members
}

// notePushName keeps the sender's profile name on their member row, which is
// how a member nobody saved still gets a name in the list and in mentions.
func (h *inboundHandler) notePushName(groupID uuid.UUID, info types.MessageInfo) {
	if info.IsFromMe || info.PushName == "" {
		return
	}
	addresses := []string{info.Sender.ToNonAD().String()}
	if !info.SenderAlt.IsEmpty() {
		addresses = append(addresses, info.SenderAlt.ToNonAD().String())
	}
	if err := h.groups.NotePushName(groupID, addresses, info.PushName); err != nil {
		h.logger.Warn("Could not record a member's profile name", zap.Error(err))
	}
}

// bookName is what the number's address book calls the sender, "" when it
// does not know them: the page then shows their profile name as "~Name",
// the way a phone does.
func bookName(info types.MessageInfo, members []models.WAGroupParticipant) string {
	for _, member := range members {
		if member.Name != "" && memberIs(member, info.Sender, info.SenderAlt) {
			return member.Name
		}
	}
	return ""
}

func memberIs(member models.WAGroupParticipant, jids ...types.JID) bool {
	for _, jid := range jids {
		if jid.IsEmpty() {
			continue
		}
		address := jid.ToNonAD().String()
		if address == member.JID || address == member.LID || jid.User == member.Phone {
			return true
		}
	}
	return false
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

// announceGroup tells the browsers about group traffic. Like announce, a
// failure is logged and nothing more: the next refetch closes the gap.
func (h *inboundHandler) announceGroup(ctx context.Context, event Event) {
	if err := h.publisher.Publish(ctx, event); err != nil {
		h.logger.Warn("Could not announce a group change",
			zap.String("group_id", event.GroupID), zap.Error(err))
	}
}
