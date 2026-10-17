package wa

import (
	"context"
	"fmt"
	"time"

	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"go.uber.org/zap"
)

// historyBatch is how many older messages one request asks the phone for,
// the number whatsmeow recommends.
const historyBatch = 50

// HistoryAnchor is the oldest message a group holds, the point the phone
// counts back from.
type HistoryAnchor struct {
	WAMessageID string
	SenderJID   string
	FromMe      bool
	At          time.Time
}

// RequestHistory asks the phone holding the number for the messages before
// the anchor. The phone answers later, and only while it is online, as a
// HistorySync event that storeGroupHistory takes in.
func (c *Client) RequestHistory(ctx context.Context, groupJID string, anchor HistoryAnchor) error {
	chat, err := types.ParseJID(groupJID)
	if err != nil {
		return fmt.Errorf("grup tidak valid %q: %w", groupJID, err)
	}
	info := &types.MessageInfo{
		MessageSource: types.MessageSource{Chat: chat, IsFromMe: anchor.FromMe, IsGroup: true},
		ID:            anchor.WAMessageID,
		Timestamp:     anchor.At,
	}
	if sender, err := types.ParseJID(anchor.SenderJID); err == nil && anchor.SenderJID != "" {
		info.Sender = sender
	}
	_, err = c.wa.SendPeerMessage(ctx, c.wa.BuildHistorySyncRequest(info, historyBatch))
	return err
}

// storeGroupHistory takes in the group messages a history sync carries: the
// answer to RequestHistory, or the history the phone sends when a number is
// first linked. They go through the same path as live ones, so they are
// stored once however often they arrive, and the summary only moves for one
// newer than what the list already shows.
func (c *Client) storeGroupHistory(evt *events.HistorySync) {
	for _, conv := range evt.Data.GetConversations() {
		chat, err := types.ParseJID(conv.GetID())
		if err != nil || chat.Server != types.GroupServer {
			continue
		}
		stored := 0
		for _, item := range conv.GetMessages() {
			msg, err := c.wa.ParseWebMessage(chat, item.GetMessage())
			if err != nil {
				continue
			}
			if err := c.inbound.handleGroup(c.ctx, msg); err != nil {
				c.logger.Warn("Could not store a group message from history", zap.Error(err))
				continue
			}
			stored++
		}
		c.logger.Info("Took in group history", zap.String("group", chat.String()), zap.Int("messages", stored))
	}
}
