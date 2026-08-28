package services

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// AggregateTrafficPoint is the traffic of every ONU under one PON or OLT at a
// point in time. Rates are summed because that is the load the PON carries;
// peaks are summed too, so the figure is the worst case the port had to serve
// rather than the peak of an average.
type AggregateTrafficPoint struct {
	Time       time.Time `json:"time"`
	RxMbps     float64   `json:"rx_mbps"`
	TxMbps     float64   `json:"tx_mbps"`
	RxMaxMbps  float64   `json:"rx_max_mbps"`
	TxMaxMbps  float64   `json:"tx_max_mbps"`
	OnlineOnts int       `json:"online_onts"`
}

// GetOLTAggregateTraffic returns the summed traffic under an OLT, or under one
// of its PON ports when port is not nil. It reads the same tiered stores a
// per-ONT graph does, so a 30-day view is answerable.
func (s *MetricsService) GetOLTAggregateTraffic(oltID uuid.UUID, slot, port *int, period string) ([]AggregateTrafficPoint, error) {
	window := trafficPeriodWindow(period)
	tier := trafficTierFor(window)
	endTime := time.Now().UTC()
	startTime := endTime.Add(-window)

	timeColumn, rxAvg, rxMax, txAvg, txMax := "time", "rx_rate_mbps", "rx_rate_mbps", "tx_rate_mbps", "tx_rate_mbps"
	if tier.table != rawTrafficTable {
		timeColumn, rxAvg, rxMax, txAvg, txMax = "bucket", "avg_rx_mbps", "max_rx_mbps", "avg_tx_mbps", "max_tx_mbps"
	}

	// The ONT rows carry the position, so the join is what restricts the sum to
	// one card and PON port.
	filter := "o.olt_id = ?"
	args := []interface{}{startTime, endTime, oltID}
	if slot != nil {
		filter += " AND o.slot = ?"
		args = append(args, *slot)
	}
	if port != nil {
		filter += " AND o.port_id = ?"
		args = append(args, *port)
	}

	query := fmt.Sprintf(`
		SELECT time_bucket(?::interval, m.%[1]s) AS time,
		       COALESCE(SUM(m.%[2]s), 0) AS rx_mbps,
		       COALESCE(SUM(m.%[4]s), 0) AS tx_mbps,
		       COALESCE(SUM(m.%[3]s), 0) AS rx_max_mbps,
		       COALESCE(SUM(m.%[5]s), 0) AS tx_max_mbps,
		       COUNT(DISTINCT m.ont_id) AS online_onts
		FROM %[6]s m
		JOIN onts o ON o.id = m.ont_id
		WHERE m.%[1]s >= ? AND m.%[1]s <= ? AND %[7]s
		GROUP BY 1
		ORDER BY 1 ASC`,
		timeColumn, rxAvg, rxMax, txAvg, txMax, tier.table, filter)

	var points []AggregateTrafficPoint
	callArgs := append([]interface{}{fmt.Sprintf("%d seconds", int(tier.step.Seconds()))}, args...)
	if err := s.db.Raw(query, callArgs...).Scan(&points).Error; err != nil {
		return nil, fmt.Errorf("failed to aggregate traffic: %w", err)
	}

	return points, nil
}
