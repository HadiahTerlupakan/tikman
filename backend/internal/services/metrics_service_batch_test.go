package services

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestGetLatestMetricsBatch(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	err = db.Exec(`
		CREATE TABLE ont_metrics (
			time TIMESTAMP NOT NULL,
			ont_id TEXT NOT NULL,
			rx_power REAL,
			tx_power REAL,
			temperature REAL,
			voltage REAL,
			tx_bias_current REAL,
			distance INTEGER,
			rx_bytes BIGINT,
			tx_bytes BIGINT,
			rx_packets BIGINT,
			tx_packets BIGINT,
			rx_errors BIGINT,
			tx_errors BIGINT
		)
	`).Error
	assert.NoError(t, err)

	metricsService := NewMetricsService(db)

	ontID1 := uuid.New()
	ontID2 := uuid.New()
	ontID3 := uuid.New()

	rxPower1 := -20.5
	txPower1 := 2.3
	rxPower2 := -18.0
	txPower2 := 2.5

	err = metricsService.StoreMetrics(ontID1, &connectivity.ONTMetrics{
		RxPower:     &rxPower1,
		TxPower:     &txPower1,
		Temperature: 45.0,
		Voltage:     3.3,
		Distance:    1500,
		RxBytes:     1000000,
		TxBytes:     500000,
		RxPackets:   10000,
		TxPackets:   5000,
		RxErrors:    10,
		TxErrors:    5,
	})
	assert.NoError(t, err)

	time.Sleep(10 * time.Millisecond)

	err = metricsService.StoreMetrics(ontID2, &connectivity.ONTMetrics{
		RxPower:     &rxPower2,
		TxPower:     &txPower2,
		Temperature: 42.0,
		Voltage:     3.2,
		Distance:    2000,
		RxBytes:     2000000,
		TxBytes:     1000000,
		RxPackets:   20000,
		TxPackets:   10000,
		RxErrors:    20,
		TxErrors:    10,
	})
	assert.NoError(t, err)

	t.Run("fetch multiple ONTs", func(t *testing.T) {
		metricsMap, err := metricsService.GetLatestMetricsBatch([]uuid.UUID{ontID1, ontID2})
		assert.NoError(t, err)
		assert.Len(t, metricsMap, 2)

		assert.NotNil(t, metricsMap[ontID1])
		assert.Equal(t, -20.5, *metricsMap[ontID1].RxPower)
		assert.Equal(t, 1500, metricsMap[ontID1].Distance)

		assert.NotNil(t, metricsMap[ontID2])
		assert.Equal(t, -18.0, *metricsMap[ontID2].RxPower)
		assert.Equal(t, 2000, metricsMap[ontID2].Distance)
	})

	t.Run("empty slice returns empty map", func(t *testing.T) {
		metricsMap, err := metricsService.GetLatestMetricsBatch([]uuid.UUID{})
		assert.NoError(t, err)
		assert.Empty(t, metricsMap)
	})

	t.Run("ONT without metrics not in map", func(t *testing.T) {
		metricsMap, err := metricsService.GetLatestMetricsBatch([]uuid.UUID{ontID1, ontID3})
		assert.NoError(t, err)
		assert.Len(t, metricsMap, 1)
		assert.NotNil(t, metricsMap[ontID1])
		assert.Nil(t, metricsMap[ontID3])
	})

	t.Run("latest metric returned when multiple exist", func(t *testing.T) {
		newerRxPower := -19.0
		err = metricsService.StoreMetrics(ontID1, &connectivity.ONTMetrics{
			RxPower:  &newerRxPower,
			TxPower:  &txPower1,
			Distance: 1600,
		})
		assert.NoError(t, err)

		metricsMap, err := metricsService.GetLatestMetricsBatch([]uuid.UUID{ontID1})
		assert.NoError(t, err)
		assert.Len(t, metricsMap, 1)
		assert.Equal(t, -19.0, *metricsMap[ontID1].RxPower)
		assert.Equal(t, 1600, metricsMap[ontID1].Distance)
	})
}

func TestONTListPerformance(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	err = db.AutoMigrate(&models.ONT{}, &models.OLT{}, &models.Site{})
	assert.NoError(t, err)

	err = db.Exec(`
		CREATE TABLE ont_metrics (
			time TIMESTAMP NOT NULL,
			ont_id TEXT NOT NULL,
			rx_power REAL,
			tx_power REAL,
			temperature REAL,
			voltage REAL,
			tx_bias_current REAL,
			distance INTEGER,
			rx_bytes BIGINT,
			tx_bytes BIGINT,
			rx_packets BIGINT,
			tx_packets BIGINT,
			rx_errors BIGINT,
			tx_errors BIGINT
		)
	`).Error
	assert.NoError(t, err)

	siteID := uuid.New()
	db.Create(&models.Site{ID: siteID, Name: "Test Site"})

	oltID := uuid.New()
	db.Create(&models.OLT{ID: oltID, SiteID: siteID, Name: "Test OLT"})

	metricsService := NewMetricsService(db)
	ontIDs := make([]uuid.UUID, 50)

	for i := 0; i < 50; i++ {
		ontID := uuid.New()
		ontIDs[i] = ontID
		db.Create(&models.ONT{
			ID:           ontID,
			OLTID:        oltID,
			PortID:       i,
			ONTID:        i,
			SerialNumber: uuid.New().String(),
			Status:       models.ONTStatusOnline,
		})

		rxPower := -20.0 + float64(i)*0.1
		txPower := 2.0 + float64(i)*0.01
		_ = metricsService.StoreMetrics(ontID, &connectivity.ONTMetrics{
			RxPower:  &rxPower,
			TxPower:  &txPower,
			Distance: 1000 + i*10,
		})
	}

	start := time.Now()
	metricsMap, err := metricsService.GetLatestMetricsBatch(ontIDs)
	batchDuration := time.Since(start)

	assert.NoError(t, err)
	assert.Len(t, metricsMap, 50)

	start = time.Now()
	for _, ontID := range ontIDs {
		_, _ = metricsService.GetLatestMetrics(ontID)
	}
	loopDuration := time.Since(start)

	t.Logf("Batch fetch (50 ONTs): %v", batchDuration)
	t.Logf("Loop fetch (50 ONTs): %v", loopDuration)
	t.Logf("Speedup: %.2fx", float64(loopDuration)/float64(batchDuration))

	assert.Less(t, batchDuration, loopDuration, "Batch should be faster than loop")
}
