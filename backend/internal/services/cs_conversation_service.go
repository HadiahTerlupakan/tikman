package services

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/utils"
	"gorm.io/gorm"
)

// ErrNotHolder is returned when someone tries to answer a conversation another
// CS is holding. Round-robin decides who answers; this is what enforces it.
var ErrNotHolder = errors.New("percakapan sedang dipegang orang lain")

// defaultConversationLimit keeps one inbox page bounded; at thousands of chats
// a day an unbounded list would fetch a year of history to draw one screen.
const defaultConversationLimit = 50

// CSConversationService owns a customer's thread: who they are, which ONT is
// theirs, who is holding the thread, and when it is done.
type CSConversationService struct {
	db *gorm.DB
}

// NewCSConversationService constructs a CSConversationService.
func NewCSConversationService(db *gorm.DB) *CSConversationService {
	return &CSConversationService{db: db}
}

// IncomingPeer is a customer as WhatsApp describes them.
type IncomingPeer struct {
	WAAccountID uuid.UUID
	JID         string
	Phone       string
	Name        string
}

// ConversationFilter narrows the inbox to one of the views a CS switches
// between: their own threads, the ones nobody holds, or the finished ones.
type ConversationFilter struct {
	Mine       *uuid.UUID
	Unassigned bool
	Closed     bool
	Search     string
	Limit      int
	Offset     int
}

// FindOrCreate returns the thread for one customer on one number, creating it
// on first contact. A thread that was closed is reopened and released, because
// a customer writing again has a new problem and it must reach somebody's queue.
func (s *CSConversationService) FindOrCreate(p IncomingPeer) (*models.CSConversation, error) {
	phone, err := utils.NormalizePhone(p.Phone)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrValidation, err.Error())
	}

	var conv models.CSConversation
	err = s.db.Where("wa_account_id = ? AND customer_jid = ?", p.WAAccountID, p.JID).First(&conv).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return s.create(p, phone)
	}
	if err != nil {
		return nil, fmt.Errorf("load conversation: %w", err)
	}

	updates := map[string]any{}
	if p.Name != "" && p.Name != conv.CustomerName {
		updates["customer_name"] = p.Name
	}
	if conv.Status == models.ConversationClosed {
		updates["status"] = models.ConversationUnassigned
		updates["assigned_user_id"] = nil
	}
	if len(updates) > 0 {
		if err := s.db.Model(&conv).Updates(updates).Error; err != nil {
			return nil, fmt.Errorf("reopen conversation: %w", err)
		}
	}
	return s.Get(conv.ID)
}

func (s *CSConversationService) create(p IncomingPeer, phone string) (*models.CSConversation, error) {
	conv := models.CSConversation{
		WAAccountID:   p.WAAccountID,
		CustomerJID:   p.JID,
		CustomerPhone: phone,
		CustomerName:  p.Name,
		Status:        models.ConversationUnassigned,
		LastMessageAt: time.Now(),
		ONTID:         s.ontOwning(phone),
	}
	if err := s.db.Create(&conv).Error; err != nil {
		return nil, fmt.Errorf("create conversation: %w", err)
	}
	return &conv, nil
}

// ontOwning finds the subscriber's ONT by number. A number nobody recorded is
// the normal case early on, so a miss is silence rather than an error.
func (s *CSConversationService) ontOwning(phone string) *uuid.UUID {
	var ont models.ONT
	if err := s.db.Where("phone = ?", phone).First(&ont).Error; err != nil {
		return nil
	}
	return &ont.ID
}

// Get loads one conversation.
func (s *CSConversationService) Get(id uuid.UUID) (*models.CSConversation, error) {
	var conv models.CSConversation
	if err := s.db.First(&conv, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("load conversation: %w", err)
	}
	return &conv, nil
}

// List draws one page of the inbox, newest first.
func (s *CSConversationService) List(f ConversationFilter) ([]models.CSConversation, error) {
	q := s.db.Model(&models.CSConversation{})

	switch {
	case f.Mine != nil:
		q = q.Where("assigned_user_id = ? AND status <> ?", *f.Mine, models.ConversationClosed)
	case f.Unassigned:
		q = q.Where("status = ?", models.ConversationUnassigned)
	case f.Closed:
		q = q.Where("status = ?", models.ConversationClosed)
	}

	if f.Search != "" {
		like := "%" + f.Search + "%"
		q = q.Where("customer_name LIKE ? OR customer_phone LIKE ?", like, like)
	}

	limit := f.Limit
	if limit <= 0 || limit > defaultConversationLimit {
		limit = defaultConversationLimit
	}

	var rows []models.CSConversation
	if err := q.Order("last_message_at DESC").Limit(limit).Offset(f.Offset).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list conversations: %w", err)
	}
	return rows, nil
}

// Assign hands a conversation to one CS. Taking over someone else's thread goes
// through here too; the audit trail is written by the handler that called it.
func (s *CSConversationService) Assign(conversationID, holderID uuid.UUID) error {
	return s.update(conversationID, map[string]any{
		"assigned_user_id": holderID,
		"status":           models.ConversationOpen,
	})
}

// Close marks a conversation finished. The holder stays on the row, so the
// history still says who dealt with it.
func (s *CSConversationService) Close(conversationID uuid.UUID) error {
	return s.update(conversationID, map[string]any{"status": models.ConversationClosed})
}

// LinkONT ties a thread to a subscriber's ONT, or unties it when ontID is nil.
func (s *CSConversationService) LinkONT(conversationID uuid.UUID, ontID *uuid.UUID) error {
	return s.update(conversationID, map[string]any{"ont_id": ontID})
}

// Touch records that a conversation just saw traffic, which is what orders the
// inbox.
func (s *CSConversationService) Touch(conversationID uuid.UUID, at time.Time) error {
	return s.update(conversationID, map[string]any{"last_message_at": at})
}

// EnsureHolder reports whether this user may answer this conversation.
func (s *CSConversationService) EnsureHolder(conversationID, userID uuid.UUID) error {
	conv, err := s.Get(conversationID)
	if err != nil {
		return err
	}
	if conv.AssignedUserID == nil || *conv.AssignedUserID != userID {
		return ErrNotHolder
	}
	return nil
}

// touchTx is Touch inside a caller's transaction, so that a message and the
// inbox ordering that surfaces it commit together or not at all.
func (s *CSConversationService) touchTx(tx *gorm.DB, conversationID uuid.UUID, at time.Time) error {
	return updateConversation(tx, conversationID, map[string]any{"last_message_at": at})
}

func (s *CSConversationService) update(conversationID uuid.UUID, fields map[string]any) error {
	return updateConversation(s.db, conversationID, fields)
}

func updateConversation(db *gorm.DB, conversationID uuid.UUID, fields map[string]any) error {
	res := db.Model(&models.CSConversation{}).Where("id = ?", conversationID).Updates(fields)
	if res.Error != nil {
		return fmt.Errorf("update conversation: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
