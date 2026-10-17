package services

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AvatarSubject is someone, or some group, whose profile photo is due a look.
type AvatarSubject struct {
	ID       uuid.UUID
	JID      string
	AvatarID string
}

// WAAvatarService keeps the profile photos of groups and their members.
type WAAvatarService struct {
	db *gorm.DB
}

// NewWAAvatarService constructs a WAAvatarService.
func NewWAAvatarService(db *gorm.DB) *WAAvatarService {
	return &WAAvatarService{db: db}
}

// Track makes sure every JID named has a row, so the sweep gets round to it.
// Rows already there are left as they are.
func (s *WAAvatarService) Track(accountID uuid.UUID, jids []string) error {
	if len(jids) == 0 {
		return nil
	}
	rows := make([]models.WAAvatar, 0, len(jids))
	for _, jid := range uniqueStrings(jids) {
		rows = append(rows, models.WAAvatar{WAAccountID: accountID, JID: jid})
	}
	if err := s.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error; err != nil {
		return fmt.Errorf("track avatars: %w", err)
	}
	return nil
}

// Invalidate asks for a photo to be looked at again on the next sweep, as
// when WhatsApp says it changed.
func (s *WAAvatarService) Invalidate(accountID uuid.UUID, jid string) error {
	if err := s.Track(accountID, []string{jid}); err != nil {
		return err
	}
	err := s.db.Model(&models.WAAvatar{}).Where("wa_account_id = ? AND jid = ?", accountID, jid).
		Update("checked_at", nil).Error
	if err != nil {
		return fmt.Errorf("invalidate avatar: %w", err)
	}
	return nil
}

// StaleSubjects answers the photos due a look, never-checked first and groups
// before people: a group's face is on every row of the list.
func (s *WAAvatarService) StaleSubjects(accountID uuid.UUID, limit int, before time.Time) ([]AvatarSubject, error) {
	var rows []models.WAAvatar
	err := s.db.Where("wa_account_id = ? AND (checked_at IS NULL OR checked_at < ?)", accountID, before).
		Order("checked_at IS NOT NULL, jid NOT LIKE '%@g.us', checked_at").Limit(limit).Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list stale avatars: %w", err)
	}
	out := make([]AvatarSubject, len(rows))
	for i, row := range rows {
		out[i] = AvatarSubject{ID: row.ID, JID: row.JID, AvatarID: row.AvatarID}
	}
	return out, nil
}

// SetAvatarChecked records a look that found nothing new.
func (s *WAAvatarService) SetAvatarChecked(id uuid.UUID, at time.Time) error {
	if err := s.db.Model(&models.WAAvatar{}).Where("id = ?", id).Update("checked_at", at).Error; err != nil {
		return fmt.Errorf("mark avatar checked: %w", err)
	}
	return nil
}

// SetAvatar records a new photo, or none, answering the path it replaced so
// the caller can remove that file.
func (s *WAAvatarService) SetAvatar(id uuid.UUID, avatarID, path string) (string, error) {
	var row models.WAAvatar
	if err := s.db.First(&row, "id = ?", id).Error; err != nil {
		return "", err
	}
	err := s.db.Model(&models.WAAvatar{}).Where("id = ?", id).Updates(map[string]any{
		"avatar_id": avatarID, "path": path, "checked_at": time.Now(),
	}).Error
	if err != nil {
		return "", fmt.Errorf("store avatar: %w", err)
	}
	return row.Path, nil
}

// Path answers where one photo is on disk, "" when there is none.
func (s *WAAvatarService) Path(accountID uuid.UUID, jid string) (string, error) {
	var rows []models.WAAvatar
	err := s.db.Where("wa_account_id = ? AND jid = ? AND path <> ''", accountID, jid).Limit(1).Find(&rows).Error
	if err != nil {
		return "", fmt.Errorf("find avatar: %w", err)
	}
	if len(rows) == 0 {
		return "", nil
	}
	return rows[0].Path, nil
}

type avatarKey struct {
	accountID uuid.UUID
	jid       string
}

// avatarsShown answers which of the JIDs have a photo on disk, by number.
func avatarsShown(db *gorm.DB, jids []string) (map[avatarKey]struct{}, error) {
	var rows []models.WAAvatar
	if err := db.Select("wa_account_id", "jid").Where("jid IN ? AND path <> ''", uniqueStrings(jids)).
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("find avatars: %w", err)
	}
	shown := make(map[avatarKey]struct{}, len(rows))
	for _, row := range rows {
		shown[avatarKey{row.WAAccountID, row.JID}] = struct{}{}
	}
	return shown, nil
}
