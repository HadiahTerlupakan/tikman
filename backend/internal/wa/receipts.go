package wa

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// receiptHandler records how far the replies we sent have travelled.
type receiptHandler struct {
	messages  *services.CSMessageService
	publisher announcer
}

// handle applies one receipt to every message it names. WhatsApp reports a
// batch of ids at once, and a receipt for a message we never stored is ignored
// by ApplyReceipt rather than treated as an error.
func (h *receiptHandler) handle(ctx context.Context, evt *events.Receipt) error {
	status, tracked := receiptStatus(evt.Type)
	if !tracked {
		return nil
	}

	// One id that will not apply must not abandon the rest of the batch:
	// WhatsApp does not send a receipt twice, so a message skipped here keeps
	// the tick it had until the customer happens to act on it again.
	var failures []error
	touched := make(map[uuid.UUID]struct{})
	for _, id := range evt.MessageIDs {
		conversationID, err := h.messages.ApplyReceipt(id, status)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		if conversationID != uuid.Nil {
			touched[conversationID] = struct{}{}
		}
	}

	// Named, one per thread the batch actually moved. A status event carrying
	// no conversation only refreshes the inbox list — the ticks a CS is
	// watching inside an open thread would sit unchanged until they navigated
	// away and back, which is exactly what a receipt is supposed to prevent.
	for conversationID := range touched {
		event := Event{Type: EventStatus, ConversationID: conversationID.String()}
		if err := h.publisher.Publish(ctx, event); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
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
