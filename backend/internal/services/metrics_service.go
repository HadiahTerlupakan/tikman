package services

import (
	"fmt"
	"strings"
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

func (s *MetricsService) GetDB() *gorm.DB {
	return s.db
}

// StoreMetrics saves ONT metrics to ont_metrics hypertable. Rates come from
// the OLT's live octet-rate gauges; nil means no gauge reading was available.
// MetricSample is one ONT's reading from a poll cycle.
type MetricSample struct {
	ONTID   uuid.UUID
	Metrics *connectivity.ONTMetrics
	Rates   *connectivity.ONUTrafficRates
}

// metricsInsertChunk is how many samples go into one INSERT. Postgres caps a
// statement at 65535 parameters, which across these thirteen columns allows
// about five thousand; a thousand leaves room for a column to be added without
// silently crossing it.
const metricsInsertChunk = 1000

// StoreMetrics writes one ONT's reading.
//
// It is the single-sample form of StoreMetricsBatch, kept so the callers that
// have one reading in hand do not have to build a slice, and so there is only
// one statement that writes this table.
func (s *MetricsService) StoreMetrics(ontID uuid.UUID, metrics *connectivity.ONTMetrics, rates *connectivity.ONUTrafficRates) error {
	return s.StoreMetricsBatch([]MetricSample{{ONTID: ontID, Metrics: metrics, Rates: rates}})
}

// StoreMetricsBatch writes a poll cycle's readings.
//
// One INSERT per ONT was the shape of the old write path, along with a log line
// each. At a thousand subscribers that is a thousand round trips to the
// database every cycle, and the cost grows with every ONT added.
//
// rx_errors and tx_errors are left out: no confirmed counter column for them
// was found on the C300, and the OIDs that used to fill them addressed the
// wrong table.
func (s *MetricsService) StoreMetricsBatch(samples []MetricSample) error {
	if len(samples) == 0 {
		return nil
	}

	for start := 0; start < len(samples); start += metricsInsertChunk {
		end := start + metricsInsertChunk
		if end > len(samples) {
			end = len(samples)
		}
		if err := s.insertChunk(samples[start:end]); err != nil {
			return err
		}
	}
	return nil
}

func (s *MetricsService) insertChunk(samples []MetricSample) error {
	const columns = 13

	placeholders := make([]string, 0, len(samples))
	args := make([]interface{}, 0, len(samples)*columns)
	now := time.Now()

	for _, sample := range samples {
		placeholders = append(placeholders, "(?,?,?,?,?,?,?,?,?,?,?,?,?)")
		args = append(args, sampleArgs(now, sample)...)
	}

	query := `INSERT INTO ont_metrics (time, ont_id, rx_power, tx_power, temperature, voltage, distance, rx_bytes, tx_bytes, rx_packets, tx_packets, rx_rate_mbps, tx_rate_mbps) VALUES ` +
		strings.Join(placeholders, ",")

	return s.db.Exec(query, args...).Error
}

// sampleArgs flattens one reading into the column order of the insert. Traffic
// counters are the OLT's lifetime Counter64 totals, so usage over a period is
// the difference between two rows.
func sampleArgs(at time.Time, sample MetricSample) []interface{} {
	metrics := sample.Metrics
	if metrics == nil {
		metrics = &connectivity.ONTMetrics{}
	}

	var rxRateMbps, txRateMbps *float64
	var rxBytes, txBytes, rxPackets, txPackets *uint64
	if sample.Rates != nil {
		rx := float64(sample.Rates.RxOctetBps) * 8 / 1000000
		tx := float64(sample.Rates.TxOctetBps) * 8 / 1000000
		rxRateMbps, txRateMbps = &rx, &tx
		rxBytes, txBytes = &sample.Rates.RxOctets, &sample.Rates.TxOctets
		rxPackets, txPackets = &sample.Rates.RxPackets, &sample.Rates.TxPackets
	}

	return []interface{}{
		at, sample.ONTID,
		metrics.RxPower, metrics.TxPower, metrics.Temperature, metrics.Voltage, metrics.Distance,
		rxBytes, txBytes, rxPackets, txPackets, rxRateMbps, txRateMbps,
	}
}

func formatPowerValue(val *float64) string {
	if val == nil {
		return "null"
	}
	return fmt.Sprintf("%.2f dBm", *val)
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
	// Both ends of the bucket's cumulative counters, so data used over a window
	// can be measured across buckets without reading every sample.
	FirstRxBytes *uint64
	LastRxBytes  *uint64
	FirstTxBytes *uint64
	LastTxBytes  *uint64
}
