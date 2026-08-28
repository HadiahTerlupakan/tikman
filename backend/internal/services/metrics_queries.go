package services

import (
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
)

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

	driver, err := connectivity.DriverFor(olt.Model)
	if err != nil {
		return nil, err
	}

	snmpMetrics, err := driver.QueryONTMetrics(
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
	}

	// Live rate gauges are separate tables; failure here only means rates stay
	// unset, the rest of the metrics are still returned. A model with no known
	// rate OIDs reports ErrUnsupported and lands in the same branch.
	// The lifetime counters come from the same table as the gauges, so they are
	// read here rather than with the optical metrics, whose index space does not
	// address them.
	if rates, err := driver.QueryTrafficRates(olt.IPAddress, olt.SNMPCommunity, olt.SNMPPort, slot, port, ontIDInt); err == nil {
		rx := float64(rates.RxOctetBps) * 8 / 1000000
		tx := float64(rates.TxOctetBps) * 8 / 1000000
		row.RxRateMbps, row.TxRateMbps = &rx, &tx
		row.RxBytes, row.TxBytes = rates.RxOctets, rates.TxOctets
		row.RxPackets, row.TxPackets = rates.RxPackets, rates.TxPackets
	} else {
		log.Printf("[Realtime] Rate gauges unavailable for ONT %s: %v", ontID, err)
	}

	return row, nil
}
