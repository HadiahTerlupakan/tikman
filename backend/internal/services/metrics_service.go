package services

import (
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"gorm.io/gorm"
)

// MetricsService handles ONT metrics operations
type MetricsService struct {
	db *gorm.DB
}

// NewMetricsService creates a new metrics service
func NewMetricsService(db *gorm.DB) *MetricsService {
	return &MetricsService{db: db}
}

// StoreMetrics saves ONT metrics to ont_metrics hypertable
func (s *MetricsService) StoreMetrics(ontID uuid.UUID, metrics *connectivity.ONTMetrics) error {
	query := `
		INSERT INTO ont_metrics (time, ont_id, rx_power, tx_power, temperature, voltage, distance, rx_bytes, tx_bytes)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	return s.db.Exec(query,
		time.Now(),
		ontID,
		metrics.RxPower,
		metrics.TxPower,
		metrics.Temperature,
		metrics.Voltage,
		metrics.Distance,
		metrics.RxBytes,
		metrics.TxBytes,
	).Error
}

// ONTMetricsRow represents a metrics row from database
type ONTMetricsRow struct {
	Time        time.Time
	ONTID       uuid.UUID
	RxPower     float64
	TxPower     float64
	Temperature float64
	Voltage     float64
	Distance    int
	RxBytes     uint64
	TxBytes     uint64
}

// GetLatestMetrics retrieves the latest metrics for an ONT
func (s *MetricsService) GetLatestMetrics(ontID uuid.UUID) (*ONTMetricsRow, error) {
	var metrics ONTMetricsRow

	err := s.db.Raw(`
		SELECT time, ont_id, rx_power, tx_power, temperature, voltage, distance, rx_bytes, tx_bytes
		FROM ont_metrics
		WHERE ont_id = $1
		ORDER BY time DESC
		LIMIT 1
	`, ontID).Scan(&metrics).Error

	if err != nil {
		return nil, err
	}

	return &metrics, nil
}

// GetMetricsHistory retrieves metrics history for an ONT within a time range
func (s *MetricsService) GetMetricsHistory(ontID uuid.UUID, startTime, endTime time.Time) ([]ONTMetricsRow, error) {
	var metrics []ONTMetricsRow

	err := s.db.Raw(`
		SELECT time, ont_id, rx_power, tx_power, temperature, voltage, distance, rx_bytes, tx_bytes
		FROM ont_metrics
		WHERE ont_id = $1 AND time >= $2 AND time <= $3
		ORDER BY time DESC
	`, ontID, startTime, endTime).Scan(&metrics).Error

	return metrics, err
}
