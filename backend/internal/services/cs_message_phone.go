package services

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// SaveFromPhone stores a message the number sent from outside TikMan — its
// phone, or another device linked to it — to a customer, answering false when
// WhatsApp had already delivered it.
//
// A CS sometimes answers that way instead of from TikMan. Left out, the
// thread stays in "Belum dibalas" with the customer already answered, and the
// next CS answers them again. No sender is recorded: the number's devices are
// shared, and naming the wrong CS would be worse than naming none.
func (s *CSMessageService) SaveFromPhone(in InboundMessage) (*models.CSMessage, bool, error) {
	return s.saveArrived(in, phoneRow, s.phoneMessageStored)
}

// phoneMessageStored brings the thread up to date and records the wait this
// answer ended, under the same savepoint rule as a customer's message.
func (s *CSMessageService) phoneMessageStored(tx *gorm.DB, msg *models.CSMessage) error {
	if err := markAnsweredFromPhone(tx, msg.ConversationID, msg.WATimestamp); err != nil {
		return err
	}
	s.conversations.waits.record(tx, msg.ConversationID, "phone reply", func(tx *gorm.DB) error {
		return answeredFromPhone(tx, msg)
	})
	return nil
}

// phoneRow renders a phone message as an outbound row with no sender, which is
// how every later reader tells it apart from a reply sent through TikMan.
func phoneRow(tx *gorm.DB, in InboundMessage) models.CSMessage {
	return arrivedRow(tx, in, models.MessageOut, models.MessageSent)
}

// markAnsweredFromPhone takes the thread out of "Belum dibalas" only when the
// phone's message is the newest thing in it. WhatsApp can deliver a phone reply
// late, after a reconnect, when the customer has already written again; that
// newer message must stay waiting, and its badge must stay with it.
func markAnsweredFromPhone(tx *gorm.DB, conversationID uuid.UUID, at time.Time) error {
	err := tx.Model(&models.CSConversation{}).
		Where("id = ? AND last_message_at <= ?", conversationID, at).
		Updates(map[string]any{
			"last_message_at":        at,
			"last_message_direction": models.MessageOut,
			"unread_count":           0,
		}).Error
	if err != nil {
		return fmt.Errorf("mark thread answered from phone: %w", err)
	}
	return nil
}
