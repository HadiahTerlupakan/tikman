package services

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// CSChannelService holds the mirror of the WhatsApp Channels each paired
// number administers. Nothing here talks to WhatsApp — the wa process does
// that and hands the answer to Replace.
type CSChannelService struct {
	db *gorm.DB
}

// NewCSChannelService constructs a CSChannelService.
func NewCSChannelService(db *gorm.DB) *CSChannelService {
	return &CSChannelService{db: db}
}

// List answers every channel the team may post to, across every number, named
// in the order the picker shows them.
func (s *CSChannelService) List() ([]models.WAChannel, error) {
	var rows []models.WAChannel
	if err := s.db.Order("name ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list channels: %w", err)
	}
	return rows, nil
}

// Get loads one channel. It is what turns a picked id into the JID and the
// number an update must be sent through, and what refuses an id that is no
// longer in the mirror.
func (s *CSChannelService) Get(id uuid.UUID) (*models.WAChannel, error) {
	var row models.WAChannel
	if err := s.db.First(&row, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("load channel: %w", err)
	}
	return &row, nil
}

// Replace swaps one number's channels for what WhatsApp just answered.
//
// The delete and the insert are one transaction: a sync that failed halfway
// would otherwise leave the picker empty for a number that still administers
// channels, and nothing would rebuild it until the next hour.
func (s *CSChannelService) Replace(accountID uuid.UUID, channels []models.WAChannel) error {
	syncedAt := time.Now()
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("wa_account_id = ?", accountID).Delete(&models.WAChannel{}).Error; err != nil {
			return fmt.Errorf("clear channels: %w", err)
		}
		if len(channels) == 0 {
			return nil
		}
		for i := range channels {
			channels[i].ID = uuid.Nil
			channels[i].WAAccountID = accountID
			channels[i].SyncedAt = syncedAt
		}
		if err := tx.Create(&channels).Error; err != nil {
			return fmt.Errorf("store channels: %w", err)
		}
		return nil
	})
}
