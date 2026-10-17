package services

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/wa/linkpreview"
	"gorm.io/gorm"
)

// QueuedGroupMessage is a waiting message together with the group it goes to,
// which is all the drainer needs to send it.
type QueuedGroupMessage struct {
	models.WAGroupMessage
	GroupJID string `gorm:"column:group_jid"`
}

// ClaimQueued answers the group messages still waiting on one number, oldest
// first. Scoped to the account so each session sends only through the number
// that is actually in the group.
func (s *WAGroupMessageService) ClaimQueued(accountID uuid.UUID, limit int) ([]QueuedGroupMessage, error) {
	if limit <= 0 {
		limit = defaultHistoryLimit
	}
	var rows []QueuedGroupMessage
	err := s.db.Table("wa_group_messages AS m").
		Select("m.*, g.jid AS group_jid").
		Joins("JOIN wa_groups g ON g.id = m.group_id").
		Where("m.status = ? AND g.wa_account_id = ?", models.MessageQueued, accountID).
		Order("m.created_at ASC").Limit(limit).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("claim queued group messages: %w", err)
	}
	return rows, nil
}

// MarkSent records that WhatsApp accepted a message, with the link card that
// went out with it.
func (s *WAGroupMessageService) MarkSent(id uuid.UUID, waMessageID string, preview *linkpreview.Preview) error {
	fields := map[string]any{
		"status":        models.MessageSent,
		"wa_message_id": waMessageID,
		"fail_reason":   "",
	}
	if preview != nil {
		fields["preview_url"] = preview.URL
		fields["preview_title"] = preview.Title
		fields["preview_description"] = preview.Description
		fields["preview_thumbnail"] = preview.Thumbnail
	}
	return s.update(id, fields)
}

// MarkFailed records why a message could not be sent.
func (s *WAGroupMessageService) MarkFailed(id uuid.UUID, reason string) error {
	return s.update(id, map[string]any{
		"status":      models.MessageFailed,
		"fail_reason": reason,
	})
}

func (s *WAGroupMessageService) update(id uuid.UUID, fields map[string]any) error {
	res := s.db.Model(&models.WAGroupMessage{}).Where("id = ?", id).Updates(fields)
	if res.Error != nil {
		return fmt.Errorf("update group message: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
