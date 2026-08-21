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

func (s *MetricsService) GetDB() *gorm.DB {
	return s.db
}

// StoreMetrics saves ONT metrics to ont_metrics hypertable. Rates come from
// the OLT's live octet-rate gauges; nil means no gauge reading was available.
func (s *MetricsService) StoreMetrics(ontID uuid.UUID, metrics *connectivity.ONTMetrics, rates *connectivity.ONUTrafficRates) error {
	query := `
		INSERT INTO ont_metrics (time, ont_id, rx_power, tx_power, temperature, voltage, distance, rx_bytes, tx_bytes, rx_packets, tx_packets, rx_errors, tx_errors, rx_rate_mbps, tx_rate_mbps)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`

	var rxRateMbps, txRateMbps *float64
	if rates != nil {
		rx := float64(rates.RxOctetBps) * 8 / 1000000
		tx := float64(rates.TxOctetBps) * 8 / 1000000
		rxRateMbps, txRateMbps = &rx, &tx
	}

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
		rxRateMbps,
		txRateMbps,
	).Error

	if err == nil {
		log.Printf("[Metrics] ✅ Stored for %s: RX=%v TX=%v Dist=%dm RxPkts=%d TxPkts=%d Rates(up=%v down=%v Mbps)",
			ontID.String(),
			formatPowerValue(metrics.RxPower),
			formatPowerValue(metrics.TxPower),
			metrics.Distance,
			metrics.RxPackets,
			metrics.TxPackets,
			rxRateMbps,
			txRateMbps)
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

	var ontsPolled int64
	tenMinutesAgo := time.Now().Add(-10 * time.Minute)
	s.db.Raw(`
		SELECT COUNT(DISTINCT om.ont_id)
		FROM ont_metrics om
		JOIN onts o ON om.ont_id = o.id
		WHERE om.time >= ?
	`, tenMinutesAgo).Scan(&ontsPolled)

	percentage := float64(0)
	if totalONTs > 0 {
		percentage = float64(ontsPolled) / float64(totalONTs) * 100
	}

	stats["total_onts"] = totalONTs
	stats["onts_with_metrics"] = ontsPolled
	stats["percentage"] = percentage
	stats["last_poll_time"] = time.Now()

	log.Printf("[Metrics Stats] Total ONTs=%d, Polled=%d (%.1f%%), Last Poll=%v",
		totalONTs, ontsPolled, percentage, stats["last_poll_time"])

	return stats
}

// GetOLTPollingStats returns polling statistics for a specific OLT
func (s *MetricsService) GetOLTPollingStats(oltID uuid.UUID) map[string]interface{} {
	stats := make(map[string]interface{})

	// Count total ONTs for this OLT
	var totalONTs int64
	s.db.Model(&models.ONT{}).Where("olt_id = ?", oltID).Count(&totalONTs)

	var ontsPolled int64
	tenMinutesAgo := time.Now().Add(-10 * time.Minute)
	s.db.Raw(`
		SELECT COUNT(DISTINCT om.ont_id)
		FROM ont_metrics om
		JOIN onts o ON om.ont_id = o.id
		WHERE o.olt_id = ? AND om.time >= ?
	`, oltID, tenMinutesAgo).Scan(&ontsPolled)

	percentage := float64(0)
	if totalONTs > 0 {
		percentage = float64(ontsPolled) / float64(totalONTs) * 100
	}

	stats["total_onts"] = totalONTs
	stats["onts_with_metrics"] = ontsPolled
	stats["percentage"] = percentage
	stats["last_poll_time"] = time.Now()
	stats["olt_id"] = oltID.String()

	log.Printf("[OLT Stats] OLT=%s: Total ONTs=%d, Polled=%d (%.1f%%)",
		oltID.String(), totalONTs, ontsPolled, percentage)

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
	RxRateMbps    *float64
	TxRateMbps    *float64
}

// GetLatestMetrics retrieves the latest metrics for an ONT
func (s *MetricsService) GetLatestMetrics(ontID uuid.UUID) (*ONTMetricsRow, error) {
	var metrics ONTMetricsRow

	result := s.db.Raw(`
		SELECT time, ont_id, rx_power, tx_power, temperature, voltage, tx_bias_current,
		       distance, rx_bytes, tx_bytes, rx_packets, tx_packets, rx_errors, tx_errors,
		       rx_rate_mbps, tx_rate_mbps
		FROM ont_metrics
		WHERE ont_id = $1
		ORDER BY time DESC
		LIMIT 1
	`, ontID).Scan(&metrics)

	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, fmt.Errorf("metrics not found")
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

func (s *MetricsService) GetRealtimeMetrics(ontID uuid.UUID) (*ONTMetricsRow, error) {
	var ont models.ONT
	err := s.db.First(&ont, "id = ?", ontID).Error
	if err != nil {
		return nil, fmt.Errorf("ONT not found: %w", err)
	}

	var olt models.OLT
	err = s.db.First(&olt, "id = ?", ont.OLTID).Error
	if err != nil {
		return nil, fmt.Errorf("OLT not found: %w", err)
	}

	if ont.Slot == nil {
		return nil, fmt.Errorf("ONT slot not yet discovered by worker")
	}

	slot := *ont.Slot
	port := int(ont.PortID)
	ontIDInt := int(ont.ONTID)

	snmpMetrics, err := connectivity.QuerySingleONTMetrics(
		olt.IPAddress,
		olt.SNMPCommunity,
		olt.SNMPPort,
		slot,
		port,
		ontIDInt,
	)
	if err != nil {
		return nil, fmt.Errorf("SNMP query failed: %w", err)
	}

	row := &ONTMetricsRow{
		Time:          time.Now(),
		ONTID:         ontID,
		RxPower:       snmpMetrics.RxPower,
		TxPower:       snmpMetrics.TxPower,
		Temperature:   snmpMetrics.Temperature,
		Voltage:       snmpMetrics.Voltage,
		TxBiasCurrent: snmpMetrics.TxBiasCurrent,
		Distance:      snmpMetrics.Distance,
		RxBytes:       snmpMetrics.RxBytes,
		TxBytes:       snmpMetrics.TxBytes,
		RxPackets:     snmpMetrics.RxPackets,
		TxPackets:     snmpMetrics.TxPackets,
		RxErrors:      snmpMetrics.RxErrors,
		TxErrors:      snmpMetrics.TxErrors,
	}

	// Live rate gauges are separate tables; failure here only means rates stay
	// unset, the rest of the metrics are still returned.
	if rates, err := connectivity.QueryONUTrafficRates(olt.IPAddress, olt.SNMPCommunity, olt.SNMPPort, slot, port, ontIDInt); err == nil {
		rx := float64(rates.RxOctetBps) * 8 / 1000000
		tx := float64(rates.TxOctetBps) * 8 / 1000000
		row.RxRateMbps, row.TxRateMbps = &rx, &tx
	} else {
		log.Printf("[Realtime] Rate gauges unavailable for ONT %s: %v", ontID, err)
	}

	return row, nil
}

func (s *MetricsService) GetONTTrafficTimeSeries(ontID uuid.UUID, period string) ([]ONTMetricsRow, error) {
	var interval string
	var duration string

	switch period {
	case "3h":
		interval = "5 minutes"
		duration = "3 hours"
	case "6h":
		interval = "10 minutes"
		duration = "6 hours"
	case "1d":
		interval = "30 minutes"
		duration = "1 day"
	case "3d":
		interval = "1 hour"
		duration = "3 days"
	case "7d":
		interval = "2 hours"
		duration = "7 days"
	case "30d":
		interval = "6 hours"
		duration = "30 days"
	default:
		interval = "5 minutes"
		duration = "3 hours"
	}

	var results []ONTMetricsRow
	query := fmt.Sprintf(`
		SELECT
			time_bucket('%s', time) AS time,
			? AS ont_id,
			AVG(rx_power) AS rx_power,
			AVG(tx_power) AS tx_power,
			AVG(temperature) AS temperature,
			AVG(voltage) AS voltage,
			ROUND(AVG(distance))::integer AS distance,
			MAX(rx_bytes) AS rx_bytes,
			MAX(tx_bytes) AS tx_bytes,
			MAX(rx_packets) AS rx_packets,
			MAX(tx_packets) AS tx_packets,
			SUM(rx_errors) AS rx_errors,
			SUM(tx_errors) AS tx_errors,
			AVG(rx_rate_mbps) AS rx_rate_mbps,
			AVG(tx_rate_mbps) AS tx_rate_mbps
		FROM ont_metrics
		WHERE ont_id = ?
		  AND time > NOW() - INTERVAL '%s'
		GROUP BY time_bucket('%s', time)
		ORDER BY time ASC
	`, interval, duration, interval)

	err := s.db.Raw(query, ontID, ontID).Scan(&results).Error

	if err != nil {
		return nil, fmt.Errorf("failed to query time series: %w", err)
	}

	return results, nil
}
