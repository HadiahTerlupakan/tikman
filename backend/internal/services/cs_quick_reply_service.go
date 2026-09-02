package services

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// CSQuickReplyService owns the canned answers a CS inserts instead of retyping
// the same sentence forty times a day.
type CSQuickReplyService struct {
	db *gorm.DB
}

// NewCSQuickReplyService constructs a CSQuickReplyService.
func NewCSQuickReplyService(db *gorm.DB) *CSQuickReplyService {
	return &CSQuickReplyService{db: db}
}

// List returns every template, by title.
func (s *CSQuickReplyService) List() ([]models.CSQuickReply, error) {
	var rows []models.CSQuickReply
	if err := s.db.Order("title ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list quick replies: %w", err)
	}
	return rows, nil
}

// Create records a template.
func (s *CSQuickReplyService) Create(title, body string, createdBy uuid.UUID) (*models.CSQuickReply, error) {
	title, body, err := validQuickReply(title, body)
	if err != nil {
		return nil, err
	}

	row := models.CSQuickReply{Title: title, Body: body, CreatedBy: createdBy}
	if err := s.db.Create(&row).Error; err != nil {
		return nil, fmt.Errorf("create quick reply: %w", err)
	}
	return &row, nil
}

// Update rewrites a template.
func (s *CSQuickReplyService) Update(id uuid.UUID, title, body string) (*models.CSQuickReply, error) {
	title, body, err := validQuickReply(title, body)
	if err != nil {
		return nil, err
	}

	res := s.db.Model(&models.CSQuickReply{}).Where("id = ?", id).
		Updates(map[string]any{"title": title, "body": body})
	if res.Error != nil {
		return nil, fmt.Errorf("update quick reply: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	var row models.CSQuickReply
	if err := s.db.First(&row, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("load quick reply: %w", err)
	}
	return &row, nil
}

// Delete removes a template.
func (s *CSQuickReplyService) Delete(id uuid.UUID) error {
	res := s.db.Delete(&models.CSQuickReply{}, "id = ?", id)
	if res.Error != nil {
		return fmt.Errorf("delete quick reply: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func validQuickReply(title, body string) (string, string, error) {
	title, body = strings.TrimSpace(title), strings.TrimSpace(body)
	if title == "" {
		return "", "", fmt.Errorf("%w: judul balasan cepat wajib diisi", ErrValidation)
	}
	if body == "" {
		return "", "", fmt.Errorf("%w: isi balasan cepat wajib diisi", ErrValidation)
	}
	return title, body, nil
}
