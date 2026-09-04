package wa

import (
	"context"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/services"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// presenceHandler turns a customer's typing into a line the inbox can show.
type presenceHandler struct {
	accountID     uuid.UUID
	conversations *services.CSConversationService
	publisher     announcer
}

// handle announces that one customer started or stopped writing.
//
// Nothing is stored. A typing state is true for a few seconds and then is not,
// and a row recording it would be wrong far longer than it was right — the
// browsers that care are the ones with the thread open right now, and they hear
// it on the same stream as everything else.
func (h *presenceHandler) handle(ctx context.Context, evt *events.ChatPresence) error {
	if evt.IsGroup || evt.IsFromMe {
		return nil
	}
	conv, err := h.conversations.FindByPeer(h.accountID, evt.Chat.ToNonAD().String())
	if err != nil {
		return err
	}
	if conv == nil {
		// A stranger opened the chat and started writing. There is no thread to
		// show it against, and making one would fill the inbox with rows
		// holding nothing to answer.
		return nil
	}
	return h.publisher.Publish(ctx, Event{
		Type:           EventTyping,
		ConversationID: conv.ID.String(),
		Typing:         evt.State == types.ChatPresenceComposing,
	})
}
