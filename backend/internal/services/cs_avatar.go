package services

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
)

// StaleAvatars returns the conversations whose profile photo is due a look:
// the ones never asked about first, then the ones asked about longest ago.
//
// Never-checked first because that is the customer who has just written in and
// whose face a CS is about to want. before is the age at which a photo already
// held is asked about again.
// Scoped to one number: a session can only ask WhatsApp about customers who
// have written to the number it holds, so asking on behalf of another number's
// threads would query a stranger over the wrong connection.
func (s *CSConversationService) StaleAvatars(accountID uuid.UUID, limit int, before time.Time) ([]models.CSConversation, error) {
	if limit <= 0 {
		limit = 1
	}
	var rows []models.CSConversation
	err := s.db.
		Where("wa_account_id = ?", accountID).
		Where("avatar_checked_at IS NULL OR avatar_checked_at < ?", before).
		Order("avatar_checked_at ASC NULLS FIRST").
		Limit(limit).Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("find conversations due a profile photo: %w", err)
	}
	return rows, nil
}

// SetAvatar records the photo now shown for a conversation and answers the
// file it replaced, which is then referenced by nothing.
//
// Empty arguments mean "there is no photo": a customer taking theirs down is
// as real a change as putting one up, and left alone the inbox would keep
// showing a face they have removed. The caller deletes the replaced file —
// CSMediaRetention sweeps from message rows and would never reach it.
func (s *CSConversationService) SetAvatar(id uuid.UUID, avatarID, path string) (string, error) {
	current, err := s.Get(id)
	if err != nil {
		return "", err
	}

	err = s.db.Model(&models.CSConversation{}).Where("id = ?", id).Updates(map[string]any{
		"avatar_id":         avatarID,
		"avatar_path":       path,
		"avatar_checked_at": time.Now(),
	}).Error
	if err != nil {
		return "", fmt.Errorf("store profile photo: %w", err)
	}

	if current.AvatarPath == path {
		return "", nil
	}
	return current.AvatarPath, nil
}

// SetAvatarChecked records that the question was put and nothing came back to
// store, without disturbing a photo already held.
//
// This is the common path: most customers hide their photo from anyone not in
// their contacts, and without a record of having asked, the sweep would put
// the same question to WhatsApp about the same person on every run.
func (s *CSConversationService) SetAvatarChecked(id uuid.UUID, at time.Time) error {
	err := s.db.Model(&models.CSConversation{}).Where("id = ?", id).
		Update("avatar_checked_at", at).Error
	if err != nil {
		return fmt.Errorf("record profile photo check: %w", err)
	}
	return nil
}
