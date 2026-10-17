package wa

import (
	"context"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/services"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// groupReceipts records which members got and read our group messages.
type groupReceipts struct {
	accountID uuid.UUID
	groups    *services.WAGroupService
	messages  *services.WAGroupMessageService
	publisher announcer
}

// handle applies one member's receipt. Our own other devices also send
// receipts, and those say nothing about the members.
func (g *groupReceipts) handle(ctx context.Context, evt *events.Receipt) error {
	if evt.IsFromMe {
		return nil
	}
	read := evt.Type == types.ReceiptTypeRead || evt.Type == types.ReceiptTypePlayed
	if !read && evt.Type != types.ReceiptTypeDelivered {
		return nil
	}
	group, err := g.groups.FindByJID(g.accountID, evt.Chat.ToNonAD().String())
	if err != nil || group == nil {
		return err
	}
	ids := make([]string, len(evt.MessageIDs))
	for i, id := range evt.MessageIDs {
		ids[i] = string(id)
	}
	moved, err := g.messages.ApplyReceipt(group.ID, ids, evt.Sender.ToNonAD().String(), read, evt.Timestamp)
	for _, id := range moved {
		_ = g.publisher.Publish(ctx, Event{Type: EventGroupMessage, GroupID: group.ID.String(), MessageID: id.String()})
	}
	return err
}
