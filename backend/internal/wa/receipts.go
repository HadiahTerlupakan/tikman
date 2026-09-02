package wa

import (
	"context"

	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// receiptHandler records how far the replies we sent have travelled.
type receiptHandler struct {
	messages  *services.CSMessageService
	publisher *Publisher
}

// handle applies one receipt to every message it names. WhatsApp reports a
// batch of ids at once, and a receipt for a message we never stored is ignored
// by ApplyReceipt rather than treated as an error.
func (h *receiptHandler) handle(ctx context.Context, evt *events.Receipt) error {
	status, tracked := receiptStatus(evt.Type)
	if !tracked {
		return nil
	}

	for _, id := range evt.MessageIDs {
		if err := h.messages.ApplyReceipt(id, status); err != nil {
			return err
		}
	}

	// The ticks a CS watches are the whole point of a receipt, so the browsers
	// are told to look again rather than waiting for their next poll.
	return h.publisher.Publish(ctx, Event{Type: EventStatus})
}

// receiptStatus maps the receipts that move a message forward. Everything else
// WhatsApp sends — retries, the echo from our own other devices, view-once
// playback — says nothing about whether the customer got the reply.
func receiptStatus(kind types.ReceiptType) (models.MessageStatus, bool) {
	switch kind {
	case types.ReceiptTypeDelivered:
		return models.MessageDelivered, true
	case types.ReceiptTypeRead:
		return models.MessageRead, true
	default:
		return "", false
	}
}
