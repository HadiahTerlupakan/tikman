package services

import (
	"fmt"
	"log"
	"strings"
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

// GetMetricsHistory retrieves metrics history for an ONT within a time range,
// reading whichever source still holds that range.
//
// Raw samples only survive a week. Asking them for a month returned an empty
// chart while the five-minute and hourly rollups — kept for thirty days and a
// year — sat unread beside them.
//
// A rollup is materialized on its own schedule, so a range reaching up to now
// can lag by up to one refresh at its right-hand edge. That only applies to
// ranges old enough to need a rollup at all, where a bucket's delay against a
// month or a year of history is not what anyone is reading the chart for.
func (s *MetricsService) GetMetricsHistory(ontID uuid.UUID, startTime, endTime time.Time) ([]ONTMetricsRow, error) {
	var metrics []ONTMetricsRow

	source := sourceForRange(startTime, time.Now())
	query := rawHistoryQuery
	if source.aggregated() {
		// Rollups carry averages over a bucket, and neither voltage nor
		// distance is aggregated: those columns come back empty rather than
		// wrong.
		query = fmt.Sprintf(aggregateHistoryQuery, source)
	}

	err := s.db.Raw(query, ontID, startTime, endTime).Scan(&metrics).Error
	return metrics, err
}

const rawHistoryQuery = `
	SELECT time, ont_id, rx_power, tx_power, temperature, voltage, distance, rx_bytes, tx_bytes
	FROM ont_metrics
	WHERE ont_id = $1 AND time >= $2 AND time <= $3
	ORDER BY time DESC`

const aggregateHistoryQuery = `
	SELECT bucket AS time, ont_id,
	       avg_rx_power AS rx_power, avg_tx_power AS tx_power,
	       avg_temperature AS temperature,
	       0 AS voltage, 0 AS distance,
	       last_rx_bytes AS rx_bytes, last_tx_bytes AS tx_bytes
	FROM %s
	WHERE ont_id = $1 AND bucket >= $2 AND bucket <= $3
	ORDER BY bucket DESC`

// GetLatestMetricsBatch retrieves the latest metrics for multiple ONTs in a
// single query.
//
// A lateral join, because the ROW_NUMBER version this replaces read every row
// each ONT had ever recorded in order to keep one of them: measured against
// production, nine ONTs cost 3,929 rows read to return nine, and the waste grew
// with the retention window rather than with the page. Compressing the older
// chunks made that scan more expensive still.
//
// LIMIT 1 against the (ont_id, time) index reads one row per ONT however long
// the history is: nine ONTs now cost 37 buffer hits where the old query cost
// 683. An ONT with no metrics yields no row, exactly as before, so a
// chassis polled hours ago still shows its last reading rather than nothing.
func (s *MetricsService) GetLatestMetricsBatch(ontIDs []uuid.UUID) (map[uuid.UUID]*ONTMetricsRow, error) {
	if len(ontIDs) == 0 {
		return make(map[uuid.UUID]*ONTMetricsRow), nil
	}

	var metrics []ONTMetricsRow

	err := s.db.Raw(`
		SELECT latest.*
		FROM unnest($1::uuid[]) AS wanted(ont_id)
		CROSS JOIN LATERAL (
			SELECT time, ont_id, rx_power, tx_power, temperature, voltage, tx_bias_current,
			       distance, rx_bytes, tx_bytes, rx_packets, tx_packets, rx_errors, tx_errors
			FROM ont_metrics
			WHERE ont_id = wanted.ont_id
			ORDER BY time DESC
			LIMIT 1
		) latest
	`, uuidArray(ontIDs)).Scan(&metrics).Error

	if err != nil {
		return nil, err
	}

	metricsMap := make(map[uuid.UUID]*ONTMetricsRow, len(metrics))
	for i := range metrics {
		metricsMap[metrics[i].ONTID] = &metrics[i]
	}

	return metricsMap, nil
}

// bitsPerMegabit converts the octet-per-second gauges the OLT reports into the
// Mbps the graphs are drawn in.
const bitsPerMegabit = 1000000

func (s *MetricsService) GetRealtimeMetrics(ontID uuid.UUID) (*ONTMetricsRow, error) {
	olt, loc, driver, err := s.locateONT(ontID)
	if err != nil {
		return nil, err
	}

	snmpMetrics, err := driver.QueryONTMetrics(olt.IPAddress, olt.SNMPCommunity, olt.SNMPPort,
		loc.Slot, loc.Port, loc.ONTID)
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
	addTrafficRates(row, driver, olt, loc)

	return row, nil
}

// locateONT resolves an ONT to the chassis, position and driver a live read
// needs. An ONT the worker has not placed on a card yet cannot be read: the
// optical tables are addressed by position, not by serial.
func (s *MetricsService) locateONT(ontID uuid.UUID) (*models.OLT, connectivity.ONTLocation, connectivity.Driver, error) {
	var loc connectivity.ONTLocation

	var ont models.ONT
	if err := s.db.First(&ont, "id = ?", ontID).Error; err != nil {
		return nil, loc, nil, fmt.Errorf("ONT not found: %w", err)
	}
	var olt models.OLT
	if err := s.db.First(&olt, "id = ?", ont.OLTID).Error; err != nil {
		return nil, loc, nil, fmt.Errorf("OLT not found: %w", err)
	}
	if ont.Slot == nil {
		return nil, loc, nil, fmt.Errorf("ONT slot not yet discovered by worker")
	}

	driver, err := connectivity.DriverFor(olt.Model)
	if err != nil {
		return nil, loc, nil, err
	}

	loc = connectivity.ONTLocation{Slot: *ont.Slot, Port: int(ont.PortID), ONTID: int(ont.ONTID)}
	return &olt, loc, driver, nil
}

// addTrafficRates fills in the live rate gauges, leaving them unset when the
// chassis cannot answer: the optical metrics are still worth returning. A
// model with no known rate OIDs reports ErrUnsupported and lands here too.
//
// The lifetime counters come from the same table as the gauges, so they are
// read here rather than with the optical metrics, whose index space does not
// address them.
func addTrafficRates(row *ONTMetricsRow, driver connectivity.Driver, olt *models.OLT, loc connectivity.ONTLocation) {
	rates, err := driver.QueryTrafficRates(olt.IPAddress, olt.SNMPCommunity, olt.SNMPPort,
		loc.Slot, loc.Port, loc.ONTID)
	if err != nil {
		log.Printf("[Realtime] Rate gauges unavailable for ONT %s: %v", row.ONTID, err)
		return
	}

	rx := float64(rates.RxOctetBps) * 8 / bitsPerMegabit
	tx := float64(rates.TxOctetBps) * 8 / bitsPerMegabit
	row.RxRateMbps, row.TxRateMbps = &rx, &tx
	row.RxBytes, row.TxBytes = rates.RxOctets, rates.TxOctets
	row.RxPackets, row.TxPackets = rates.RxPackets, rates.TxPackets
}

// uuidArray renders identifiers as the array literal Postgres parses for
// uuid[].
//
// Written out rather than taken from a driver helper because that would mean a
// new dependency for one call site. A uuid's own String is hex and dashes, so
// nothing here can carry anything but an identifier into the statement.
func uuidArray(ids []uuid.UUID) string {
	rendered := make([]string, len(ids))
	for i, id := range ids {
		rendered[i] = id.String()
	}
	return "{" + strings.Join(rendered, ",") + "}"
}
