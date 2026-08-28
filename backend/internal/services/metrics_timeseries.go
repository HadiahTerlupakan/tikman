package services

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// TrafficSeries is a consolidated series with the data used over its window.
type TrafficSeries struct {
	Points []ONTMetricsRow
	Usage  TrafficUsage
}

// GetONTTrafficSeries returns the series for a period together with the volume
// moved over it, which the counters give and the rates cannot.
func (s *MetricsService) GetONTTrafficSeries(ontID uuid.UUID, period string) (TrafficSeries, error) {
	points, err := s.GetONTTrafficTimeSeries(ontID, period)
	if err != nil {
		return TrafficSeries{}, err
	}
	return TrafficSeries{Points: points, Usage: trafficUsageOver(points, trafficTierFor(trafficPeriodWindow(period)).step)}, nil
}

func (s *MetricsService) GetONTTrafficSeriesRange(ontID uuid.UUID, startTime, endTime time.Time, bucket string) (TrafficSeries, error) {
	points, err := s.GetONTTrafficTimeSeriesRangeBucket(ontID, startTime, endTime, bucket)
	if err != nil {
		return TrafficSeries{}, err
	}
	step, _, _, err := trafficBucketBounds(startTime, endTime, bucket)
	if err != nil {
		return TrafficSeries{}, err
	}
	return TrafficSeries{Points: points, Usage: trafficUsageOver(points, step)}, nil
}

func (s *MetricsService) GetONTTrafficTimeSeries(ontID uuid.UUID, period string) ([]ONTMetricsRow, error) {
	endTime := time.Now().UTC()
	startTime := endTime.Add(-trafficPeriodWindow(period))
	return s.getONTTrafficTimeSeriesDurationBucket(ontID, startTime, endTime, 0)
}

func trafficPeriodWindow(period string) time.Duration {
	switch period {
	case "6h":
		return 6 * time.Hour
	case "1d":
		return 24 * time.Hour
	case "3d":
		return 3 * 24 * time.Hour
	case "7d":
		return 7 * 24 * time.Hour
	case "30d":
		return 30 * 24 * time.Hour
	default:
		return 3 * time.Hour
	}
}

// step of 0 lets the tier choose, which is what a period button wants; an
// explicit step is honoured so a custom range keeps the caller's resolution.
func (s *MetricsService) getONTTrafficTimeSeriesDurationBucket(ontID uuid.UUID, startTime, endTime time.Time, step time.Duration) ([]ONTMetricsRow, error) {
	tier := trafficTierFor(endTime.Sub(startTime))
	if step == 0 {
		step = tier.step
	}

	rows, err := s.queryTrafficRows(ontID, startTime, endTime, tier)
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
	// Both ends of the bucket's counters, so usage can be measured across the
	// consolidated series rather than only over the stored one.
	firstRx, lastRx *uint64
	firstTx, lastTx *uint64
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
		if peak := trafficPeak(row.RxMaxMbps, *row.RxRateMbps); peak > total.rxMax {
			total.rxMax = peak
		}
	}
	if row.TxRateMbps != nil {
		total.txSum += *row.TxRateMbps
		if peak := trafficPeak(row.TxMaxMbps, *row.TxRateMbps); peak > total.txMax {
			total.txMax = peak
		}
	}
	total.count++

	if row.FirstRxBytes != nil && total.firstRx == nil {
		total.firstRx = row.FirstRxBytes
	}
	if row.LastRxBytes != nil {
		total.lastRx = row.LastRxBytes
	}
	if row.FirstTxBytes != nil && total.firstTx == nil {
		total.firstTx = row.FirstTxBytes
	}
	if row.LastTxBytes != nil {
		total.lastTx = row.LastTxBytes
	}
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
		Time:         bucketTime,
		ONTID:        ontID,
		RxRateMbps:   &rx,
		TxRateMbps:   &tx,
		RxMaxMbps:    &rxMax,
		TxMaxMbps:    &txMax,
		FirstRxBytes: total.firstRx,
		LastRxBytes:  total.lastRx,
		FirstTxBytes: total.firstTx,
		LastTxBytes:  total.lastTx,
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

// trafficPeak prefers the peak a rollup already recorded. Deriving one from
// averages would flatten a spike that lasted less than the source bucket.
func trafficPeak(recorded *float64, average float64) float64 {
	if recorded != nil && *recorded > average {
		return *recorded
	}
	return average
}

// TrafficUsage is the data an ONU moved over a window, measured from the OLT's
// cumulative counters.
type TrafficUsage struct {
	DownloadBytes uint64 `json:"download_bytes"`
	UploadBytes   uint64 `json:"upload_bytes"`
}

// gponLineRateBps is the downstream line rate of a GPON PON, used as the
// ceiling for what one ONU could physically have moved in a bucket. The
// upstream is half that, so the same figure is a safe bound for both.
const gponLineRateBps = 2_488_320_000

// trafficUsageOver sums the counter's forward movement across the window.
//
// Two discontinuities are excluded rather than counted. A counter that goes
// backwards means the ONU restarted. A jump larger than the link could carry
// in the elapsed time is not traffic either: replacing an ONU, or the day this
// system started reading the right OID, moves the counter by hundreds of
// gigabytes in one step and would otherwise be billed as usage.
func trafficUsageOver(rows []ONTMetricsRow, step time.Duration) TrafficUsage {
	ceiling := plausibleBytes(step)

	var usage TrafficUsage
	var prevRx, prevTx *uint64

	for i := range rows {
		usage.UploadBytes += counterAdvance(prevRx, rows[i].FirstRxBytes, rows[i].LastRxBytes, ceiling)
		usage.DownloadBytes += counterAdvance(prevTx, rows[i].FirstTxBytes, rows[i].LastTxBytes, ceiling)
		if rows[i].LastRxBytes != nil {
			prevRx = rows[i].LastRxBytes
		}
		if rows[i].LastTxBytes != nil {
			prevTx = rows[i].LastTxBytes
		}
	}

	return usage
}

// plausibleBytes is the most one ONU could move in a bucket, used to tell
// traffic from a counter discontinuity.
func plausibleBytes(step time.Duration) uint64 {
	if step <= 0 {
		return 0
	}
	return uint64(step.Seconds() * gponLineRateBps / 8)
}

// counterAdvance measures one bucket's contribution: the traffic since the
// previous bucket's last reading, plus the movement within this one. A step
// beyond what the link could carry is dropped, not clamped: the true figure is
// unknown, and a ceiling reported as fact would be worse than a gap.
func counterAdvance(previous, first, last *uint64, ceiling uint64) uint64 {
	if first == nil || last == nil || *last < *first {
		return 0
	}

	advance := *last - *first
	if ceiling > 0 && advance > ceiling {
		return 0
	}

	if previous != nil && *first >= *previous {
		gap := *first - *previous
		if ceiling == 0 || gap <= ceiling {
			advance += gap
		}
	}
	return advance
}
