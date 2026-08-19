package services

import (
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
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
		INSERT INTO ont_metrics (time, ont_id, rx_power, tx_power, temperature, voltage, distance, rx_bytes, tx_bytes, rx_packets, tx_packets, rx_errors, tx_errors)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`

	err := s.db.Exec(query,
		time.Now(),
		ontID,
		metrics.RxPower,
		metrics.TxPower,
		metrics.Temperature,
		metrics.Voltage,
		metrics.Distance,
		metrics.RxBytes,
		metrics.TxBytes,
		metrics.RxPackets,
		metrics.TxPackets,
		metrics.RxErrors,
		metrics.TxErrors,
	).Error

	if err == nil {
		log.Printf("[Metrics] ✅ Stored for %s: RX=%v TX=%v Dist=%dm RxPkts=%d TxPkts=%d",
			ontID.String(),
			formatPowerValue(metrics.RxPower),
			formatPowerValue(metrics.TxPower),
			metrics.Distance,
			metrics.RxPackets,
			metrics.TxPackets)
	} else {
		log.Printf("[Metrics] ❌ Failed: %v", err)
	}

	return err
}

// formatPowerValue formats power value for logging
func formatPowerValue(val *float64) string {
	if val == nil {
		return "null"
	}
	return fmt.Sprintf("%.2f dBm", *val)
}

// GetPollingStats returns current polling statistics
func (s *MetricsService) GetPollingStats() map[string]interface{} {
	stats := make(map[string]interface{})

	var totalONTs int64
	s.db.Model(&models.ONT{}).Count(&totalONTs)

	var ontsWithMetrics int64
	tenMinutesAgo := time.Now().Add(-10 * time.Minute)
	s.db.Raw(`
		SELECT COUNT(DISTINCT ont_id) FROM ont_metrics
		WHERE time >= ?
	`, tenMinutesAgo).Scan(&ontsWithMetrics)

	percentage := float64(0)
	if totalONTs > 0 {
		percentage = float64(ontsWithMetrics) / float64(totalONTs) * 100
	}

	stats["total_onts"] = totalONTs
	stats["onts_with_metrics"] = ontsWithMetrics
	stats["percentage"] = percentage
	stats["last_poll_time"] = time.Now()

	log.Printf("[Metrics Stats] Total ONTs=%d, With Metrics=%d (%.1f%%), Last Poll=%v",
		totalONTs, ontsWithMetrics, percentage, stats["last_poll_time"])

	return stats
}

// GetOLTPollingStats returns polling statistics for a specific OLT
func (s *MetricsService) GetOLTPollingStats(oltID uuid.UUID) map[string]interface{} {
	stats := make(map[string]interface{})

	// Count total ONTs for this OLT
	var totalONTs int64
	s.db.Model(&models.ONT{}).Where("olt_id = ?", oltID).Count(&totalONTs)

	// Count ONTs with recent metrics (last 10 minutes) for this OLT
	var ontsWithMetrics int64
	tenMinutesAgo := time.Now().Add(-10 * time.Minute)
	s.db.Raw(`
		SELECT COUNT(DISTINCT om.ont_id)
		FROM ont_metrics om
		JOIN onts o ON om.ont_id = o.id
		WHERE o.olt_id = ? AND om.time >= ?
	`, oltID, tenMinutesAgo).Scan(&ontsWithMetrics)

	percentage := float64(0)
	if totalONTs > 0 {
		percentage = float64(ontsWithMetrics) / float64(totalONTs) * 100
	}

	stats["total_onts"] = totalONTs
	stats["onts_with_metrics"] = ontsWithMetrics
	stats["percentage"] = percentage
	stats["last_poll_time"] = time.Now()
	stats["olt_id"] = oltID.String()

	log.Printf("[OLT Stats] OLT=%s: Total ONTs=%d, With Metrics=%d (%.1f%%)",
		oltID.String(), totalONTs, ontsWithMetrics, percentage)

	return stats
}

// ONTMetricsRow represents a metrics row from database.
// Power columns are nullable: NULL means the ONT reported no optical signal.
type ONTMetricsRow struct {
	Time          time.Time
	ONTID         uuid.UUID
	RxPower       *float64
	TxPower       *float64
	Temperature   float64
	Voltage       float64
	TxBiasCurrent float64
	Distance      int
	RxBytes       uint64
	TxBytes       uint64
	RxPackets     uint64
	TxPackets     uint64
	RxErrors      uint64
	TxErrors      uint64
}

// GetLatestMetrics retrieves the latest metrics for an ONT
func (s *MetricsService) GetLatestMetrics(ontID uuid.UUID) (*ONTMetricsRow, error) {
	var metrics ONTMetricsRow

	err := s.db.Raw(`
		SELECT time, ont_id, rx_power, tx_power, temperature, voltage, tx_bias_current,
		       distance, rx_bytes, tx_bytes, rx_packets, tx_packets, rx_errors, tx_errors
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

// GetLatestMetricsBatch retrieves the latest metrics for multiple ONTs in a single query
func (s *MetricsService) GetLatestMetricsBatch(ontIDs []uuid.UUID) (map[uuid.UUID]*ONTMetricsRow, error) {
	if len(ontIDs) == 0 {
		return make(map[uuid.UUID]*ONTMetricsRow), nil
	}

	var metrics []ONTMetricsRow

	err := s.db.Raw(`
		WITH ranked_metrics AS (
			SELECT 
				time, ont_id, rx_power, tx_power, temperature, voltage, tx_bias_current,
				distance, rx_bytes, tx_bytes, rx_packets, tx_packets, rx_errors, tx_errors,
				ROW_NUMBER() OVER (PARTITION BY ont_id ORDER BY time DESC) as rn
			FROM ont_metrics
			WHERE ont_id IN ?
		)
		SELECT time, ont_id, rx_power, tx_power, temperature, voltage, tx_bias_current,
		       distance, rx_bytes, tx_bytes, rx_packets, tx_packets, rx_errors, tx_errors
		FROM ranked_metrics
		WHERE rn = 1
	`, ontIDs).Scan(&metrics).Error

	if err != nil {
		return nil, err
	}

	metricsMap := make(map[uuid.UUID]*ONTMetricsRow, len(metrics))
	for i := range metrics {
		metricsMap[metrics[i].ONTID] = &metrics[i]
	}

	return metricsMap, nil
}
