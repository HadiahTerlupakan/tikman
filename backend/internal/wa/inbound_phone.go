package wa

import (
	"context"

	"github.com/tikman/olt-provisioning/internal/models"
	"go.mau.fi/whatsmeow/types/events"
	"go.uber.org/zap"
)

// handleFromPhone stores a message the phone holding this number sent to a
// customer who already has a thread here.
//
// The number's other devices — its phone, or WhatsApp Web/Desktop linked to
// it — produce these, never this process: whatsmeow never encrypts a copy of
// what this process sends for this process's own device, so nothing TikMan
// sent comes back as IsFromMe. A chat the phone starts with someone who has
// no thread stays out, so the phone's other chats do not fill the inbox.
func (h *inboundHandler) handleFromPhone(ctx context.Context, evt *events.Message, att attachment) error {
	conv, err := h.threadForPhoneMessage(evt)
	if err != nil {
		return err
	}
	if conv == nil {
		h.logger.Debug("Skipping a phone message to someone with no thread",
			zap.String("wa_message_id", evt.Info.ID))
		return nil
	}
	return h.store(ctx, evt, att, conv.ID, h.messages.SaveFromPhone)
}

// threadForPhoneMessage finds the customer's thread by the address the phone
// used, then by the other one. WhatsApp names the same person by phone number
// or by LID, and a thread opened under one can be answered under the other.
func (h *inboundHandler) threadForPhoneMessage(evt *events.Message) (*models.CSConversation, error) {
	conv, err := h.conversations.FindByPeer(h.accountID, evt.Info.Chat.ToNonAD().String())
	if err != nil || conv != nil || evt.Info.RecipientAlt.IsEmpty() {
		return conv, err
	}
	return h.conversations.FindByPeer(h.accountID, evt.Info.RecipientAlt.ToNonAD().String())
}
