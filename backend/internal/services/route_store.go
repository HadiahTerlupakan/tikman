package services

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/datatypes"
)

// SetODPRoute records the path a distribution cable takes, or clears it.
//
// Clearing rather than storing the two ends is what "automatic" means here: the
// map then draws the straight line between whatever the endpoints are today, so
// moving the box moves its cable instead of leaving a line to where it stood.
func (s *DistributionService) SetODPRoute(odpID uuid.UUID, points []models.RoutePoint) error {
	stored, meters, err := encodeRoute(points)
	if err != nil {
		return err
	}
	return s.db.Model(&models.ODP{}).Where("id = ?", odpID).
		Updates(map[string]interface{}{"route": stored, "route_meters": meters}).Error
}

// SetODCFeedRoute records the path a feeder cable takes, or clears it.
func (s *DistributionService) SetODCFeedRoute(feedID uuid.UUID, points []models.RoutePoint) error {
	stored, meters, err := encodeRoute(points)
	if err != nil {
		return err
	}
	return s.db.Model(&models.ODCFeed{}).Where("id = ?", feedID).
		Updates(map[string]interface{}{"route": stored, "route_meters": meters}).Error
}

// encodeRoute turns a traced path into what is stored beside it: the vertices,
// and the length they add up to.
func encodeRoute(points []models.RoutePoint) (interface{}, float64, error) {
	if len(points) == 0 {
		return nil, 0.0, nil
	}
	if len(points) < 2 {
		return nil, 0, fmt.Errorf("%w: a route needs at least two points", ErrValidation)
	}
	raw, err := json.Marshal(points)
	if err != nil {
		return nil, 0, err
	}
	// datatypes.JSON, not the raw []byte: the pool talks the simple protocol,
	// where a []byte parameter is written as a bytea literal and jsonb refuses
	// it. datatypes.JSON hands the driver a string, which jsonb parses.
	return datatypes.JSON(raw), RouteMeters(points), nil
}

// ODPByID reads one distribution box, path included.
func (s *DistributionService) ODPByID(odpID uuid.UUID) (*models.ODP, error) {
	var odp models.ODP
	if err := s.db.First(&odp, "id = ?", odpID).Error; err != nil {
		return nil, err
	}
	return &odp, nil
}
