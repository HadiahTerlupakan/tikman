package services

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// csCustomerSilence is how long a customer can go quiet before writing again
// starts a new wait instead of continuing the old one. Without it, "ok makasih"
// followed by a new question days later reads as days of waiting.
const csCustomerSilence = 24 * time.Hour

// csWaits records how long customers wait for an answer, inside the
// transactions that store what the waits are made of.
type csWaits struct {
	logger *zap.Logger
}

// record runs one step of wait bookkeeping under a savepoint of the caller's
// transaction. A failing step is rolled back alone and logged, and the caller
// carries on: storing the message, the reply or the close is the inbox's job,
// and a figure must never be why one of those is lost.
func (w csWaits) record(tx *gorm.DB, conversationID uuid.UUID, step string, fn func(*gorm.DB) error) {
	if err := tx.Transaction(fn); err != nil {
		w.logger.Error("Could not record a CS wait",
			zap.String("step", step),
			zap.String("conversation_id", conversationID.String()),
			zap.Error(err))
	}
}

// customerWrote folds a customer's message into the thread's open wait, or
// starts one. An open wait whose customer went quiet for csCustomerSilence ends
// as abandoned, and a new one starts from this message.
func customerWrote(tx *gorm.DB, msg *models.CSMessage) error {
	extended, err := extendOpenWait(tx, msg)
	if err != nil || extended {
		return err
	}
	if _, err := endOpenWait(tx, msg.ConversationID, models.WaitAbandoned, msg.CreatedAt, nil, nil); err != nil {
		return err
	}
	return openWait(tx, msg.ConversationID, msg.CreatedAt, msg.WATimestamp, msg.WATimestamp)
}

// extendOpenWait adds a message to an open wait the customer has not gone quiet
// on. The newest send time is kept, so a message WhatsApp hands over late
// cannot move the wait backwards.
func extendOpenWait(tx *gorm.DB, msg *models.CSMessage) (bool, error) {
	res := tx.Model(&models.CSWait{}).
		Where("conversation_id = ? AND ended_at IS NULL AND last_customer_sent_at >= ?",
			msg.ConversationID, msg.WATimestamp.Add(-csCustomerSilence)).
		Update("last_customer_sent_at", gorm.Expr(
			"CASE WHEN last_customer_sent_at < ? THEN ? ELSE last_customer_sent_at END",
			msg.WATimestamp, msg.WATimestamp))
	if res.Error != nil {
		return false, fmt.Errorf("extend open wait: %w", res.Error)
	}
	return res.RowsAffected > 0, nil
}

// endOpenWait ends the thread's open wait, answering whether there was one. The
// condition on ended_at is what makes two enders safe: whichever commits second
// finds nothing left to end.
func endOpenWait(tx *gorm.DB, conversationID uuid.UUID, reason models.WaitEndReason, at time.Time,
	endedBy, replyMessageID *uuid.UUID) (bool, error) {
	res := tx.Model(&models.CSWait{}).
		Where("conversation_id = ? AND ended_at IS NULL", conversationID).
		Updates(map[string]any{
			"ended_at":         at,
			"end_reason":       reason,
			"ended_by":         endedBy,
			"reply_message_id": replyMessageID,
		})
	if res.Error != nil {
		return false, fmt.Errorf("end open wait: %w", res.Error)
	}
	return res.RowsAffected > 0, nil
}

// openWait starts a wait on a thread.
func openWait(tx *gorm.DB, conversationID uuid.UUID, startedAt, sentAt, lastSentAt time.Time) error {
	var conv models.CSConversation
	if err := tx.Select("wa_account_id").First(&conv, "id = ?", conversationID).Error; err != nil {
		return fmt.Errorf("load thread for wait: %w", err)
	}
	wait := models.CSWait{
		ConversationID:     conversationID,
		WAAccountID:        conv.WAAccountID,
		StartedAt:          startedAt,
		CustomerSentAt:     sentAt,
		LastCustomerSentAt: lastSentAt,
	}
	if err := tx.Create(&wait).Error; err != nil {
		return fmt.Errorf("open wait: %w", err)
	}
	return nil
}

// replied ends the thread's open wait as answered by the CS who queued msg.
func replied(tx *gorm.DB, msg *models.CSMessage) error {
	_, err := endOpenWait(tx, msg.ConversationID, models.WaitReplied, msg.WATimestamp, msg.SenderUserID, &msg.ID)
	return err
}

// answeredFromPhone ends the open wait a phone reply answers. A reply sent
// before the wait's first message answered an earlier wait and changes nothing.
// When WhatsApp hands the reply over late, the customer's messages sent after it
// are still unanswered, so a new wait starts from the first of them.
func answeredFromPhone(tx *gorm.DB, msg *models.CSMessage) error {
	var open models.CSWait
	res := tx.Where("conversation_id = ? AND ended_at IS NULL", msg.ConversationID).Limit(1).Find(&open)
	if res.Error != nil {
		return fmt.Errorf("load open wait: %w", res.Error)
	}
	if res.RowsAffected == 0 || msg.WATimestamp.Before(open.CustomerSentAt) {
		return nil
	}
	endedAt := msg.WATimestamp
	if endedAt.Before(open.StartedAt) {
		endedAt = open.StartedAt
	}
	endedWait, err := endOpenWait(tx, msg.ConversationID, models.WaitPhone, endedAt, nil, &msg.ID)
	if err != nil || !endedWait {
		return err
	}
	return reopenAfter(tx, msg.ConversationID, msg.WATimestamp)
}

// reopenAfter starts a wait from the customer messages sent after a moment.
func reopenAfter(tx *gorm.DB, conversationID uuid.UUID, after time.Time) error {
	var later []models.CSMessage
	err := tx.Where("conversation_id = ? AND direction = ? AND wa_timestamp > ?",
		conversationID, models.MessageIn, after).
		Order("wa_timestamp ASC").Find(&later).Error
	if err != nil {
		return fmt.Errorf("load messages sent after a phone reply: %w", err)
	}
	if len(later) == 0 {
		return nil
	}
	first, last := later[0], later[len(later)-1]
	return openWait(tx, conversationID, first.CreatedAt, first.WATimestamp, last.WATimestamp)
}

// closedWithoutReply ends the thread's open wait as closed by a CS.
func closedWithoutReply(tx *gorm.DB, conversationID, closedBy uuid.UUID, at time.Time) error {
	_, err := endOpenWait(tx, conversationID, models.WaitClosed, at, &closedBy, nil)
	return err
}
