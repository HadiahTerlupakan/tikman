package wa

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/services"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"go.uber.org/zap"
)

// mediaUnavailable is appended to the body of a message whose attachment could
// not be fetched, so a CS reading the thread knows something was sent and can
// ask for it again.
const mediaUnavailable = "[media gagal diunduh]"

// inboundHandler turns WhatsApp events into stored messages.
type inboundHandler struct {
	wa            *whatsmeow.Client
	accountID     uuid.UUID
	conversations *services.CSConversationService
	messages      *services.CSMessageService
	assignment    *services.CSAssignmentService
	publisher     *Publisher
	media         mediaStore
	logger        *zap.Logger
}

// handle turns one WhatsApp event into a stored message. The order matters: the
// thread must exist before the message lands in it, and the message must be
// stored before anyone is told to come and read it.
func (h *inboundHandler) handle(ctx context.Context, evt *events.Message) error {
	att, keep := h.attachmentFor(evt)
	if !keep {
		return nil
	}

	conv, err := h.conversations.FindOrCreate(services.IncomingPeer{
		WAAccountID: h.accountID,
		JID:         evt.Info.Chat.ToNonAD().String(),
		Phone:       senderPhone(evt.Info.MessageSource),
		Name:        evt.Info.PushName,
	})
	if err != nil {
		return err
	}

	body, media := h.fetch(ctx, evt, att)

	msg, created, err := h.messages.SaveInbound(services.InboundMessage{
		ConversationID: conv.ID,
		WAMessageID:    evt.Info.ID,
		ReplyToWAID:    quotedStanzaID(evt.Message),
		Kind:           att.kind,
		Body:           body,
		Preview:        inboundPreview(evt.Message),
		Media:          media,
		At:             evt.Info.Timestamp,
	})
	if err != nil {
		// Safe to delete because SaveInbound is one transaction: an error here
		// means no row was committed, so nothing names this file.
		h.discard(evt, media)
		return err
	}
	if !created {
		// WhatsApp re-delivered one it had already given us, and it does that on
		// every reconnect. The row it belongs to already names its own copy, so
		// this second file is referenced by nothing — and CSMediaRetention
		// sweeps from rows, so nothing would ever collect it.
		h.discard(evt, media)
		return nil
	}

	h.assignAndAnnounce(ctx, conv.ID, msg.ID)
	return nil
}

// attachmentFor decides whether a message belongs in this inbox at all, and
// what shape it has.
//
// This inbox answers customers, not groups, not its own echo, and not
// channels. The newsletter check cannot be folded into IsGroup: whatsmeow sets
// that flag only for GroupServer and BroadcastServer, so a channel arrives
// looking like a one-to-one chat. Nor does IsFromMe catch our own updates
// coming back — WhatsApp delivers those to the number that posted them with
// the channel as the sender, and a CS answering that thread would publish
// their reply to the channel's subscribers.
func (h *inboundHandler) attachmentFor(evt *events.Message) (attachment, bool) {
	if evt.Info.IsGroup || evt.Info.IsFromMe || evt.Info.Chat.Server == types.NewsletterServer {
		return attachment{}, false
	}

	att, readable := describe(evt.Message)
	if !readable {
		// Stickers, locations, contact cards and polls land here alongside the
		// reactions and protocol messages. A CS who is never told cannot ask
		// the customer to send it in a form the inbox can hold.
		h.logger.Info("Ignoring a WhatsApp message this inbox cannot store",
			zap.String("wa_message_id", evt.Info.ID),
			zap.String("shape", messageShape(evt.Message)))
		return attachment{}, false
	}
	return att, true
}

// assignAndAnnounce hands the thread to a CS and tells the browsers. Neither
// step decides whether the message was stored, and both have a safety net:
// AssignWaiting sweeps every minute, and a browser that misses the
// announcement still sees the message on its next poll.
func (h *inboundHandler) assignAndAnnounce(ctx context.Context, conversationID, messageID uuid.UUID) {
	if _, err := h.assignment.AssignOne(ctx, conversationID); err != nil {
		h.logger.Error("Could not assign an incoming conversation",
			zap.String("conversation_id", conversationID.String()), zap.Error(err))
	}

	err := h.publisher.Publish(ctx, Event{
		Type:           EventMessage,
		ConversationID: conversationID.String(),
		MessageID:      messageID.String(),
	})
	if err != nil {
		h.logger.Warn("Could not announce an incoming WhatsApp message",
			zap.String("conversation_id", conversationID.String()), zap.Error(err))
	}
}

// discard removes an attachment that ended up belonging to no message row.
func (h *inboundHandler) discard(evt *events.Message, media *services.MediaFile) {
	if media == nil {
		return
	}
	if err := h.media.remove(media.Path); err != nil {
		h.logger.Warn("Could not remove an unreferenced attachment",
			zap.String("wa_message_id", evt.Info.ID),
			zap.String("path", media.Path), zap.Error(err))
	}
}

// fetch returns the body and the stored attachment for one message. A download
// that fails costs the picture, not the message: the row is still written, and
// its body says what happened so the CS can ask the customer to resend.
func (h *inboundHandler) fetch(ctx context.Context, evt *events.Message, att attachment) (string, *services.MediaFile) {
	if att.download == nil {
		return att.caption, nil
	}

	file, err := h.media.save(ctx, h.wa, att)
	if err != nil {
		h.logger.Warn("Could not store WhatsApp attachment",
			zap.String("wa_message_id", evt.Info.ID), zap.Error(err))
		return strings.TrimSpace(att.caption + " " + mediaUnavailable), nil
	}
	return att.caption, file
}

// senderPhone digs out the customer's actual number.
//
// WhatsApp now addresses many chats by LID — a privacy identifier that looks
// like a number and is not one. When it does, Sender holds the LID and
// SenderAlt holds the phone number; addressed the old way, Sender is the
// number and SenderAlt is empty. Reading Sender unconditionally is how a real
// customer arrived as "213911014010978" and was thrown away for not looking
// Indonesian.
func senderPhone(src types.MessageSource) string {
	if src.Sender.Server == types.DefaultUserServer {
		return src.Sender.User
	}
	if src.SenderAlt.Server == types.DefaultUserServer {
		return src.SenderAlt.User
	}
	// Neither is a phone number: keep the LID so the thread still has a label.
	return src.Sender.User
}
