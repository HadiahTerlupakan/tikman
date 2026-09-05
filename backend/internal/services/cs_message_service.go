package services

import (
	"errors"
	"fmt"
	"github.com/tikman/olt-provisioning/internal/wa/linkpreview"
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

	// ReplyToWAID is the WhatsApp id of the message this one quotes, empty when
	// it quotes nothing. It is a WhatsApp id rather than one of ours because
	// that is all the customer's phone sends; SaveInbound turns it into a row.
	ReplyToWAID string

	// Preview is the link card the customer's own WhatsApp built and sent
	// with the message. Stored rather than resolved again: it costs no
	// request, and it is exactly what the customer is looking at.
	Preview *linkpreview.Preview
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
	var (
		stored  models.CSMessage
		created bool
	)

	// One transaction, for two reasons. The caller in the wa process deletes the
	// attachment when this answers with an error, so a message row that survived
	// a failed conversation bump would have its file deleted out from under it —
	// and nothing repairs that, because Sweep tolerates a missing file. And a
	// message stored behind a stale last_message_at would sit in the inbox
	// without surfacing. Storing the message and the inbox knowing about it are
	// one fact, so they commit as one.
	err := s.db.Transaction(func(tx *gorm.DB) error {
		lookup := tx.Where("wa_message_id = ?", in.WAMessageID).First(&stored).Error
		if lookup == nil {
			return nil // WhatsApp re-delivered one it had already given us
		}
		if !errors.Is(lookup, gorm.ErrRecordNotFound) {
			return fmt.Errorf("look for existing message: %w", lookup)
		}

		waID := in.WAMessageID
		stored = models.CSMessage{
			ConversationID:     in.ConversationID,
			WAMessageID:        &waID,
			Direction:          models.MessageIn,
			Kind:               in.Kind,
			Body:               in.Body,
			PreviewURL:         previewURL(in.Preview),
			PreviewTitle:       previewTitle(in.Preview),
			PreviewDescription: previewDescription(in.Preview),
			PreviewThumbnail:   previewThumbnail(in.Preview),
			Status:             models.MessageDelivered,
			ReplyToID:          quotedRow(tx, in.ConversationID, in.ReplyToWAID),
			WATimestamp:        in.At,
		}
		applyMedia(&stored, in.Media)

		if err := tx.Create(&stored).Error; err != nil {
			return fmt.Errorf("store inbound message: %w", err)
		}
		if err := bumpConversation(tx, in.ConversationID, in.At); err != nil {
			return err
		}

		created = true
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return &stored, created, nil
}

// Queue writes a CS reply as waiting to be sent. The row is the outbox: the wa
// process claims it, and one that was written while that process was down is
// still here when it comes back.
func (s *CSMessageService) Queue(
	conversationID, senderUserID uuid.UUID,
	kind models.MessageKind, body string, media *MediaFile, replyTo *uuid.UUID,
) (*models.CSMessage, error) {
	sender := senderUserID
	msg := models.CSMessage{
		ConversationID: conversationID,
		Direction:      models.MessageOut,
		SenderUserID:   &sender,
		Kind:           kind,
		Body:           body,
		Status:         models.MessageQueued,
		ReplyToID:      replyTo,
		WATimestamp:    time.Now(),
	}
	applyMedia(&msg, media)

	// Same reason as SaveInbound: a reply queued behind a stale last_message_at
	// would sink down the inbox while the handler told the CS it had failed.
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&msg).Error; err != nil {
			return fmt.Errorf("queue message: %w", err)
		}
		return s.conversations.touchTx(tx, conversationID, msg.WATimestamp)
	})
	if err != nil {
		return nil, err
	}
	return &msg, nil
}

// ClaimQueued returns the messages still waiting to reach WhatsApp on one
// number, oldest first so a thread's replies arrive in the order the CS wrote
// them.
//
// Scoping to the account is what makes a reply leave from the number the
// customer wrote to. The session holding that number claims only its own
// threads, so the guarantee is the shape of the query rather than a rule
// somebody has to remember at the point of sending — where a second sender,
// added later, would quietly not know about it. Unscoped, a reply written on
// one number went out from whichever session drained first.
func (s *CSMessageService) ClaimQueued(accountID uuid.UUID, limit int) ([]models.CSMessage, error) {
	if limit <= 0 {
		limit = defaultHistoryLimit
	}
	var rows []models.CSMessage
	err := s.db.
		Joins("JOIN cs_conversations ON cs_conversations.id = cs_messages.conversation_id").
		Where("cs_messages.status = ? AND cs_conversations.wa_account_id = ?",
			models.MessageQueued, accountID).
		Order("cs_messages.created_at ASC").Limit(limit).Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("claim queued messages: %w", err)
	}
	return rows, nil
}

// MarkSent records that WhatsApp accepted a message.
func (s *CSMessageService) MarkSent(id uuid.UUID, waMessageID string, preview *linkpreview.Preview) error {
	fields := map[string]any{
		"status":        models.MessageSent,
		"wa_message_id": waMessageID,
		"fail_reason":   "",
	}
	// Written in the same update that records the send, so the thread never
	// shows a card for a message that did not reach the customer.
	if preview != nil {
		fields["preview_url"] = preview.URL
		fields["preview_title"] = preview.Title
		fields["preview_description"] = preview.Description
		fields["preview_thumbnail"] = preview.Thumbnail
	}
	return s.updateMessage(id, fields)
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
// refuses to walk it back when receipts arrive out of order. It answers which
// thread the message belongs to, so the caller can tell the browsers holding
// that thread open to look again. uuid.Nil comes back when nothing moved — an
// unknown message, or a receipt that does not advance the status it already
// has — and the caller announces nothing for it.
func (s *CSMessageService) ApplyReceipt(waMessageID string, status models.MessageStatus) (uuid.UUID, error) {
	var msg models.CSMessage
	err := s.db.Where("wa_message_id = ?", waMessageID).First(&msg).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return uuid.Nil, nil
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("load message for receipt: %w", err)
	}
	if receiptRank[status] <= receiptRank[msg.Status] {
		return uuid.Nil, nil
	}
	if err := s.updateMessage(msg.ID, map[string]any{"status": status}); err != nil {
		return uuid.Nil, err
	}
	return msg.ConversationID, nil
}

// InboundAwaitingRead returns the customer's messages in a thread that WhatsApp
// has not yet been told we read, oldest first. Inbound messages are stored as
// delivered, so the status is the record of what has been acknowledged: without
// it every reply in a long thread would re-acknowledge the whole history.
func (s *CSMessageService) InboundAwaitingRead(conversationID uuid.UUID) ([]models.CSMessage, error) {
	var msgs []models.CSMessage
	err := s.db.Where(
		"conversation_id = ? AND direction = ? AND status <> ? AND wa_message_id IS NOT NULL",
		conversationID, models.MessageIn, models.MessageRead,
	).Order("wa_timestamp ASC").Find(&msgs).Error
	if err != nil {
		return nil, fmt.Errorf("load inbound awaiting read: %w", err)
	}
	return msgs, nil
}

// MarkInboundRead records that WhatsApp has been told these messages were read.
func (s *CSMessageService) MarkInboundRead(ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	err := s.db.Model(&models.CSMessage{}).Where("id IN ?", ids).
		Update("status", models.MessageRead).Error
	if err != nil {
		return fmt.Errorf("mark inbound read: %w", err)
	}
	return nil
}

// Get loads one message. ServeMedia uses it to find where an attachment for
// this message is stored on disk before serving it.
func (s *CSMessageService) Get(id uuid.UUID) (*models.CSMessage, error) {
	var msg models.CSMessage
	if err := s.db.First(&msg, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("load message: %w", err)
	}
	return &msg, nil
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
	if err := s.attachQuotes(rows); err != nil {
		return nil, err
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

// bumpConversation runs on the caller's transaction, never on s.db: it is half
// of storing a message, and must not be able to commit or fail on its own.
func bumpConversation(tx *gorm.DB, conversationID uuid.UUID, at time.Time) error {
	err := tx.Model(&models.CSConversation{}).Where("id = ?", conversationID).
		Updates(map[string]any{
			"last_message_at":        at,
			"last_message_direction": models.MessageIn,
			"unread_count":           gorm.Expr("unread_count + 1"),
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

// The four accessors keep the nil check in one place rather than repeating it
// at each field of the row being built.

func previewURL(p *linkpreview.Preview) string {
	if p == nil {
		return ""
	}
	return p.URL
}

func previewTitle(p *linkpreview.Preview) string {
	if p == nil {
		return ""
	}
	return p.Title
}

func previewDescription(p *linkpreview.Preview) string {
	if p == nil {
		return ""
	}
	return p.Description
}

func previewThumbnail(p *linkpreview.Preview) []byte {
	if p == nil {
		return nil
	}
	return p.Thumbnail
}
