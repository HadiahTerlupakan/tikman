package services

import (
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// ErrSiteHasTunnel is returned when a site cannot be deleted because a
// WireGuard peer still points at it.
var ErrSiteHasTunnel = errors.New("site still has a VPN tunnel")

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

// Create adds a site without coordinates. It exists alongside
// CreateWithCoordinates, rather than taking latitude/longitude as optional
// arguments, because the ~80 existing call sites across the test suite have
// nothing to do with coordinates and should not need to change to add them.
func (s *SiteService) Create(name, location, description string) (*models.Site, error) {
	return s.CreateWithCoordinates(name, location, description, nil, nil)
}

// CreateWithCoordinates adds a site with an optional latitude/longitude pair.
func (s *SiteService) CreateWithCoordinates(name, location, description string, latitude, longitude *float64) (*models.Site, error) {
	if err := validateCoordinates(latitude, longitude); err != nil {
		return nil, err
	}

	site := &models.Site{
		Name:        strings.TrimSpace(name),
		Location:    strings.TrimSpace(location),
		Description: strings.TrimSpace(description),
		Latitude:    latitude,
		Longitude:   longitude,
	}

	if err := s.db.Create(site).Error; err != nil {
		return nil, fmt.Errorf("failed to create site: %w", err)
	}

	return site, nil
}

// validateCoordinates rejects a point that cannot exist and a pair that is only
// half given. A lone latitude is not partial data: it would place a pin on the
// prime meridian and look like a deliberate answer.
func validateCoordinates(latitude, longitude *float64) error {
	if (latitude == nil) != (longitude == nil) {
		return fmt.Errorf("%w: latitude and longitude must be given together, or both left empty", ErrValidation)
	}
	if latitude == nil {
		return nil
	}
	if *latitude < -90 || *latitude > 90 {
		return fmt.Errorf("%w: latitude %v is outside -90..90", ErrValidation, *latitude)
	}
	if *longitude < -180 || *longitude > 180 {
		return fmt.Errorf("%w: longitude %v is outside -180..180", ErrValidation, *longitude)
	}
	return nil
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
	latitude, hasLatitude := updates["latitude"].(*float64)
	longitude, hasLongitude := updates["longitude"].(*float64)
	if hasLatitude || hasLongitude {
		if err := validateCoordinates(latitude, longitude); err != nil {
			return err
		}
	}

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

// Delete removes a site, refusing while it still has a VPN tunnel. Deleting it
// anyway would leave the peer row applied to the kernel and holding its subnet,
// with nothing left in the UI to remove it from.
func (s *SiteService) Delete(id uuid.UUID) error {
	var tunnels int64
	if err := s.db.Model(&models.WireGuardPeer{}).Where("site_id = ?", id).Count(&tunnels).Error; err != nil {
		return fmt.Errorf("failed to check site tunnels: %w", err)
	}
	if tunnels > 0 {
		return fmt.Errorf("%w: remove the site's tunnel on the VPN page first", ErrSiteHasTunnel)
	}

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
