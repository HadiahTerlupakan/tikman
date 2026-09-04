package services

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
)

// PushSender delivers a push notification to a set of Firebase Installation
// IDs and reports which of them the push service considers dead, so the caller
// can stop holding onto them.
//
// Declared here rather than in internal/push so that this package — imported
// by the wa, worker, trapd and seed-events binaries — never pulls the
// Firebase SDK into their builds. push.Client satisfies it structurally.
type PushSender interface {
	SendEach(ctx context.Context, fids []string, title, body string, data map[string]string) (invalidFIDs []string, err error)
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
//
// It answers how many devices were sent to, which is the difference between
// "nobody has notifications turned on" and "we pushed and the phone stayed
// quiet" — two very different problems that looked identical from the log.
func (s *PushNotifierService) NotifyIncomingMessage(ctx context.Context, conversationID, messageID uuid.UUID) (int, error) {
	conv, err := s.conversations.Get(conversationID)
	if err != nil {
		return 0, fmt.Errorf("look up conversation: %w", err)
	}
	msg, err := s.messages.Get(messageID)
	if err != nil {
		return 0, fmt.Errorf("look up message: %w", err)
	}

	// cs:events carries an EventMessage for outbound replies too (see
	// CSHandler.announce), and pushing a CS's own reply to the whole team is
	// worse than useless — it names the customer and quotes the sender.
	if msg.Direction != models.MessageIn {
		return 0, nil
	}

	fids, err := s.subscriptions.FIDsForRoles(pushEligibleRoles...)
	if err != nil {
		return 0, fmt.Errorf("list push FIDs: %w", err)
	}
	if len(fids) == 0 {
		return 0, nil
	}

	title := conv.CustomerName
	if title == "" {
		title = conv.CustomerPhone
	}

	// Both answers matter, and the pruning comes first: a batch can name dead
	// devices and fail on live ones at the same time, and returning on the
	// error would leave the dead ones registered to fail again forever.
	invalid, sendErr := s.sender.SendEach(ctx, fids, title, previewFor(msg), map[string]string{
		"conversation_id": conversationID.String(),
	})
	if len(invalid) > 0 {
		if err := s.subscriptions.RemoveFIDs(invalid); err != nil {
			return 0, fmt.Errorf("remove invalid push FIDs: %w", err)
		}
	}
	if sendErr != nil {
		return 0, fmt.Errorf("send push: %w", sendErr)
	}
	return len(fids) - len(invalid), nil
}

// mediaKindLabels stand in for the body of a photo, document, voice note or
// video that arrived without a caption — most of them do, and a notification
// showing only the customer's name over a blank line tells a CS nothing about
// whether it is worth opening.
var mediaKindLabels = map[models.MessageKind]string{
	models.MessageKindImage:    "📷 Foto",
	models.MessageKindDocument: "📄 Dokumen",
	models.MessageKindAudio:    "🎤 Pesan suara",
	models.MessageKindVideo:    "🎬 Video",
}

// previewFor is what the notification shows under the customer's name: the
// message body, or a label naming what arrived when a media message carries
// no caption.
func previewFor(msg *models.CSMessage) string {
	if msg.Body == "" {
		if label, ok := mediaKindLabels[msg.Kind]; ok {
			return label
		}
	}
	return previewOf(msg.Body)
}

// previewOf cuts a message body to pushPreviewRunes runes — counting runes,
// not bytes, so a body of Indonesian text or emoji is never split mid-character
// — and appends an ellipsis to mark that more was said. The cut lands wherever
// the limit falls, mid-word included.
func previewOf(body string) string {
	runes := []rune(body)
	if len(runes) <= pushPreviewRunes {
		return body
	}
	return string(runes[:pushPreviewRunes]) + "…"
}
