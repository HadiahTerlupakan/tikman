package services

import (
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/datatypes"
)

// ODPWithUsage is a distribution box and how much room is left in it, which is
// the one thing the map has to say without being clicked.
type ODPWithUsage struct {
	ID        uuid.UUID  `json:"id"`
	Code      string     `json:"code"`
	PortCount int        `json:"port_count"`
	UsedPorts int        `json:"used_ports"`
	Latitude  *float64   `json:"latitude,omitempty"`
	Longitude *float64   `json:"longitude,omitempty"`
	Address   string     `json:"address"`
	Notes     string     `json:"notes"`
	ODCID     *uuid.UUID `json:"odc_id,omitempty"`
	OLTID     *uuid.UUID `json:"olt_id,omitempty"`
	Slot      *int       `json:"slot,omitempty"`
	PortID    *int       `json:"port_id,omitempty"`
	// The traced cable path, empty when nobody has traced it and the map draws
	// the straight line between the ends instead.
	Route       datatypes.JSON `json:"route,omitempty"`
	RouteMeters float64        `json:"route_meters"`
}

// ODCWithUsage is a cabinet, the ports feeding it, and how many boxes hang off
// it.
type ODCWithUsage struct {
	ID        uuid.UUID `json:"id"`
	SiteID    uuid.UUID `json:"site_id"`
	Code      string    `json:"code"`
	Latitude  *float64  `json:"latitude,omitempty"`
	Longitude *float64  `json:"longitude,omitempty"`
	Address   string    `json:"address"`
	Notes     string    `json:"notes"`
	FeedCount int       `json:"feed_count"`
	ODPCount  int       `json:"odp_count"`
}

// ListODPs returns every distribution box with the number of ports taken.
//
// A LEFT JOIN and a GROUP BY rather than a count per box: the map draws all of
// them at once, and a query per marker is a query per marker.
func (s *DistributionService) ListODPs() ([]ODPWithUsage, error) {
	rows := []ODPWithUsage{}
	err := s.db.Model(&models.ODP{}).
		Select(`odps.id, odps.code, odps.port_count, odps.route, odps.route_meters,
		        count(onts.id) AS used_ports,
		        odps.latitude, odps.longitude, odps.address, odps.notes,
		        odps.odc_id, odps.olt_id, odps.slot, odps.port_id`).
		Joins("LEFT JOIN onts ON onts.odp_id = odps.id").
		Group("odps.id").
		Order("odps.code").
		Scan(&rows).Error
	return rows, err
}

// ListODCs returns every cabinet with its feed and box counts.
func (s *DistributionService) ListODCs() ([]ODCWithUsage, error) {
	rows := []ODCWithUsage{}
	err := s.db.Model(&models.ODC{}).
		Select(`odcs.id, odcs.site_id, odcs.code,
		        odcs.latitude, odcs.longitude, odcs.address, odcs.notes,
		        (SELECT count(*) FROM odc_feeds f WHERE f.odc_id = odcs.id) AS feed_count,
		        (SELECT count(*) FROM odps p WHERE p.odc_id = odcs.id) AS odp_count`).
		Order("odcs.code").
		Scan(&rows).Error
	return rows, err
}

// ListODCFeeds returns every feeder, so the map can draw them all at once
// rather than asking cabinet by cabinet.
func (s *DistributionService) ListODCFeeds() ([]models.ODCFeed, error) {
	feeds := []models.ODCFeed{}
	err := s.db.Order("slot, port_id").Find(&feeds).Error
	return feeds, err
}

// ODCFeedsFor returns the PON ports supplying one cabinet.
func (s *DistributionService) ODCFeedsFor(odcID uuid.UUID) ([]models.ODCFeed, error) {
	feeds := []models.ODCFeed{}
	err := s.db.Where("odc_id = ?", odcID).Order("slot, port_id").Find(&feeds).Error
	return feeds, err
}

// SubscribersOn returns the ONTs landing on one distribution box, in port order,
// so a panel can show which ports are taken and by whom.
func (s *DistributionService) SubscribersOn(odpID uuid.UUID) ([]models.ONT, error) {
	onts := []models.ONT{}
	err := s.db.Where("odp_id = ?", odpID).Order("odp_port").Find(&onts).Error
	return onts, err
}
