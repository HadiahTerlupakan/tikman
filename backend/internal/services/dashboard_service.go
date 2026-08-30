package services

import (
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// weakSignalCount is how many of the worst links the dashboard shows. Enough to
// name today's site visits without turning the card into a second ONT list.
const weakSignalCount = 5

// ONTStatusCounts is how many ONTs are in each state, across every OLT.
type ONTStatusCounts struct {
	Total     int64 `json:"total"`
	Online    int64 `json:"online"`
	Offline   int64 `json:"offline"`
	LOS       int64 `json:"los"`
	DyingGasp int64 `json:"dying_gasp"`
	Unknown   int64 `json:"unknown"`
}

// OLTBreakdown is one row of the dashboard's per-OLT table.
type OLTBreakdown struct {
	OLTID     uuid.UUID `json:"olt_id"`
	OLTName   string    `json:"olt_name"`
	OLTStatus string    `json:"olt_status"`
	ONTTotal  int64     `json:"ont_total"`
	Online    int64     `json:"online"`
	Impaired  int64     `json:"impaired"`
}

// WeakSignal is one of the worst optical readings currently being received.
type WeakSignal struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	SerialNumber string    `json:"serial_number"`
	OLTName      string    `json:"olt_name"`
	RxPower      float64   `json:"rx_power"`
}

// DashboardStats is everything the overview page needs, counted by the
// database rather than by the browser.
type DashboardStats struct {
	ONTs           ONTStatusCounts `json:"onts"`
	OLTs           []OLTBreakdown  `json:"olts"`
	WeakestSignals []WeakSignal    `json:"weakest_signals"`
}

// DashboardService answers the overview page in aggregate.
//
// The page used to fetch a page of ONTs and count them in the browser, which
// silently described only the first 500 of them: a chassis with 651 ONTs
// appeared to have 221, and the availability figures were drawn from whichever
// rows happened to fit. Counting in SQL is correct at any size and sends no ONT
// rows at all.
type DashboardService struct {
	db *gorm.DB
}

// NewDashboardService creates a new dashboard service.
func NewDashboardService(db *gorm.DB) *DashboardService {
	return &DashboardService{db: db}
}

// Stats returns the whole overview in one read.
func (s *DashboardService) Stats() (*DashboardStats, error) {
	onts, err := s.ontStatusCounts()
	if err != nil {
		return nil, err
	}
	olts, err := s.oltBreakdown()
	if err != nil {
		return nil, err
	}
	weakest, err := s.weakestSignals()
	if err != nil {
		return nil, err
	}
	return &DashboardStats{ONTs: onts, OLTs: olts, WeakestSignals: weakest}, nil
}

func (s *DashboardService) ontStatusCounts() (ONTStatusCounts, error) {
	var rows []struct {
		Status string
		Count  int64
	}
	if err := s.db.Model(&models.ONT{}).
		Select("status, COUNT(*) AS count").
		Group("status").
		Scan(&rows).Error; err != nil {
		return ONTStatusCounts{}, err
	}

	counts := ONTStatusCounts{}
	for _, row := range rows {
		counts.Total += row.Count
		switch models.ONTStatus(row.Status) {
		case models.ONTStatusOnline:
			counts.Online = row.Count
		case models.ONTStatusOffline:
			counts.Offline = row.Count
		case models.ONTStatusLOS:
			counts.LOS = row.Count
		case models.ONTStatusDyingGas:
			counts.DyingGasp = row.Count
		default:
			// An unrecognised state is still an ONT the operator owns, so it is
			// counted rather than dropped from the total it belongs to.
			counts.Unknown += row.Count
		}
	}
	return counts, nil
}

// oltBreakdown lists every OLT, including ones with no ONTs: an OLT that has
// gone quiet is exactly what the table exists to show, and leaving it out would
// make an empty result look like a healthy one.
func (s *DashboardService) oltBreakdown() ([]OLTBreakdown, error) {
	rows := make([]OLTBreakdown, 0)
	err := s.db.Model(&models.OLT{}).
		Select(`olts.id AS olt_id, olts.name AS olt_name, olts.status AS olt_status,
			COUNT(onts.id) AS ont_total,
			SUM(CASE WHEN onts.status = ? THEN 1 ELSE 0 END) AS online`, models.ONTStatusOnline).
		Joins("LEFT JOIN onts ON onts.olt_id = olts.id").
		Group("olts.id, olts.name, olts.status").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	for i := range rows {
		rows[i].Impaired = rows[i].ONTTotal - rows[i].Online
	}
	return rows, nil
}

// weakestSignals returns the worst readings among ONTs that are still up. An
// offline ONT reports whatever it last measured before going dark, and those
// stale readings are the most negative in the table — they would fill the list
// and hide the links that are still carrying traffic while degrading, which are
// the ones worth a site visit.
func (s *DashboardService) weakestSignals() ([]WeakSignal, error) {
	rows := make([]WeakSignal, 0, weakSignalCount)
	err := s.db.Model(&models.ONT{}).
		Select("onts.id, onts.name, onts.serial_number, olts.name AS olt_name, onts.rx_power").
		Joins("JOIN olts ON olts.id = onts.olt_id").
		Where("onts.status = ? AND onts.rx_power IS NOT NULL", models.ONTStatusOnline).
		Order("onts.rx_power ASC").
		Limit(weakSignalCount).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}
