// Package wa holds the one process that talks to WhatsApp.
package wa

import (
	"context"
	"github.com/tikman/olt-provisioning/internal/wa/linkpreview"
	"time"

	"github.com/tikman/olt-provisioning/internal/models"
)

// Sender is the part of whatsmeow that sending needs. It exists so the logic
// worth getting right — which message goes next, what happens when WhatsApp
// refuses — can be tested without a WhatsApp connection, while the connection
// itself stays in client.go where the network-code exemption applies.
type Sender interface {
	SendText(ctx context.Context, jid, body string, quote *Quote, preview *linkpreview.Preview) (waMessageID string, err error)
	SendMedia(ctx context.Context, jid string, kind models.MessageKind, path, mime, filename, caption string, quote *Quote) (waMessageID string, err error)
	// MarkRead tells WhatsApp that the customer's messages have been read,
	// which is what turns their ticks blue. Every id must come from the same
	// sender, which holds here because a thread is one customer.
	MarkRead(ctx context.Context, chatJID string, ids []string, at time.Time) error
}
