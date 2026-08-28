package services

import (
	"fmt"
	"log"
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
func (s *MetricsService) StoreMetrics(ontID uuid.UUID, metrics *connectivity.ONTMetrics, rates *connectivity.ONUTrafficRates) error {
	// rx_errors and tx_errors are left out: no confirmed counter column for them
	// was found on the C300, and the OIDs that used to fill them addressed the
	// wrong table.
	query := `
		INSERT INTO ont_metrics (time, ont_id, rx_power, tx_power, temperature, voltage, distance, rx_bytes, tx_bytes, rx_packets, tx_packets, rx_rate_mbps, tx_rate_mbps)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`

	var rxRateMbps, txRateMbps *float64
	// The byte and packet columns hold the OLT's lifetime Counter64 totals, so
	// usage over a period is the difference between two rows. They come from the
	// rate walk, which reads the ONU-ID table these counters live in.
	var rxBytes, txBytes, rxPackets, txPackets *uint64
	if rates != nil {
		rx := float64(rates.RxOctetBps) * 8 / 1000000
		tx := float64(rates.TxOctetBps) * 8 / 1000000
		rxRateMbps, txRateMbps = &rx, &tx
		rxBytes, txBytes = &rates.RxOctets, &rates.TxOctets
		rxPackets, txPackets = &rates.RxPackets, &rates.TxPackets
	}

	err := s.db.Exec(query,
		time.Now(),
		ontID,
		metrics.RxPower,
		metrics.TxPower,
		metrics.Temperature,
		metrics.Voltage,
		metrics.Distance,
		rxBytes,
		txBytes,
		rxPackets,
		txPackets,
		rxRateMbps,
		txRateMbps,
	).Error

	if err == nil {
		log.Printf("[Metrics] ✅ Stored for %s: RX=%v TX=%v Dist=%dm Rates(up=%v down=%v Mbps)",
			ontID.String(),
			formatPowerValue(metrics.RxPower),
			formatPowerValue(metrics.TxPower),
			metrics.Distance,
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
