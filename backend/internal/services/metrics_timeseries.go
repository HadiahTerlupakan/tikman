package services

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

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
