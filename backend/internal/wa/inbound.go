package wa

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/services"
	"go.mau.fi/whatsmeow"
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
	if evt.Info.IsGroup || evt.Info.IsFromMe {
		return nil // this inbox answers customers, not groups or its own echo
	}
	att, readable := describe(evt.Message)
	if !readable {
		return nil // a reaction or a protocol message: nothing a CS would read
	}

	conv, err := h.conversations.FindOrCreate(services.IncomingPeer{
		WAAccountID: h.accountID,
		JID:         evt.Info.Sender.ToNonAD().String(),
		Phone:       evt.Info.Sender.User,
		Name:        evt.Info.PushName,
	})
	if err != nil {
		return err
	}

	body, media := h.fetch(ctx, evt, att)

	msg, created, err := h.messages.SaveInbound(services.InboundMessage{
		ConversationID: conv.ID,
		WAMessageID:    evt.Info.ID,
		Kind:           att.kind,
		Body:           body,
		Media:          media,
		At:             evt.Info.Timestamp,
	})
	if err != nil {
		return err
	}
	if !created {
		return nil // WhatsApp re-delivered one it had already given us
	}

	if _, err := h.assignment.AssignOne(ctx, conv.ID); err != nil {
		return err
	}

	return h.publisher.Publish(ctx, Event{
		Type:           EventMessage,
		ConversationID: conv.ID.String(),
		MessageID:      msg.ID.String(),
	})
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
