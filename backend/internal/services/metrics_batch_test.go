package services

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/connectivity"
)

func TestStoreMetricsBatchWritesEverySampleAcrossChunkBoundaries(t *testing.T) {
	// A poll cycle hands over one chassis at a time, which is more samples than
	// a single INSERT can carry. A chunk boundary that dropped rows would lose
	// readings without failing.
	db := setupMetricsTestDB(t)
	service := NewMetricsService(db)

	const count = 2500
	samples := make([]MetricSample, 0, count)
	for i := 0; i < count; i++ {
		rx := -20.0
		samples = append(samples, MetricSample{
			ONTID:   uuid.New(),
			Metrics: &connectivity.ONTMetrics{RxPower: &rx},
		})
	}

	require.NoError(t, service.StoreMetricsBatch(samples))

	var stored int64
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM ont_metrics").Scan(&stored).Error)
	require.Equal(t, int64(count), stored)
}

func TestStoreMetricsBatchKeepsEachSampleWithItsOwnONT(t *testing.T) {
	// Flattening samples into one statement is where a row could pick up its
	// neighbour's values. Counting rows alone would not notice.
	db := setupMetricsTestDB(t)
	service := NewMetricsService(db)

	first, second := uuid.New(), uuid.New()
	firstRx, secondRx := -18.5, -27.25

	require.NoError(t, service.StoreMetricsBatch([]MetricSample{
		{ONTID: first, Metrics: &connectivity.ONTMetrics{RxPower: &firstRx}},
		{ONTID: second, Metrics: &connectivity.ONTMetrics{RxPower: &secondRx}},
	}))

	var got float64
	require.NoError(t, db.Raw("SELECT rx_power FROM ont_metrics WHERE ont_id = ?", second).Scan(&got).Error)
	require.Equal(t, secondRx, got)

	require.NoError(t, db.Raw("SELECT rx_power FROM ont_metrics WHERE ont_id = ?", first).Scan(&got).Error)
	require.Equal(t, firstRx, got)
}

func TestStoreMetricsBatchCarriesTrafficCountersWhenPresent(t *testing.T) {
	db := setupMetricsTestDB(t)
	service := NewMetricsService(db)

	ontID := uuid.New()
	require.NoError(t, service.StoreMetricsBatch([]MetricSample{{
		ONTID:   ontID,
		Metrics: &connectivity.ONTMetrics{},
		Rates: &connectivity.ONUTrafficRates{
			RxOctets: 4096, TxOctets: 8192, RxOctetBps: 125000,
		},
	}}))

	var rxBytes uint64
	require.NoError(t, db.Raw("SELECT rx_bytes FROM ont_metrics WHERE ont_id = ?", ontID).Scan(&rxBytes).Error)
	require.Equal(t, uint64(4096), rxBytes)

	var rxRate float64
	require.NoError(t, db.Raw("SELECT rx_rate_mbps FROM ont_metrics WHERE ont_id = ?", ontID).Scan(&rxRate).Error)
	require.Equal(t, 1.0, rxRate, "125000 bytes per second is one megabit")
}

func TestStoreMetricsBatchAcceptsASampleWithNoReading(t *testing.T) {
	// An ONT the walk returned nothing for still gets a row, which is how the
	// gap between polls stays visible in the history.
	db := setupMetricsTestDB(t)
	service := NewMetricsService(db)

	require.NoError(t, service.StoreMetricsBatch([]MetricSample{{ONTID: uuid.New()}}))

	var stored int64
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM ont_metrics").Scan(&stored).Error)
	require.Equal(t, int64(1), stored)
}

func TestStoreMetricsBatchOnNoSamplesTouchesNothing(t *testing.T) {
	db := setupMetricsTestDB(t)

	require.NoError(t, NewMetricsService(db).StoreMetricsBatch(nil))

	var stored int64
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM ont_metrics").Scan(&stored).Error)
	require.Zero(t, stored)
}
