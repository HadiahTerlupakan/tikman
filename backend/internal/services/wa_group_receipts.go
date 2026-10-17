package services

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ApplyReceipt records that one member got, or read, some of our messages,
// and moves each message's ticks: delivered once anyone has it, read once
// every member it was sent to has read it, as on a phone. It answers the
// messages it moved. A receipt for a message not stored here is ignored.
func (s *WAGroupMessageService) ApplyReceipt(
	groupID uuid.UUID, waIDs []string, reader string, read bool, at time.Time,
) ([]uuid.UUID, error) {
	var rows []models.WAGroupMessage
	err := s.db.Where("group_id = ? AND direction = ? AND wa_message_id IN ?", groupID, models.MessageOut, waIDs).
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("find messages a receipt names: %w", err)
	}
	moved := make([]uuid.UUID, 0, len(rows))
	for i := range rows {
		if err := s.applyOneReceipt(&rows[i], reader, read, at); err != nil {
			return moved, err
		}
		moved = append(moved, rows[i].ID)
	}
	return moved, nil
}

func (s *WAGroupMessageService) applyOneReceipt(msg *models.WAGroupMessage, reader string, read bool, at time.Time) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		row := models.WAGroupReceipt{MessageID: msg.ID, ReaderJID: reader, DeliveredAt: &at}
		update := []string{"delivered_at"}
		if read {
			row.ReadAt = &at
			update = append(update, "read_at")
		}
		err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "message_id"}, {Name: "reader_jid"}},
			DoUpdates: clause.AssignmentColumns(update),
		}).Create(&row).Error
		if err != nil {
			return fmt.Errorf("store group receipt: %w", err)
		}
		return moveTicks(tx, msg)
	})
}

// moveTicks never walks a message back: receipts arrive out of order, and a
// late "delivered" must not undo "read".
func moveTicks(tx *gorm.DB, msg *models.WAGroupMessage) error {
	var readers int64
	if err := tx.Model(&models.WAGroupReceipt{}).Where("message_id = ? AND read_at IS NOT NULL", msg.ID).
		Count(&readers).Error; err != nil {
		return fmt.Errorf("count readers: %w", err)
	}
	status := models.MessageDelivered
	if msg.MemberCount > 0 && int(readers) >= msg.MemberCount {
		status = models.MessageRead
	}
	if receiptRank[status] <= receiptRank[msg.Status] {
		return nil
	}
	if err := tx.Model(&models.WAGroupMessage{}).Where("id = ?", msg.ID).Update("status", status).Error; err != nil {
		return fmt.Errorf("move group message ticks: %w", err)
	}
	return nil
}

// Receipts answers who got and who read one of our messages, by name.
func (s *WAGroupMessageService) Receipts(messageID uuid.UUID) ([]models.WAGroupReceipt, error) {
	msg, err := s.Get(messageID)
	if err != nil {
		return nil, err
	}
	var rows []models.WAGroupReceipt
	if err := s.db.Where("message_id = ?", messageID).Order("read_at IS NULL, read_at, delivered_at").
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list group receipts: %w", err)
	}
	names := memberNamesOf(s.db, msg.GroupID)
	for i := range rows {
		rows[i].Name = names[jidUser(rows[i].ReaderJID)]
	}
	return rows, nil
}
