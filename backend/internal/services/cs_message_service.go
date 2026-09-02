package services

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// defaultHistoryLimit is one screen of a thread.
const defaultHistoryLimit = 50

// receiptRank orders the receipts WhatsApp sends. They can arrive out of order,
// and a late "delivered" must not walk a message back from "read".
var receiptRank = map[models.MessageStatus]int{
	models.MessageQueued:    0,
	models.MessageSent:      1,
	models.MessageDelivered: 2,
	models.MessageRead:      3,
}

// MediaFile is a stored attachment.
type MediaFile struct {
	Path     string
	Mime     string
	Filename string
	Size     int64
}

// InboundMessage is a message as it arrived from WhatsApp.
type InboundMessage struct {
	ConversationID uuid.UUID
	WAMessageID    string
	Kind           models.MessageKind
	Body           string
	Media          *MediaFile
	At             time.Time
}

// CSMessageService stores the traffic in a thread, in both directions.
type CSMessageService struct {
	db            *gorm.DB
	conversations *CSConversationService
}

// NewCSMessageService constructs a CSMessageService.
func NewCSMessageService(db *gorm.DB, conversations *CSConversationService) *CSMessageService {
	return &CSMessageService{db: db, conversations: conversations}
}

// SaveInbound stores an incoming message, answering false when this WhatsApp
// message was already stored. WhatsApp re-delivers events it is unsure about,
// and the duplicate would otherwise be shown to the CS and counted as unread.
//
// The lookup is done here as well as by the partial unique index in migration
// 41, because SQLite tests never get that index.
func (s *CSMessageService) SaveInbound(in InboundMessage) (*models.CSMessage, bool, error) {
	var existing models.CSMessage
	err := s.db.Where("wa_message_id = ?", in.WAMessageID).First(&existing).Error
	if err == nil {
		return &existing, false, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, fmt.Errorf("look for existing message: %w", err)
	}

	waID := in.WAMessageID
	msg := models.CSMessage{
		ConversationID: in.ConversationID,
		WAMessageID:    &waID,
		Direction:      models.MessageIn,
		Kind:           in.Kind,
		Body:           in.Body,
		Status:         models.MessageDelivered,
		WATimestamp:    in.At,
	}
	applyMedia(&msg, in.Media)

	if err := s.db.Create(&msg).Error; err != nil {
		return nil, false, fmt.Errorf("store inbound message: %w", err)
	}

	if err := s.bumpConversation(in.ConversationID, in.At); err != nil {
		return nil, false, err
	}
	return &msg, true, nil
}

// Queue writes a CS reply as waiting to be sent. The row is the outbox: the wa
// process claims it, and one that was written while that process was down is
// still here when it comes back.
func (s *CSMessageService) Queue(
	conversationID, senderUserID uuid.UUID,
	kind models.MessageKind, body string, media *MediaFile,
) (*models.CSMessage, error) {
	sender := senderUserID
	msg := models.CSMessage{
		ConversationID: conversationID,
		Direction:      models.MessageOut,
		SenderUserID:   &sender,
		Kind:           kind,
		Body:           body,
		Status:         models.MessageQueued,
		WATimestamp:    time.Now(),
	}
	applyMedia(&msg, media)

	if err := s.db.Create(&msg).Error; err != nil {
		return nil, fmt.Errorf("queue message: %w", err)
	}
	if err := s.conversations.Touch(conversationID, msg.WATimestamp); err != nil {
		return nil, err
	}
	return &msg, nil
}

// ClaimQueued returns the messages still waiting to reach WhatsApp, oldest
// first so a thread's replies arrive in the order the CS wrote them.
func (s *CSMessageService) ClaimQueued(limit int) ([]models.CSMessage, error) {
	if limit <= 0 {
		limit = defaultHistoryLimit
	}
	var rows []models.CSMessage
	err := s.db.Where("status = ?", models.MessageQueued).
		Order("created_at ASC").Limit(limit).Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("claim queued messages: %w", err)
	}
	return rows, nil
}

// MarkSent records that WhatsApp accepted a message.
func (s *CSMessageService) MarkSent(id uuid.UUID, waMessageID string) error {
	return s.updateMessage(id, map[string]any{
		"status":        models.MessageSent,
		"wa_message_id": waMessageID,
		"fail_reason":   "",
	})
}

// MarkFailed records why a message could not be sent, so the CS reads a reason
// rather than watching a reply quietly disappear.
func (s *CSMessageService) MarkFailed(id uuid.UUID, reason string) error {
	return s.updateMessage(id, map[string]any{
		"status":      models.MessageFailed,
		"fail_reason": reason,
	})
}

// ApplyReceipt walks a message forward through sent, delivered and read, and
// refuses to walk it back when receipts arrive out of order.
func (s *CSMessageService) ApplyReceipt(waMessageID string, status models.MessageStatus) error {
	var msg models.CSMessage
	err := s.db.Where("wa_message_id = ?", waMessageID).First(&msg).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("load message for receipt: %w", err)
	}
	if receiptRank[status] <= receiptRank[msg.Status] {
		return nil
	}
	return s.updateMessage(msg.ID, map[string]any{"status": status})
}

// History returns one page of a thread, newest first.
func (s *CSMessageService) History(conversationID uuid.UUID, limit, offset int) ([]models.CSMessage, error) {
	if limit <= 0 || limit > defaultHistoryLimit {
		limit = defaultHistoryLimit
	}
	var rows []models.CSMessage
	err := s.db.Where("conversation_id = ?", conversationID).
		Order("wa_timestamp DESC").Limit(limit).Offset(offset).Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("load history: %w", err)
	}
	return rows, nil
}

// Search finds messages by their words. It leans on the tsvector column added
// in migration 41, so it answers on Postgres only.
func (s *CSMessageService) Search(term string, limit int) ([]models.CSMessage, error) {
	if limit <= 0 || limit > defaultHistoryLimit {
		limit = defaultHistoryLimit
	}
	var rows []models.CSMessage
	err := s.db.Where("tsv @@ plainto_tsquery('simple', ?)", term).
		Order("wa_timestamp DESC").Limit(limit).Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("search messages: %w", err)
	}
	return rows, nil
}

func (s *CSMessageService) bumpConversation(conversationID uuid.UUID, at time.Time) error {
	err := s.db.Model(&models.CSConversation{}).Where("id = ?", conversationID).
		Updates(map[string]any{
			"last_message_at": at,
			"unread_count":    gorm.Expr("unread_count + 1"),
		}).Error
	if err != nil {
		return fmt.Errorf("bump conversation: %w", err)
	}
	return nil
}

func (s *CSMessageService) updateMessage(id uuid.UUID, fields map[string]any) error {
	res := s.db.Model(&models.CSMessage{}).Where("id = ?", id).Updates(fields)
	if res.Error != nil {
		return fmt.Errorf("update message: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func applyMedia(msg *models.CSMessage, media *MediaFile) {
	if media == nil {
		return
	}
	msg.MediaPath = media.Path
	msg.MediaMime = media.Mime
	msg.MediaFilename = media.Filename
	msg.MediaSize = media.Size
}
