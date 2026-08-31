package services

import (
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// setupMetricsPostgres connects to the Postgres these queries are written for.
//
// The batch lookup uses a lateral join, which SQLite cannot parse at all, so the
// behaviour cannot be exercised anywhere else. A missing DSN fails rather than
// skips under CI, for the same reason the queue's tests do: a test that quietly
// never runs is worse than no test.
func setupMetricsPostgres(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		if os.Getenv("CI") != "" {
			t.Fatal("TEST_POSTGRES_DSN is unset under CI; the metrics queries are then never tested")
		}
		t.Skip("set TEST_POSTGRES_DSN to run the metrics queries against Postgres")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)

	// ont_metrics is built by the SQL migrations rather than AutoMigrate, and
	// this database has neither. Each test owns the table: a row left by an
	// earlier one would decide which sample counts as latest.
	require.NoError(t, db.Exec(`DROP TABLE IF EXISTS ont_metrics`).Error)
	require.NoError(t, db.Exec(`
		CREATE TABLE ont_metrics (
			time TIMESTAMPTZ NOT NULL,
			ont_id UUID NOT NULL,
			rx_power DOUBLE PRECISION,
			tx_power DOUBLE PRECISION,
			temperature DOUBLE PRECISION,
			voltage DOUBLE PRECISION,
			tx_bias_current DOUBLE PRECISION,
			distance INTEGER,
			rx_bytes BIGINT,
			tx_bytes BIGINT,
			rx_packets BIGINT,
			tx_packets BIGINT,
			rx_errors BIGINT,
			tx_errors BIGINT,
			rx_rate_mbps DOUBLE PRECISION,
			tx_rate_mbps DOUBLE PRECISION
		)`).Error)
	require.NoError(t, db.Exec(
		`CREATE INDEX idx_ont_metrics_ont_time ON ont_metrics (ont_id, time DESC)`).Error)

	t.Cleanup(func() { _ = db.Exec(`DROP TABLE IF EXISTS ont_metrics`).Error })
	return db
}

func storeReading(t *testing.T, service *MetricsService, ontID uuid.UUID, rxPower float64, distance int) {
	t.Helper()
	require.NoError(t, service.StoreMetrics(ontID, &connectivity.ONTMetrics{
		RxPower:     &rxPower,
		Temperature: 45.0,
		Voltage:     3.3,
		Distance:    distance,
		RxBytes:     1000000,
		TxBytes:     500000,
	}, nil))
}

func TestGetLatestMetricsBatchReturnsOneReadingPerONT(t *testing.T) {
	db := setupMetricsPostgres(t)
	service := NewMetricsService(db)

	first, second := uuid.New(), uuid.New()
	storeReading(t, service, first, -20.5, 1500)
	storeReading(t, service, second, -18.0, 2000)

	latest, err := service.GetLatestMetricsBatch([]uuid.UUID{first, second})
	require.NoError(t, err)
	require.Len(t, latest, 2)

	assert.Equal(t, -20.5, *latest[first].RxPower)
	assert.Equal(t, 1500, latest[first].Distance)
	assert.Equal(t, -18.0, *latest[second].RxPower)
	assert.Equal(t, 2000, latest[second].Distance)
}

func TestGetLatestMetricsBatchTakesTheNewestOfManyReadings(t *testing.T) {
	db := setupMetricsPostgres(t)
	service := NewMetricsService(db)

	ontID := uuid.New()
	storeReading(t, service, ontID, -20.5, 1500)
	time.Sleep(10 * time.Millisecond)
	storeReading(t, service, ontID, -19.0, 1600)

	latest, err := service.GetLatestMetricsBatch([]uuid.UUID{ontID})
	require.NoError(t, err)
	require.Len(t, latest, 1)

	// One row per ONT however deep the history: this is what replaced ranking
	// every sample the ONT ever recorded in order to keep the newest.
	assert.Equal(t, -19.0, *latest[ontID].RxPower)
	assert.Equal(t, 1600, latest[ontID].Distance)
}

func TestGetLatestMetricsBatchLeavesOutAnONTWithNoReadings(t *testing.T) {
	db := setupMetricsPostgres(t)
	service := NewMetricsService(db)

	known, unknown := uuid.New(), uuid.New()
	storeReading(t, service, known, -20.5, 1500)

	latest, err := service.GetLatestMetricsBatch([]uuid.UUID{known, unknown})
	require.NoError(t, err)

	// Absent rather than present-and-empty, which is what the caller renders as
	// "no metrics yet" for an ONT the poller has not reached.
	assert.Len(t, latest, 1)
	assert.NotNil(t, latest[known])
	assert.Nil(t, latest[unknown])
}

func TestGetLatestMetricsBatchAsksNothingForAnEmptyList(t *testing.T) {
	db := setupMetricsPostgres(t)

	latest, err := NewMetricsService(db).GetLatestMetricsBatch(nil)
	require.NoError(t, err)
	assert.Empty(t, latest)
}

func TestGetMetricsHistoryReadsRawSamplesInsideRetention(t *testing.T) {
	db := setupMetricsPostgres(t)
	service := NewMetricsService(db)

	ontID := uuid.New()
	storeReading(t, service, ontID, -20.5, 1500)

	// A recent range reads ont_metrics, the only source that carries full
	// resolution and the minutes since the last rollup refresh. The rollup
	// sources are TimescaleDB views and cannot be exercised here.
	history, err := service.GetMetricsHistory(ontID,
		time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	require.NoError(t, err)
	require.Len(t, history, 1)
	assert.Equal(t, -20.5, *history[0].RxPower)
}
