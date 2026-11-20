package services

import (
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// SwitchBackupSummary is a backup without its text, for the list.
type SwitchBackupSummary struct {
	ID        uuid.UUID `json:"id"`
	TakenAt   time.Time `json:"taken_at"`
	Reason    string    `json:"reason"`
	RawSHA256 string    `json:"raw_sha256"`
	Size      int       `json:"size"`
}

// SwitchQueryService reads what the poller stored about a switch.
type SwitchQueryService struct {
	db *gorm.DB
}

// NewSwitchQueryService creates a switch query service.
func NewSwitchQueryService(db *gorm.DB) *SwitchQueryService {
	return &SwitchQueryService{db: db}
}

// Interfaces returns a switch's ports with their last readings.
func (s *SwitchQueryService) Interfaces(switchID uuid.UUID) ([]models.SwitchInterface, error) {
	var ports []models.SwitchInterface
	return ports, s.db.Where("switch_id = ?", switchID).Order("name").Find(&ports).Error
}

// VLANs returns the VLANs defined on a switch.
func (s *SwitchQueryService) VLANs(switchID uuid.UUID) ([]models.SwitchVLAN, error) {
	var vlans []models.SwitchVLAN
	return vlans, s.db.Where("switch_id = ?", switchID).Order("vid").Find(&vlans).Error
}

// Neighbors returns what was seen behind a switch's ports.
func (s *SwitchQueryService) Neighbors(switchID uuid.UUID) ([]models.SwitchNeighbor, error) {
	var neighbors []models.SwitchNeighbor
	return neighbors, s.db.Where("switch_id = ?", switchID).Order("port, kind, mac").Find(&neighbors).Error
}

// Backups lists a switch's backups, newest first, without the text.
func (s *SwitchQueryService) Backups(switchID uuid.UUID) ([]SwitchBackupSummary, error) {
	var list []SwitchBackupSummary
	err := s.db.Model(&models.SwitchConfigBackup{}).
		Select("id, taken_at, reason, raw_sha256, size").
		Where("switch_id = ?", switchID).Order("taken_at DESC").Scan(&list).Error
	return list, err
}

// Backup returns one backup with its masked text. A backup of another switch
// is not found.
func (s *SwitchQueryService) Backup(switchID, backupID uuid.UUID) (*models.SwitchConfigBackup, error) {
	var backup models.SwitchConfigBackup
	if err := s.db.First(&backup, "id = ? AND switch_id = ?", backupID, switchID).Error; err != nil {
		return nil, err
	}
	return &backup, nil
}

// Diff returns the unified diff of two backups' masked texts.
func (s *SwitchQueryService) Diff(switchID, fromID, toID uuid.UUID) (string, error) {
	from, err := s.Backup(switchID, fromID)
	if err != nil {
		return "", err
	}
	to, err := s.Backup(switchID, toID)
	if err != nil {
		return "", err
	}
	return unifiedDiff(from.TakenAt.Format(time.RFC3339), to.TakenAt.Format(time.RFC3339), from.Masked, to.Masked), nil
}
