package main

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/tikman/olt-provisioning/internal/services"
	"github.com/tikman/olt-provisioning/internal/wa"
	"go.uber.org/zap"
)

// presenceLoop carries "a CS is typing" from the API to the customer's phone.
// The API holds the browsers and this process holds the WhatsApp connections,
// so the announcement has to cross between them.
func presenceLoop(
	ctx context.Context,
	redisClient *redis.Client,
	live *sessions,
	conversations *services.CSConversationService,
	groups *services.WAGroupService,
	logger *zap.Logger,
) {
	sub := redisClient.Subscribe(ctx, wa.PresenceChannel)
	defer func() {
		_ = sub.Close()
	}()

	updates := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, open := <-updates:
			if !open {
				return
			}
			applyPresence(ctx, msg.Payload, live, conversations, groups, logger)
		}
	}
}

// applyPresence tells one customer's phone whether a CS is writing to them.
//
// Everything here fails quietly. A typing line is worth nothing once the moment
// it described has passed, so a thread that has been deleted, a number whose
// session is not held by this process, or a refusal from WhatsApp all cost the
// line and nothing else.
func applyPresence(
	ctx context.Context,
	payload string,
	live *sessions,
	conversations *services.CSConversationService,
	groups *services.WAGroupService,
	logger *zap.Logger,
) {
	var msg wa.PresenceMessage
	if err := json.Unmarshal([]byte(payload), &msg); err != nil {
		logger.Warn("Could not decode a CS typing update", zap.Error(err))
		return
	}
	if msg.GroupID != "" {
		applyGroupPresence(ctx, msg, live, groups, logger)
		return
	}
	conversationID, err := uuid.Parse(msg.ConversationID)
	if err != nil {
		logger.Warn("CS typing update named no valid conversation",
			zap.String("conversation_id", msg.ConversationID))
		return
	}
	conv, err := conversations.Get(conversationID)
	if err != nil {
		return
	}
	client := live.client(conv.WAAccountID)
	if client == nil {
		return
	}
	if err := client.SetTyping(ctx, conv.CustomerJID, msg.Typing); err != nil {
		logger.Warn("Could not tell WhatsApp a CS is typing",
			zap.String("conversation_id", conversationID.String()), zap.Error(err))
	}
}

// applyGroupPresence shows a group that someone on the team is writing to it.
// It fails as quietly as the customer case, for the same reason.
func applyGroupPresence(ctx context.Context, msg wa.PresenceMessage, live *sessions, groups *services.WAGroupService, logger *zap.Logger) {
	groupID, err := uuid.Parse(msg.GroupID)
	if err != nil {
		return
	}
	group, err := groups.Get(groupID)
	if err != nil || group.Left {
		return
	}
	client := live.client(group.WAAccountID)
	if client == nil {
		return
	}
	if err := client.SetTyping(ctx, group.JID, msg.Typing); err != nil {
		logger.Warn("Could not tell a group a CS is typing", zap.String("group_id", msg.GroupID), zap.Error(err))
	}
}
