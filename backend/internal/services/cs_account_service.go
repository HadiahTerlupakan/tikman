package services

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// CSAccountService owns the WhatsApp numbers the team answers from. Pairing
// and disconnecting the session itself is the wa process's job — this
// service only reads the rows and marks the one transition (pairing) an
// admin action starts before the wa process confirms it.
type CSAccountService struct {
	db *gorm.DB
}

// NewCSAccountService constructs a CSAccountService.
func NewCSAccountService(db *gorm.DB) *CSAccountService {
	return &CSAccountService{db: db}
}

// List answers every WhatsApp account, oldest first.
func (s *CSAccountService) List() ([]models.WAAccount, error) {
	var rows []models.WAAccount
	if err := s.db.Order("created_at ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list wa accounts: %w", err)
	}
	return rows, nil
}

// Get answers one account, or gorm.ErrRecordNotFound.
func (s *CSAccountService) Get(id uuid.UUID) (*models.WAAccount, error) {
	var row models.WAAccount
	if err := s.db.First(&row, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// MarkPairing records that an admin asked to connect this number, ahead of
// the wa process actually doing it: a browser polling the account list sees
// the change immediately instead of waiting on a process it cannot see.
func (s *CSAccountService) MarkPairing(id uuid.UUID) error {
	res := s.db.Model(&models.WAAccount{}).Where("id = ?", id).
		Update("status", models.WAAccountPairing)
	if res.Error != nil {
		return fmt.Errorf("mark wa account pairing: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
