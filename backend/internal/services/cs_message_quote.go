package services

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

var (
	// ErrQuoteNotInThread is a quote of a message this thread does not hold.
	// WhatsApp cannot render a quote across chats, and storing one would put
	// another customer's words on this customer's screen.
	ErrQuoteNotInThread = errors.New("pesan yang dikutip bukan milik percakapan ini")

	// ErrQuoteNotSent is a quote of a reply still waiting in the outbox. It has
	// no WhatsApp id yet, so the customer's phone would draw an empty grey box.
	ErrQuoteNotSent = errors.New("pesan yang dikutip belum terkirim ke WhatsApp")
)

// QuoteTarget answers with the message a reply may quote, refusing one this
// thread does not hold and one WhatsApp has never seen.
func (s *CSMessageService) QuoteTarget(conversationID, messageID uuid.UUID) (*models.CSMessage, error) {
	var target models.CSMessage
	err := s.db.Where("id = ? AND conversation_id = ?", messageID, conversationID).First(&target).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrQuoteNotInThread
	}
	if err != nil {
		return nil, fmt.Errorf("load quoted message: %w", err)
	}
	if target.WAMessageID == nil {
		return nil, ErrQuoteNotSent
	}
	return &target, nil
}

// quotedRow turns the WhatsApp id a customer quoted into the row that holds it.
//
// A miss is normal and answers nil: customers quote messages older than this
// inbox, and ones sent from the phone itself before it was linked. Dropping
// their message over a quote nobody can draw would lose the actual complaint,
// so the message is stored without one. A failed lookup is treated the same
// way for the same reason — the quote is the smaller half of the fact.
func quotedRow(tx *gorm.DB, conversationID uuid.UUID, waMessageID string) *uuid.UUID {
	if waMessageID == "" {
		return nil
	}
	var quoted models.CSMessage
	err := tx.Select("id").
		Where("wa_message_id = ? AND conversation_id = ?", waMessageID, conversationID).
		First(&quoted).Error
	if err != nil {
		return nil
	}
	return &quoted.ID
}

// attachQuotes fills in the quoted message for every reply on a page, in one
// query rather than one per row.
//
// A quote that resolves to nothing stays nil: retention sweeps old messages,
// and a reply must outlive the message it answered. Losing the grey block is a
// far smaller loss than dropping a CS's own words out of the thread.
func (s *CSMessageService) attachQuotes(rows []models.CSMessage) error {
	wanted := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		if row.ReplyToID != nil {
			wanted = append(wanted, *row.ReplyToID)
		}
	}
	if len(wanted) == 0 {
		return nil
	}

	var quoted []models.CSMessage
	if err := s.db.Where("id IN ?", wanted).Find(&quoted).Error; err != nil {
		return fmt.Errorf("load quoted messages: %w", err)
	}

	byID := make(map[uuid.UUID]*models.QuotedMessage, len(quoted))
	for i := range quoted {
		byID[quoted[i].ID] = quoted[i].AsQuote()
	}

	for i := range rows {
		if rows[i].ReplyToID == nil {
			continue
		}
		rows[i].ReplyTo = byID[*rows[i].ReplyToID]
	}
	return nil
}
