package services

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// A graph reads whichever store still holds its window at a resolution worth
// plotting. The raw table is dropped after 7 days, so a 30-day graph that read
// it could only ever draw a week; the rollups exist to answer the wider ones.
//
// The step is the plotted bucket, chosen so a graph lands in the low hundreds
// of points rather than thousands the chart cannot show apart.
type trafficTier struct {
	table string
	step  time.Duration
}

const (
	rawTrafficWindow  = 25 * time.Hour
	fiveMinuteWindow  = 8 * 24 * time.Hour
	rawTrafficTable   = "ont_metrics"
	fiveMinuteRollup  = "ont_metrics_5min"
	hourlyRollupTable = "ont_metrics_hourly"
)

func trafficTierFor(window time.Duration) trafficTier {
	switch {
	case window <= rawTrafficWindow:
		return trafficTier{table: rawTrafficTable, step: 5 * time.Minute}
	case window <= fiveMinuteWindow:
		return trafficTier{table: fiveMinuteRollup, step: 30 * time.Minute}
	default:
		return trafficTier{table: hourlyRollupTable, step: 6 * time.Hour}
	}
}

// queryTrafficRows returns one row per stored bucket in the window, in a shape
// the bucketing code can consolidate further. The rollups already carry an
// average and a peak, so the peak is passed through rather than recomputed
// from averages, which would flatten spikes.
func (s *MetricsService) queryTrafficRows(ontID uuid.UUID, startTime, endTime time.Time, tier trafficTier) ([]ONTMetricsRow, error) {
	var query string
	if tier.table == rawTrafficTable {
		query = `
			SELECT time, rx_rate_mbps, rx_rate_mbps AS rx_max_mbps,
			       tx_rate_mbps, tx_rate_mbps AS tx_max_mbps,
			       rx_bytes AS first_rx_bytes, rx_bytes AS last_rx_bytes,
			       tx_bytes AS first_tx_bytes, tx_bytes AS last_tx_bytes
			FROM ont_metrics
			WHERE ont_id = ? AND time >= ? AND time <= ?
			ORDER BY time ASC`
	} else {
		query = fmt.Sprintf(`
			SELECT bucket AS time, avg_rx_mbps AS rx_rate_mbps, max_rx_mbps AS rx_max_mbps,
			       avg_tx_mbps AS tx_rate_mbps, max_tx_mbps AS tx_max_mbps,
			       first_rx_bytes, last_rx_bytes, first_tx_bytes, last_tx_bytes
			FROM %s
			WHERE ont_id = ? AND bucket >= ? AND bucket <= ?
			ORDER BY bucket ASC`, tier.table)
	}

	var rows []ONTMetricsRow
	if err := s.db.Raw(query, ontID, startTime, endTime).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to query traffic from %s: %w", tier.table, err)
	}
	return rows, nil
}
