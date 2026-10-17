package services

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// MessageByWAID answers the message WhatsApp knows by this id in this group,
// nil when it is not stored here: an edit, a reaction or a receipt often
// names a message older than this inbox.
func (s *WAGroupMessageService) MessageByWAID(groupID uuid.UUID, waID string) (*models.WAGroupMessage, error) {
	var rows []models.WAGroupMessage
	if err := s.db.Where("group_id = ? AND wa_message_id = ?", groupID, waID).Limit(1).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("find group message: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return &rows[0], nil
}

// MarkEdited replaces a message's words, as its sender changed them.
func (s *WAGroupMessageService) MarkEdited(id uuid.UUID, body string, mentions []string) error {
	return s.update(id, map[string]any{"body": body, "edited": true, "mentions": encodeJIDs(mentions)})
}

// MarkDeleted empties a message deleted for everyone, answering the
// attachment it held so the caller can remove the file. The row stays, as on
// a phone, to say that something was there.
func (s *WAGroupMessageService) MarkDeleted(id uuid.UUID) (string, error) {
	msg, err := s.Get(id)
	if err != nil {
		return "", err
	}
	err = s.update(id, map[string]any{
		"deleted": true, "body": "", "extra": "", "mentions": "",
		"media_path": "", "media_size": 0, "preview_url": "", "preview_title": "",
		"preview_description": "", "preview_thumbnail": nil,
	})
	if err != nil {
		return "", err
	}
	if err := s.db.Where("message_id = ?", id).Delete(&models.WAGroupReaction{}).Error; err != nil {
		return "", fmt.Errorf("drop reactions of a deleted message: %w", err)
	}
	return msg.MediaPath, nil
}

// SetReaction records one member's emoji on a message; an empty emoji takes
// theirs away.
func (s *WAGroupMessageService) SetReaction(messageID uuid.UUID, actor models.GroupActor, emoji string) error {
	if emoji == "" {
		err := s.db.Where("message_id = ? AND reactor_jid = ?", messageID, actor.JID).
			Delete(&models.WAGroupReaction{}).Error
		if err != nil {
			return fmt.Errorf("remove reaction: %w", err)
		}
		return nil
	}
	row := models.WAGroupReaction{
		MessageID: messageID, ReactorJID: actor.JID, ReactorName: actor.Name, IsSelf: actor.IsSelf,
		Emoji: emoji, UpdatedAt: time.Now(),
	}
	err := s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "message_id"}, {Name: "reactor_jid"}},
		DoUpdates: clause.AssignmentColumns([]string{"emoji", "reactor_name", "updated_at"}),
	}).Create(&row).Error
	if err != nil {
		return fmt.Errorf("store reaction: %w", err)
	}
	return nil
}

// SetStarred stars a message for the team, or takes the star away.
func (s *WAGroupMessageService) SetStarred(id uuid.UUID, starred bool) (*models.WAGroupMessage, error) {
	var at *time.Time
	if starred {
		now := time.Now()
		at = &now
	}
	if err := s.update(id, map[string]any{"starred_at": at}); err != nil {
		return nil, err
	}
	return s.Get(id)
}

// Starred answers a group's starred messages, the most recently starred first.
func (s *WAGroupMessageService) Starred(groupID uuid.UUID) ([]models.WAGroupMessage, error) {
	var rows []models.WAGroupMessage
	err := s.db.Where("group_id = ? AND starred_at IS NOT NULL", groupID).
		Order("starred_at DESC").Limit(defaultHistoryLimit).Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list starred group messages: %w", err)
	}
	if err := s.attachQuotes(rows); err != nil {
		return nil, err
	}
	return rows, s.decorate(groupID, rows)
}

// ErrNoHistoryAnchor is a request for older history in a group that has no
// message WhatsApp knows to count back from.
var ErrNoHistoryAnchor = errors.New("grup ini belum punya pesan untuk dijadikan patokan riwayat")

// OldestAnchor answers the oldest message WhatsApp has an id for, the point
// the phone is asked to count back from.
func (s *WAGroupMessageService) OldestAnchor(groupID uuid.UUID) (*models.WAGroupMessage, error) {
	var msg models.WAGroupMessage
	err := s.db.Where("group_id = ? AND wa_message_id IS NOT NULL AND kind <> ?", groupID, models.MessageKindSystem).
		Order("wa_timestamp ASC").First(&msg).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNoHistoryAnchor
	}
	if err != nil {
		return nil, fmt.Errorf("find oldest group message: %w", err)
	}
	return &msg, nil
}
