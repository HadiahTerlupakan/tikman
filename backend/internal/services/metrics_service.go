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
	RxMaxMbps     *float64
	TxMaxMbps     *float64
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
	interval, duration := trafficPeriodSettings(period)
	endTime := time.Now().UTC()
	startTime := endTime.Add(-duration)
	return s.getONTTrafficTimeSeriesDurationBucket(ontID, startTime, endTime, interval)
}

func trafficPeriodSettings(period string) (time.Duration, time.Duration) {
	switch period {
	case "3h":
		return 5 * time.Minute, 3 * time.Hour
	case "6h":
		return 5 * time.Minute, 6 * time.Hour
	case "1d":
		return 5 * time.Minute, 24 * time.Hour
	case "3d":
		return 5 * time.Minute, 3 * 24 * time.Hour
	case "7d":
		return 5 * time.Minute, 7 * 24 * time.Hour
	case "30d":
		return 5 * time.Minute, 30 * 24 * time.Hour
	default:
		return 5 * time.Minute, 3 * time.Hour
	}
}

func (s *MetricsService) getONTTrafficTimeSeriesDurationBucket(ontID uuid.UUID, startTime, endTime time.Time, step time.Duration) ([]ONTMetricsRow, error) {
	rows, err := s.GetONTTrafficTimeSeriesRange(ontID, startTime, endTime)
	if err != nil {
		return nil, err
	}

	byBucket := make(map[time.Time]*trafficTotals)
	for _, row := range rows {
		bucketTime := row.Time.UTC().Truncate(step)
		accumulateTraffic(byBucket, bucketTime, row)
	}

	startBucket := startTime.UTC().Truncate(step)
	endBucket := endTime.UTC().Truncate(step)
	results := make([]ONTMetricsRow, 0, int(endBucket.Sub(startBucket)/step)+1)
	for current := startBucket; !current.After(endBucket); current = current.Add(step) {
		row := buildTrafficRow(ontID, current, byBucket[current])
		// Skip empty buckets - they dilute the average and confuse users
		if row.ONTID != uuid.Nil {
			results = append(results, row)
		}
	}

	return results, nil
}

// GetONTTrafficTimeSeriesRange retrieves traffic rates for an ONT within an explicit time range.
func (s *MetricsService) GetONTTrafficTimeSeriesRange(ontID uuid.UUID, startTime, endTime time.Time) ([]ONTMetricsRow, error) {
	var results []ONTMetricsRow
	query := `
		SELECT
			time,
			ont_id,
			rx_power,
			tx_power,
			temperature,
			voltage,
			distance,
			rx_bytes,
			tx_bytes,
			rx_packets,
			tx_packets,
			rx_errors,
			tx_errors,
			rx_rate_mbps,
			tx_rate_mbps
		FROM ont_metrics
		WHERE ont_id = ?
		  AND time >= ?
		  AND time <= ?
		ORDER BY time ASC
	`

	if err := s.db.Raw(query, ontID, startTime, endTime).Scan(&results).Error; err != nil {
		return nil, fmt.Errorf("failed to query time series range: %w", err)
	}

	return results, nil
}

// GetONTTrafficTimeSeriesRangeBucket retrieves bucketed traffic rates for an explicit range.
func (s *MetricsService) GetONTTrafficTimeSeriesRangeBucket(ontID uuid.UUID, startTime, endTime time.Time, bucket string) ([]ONTMetricsRow, error) {
	step, startBucket, endBucket, err := trafficBucketBounds(startTime, endTime, bucket)
	if err != nil {
		return nil, err
	}

	rows, err := s.GetONTTrafficTimeSeriesRange(ontID, startTime, endTime)
	if err != nil {
		return nil, err
	}

	byBucket := make(map[time.Time]*trafficTotals)
	for _, row := range rows {
		bucketTime := truncateTrafficBucket(row.Time, bucket)
		accumulateTraffic(byBucket, bucketTime, row)
	}

	var results []ONTMetricsRow
	for current := startBucket; !current.After(endBucket); current = addTrafficBucket(current, step, bucket) {
		row := buildTrafficRow(ontID, current, byBucket[current])
		// Skip empty buckets - they dilute the average and confuse users
		if row.ONTID != uuid.Nil {
			results = append(results, row)
		}
	}

	return results, nil
}

// trafficTotals accumulates per-bucket rate samples. sum/count yield the mean;
// rxMax/txMax preserve the raw peak so wider buckets never flatten spikes the way
// a max-of-averages would (MRTG keeps AVERAGE and MAX consolidations separately).
type trafficTotals struct {
	rxSum, txSum float64
	rxMax, txMax float64
	count        int
}

func accumulateTraffic(byBucket map[time.Time]*trafficTotals, bucketTime time.Time, row ONTMetricsRow) {
	// Rows predating the rate columns (migration 08) carry NULL rates. Counting
	// them would emit a 0 Mbps point, which reads as "no traffic" instead of
	// "not measured" and flattens the chart across the whole gap.
	if row.RxRateMbps == nil && row.TxRateMbps == nil {
		return
	}

	total := byBucket[bucketTime]
	if total == nil {
		total = &trafficTotals{}
		byBucket[bucketTime] = total
	}
	if row.RxRateMbps != nil {
		total.rxSum += *row.RxRateMbps
		if *row.RxRateMbps > total.rxMax {
			total.rxMax = *row.RxRateMbps
		}
	}
	if row.TxRateMbps != nil {
		total.txSum += *row.TxRateMbps
		if *row.TxRateMbps > total.txMax {
			total.txMax = *row.TxRateMbps
		}
	}
	total.count++
}

func buildTrafficRow(ontID uuid.UUID, bucketTime time.Time, total *trafficTotals) ONTMetricsRow {
	// Skip empty buckets entirely - they dilute the average and confuse users
	// (MRTG shows gaps for missing data, not zero-filled points)
	if total == nil || total.count == 0 {
		return ONTMetricsRow{}
	}
	rx := total.rxSum / float64(total.count)
	tx := total.txSum / float64(total.count)
	rxMax := total.rxMax
	txMax := total.txMax
	return ONTMetricsRow{
		Time:       bucketTime,
		ONTID:      ontID,
		RxRateMbps: &rx,
		TxRateMbps: &tx,
		RxMaxMbps:  &rxMax,
		TxMaxMbps:  &txMax,
	}
}

func trafficBucketBounds(startTime, endTime time.Time, bucket string) (time.Duration, time.Time, time.Time, error) {
	switch bucket {
	case "hour":
		return time.Hour, truncateTrafficBucket(startTime, bucket), truncateTrafficBucket(endTime, bucket), nil
	case "day":
		return 24 * time.Hour, truncateTrafficBucket(startTime, bucket), truncateTrafficBucket(endTime, bucket), nil
	case "month":
		return 0, truncateTrafficBucket(startTime, bucket), truncateTrafficBucket(endTime, bucket), nil
	default:
		return 0, time.Time{}, time.Time{}, fmt.Errorf("unsupported traffic bucket: %s", bucket)
	}
}

// truncateTrafficBucket aligns a timestamp to its bucket start. Metrics are stored
// in UTC, so bucket keys must be derived in UTC or rows never match the buckets
// generated from the requested range.
func truncateTrafficBucket(value time.Time, bucket string) time.Time {
	utc := value.UTC()
	switch bucket {
	case "hour":
		return utc.Truncate(time.Hour)
	case "day":
		return time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
	case "month":
		return time.Date(utc.Year(), utc.Month(), 1, 0, 0, 0, 0, time.UTC)
	default:
		return utc
	}
}

func addTrafficBucket(value time.Time, step time.Duration, bucket string) time.Time {
	if bucket == "month" {
		return value.AddDate(0, 1, 0)
	}
	return value.Add(step)
}
