package wa

import (
	"bytes"
	"context"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types/events"
	"go.uber.org/zap"
)

// groupChange applies the messages that change an earlier one rather than
// add to the group: a reaction, a deletion, an edit, a poll vote. It answers
// false for anything else. A change to a message this inbox never stored is
// dropped: there is nothing to change.
func (h *inboundHandler) groupChange(
	ctx context.Context, evt *events.Message, groupID uuid.UUID, members []models.WAGroupParticipant,
) (bool, error) {
	msg := evt.Message
	// Asked for presence first: REVOKE is the enum's zero value, so a message
	// with no protocol part at all would read as a deletion.
	protocol := msg.GetProtocolMessage()
	switch {
	case msg.GetReactionMessage() != nil:
		return true, h.applyReaction(ctx, evt, groupID, members)
	case protocol != nil && protocol.GetType() == waE2E.ProtocolMessage_REVOKE:
		return true, h.applyRevoke(ctx, evt, groupID, protocol.GetKey().GetID(), members)
	case protocol != nil && protocol.GetType() == waE2E.ProtocolMessage_MESSAGE_EDIT:
		return true, h.applyEdit(ctx, evt, groupID, protocol, members)
	case msg.GetPollUpdateMessage() != nil:
		return true, h.applyVote(ctx, evt, groupID, members)
	}
	return false, nil
}

// actor names whoever sent an event the way the page shows them.
func actor(evt *events.Message, members []models.WAGroupParticipant) models.GroupActor {
	name := bookName(evt.Info, members)
	if name == "" {
		name = evt.Info.PushName
	}
	return models.GroupActor{JID: evt.Info.Sender.ToNonAD().String(), Name: name, IsSelf: evt.Info.IsFromMe}
}

func (h *inboundHandler) target(groupID uuid.UUID, waID string) *models.WAGroupMessage {
	msg, err := h.groupMessages.MessageByWAID(groupID, waID)
	if err != nil {
		h.logger.Warn("Could not find the group message a change names", zap.Error(err))
		return nil
	}
	return msg
}

func (h *inboundHandler) applyReaction(
	ctx context.Context, evt *events.Message, groupID uuid.UUID, members []models.WAGroupParticipant,
) error {
	reaction := evt.Message.GetReactionMessage()
	target := h.target(groupID, reaction.GetKey().GetID())
	if target == nil {
		return nil
	}
	if err := h.groupMessages.SetReaction(target.ID, actor(evt, members), reaction.GetText()); err != nil {
		return err
	}
	h.changed(ctx, target)
	return nil
}

func (h *inboundHandler) applyRevoke(
	ctx context.Context, evt *events.Message, groupID uuid.UUID, waID string, members []models.WAGroupParticipant,
) error {
	target := h.target(groupID, waID)
	if target == nil || target.Deleted {
		return nil
	}
	if !wroteIt(evt, target, members) && !isGroupAdmin(evt, members) {
		h.refused("deletion", evt, target)
		return nil
	}
	path, err := h.groupMessages.MarkDeleted(target.ID)
	if err != nil {
		return err
	}
	if path != "" {
		if err := h.media.remove(path); err != nil {
			h.logger.Warn("Could not remove the attachment of a deleted group message", zap.Error(err))
		}
	}
	h.changed(ctx, target)
	return nil
}

func (h *inboundHandler) applyEdit(
	ctx context.Context, evt *events.Message, groupID uuid.UUID, edit *waE2E.ProtocolMessage,
	members []models.WAGroupParticipant,
) error {
	target := h.target(groupID, edit.GetKey().GetID())
	if target == nil {
		return nil
	}
	if !wroteIt(evt, target, members) {
		h.refused("edit", evt, target)
		return nil
	}
	replacement := edit.GetEditedMessage()
	body := editedText(replacement)
	mentions := mentionedJIDs(replacement)
	body = services.ResolveMentions(body, mentions, services.MemberNames(members))
	if err := h.groupMessages.MarkEdited(target.ID, body, mentions); err != nil {
		return err
	}
	h.changed(ctx, target)
	return nil
}

// editedText is the new words of an edit, which may be a caption.
func editedText(msg *waE2E.Message) string {
	if text := textBody(msg); text != "" {
		return text
	}
	for _, caption := range []string{
		msg.GetImageMessage().GetCaption(), msg.GetVideoMessage().GetCaption(), msg.GetDocumentMessage().GetCaption(),
	} {
		if caption != "" {
			return caption
		}
	}
	return ""
}

// applyVote decrypts a poll vote. WhatsApp encrypts votes with a secret only
// the poll's own message carries, which whatsmeow kept when the poll arrived;
// a vote on a poll from before this process ever saw it cannot be read.
func (h *inboundHandler) applyVote(
	ctx context.Context, evt *events.Message, groupID uuid.UUID, members []models.WAGroupParticipant,
) error {
	update := evt.Message.GetPollUpdateMessage()
	target := h.target(groupID, update.GetPollCreationMessageKey().GetID())
	if target == nil || h.wa == nil {
		return nil
	}
	vote, err := h.wa.DecryptPollVote(ctx, evt)
	if err != nil {
		h.logger.Info("Could not read a poll vote", zap.Error(err))
		return nil
	}
	poll, err := services.DecodePoll(target)
	if err != nil {
		return err
	}
	if err := h.groupMessages.SetVote(target.ID, actor(evt, members), chosenOptions(poll, vote.GetSelectedOptions())); err != nil {
		return err
	}
	h.changed(ctx, target)
	return nil
}

// chosenOptions turns the hashes a vote carries back into option names.
func chosenOptions(poll services.PollExtra, hashes [][]byte) []string {
	names := make([]string, len(poll.Options))
	for i, option := range poll.Options {
		names[i] = option.Name
	}
	known := whatsmeow.HashPollOptions(names)
	var chosen []string
	for _, hash := range hashes {
		for i, candidate := range known {
			if bytes.Equal(hash, candidate) {
				chosen = append(chosen, names[i])
			}
		}
	}
	return chosen
}

// wroteIt says whether the member sending a change wrote the message it
// names. A message id is visible to the whole group, so without this any
// member could rewrite or erase anyone's words here; phones refuse such a
// change, and so must we. Ours are changed only from our own devices.
func wroteIt(evt *events.Message, target *models.WAGroupMessage, members []models.WAGroupParticipant) bool {
	if target.Direction == models.MessageOut {
		return evt.Info.IsFromMe
	}
	if evt.Info.IsFromMe || target.SenderJID == "" {
		return false
	}
	if evt.Info.Sender.ToNonAD().String() == target.SenderJID || evt.Info.SenderAlt.ToNonAD().String() == target.SenderJID {
		return true
	}
	// The same member may be named by phone in one message and by LID in the
	// next; the member list ties the two together.
	for _, member := range members {
		if memberIs(member, evt.Info.Sender, evt.Info.SenderAlt) && (member.JID == target.SenderJID || member.LID == target.SenderJID) {
			return true
		}
	}
	return false
}

// isGroupAdmin says whether the sender may delete other members' messages.
func isGroupAdmin(evt *events.Message, members []models.WAGroupParticipant) bool {
	for _, member := range members {
		if member.Role != models.GroupRoleMember && memberIs(member, evt.Info.Sender, evt.Info.SenderAlt) {
			return true
		}
	}
	return false
}

func (h *inboundHandler) refused(change string, evt *events.Message, target *models.WAGroupMessage) {
	h.logger.Warn("Ignoring a group "+change+" from someone who may not make it",
		zap.String("wa_message_id", evt.Info.ID), zap.String("target_id", target.ID.String()))
}

func (h *inboundHandler) changed(ctx context.Context, target *models.WAGroupMessage) {
	h.announceGroup(ctx, Event{Type: EventGroupMessage, GroupID: target.GroupID.String(), MessageID: target.ID.String()})
}
