package services

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// routerTrafficSource is the relation a traffic graph reads from.
type routerTrafficSource string

const (
	routerTrafficRaw    routerTrafficSource = "router_interface_metrics"
	routerTraffic5Min   routerTrafficSource = "router_interface_metrics_5min"
	routerTrafficHourly routerTrafficSource = "router_interface_metrics_hourly"
)

// Longest span each resolution is drawn at: past these, a graph has more
// points than pixels.
const (
	routerRawSpan  = 6 * time.Hour
	router5MinSpan = 7 * 24 * time.Hour
)

// maxEventRows bounds the event list; a flapping session over a long range
// would otherwise return thousands of rows to a page that shows a few dozen.
const maxEventRows = 500

// routerTrafficSourceFor picks the finest source fit for the span that still
// holds the range's start. Span alone is not enough: an hour ten days back is
// short, but raw rows that old are already gone.
func routerTrafficSourceFor(from, to, now time.Time) routerTrafficSource {
	span, age := to.Sub(from), now.Sub(from)
	switch {
	case span <= routerRawSpan && age < rawRetention:
		return routerTrafficRaw
	case span <= router5MinSpan && age < fiveMinRetention:
		return routerTraffic5Min
	default:
		return routerTrafficHourly
	}
}

// TrafficPoint is one point on an interface's traffic graph.
type TrafficPoint struct {
	Time   time.Time `json:"time"`
	InBps  float64   `json:"in_bps"`
	OutBps float64   `json:"out_bps"`
}

// BGPCount is a router's session tally for the router list.
type BGPCount struct {
	RouterID    uuid.UUID
	Established int
	Total       int
}

// RouterQueryService reads what the router pages show.
type RouterQueryService struct {
	db *gorm.DB
}

// NewRouterQueryService creates a router query service.
func NewRouterQueryService(db *gorm.DB) *RouterQueryService {
	return &RouterQueryService{db: db}
}

// ListInterfaces returns a router's interfaces grouped by parent port.
func (s *RouterQueryService) ListInterfaces(routerID uuid.UUID) ([]models.RouterInterface, error) {
	var ifaces []models.RouterInterface
	return ifaces, s.db.Where("router_id = ?", routerID).Order("parent_name, name").Find(&ifaces).Error
}

// ListSessions returns a router's BGP sessions by name.
func (s *RouterQueryService) ListSessions(routerID uuid.UUID) ([]models.RouterBGPSession, error) {
	var sessions []models.RouterBGPSession
	return sessions, s.db.Where("router_id = ?", routerID).Order("name").Find(&sessions).Error
}

// ListEvents returns a router's BGP events in a range, newest first.
func (s *RouterQueryService) ListEvents(routerID uuid.UUID, from, to time.Time) ([]models.RouterBGPEvent, error) {
	var events []models.RouterBGPEvent
	return events, s.db.Where("router_id = ? AND occurred_at >= ? AND occurred_at <= ?", routerID, from, to).
		Order("occurred_at DESC").Limit(maxEventRows).Find(&events).Error
}

// InterfaceTraffic returns an interface's traffic over a range. The interface
// must belong to the router, so one router's page cannot read another's.
func (s *RouterQueryService) InterfaceTraffic(routerID, interfaceID uuid.UUID, from, to time.Time) ([]TrafficPoint, error) {
	var iface models.RouterInterface
	if err := s.db.Where("id = ? AND router_id = ?", interfaceID, routerID).First(&iface).Error; err != nil {
		return nil, err
	}

	source := routerTrafficSourceFor(from, to, time.Now())
	query := `SELECT time, in_bps, out_bps FROM router_interface_metrics
		WHERE interface_id = ? AND time >= ? AND time <= ? ORDER BY time`
	if source != routerTrafficRaw {
		// source is one of the constants above, never request input.
		query = fmt.Sprintf(`SELECT bucket AS time, avg_in_bps AS in_bps, avg_out_bps AS out_bps FROM %s
			WHERE interface_id = ? AND bucket >= ? AND bucket <= ? ORDER BY bucket`, source)
	}

	var points []TrafficPoint
	return points, s.db.Raw(query, interfaceID, from, to).Scan(&points).Error
}

// SilentInterfaces counts each router's enabled interfaces marked silent, for
// the router list.
func (s *RouterQueryService) SilentInterfaces() (map[uuid.UUID]int, error) {
	var rows []struct {
		RouterID uuid.UUID
		Silent   int
	}
	err := s.db.Model(&models.RouterInterface{}).
		Select("router_id, COUNT(*) AS silent").
		Where("silent_since IS NOT NULL AND disabled = ?", false).
		Group("router_id").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	counts := make(map[uuid.UUID]int, len(rows))
	for _, row := range rows {
		counts[row.RouterID] = row.Silent
	}
	return counts, nil
}

// BGPSummary tallies sessions per router for the router list.
func (s *RouterQueryService) BGPSummary() (map[uuid.UUID]BGPCount, error) {
	var rows []BGPCount
	err := s.db.Model(&models.RouterBGPSession{}).
		Select("router_id, SUM(CASE WHEN state = ? THEN 1 ELSE 0 END) AS established, COUNT(*) AS total",
			models.BGPStateEstablished).
		Group("router_id").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	summary := make(map[uuid.UUID]BGPCount, len(rows))
	for _, row := range rows {
		summary[row.RouterID] = row
	}
	return summary, nil
}
