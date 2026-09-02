package services

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// CSMediaRetention removes attachments once they are old enough that nobody
// opens them, which is what keeps the VPS disk from filling up at a few
// gigabytes a month.
type CSMediaRetention struct {
	db       *gorm.DB
	root     string
	keepDays int
}

// NewCSMediaRetention constructs a CSMediaRetention.
func NewCSMediaRetention(db *gorm.DB, root string, keepDays int) *CSMediaRetention {
	return &CSMediaRetention{db: db, root: root, keepDays: keepDays}
}

// Sweep deletes attachments past the retention window and forgets their paths.
// The message rows stay: a CS reading old history should still see that the
// customer sent a photo, even when the photo itself is gone.
func (r *CSMediaRetention) Sweep() (int, error) {
	cutoff := time.Now().AddDate(0, 0, -r.keepDays)

	var rows []models.CSMessage
	err := r.db.Where("media_path <> '' AND wa_timestamp < ?", cutoff).Find(&rows).Error
	if err != nil {
		return 0, fmt.Errorf("list expired media: %w", err)
	}

	removed := 0
	for _, row := range rows {
		if err := os.Remove(filepath.Join(r.root, row.MediaPath)); err != nil && !os.IsNotExist(err) {
			return removed, fmt.Errorf("remove %s: %w", row.MediaPath, err)
		}
		err := r.db.Model(&models.CSMessage{}).Where("id = ?", row.ID).
			Updates(map[string]any{"media_path": "", "media_size": 0}).Error
		if err != nil {
			return removed, fmt.Errorf("forget media path: %w", err)
		}
		removed++
	}
	return removed, nil
}
