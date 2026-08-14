package services

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

type SiteService struct {
	db *gorm.DB
}

func NewSiteService(db *gorm.DB) *SiteService {
	return &SiteService{db: db}
}

func (s *SiteService) Create(name, location, description string) (*models.Site, error) {
	site := &models.Site{
		Name:        name,
		Location:    location,
		Description: description,
	}

	if err := s.db.Create(site).Error; err != nil {
		return nil, fmt.Errorf("failed to create site: %w", err)
	}

	return site, nil
}

func (s *SiteService) GetByID(id uuid.UUID) (*models.Site, error) {
	var site models.Site
	if err := s.db.Preload("OLTs").First(&site, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("site not found: %w", err)
	}
	return &site, nil
}

func (s *SiteService) List() ([]models.Site, error) {
	var sites []models.Site
	if err := s.db.Preload("OLTs").Find(&sites).Error; err != nil {
		return nil, fmt.Errorf("failed to list sites: %w", err)
	}
	return sites, nil
}

func (s *SiteService) Update(id uuid.UUID, updates map[string]interface{}) error {
	if err := s.db.Model(&models.Site{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to update site: %w", err)
	}
	return nil
}

func (s *SiteService) Delete(id uuid.UUID) error {
	if err := s.db.Delete(&models.Site{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("failed to delete site: %w", err)
	}
	return nil
}
