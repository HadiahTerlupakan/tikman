package services

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/utils"
	"gorm.io/gorm"
)

// ErrNotHolder is returned when someone tries to answer a conversation another
// CS is holding. Round-robin decides who answers; this is what enforces it.
var ErrNotHolder = errors.New("percakapan sedang dipegang orang lain")

// maxStoredPhone is the customer_phone column's width. An identifier we could
// not normalise is kept as-is up to that, because an unreadable identifier is
// still better than none when a CS asks who this was.
const maxStoredPhone = 20

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

// FindOrCreate returns the thread for one customer on one number, creating it
// on first contact. A thread that was closed is reopened and released, because
// a customer writing again has a new problem and it must reach somebody's queue.
func (s *CSConversationService) FindOrCreate(p IncomingPeer) (*models.CSConversation, error) {
	// Best effort, never a gate. Matching a chat to an ONT is a convenience;
	// storing what a customer said is the job. A number this cannot read — a
	// foreign one, or a LID WhatsApp gave us instead of a phone number — used
	// to drop the whole message, so the CS never learned anyone had written.
	phone, err := utils.NormalizePhone(p.Phone)
	if err != nil {
		phone = strings.TrimSpace(p.Phone)
		if len(phone) > maxStoredPhone {
			phone = phone[:maxStoredPhone]
		}
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

// FindByPeer answers the thread one customer already has on one number, and
// nil when there is none.
//
// Lookup only, and deliberately so: it exists for presence, which arrives for
// customers who are typing rather than for customers who have written. Creating
// a thread here would fill the inbox with rows holding nothing to answer, one
// per stranger who opened the chat and thought better of it.
func (s *CSConversationService) FindByPeer(waAccountID uuid.UUID, jid string) (*models.CSConversation, error) {
	var conv models.CSConversation
	err := s.db.Where("wa_account_id = ? AND customer_jid = ?", waAccountID, jid).First(&conv).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load conversation by peer: %w", err)
	}
	return &conv, nil
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

// MarkRead clears a thread's unread badge, answering whether there was one to
// clear. That answer is what keeps the caller from announcing a read that
// changed nothing — see markThreadRead in cs_handler_messages.go.
func (s *CSConversationService) MarkRead(conversationID uuid.UUID) (bool, error) {
	res := s.db.Model(&models.CSConversation{}).
		Where("id = ? AND unread_count > 0", conversationID).
		Update("unread_count", 0)
	if res.Error != nil {
		return false, fmt.Errorf("mark conversation read: %w", res.Error)
	}
	return res.RowsAffected > 0, nil
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
	// The direction is recorded when the reply is queued, not when it reaches
	// WhatsApp. A CS who has answered has answered; leaving the thread in the
	// waiting queue until the outbox drains would put it back in front of the
	// next agent, who would answer it again.
	return updateConversation(tx, conversationID, map[string]any{
		"last_message_at":        at,
		"last_message_direction": models.MessageOut,
	})
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
