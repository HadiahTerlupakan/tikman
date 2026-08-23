package services

import (
	"errors"
	"fmt"
	"strings"

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

// GetDB returns the database instance
func (s *SiteService) GetDB() *gorm.DB {
	return s.db
}

func (s *SiteService) Create(name, location, description string) (*models.Site, error) {
	site := &models.Site{
		Name:        strings.TrimSpace(name),
		Location:    strings.TrimSpace(location),
		Description: strings.TrimSpace(description),
	}

	if err := s.db.Create(site).Error; err != nil {
		return nil, fmt.Errorf("failed to create site: %w", err)
	}

	return site, nil
}

func (s *SiteService) GetByID(id uuid.UUID) (*models.Site, error) {
	var site models.Site
	if err := s.db.First(&site, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("site not found: %w", err)
		}
		return nil, fmt.Errorf("database error: %w", err)
	}
	return &site, nil
}

func (s *SiteService) List() ([]models.Site, error) {
	var sites []models.Site
	if err := s.db.Find(&sites).Error; err != nil {
		return nil, fmt.Errorf("failed to list sites: %w", err)
	}
	return sites, nil
}

func (s *SiteService) Update(id uuid.UUID, updates map[string]interface{}) error {
	for _, field := range []string{"name", "location", "description"} {
		if value, ok := updates[field].(string); ok {
			updates[field] = strings.TrimSpace(value)
		}
	}

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

// CountOLTsBySite returns how many OLTs are attached to a site. The DTO layer
// renders this count, but the query itself belongs to the service so the API
// layer never touches the database directly.
func (s *SiteService) CountOLTsBySite(siteID uuid.UUID) (int64, error) {
	var count int64
	err := s.db.Model(&models.OLT{}).Where("site_id = ?", siteID).Count(&count).Error
	return count, err
}
