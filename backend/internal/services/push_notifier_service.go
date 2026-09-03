package services

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
)

// PushSender delivers a push notification to a set of device tokens and
// reports which of them the push service considers dead, so the caller can
// stop holding onto them.
//
// Declared here rather than in internal/push so that this package — imported
// by the wa, worker, trapd and seed-events binaries — never pulls the
// Firebase SDK into their builds. push.Client satisfies it structurally.
type PushSender interface {
	SendEach(ctx context.Context, tokens []string, title, body string, data map[string]string) (invalidTokens []string, err error)
}

// pushPreviewRunes is how much of a message body reaches an OS notification —
// enough to judge urgency, short enough to stay well under FCM's per-message
// size limit.
const pushPreviewRunes = 120

// pushEligibleRoles are the roles that can open the CS inbox at all — the
// same three /api/v1/cs/* already admits.
var pushEligibleRoles = []models.UserRole{
	models.UserRoleAdmin, models.UserRoleCS, models.UserRoleTechnician,
}

// PushNotifierService turns one incoming customer message into a push
// notification for every eligible device registered.
type PushNotifierService struct {
	sender        PushSender
	subscriptions *PushService
	conversations *CSConversationService
	messages      *CSMessageService
}

func NewPushNotifierService(sender PushSender, subscriptions *PushService, conversations *CSConversationService, messages *CSMessageService) *PushNotifierService {
	return &PushNotifierService{
		sender:        sender,
		subscriptions: subscriptions,
		conversations: conversations,
		messages:      messages,
	}
}

// SetSender replaces the Sender after construction — used by cmd/api, which
// only knows whether a real Firebase client exists after Setup has already
// built the notifier alongside everything else it depends on.
func (s *PushNotifierService) SetSender(sender PushSender) {
	s.sender = sender
}

// NotifyIncomingMessage looks up the conversation and message an EventMessage
// named, then pushes a notification to everyone who can answer the inbox. A
// send failure here is logged by the caller (see PushEventListener) and never
// blocks message storage — push is additive, the same tolerance
// wa/inbound.go already applies to its own SSE announcement.
func (s *PushNotifierService) NotifyIncomingMessage(ctx context.Context, conversationID, messageID uuid.UUID) error {
	conv, err := s.conversations.Get(conversationID)
	if err != nil {
		return fmt.Errorf("look up conversation: %w", err)
	}
	msg, err := s.messages.Get(messageID)
	if err != nil {
		return fmt.Errorf("look up message: %w", err)
	}

	tokens, err := s.subscriptions.TokensForRoles(pushEligibleRoles...)
	if err != nil {
		return fmt.Errorf("list push tokens: %w", err)
	}
	if len(tokens) == 0 {
		return nil
	}

	title := conv.CustomerName
	if title == "" {
		title = conv.CustomerPhone
	}

	invalid, err := s.sender.SendEach(ctx, tokens, title, previewOf(msg.Body), map[string]string{
		"conversation_id": conversationID.String(),
	})
	if err != nil {
		return fmt.Errorf("send push: %w", err)
	}
	if len(invalid) > 0 {
		if err := s.subscriptions.RemoveTokens(invalid); err != nil {
			return fmt.Errorf("remove invalid push tokens: %w", err)
		}
	}
	return nil
}

// previewOf truncates a message body to what an OS notification should show,
// marking the cut with an ellipsis rather than slicing a word in half.
func previewOf(body string) string {
	runes := []rune(body)
	if len(runes) <= pushPreviewRunes {
		return body
	}
	return string(runes[:pushPreviewRunes]) + "…"
}
