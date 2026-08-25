package services

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupMetricsTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = models.AutoMigrate(db)
	require.NoError(t, err)

	err = db.Exec(`
		CREATE TABLE IF NOT EXISTS ont_metrics (
			time DATETIME NOT NULL,
			ont_id TEXT NOT NULL,
			rx_power REAL,
			tx_power REAL,
			temperature REAL,
			voltage REAL,
			tx_bias_current REAL,
			distance INTEGER,
			rx_bytes INTEGER,
			tx_bytes INTEGER,
			rx_packets INTEGER,
			tx_packets INTEGER,
			rx_errors INTEGER,
			tx_errors INTEGER,
			rx_rate_mbps REAL,
			tx_rate_mbps REAL
		)
	`).Error
	require.NoError(t, err)

	return db
}

func TestMetricsService_GetPollingStats(t *testing.T) {
	db := setupMetricsTestDB(t)
	service := NewMetricsService(db)

	siteID := uuid.New()
	oltID := uuid.New()
	olt := &models.OLT{
		ID:                oltID,
		SiteID:            siteID,
		Name:              "test-olt",
		IPAddress:         "192.168.1.1",
		PreferredProtocol: "ssh",
		Username:          "admin",
		Password:          "pass",
	}
	require.NoError(t, db.Create(olt).Error)

	ont1ID := uuid.New()
	ont2ID := uuid.New()
	ont3ID := uuid.New()

	db.Create(&models.ONT{ID: ont1ID, OLTID: oltID, SerialNumber: "ont1"})
	db.Create(&models.ONT{ID: ont2ID, OLTID: oltID, SerialNumber: "ont2"})
	db.Create(&models.ONT{ID: ont3ID, OLTID: oltID, SerialNumber: "ont3"})

	now := time.Now()
	db.Exec(`INSERT INTO ont_metrics (time, ont_id, rx_power, tx_power, temperature, voltage, distance)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		now, ont1ID.String(), -20.5, -5.0, 25.0, 3.3, 100)
	db.Exec(`INSERT INTO ont_metrics (time, ont_id, rx_power, tx_power, temperature, voltage, distance)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		now, ont2ID.String(), -21.0, -4.5, 24.8, 3.3, 95)

	oldTime := now.Add(-15 * time.Minute)
	db.Exec(`INSERT INTO ont_metrics (time, ont_id, rx_power, tx_power, temperature, voltage, distance)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		oldTime, ont1ID.String(), -20.0, -5.5, 25.1, 3.3, 105)
	db.Exec(`INSERT INTO ont_metrics (time, ont_id, rx_power, tx_power, temperature, voltage, distance)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		now, uuid.New().String(), -19.0, -4.0, 25.0, 3.3, 80)

	stats := service.GetPollingStats()

	assert.Equal(t, int64(3), stats["total_onts"])
	assert.Equal(t, int64(2), stats["onts_with_metrics"])
	percentage := stats["percentage"].(float64)
	assert.InDelta(t, 66.67, percentage, 0.1)
	assert.NotNil(t, stats["last_poll_time"])
}

func TestMetricsService_GetPollingStats_NoONTs(t *testing.T) {
	db := setupMetricsTestDB(t)
	service := NewMetricsService(db)

	stats := service.GetPollingStats()

	assert.Equal(t, int64(0), stats["total_onts"])
	assert.Equal(t, int64(0), stats["onts_with_metrics"])
	assert.Equal(t, 0.0, stats["percentage"])
}

func TestMetricsService_GetOLTPollingStats(t *testing.T) {
	db := setupMetricsTestDB(t)
	service := NewMetricsService(db)

	siteID := uuid.New()
	olt1ID := uuid.New()
	olt2ID := uuid.New()

	db.Create(&models.OLT{ID: olt1ID, SiteID: siteID, Name: "olt1", IPAddress: "192.168.1.1", PreferredProtocol: "ssh", Username: "admin", Password: "pass"})
	db.Create(&models.OLT{ID: olt2ID, SiteID: siteID, Name: "olt2", IPAddress: "192.168.1.2", PreferredProtocol: "ssh", Username: "admin", Password: "pass"})

	ont1ID := uuid.New()
	ont2ID := uuid.New()
	ont3ID := uuid.New()
	ont4ID := uuid.New()

	db.Create(&models.ONT{ID: ont1ID, OLTID: olt1ID, SerialNumber: "ont1"})
	db.Create(&models.ONT{ID: ont2ID, OLTID: olt1ID, SerialNumber: "ont2"})
	db.Create(&models.ONT{ID: ont3ID, OLTID: olt1ID, SerialNumber: "ont3"})
	db.Create(&models.ONT{ID: ont4ID, OLTID: olt2ID, SerialNumber: "ont4"})

	now := time.Now()
	db.Exec(`INSERT INTO ont_metrics (time, ont_id, rx_power, tx_power, temperature, voltage, distance)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		now, ont1ID.String(), -20.5, -5.0, 25.0, 3.3, 100)
	db.Exec(`INSERT INTO ont_metrics (time, ont_id, rx_power, tx_power, temperature, voltage, distance)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		now, ont2ID.String(), -21.0, -4.5, 24.8, 3.3, 95)

	db.Exec(`INSERT INTO ont_metrics (time, ont_id, rx_power, tx_power, temperature, voltage, distance)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		now, ont4ID.String(), -20.0, -5.0, 25.0, 3.3, 90)

	stats := service.GetOLTPollingStats(olt1ID)

	assert.Equal(t, int64(3), stats["total_onts"])
	assert.Equal(t, int64(2), stats["onts_with_metrics"])
	percentage := stats["percentage"].(float64)
	assert.InDelta(t, 66.67, percentage, 0.1)
	assert.Equal(t, olt1ID.String(), stats["olt_id"])
	assert.NotNil(t, stats["last_poll_time"])
}

func TestMetricsService_GetOLTPollingStats_NoONTs(t *testing.T) {
	db := setupMetricsTestDB(t)
	service := NewMetricsService(db)

	siteID := uuid.New()
	oltID := uuid.New()
	db.Create(&models.OLT{ID: oltID, SiteID: siteID, Name: "olt1", IPAddress: "192.168.1.1", PreferredProtocol: "ssh", Username: "admin", Password: "pass"})

	stats := service.GetOLTPollingStats(oltID)

	assert.Equal(t, int64(0), stats["total_onts"])
	assert.Equal(t, int64(0), stats["onts_with_metrics"])
	assert.Equal(t, 0.0, stats["percentage"])
	assert.Equal(t, oltID.String(), stats["olt_id"])
}

func TestMetricsService_GetLatestMetrics(t *testing.T) {
	db := setupMetricsTestDB(t)
	service := NewMetricsService(db)

	siteID := uuid.New()
	oltID := uuid.New()
	ontID := uuid.New()

	db.Create(&models.OLT{ID: oltID, SiteID: siteID, Name: "olt1", IPAddress: "192.168.1.1", PreferredProtocol: "ssh", Username: "admin", Password: "pass"})
	db.Create(&models.ONT{ID: ontID, OLTID: oltID, SerialNumber: "ont1"})

	time1 := time.Now().Add(-2 * time.Hour)
	time2 := time.Now().Add(-1 * time.Hour)
	time3 := time.Now()

	rxPower1 := -20.5
	txPower1 := -5.0
	rxPower3 := -21.5
	txPower3 := -4.5

	db.Exec(`INSERT INTO ont_metrics (time, ont_id, rx_power, tx_power, temperature, voltage, distance, rx_packets, tx_packets)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		time1, ontID.String(), rxPower1, txPower1, 25.0, 3.3, 100, 1000, 500)
	db.Exec(`INSERT INTO ont_metrics (time, ont_id, rx_power, tx_power, temperature, voltage, distance, rx_packets, tx_packets)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		time2, ontID.String(), -20.8, -4.8, 24.5, 3.3, 102, 1100, 550)
	db.Exec(`INSERT INTO ont_metrics (time, ont_id, rx_power, tx_power, temperature, voltage, distance, rx_packets, tx_packets)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		time3, ontID.String(), rxPower3, txPower3, 25.5, 3.3, 98, 1200, 600)

	metrics, err := service.GetLatestMetrics(ontID)

	require.NoError(t, err)
	require.NotNil(t, metrics)
	assert.Equal(t, ontID, metrics.ONTID)
	assert.Equal(t, rxPower3, *metrics.RxPower)
	assert.Equal(t, txPower3, *metrics.TxPower)
	assert.Equal(t, 25.5, metrics.Temperature)
	assert.Equal(t, 3.3, metrics.Voltage)
	assert.Equal(t, 98, metrics.Distance)
	assert.Equal(t, uint64(1200), metrics.RxPackets)
	assert.Equal(t, uint64(600), metrics.TxPackets)
}

func TestMetricsService_GetLatestMetrics_NotFound(t *testing.T) {
	db := setupMetricsTestDB(t)
	service := NewMetricsService(db)

	metrics, err := service.GetLatestMetrics(uuid.New())

	require.Error(t, err)
	assert.Nil(t, metrics)
}

func TestMetricsService_GetLatestMetrics_WithNullPowers(t *testing.T) {
	db := setupMetricsTestDB(t)
	service := NewMetricsService(db)

	siteID := uuid.New()
	oltID := uuid.New()
	ontID := uuid.New()

	db.Create(&models.OLT{ID: oltID, SiteID: siteID, Name: "olt1", IPAddress: "192.168.1.1", PreferredProtocol: "ssh", Username: "admin", Password: "pass"})
	db.Create(&models.ONT{ID: ontID, OLTID: oltID, SerialNumber: "ont1"})

	db.Exec(`INSERT INTO ont_metrics (time, ont_id, rx_power, tx_power, temperature, voltage, distance)
		VALUES (?, ?, NULL, NULL, ?, ?, ?)`,
		time.Now(), ontID.String(), 25.0, 3.3, 100)

	metrics, err := service.GetLatestMetrics(ontID)

	require.NoError(t, err)
	require.NotNil(t, metrics)
	assert.Nil(t, metrics.RxPower)
	assert.Nil(t, metrics.TxPower)
	assert.Equal(t, 25.0, metrics.Temperature)
}

func TestMetricsService_GetMetricsHistory(t *testing.T) {
	db := setupMetricsTestDB(t)
	service := NewMetricsService(db)

	siteID := uuid.New()
	oltID := uuid.New()
	ontID := uuid.New()

	db.Create(&models.OLT{ID: oltID, SiteID: siteID, Name: "olt1", IPAddress: "192.168.1.1", PreferredProtocol: "ssh", Username: "admin", Password: "pass"})
	db.Create(&models.ONT{ID: ontID, OLTID: oltID, SerialNumber: "ont1"})

	baseTime := time.Now()
	for i := 0; i < 5; i++ {
		insertTime := baseTime.Add(time.Duration(-i) * time.Hour)
		rxPower := -20.0 - float64(i)*0.5
		db.Exec(`INSERT INTO ont_metrics (time, ont_id, rx_power, tx_power, temperature, voltage, distance, rx_bytes, tx_bytes)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			insertTime, ontID.String(), rxPower, -5.0, 25.0, 3.3, 100, 1000, 500)
	}

	startTime := baseTime.Add(-6 * time.Hour)
	endTime := baseTime

	metrics, err := service.GetMetricsHistory(ontID, startTime, endTime)

	require.NoError(t, err)
	assert.Len(t, metrics, 5)
	assert.InDelta(t, -20.0, *metrics[0].RxPower, 0.01)
	assert.InDelta(t, -22.0, *metrics[4].RxPower, 0.01)
}

func TestMetricsService_GetMetricsHistory_OutOfRange(t *testing.T) {
	db := setupMetricsTestDB(t)
	service := NewMetricsService(db)

	siteID := uuid.New()
	oltID := uuid.New()
	ontID := uuid.New()

	db.Create(&models.OLT{ID: oltID, SiteID: siteID, Name: "olt1", IPAddress: "192.168.1.1", PreferredProtocol: "ssh", Username: "admin", Password: "pass"})
	db.Create(&models.ONT{ID: ontID, OLTID: oltID, SerialNumber: "ont1"})

	insertTime := time.Now()
	db.Exec(`INSERT INTO ont_metrics (time, ont_id, rx_power, tx_power, temperature, voltage, distance)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		insertTime, ontID.String(), -20.0, -5.0, 25.0, 3.3, 100)

	startTime := time.Now().Add(-2 * time.Hour)
	endTime := time.Now().Add(-1 * time.Hour)

	metrics, err := service.GetMetricsHistory(ontID, startTime, endTime)

	require.NoError(t, err)
	assert.Len(t, metrics, 0)
}

func TestMetricsService_GetLatestMetricsBatch(t *testing.T) {
	db := setupMetricsTestDB(t)
	service := NewMetricsService(db)

	siteID := uuid.New()
	oltID := uuid.New()
	db.Create(&models.OLT{ID: oltID, SiteID: siteID, Name: "olt1", IPAddress: "192.168.1.1", PreferredProtocol: "ssh", Username: "admin", Password: "pass"})

	ont1ID := uuid.New()
	ont2ID := uuid.New()
	ont3ID := uuid.New()

	db.Create(&models.ONT{ID: ont1ID, OLTID: oltID, SerialNumber: "ont1"})
	db.Create(&models.ONT{ID: ont2ID, OLTID: oltID, SerialNumber: "ont2"})
	db.Create(&models.ONT{ID: ont3ID, OLTID: oltID, SerialNumber: "ont3"})

	now := time.Now()
	rxPower1 := -20.5
	rxPower2 := -21.0
	rxPower3 := -19.5

	db.Exec(`INSERT INTO ont_metrics (time, ont_id, rx_power, tx_power, temperature, voltage, distance, rx_packets, tx_packets)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		now.Add(-1*time.Hour), ont1ID.String(), rxPower1, -5.0, 25.0, 3.3, 100, 1000, 500)
	db.Exec(`INSERT INTO ont_metrics (time, ont_id, rx_power, tx_power, temperature, voltage, distance, rx_packets, tx_packets)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		now.Add(-2*time.Hour), ont1ID.String(), -20.0, -5.5, 25.1, 3.3, 105, 900, 450)

	db.Exec(`INSERT INTO ont_metrics (time, ont_id, rx_power, tx_power, temperature, voltage, distance, rx_packets, tx_packets)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		now.Add(-30*time.Minute), ont2ID.String(), rxPower2, -4.5, 24.8, 3.3, 95, 1100, 550)

	db.Exec(`INSERT INTO ont_metrics (time, ont_id, rx_power, tx_power, temperature, voltage, distance, rx_packets, tx_packets)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		now, ont3ID.String(), rxPower3, -5.2, 25.2, 3.3, 102, 1200, 600)

	ontIDs := []uuid.UUID{ont1ID, ont2ID, ont3ID}
	metricsMap, err := service.GetLatestMetricsBatch(ontIDs)

	require.NoError(t, err)
	assert.Len(t, metricsMap, 3)

	m1, ok := metricsMap[ont1ID]
	assert.True(t, ok)
	assert.Equal(t, rxPower1, *m1.RxPower)
	assert.Equal(t, uint64(1000), m1.RxPackets)

	m2, ok := metricsMap[ont2ID]
	assert.True(t, ok)
	assert.Equal(t, rxPower2, *m2.RxPower)
	assert.Equal(t, uint64(1100), m2.RxPackets)

	m3, ok := metricsMap[ont3ID]
	assert.True(t, ok)
	assert.Equal(t, rxPower3, *m3.RxPower)
	assert.Equal(t, uint64(1200), m3.RxPackets)
}

func TestMetricsService_GetLatestMetricsBatch_Empty(t *testing.T) {
	db := setupMetricsTestDB(t)
	service := NewMetricsService(db)

	metricsMap, err := service.GetLatestMetricsBatch([]uuid.UUID{})

	require.NoError(t, err)
	assert.Len(t, metricsMap, 0)
}

func TestMetricsService_GetLatestMetricsBatch_PartialData(t *testing.T) {
	db := setupMetricsTestDB(t)
	service := NewMetricsService(db)

	siteID := uuid.New()
	oltID := uuid.New()
	db.Create(&models.OLT{ID: oltID, SiteID: siteID, Name: "olt1", IPAddress: "192.168.1.1", PreferredProtocol: "ssh", Username: "admin", Password: "pass"})

	ont1ID := uuid.New()
	ont2ID := uuid.New()
	ont3ID := uuid.New()

	db.Create(&models.ONT{ID: ont1ID, OLTID: oltID, SerialNumber: "ont1"})
	db.Create(&models.ONT{ID: ont2ID, OLTID: oltID, SerialNumber: "ont2"})
	db.Create(&models.ONT{ID: ont3ID, OLTID: oltID, SerialNumber: "ont3"})

	now := time.Now()
	rxPower1 := -20.5
	rxPower2 := -21.0

	db.Exec(`INSERT INTO ont_metrics (time, ont_id, rx_power, tx_power, temperature, voltage, distance)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		now, ont1ID.String(), rxPower1, -5.0, 25.0, 3.3, 100)
	db.Exec(`INSERT INTO ont_metrics (time, ont_id, rx_power, tx_power, temperature, voltage, distance)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		now, ont2ID.String(), rxPower2, -4.5, 24.8, 3.3, 95)

	ontIDs := []uuid.UUID{ont1ID, ont2ID, ont3ID}
	metricsMap, err := service.GetLatestMetricsBatch(ontIDs)

	require.NoError(t, err)
	assert.Len(t, metricsMap, 2)
	assert.NotNil(t, metricsMap[ont1ID])
	assert.NotNil(t, metricsMap[ont2ID])
	assert.Nil(t, metricsMap[ont3ID])
}

func TestMetricsService_StoreMetrics_Integration(t *testing.T) {
	db := setupMetricsTestDB(t)
	service := NewMetricsService(db)

	siteID := uuid.New()
	oltID := uuid.New()
	ontID := uuid.New()

	db.Create(&models.OLT{ID: oltID, SiteID: siteID, Name: "olt1", IPAddress: "192.168.1.1", PreferredProtocol: "ssh", Username: "admin", Password: "pass"})
	db.Create(&models.ONT{ID: ontID, OLTID: oltID, SerialNumber: "ont1"})

	rxPower := -20.5
	txPower := -5.0
	temp := 25.0
	volt := 3.3
	dist := 100
	rxBytes := uint64(1000000)
	txBytes := uint64(500000)
	rxPkts := uint64(10000)
	txPkts := uint64(5000)
	rxErrs := uint64(0)
	txErrs := uint64(0)

	metrics := &connectivity.ONTMetrics{
		RxPower:     &rxPower,
		TxPower:     &txPower,
		Temperature: temp,
		Voltage:     volt,
		Distance:    dist,
		RxBytes:     rxBytes,
		TxBytes:     txBytes,
		RxPackets:   rxPkts,
		TxPackets:   txPkts,
		RxErrors:    rxErrs,
		TxErrors:    txErrs,
	}

	err := service.StoreMetrics(ontID, metrics, &connectivity.ONUTrafficRates{
		RxOctetBps: 125000,  // 1 Mbps upload
		TxOctetBps: 2500000, // 20 Mbps download
	})
	require.NoError(t, err)

	retrieved, err := service.GetLatestMetrics(ontID)
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	assert.Equal(t, ontID, retrieved.ONTID)
	assert.Equal(t, rxPower, *retrieved.RxPower)
	assert.Equal(t, txPower, *retrieved.TxPower)
	assert.Equal(t, temp, retrieved.Temperature)
	assert.Equal(t, volt, retrieved.Voltage)
	assert.Equal(t, dist, retrieved.Distance)
	assert.Equal(t, rxBytes, retrieved.RxBytes)
	assert.Equal(t, txBytes, retrieved.TxBytes)
	assert.Equal(t, rxPkts, retrieved.RxPackets)
	assert.Equal(t, txPkts, retrieved.TxPackets)
	assert.Equal(t, rxErrs, retrieved.RxErrors)
	assert.Equal(t, txErrs, retrieved.TxErrors)
	require.NotNil(t, retrieved.RxRateMbps)
	assert.InDelta(t, 1.0, *retrieved.RxRateMbps, 0.0001)
	require.NotNil(t, retrieved.TxRateMbps)
	assert.InDelta(t, 20.0, *retrieved.TxRateMbps, 0.0001)
}

func TestMetricsService_StoreMetrics_WithNullPowers(t *testing.T) {
	db := setupMetricsTestDB(t)
	service := NewMetricsService(db)

	siteID := uuid.New()
	oltID := uuid.New()
	ontID := uuid.New()

	db.Create(&models.OLT{ID: oltID, SiteID: siteID, Name: "olt1", IPAddress: "192.168.1.1", PreferredProtocol: "ssh", Username: "admin", Password: "pass"})
	db.Create(&models.ONT{ID: ontID, OLTID: oltID, SerialNumber: "ont1"})

	temp := 25.0
	volt := 3.3
	dist := 100

	metrics := &connectivity.ONTMetrics{
		RxPower:     nil,
		TxPower:     nil,
		Temperature: temp,
		Voltage:     volt,
		Distance:    dist,
	}

	err := service.StoreMetrics(ontID, metrics, nil)
	require.NoError(t, err)

	retrieved, err := service.GetLatestMetrics(ontID)
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	assert.Nil(t, retrieved.RxPower)
	assert.Nil(t, retrieved.TxPower)
	assert.Nil(t, retrieved.RxRateMbps)
	assert.Nil(t, retrieved.TxRateMbps)
	assert.Equal(t, temp, retrieved.Temperature)
}

func TestMetricsService_GetONTTrafficTimeSeries_FillsDefaultPeriodBuckets(t *testing.T) {
	db := setupMetricsTestDB(t)
	service := NewMetricsService(db)

	siteID := uuid.New()
	oltID := uuid.New()
	ontID := uuid.New()

	require.NoError(t, db.Create(&models.OLT{ID: oltID, SiteID: siteID, Name: "olt1", IPAddress: "192.168.1.1", PreferredProtocol: "ssh", Username: "admin", Password: "pass"}).Error)
	require.NoError(t, db.Create(&models.ONT{ID: ontID, OLTID: oltID, SerialNumber: "ont1"}).Error)

	now := time.Now().UTC()
	pointTime := now.Add(-5 * time.Minute).Truncate(5 * time.Minute).Add(55 * time.Second)
	require.NoError(t, db.Exec(`INSERT INTO ont_metrics (time, ont_id, rx_rate_mbps, tx_rate_mbps)
		VALUES (?, ?, ?, ?)`,
		pointTime, ontID.String(), 1.0, 2.0,
	).Error)

	rows, err := service.GetONTTrafficTimeSeries(ontID, "3h")
	require.NoError(t, err)
	// Empty buckets are skipped, so we only get 1 row (the one with data)
	require.Len(t, rows, 1)
	require.NotNil(t, rows[0].RxRateMbps)
	require.NotNil(t, rows[0].TxRateMbps)
	assert.Equal(t, 1.0, *rows[0].RxRateMbps)
	assert.Equal(t, 2.0, *rows[0].TxRateMbps)
}

func TestMetricsService_GetONTTrafficTimeSeriesCustomRange(t *testing.T) {
	db := setupMetricsTestDB(t)
	service := NewMetricsService(db)

	siteID := uuid.New()
	oltID := uuid.New()
	ontID := uuid.New()

	db.Create(&models.OLT{ID: oltID, SiteID: siteID, Name: "olt1", IPAddress: "192.168.1.1", PreferredProtocol: "ssh", Username: "admin", Password: "pass"})
	db.Create(&models.ONT{ID: ontID, OLTID: oltID, SerialNumber: "ont1"})

	base := time.Now()
	insert := func(offsetHours float64) {
		db.Exec(`INSERT INTO ont_metrics (time, ont_id, rx_rate_mbps, tx_rate_mbps)
			VALUES (?, ?, ?, ?)`,
			base.Add(time.Duration(-offsetHours*float64(time.Hour))), ontID.String(), 1.5, 2.5)
	}
	insert(1)
	insert(10)
	insert(50)

	start := base.Add(-24 * time.Hour)
	end := base.Add(-time.Hour)

	rows, err := service.GetONTTrafficTimeSeriesRange(ontID, start, end)

	require.NoError(t, err)
	require.NotEmpty(t, rows)
	for _, r := range rows {
		assert.False(t, r.Time.After(end), "point %v should not be after end", r.Time)
		assert.False(t, r.Time.Before(start), "point %v should not be before start", r.Time)
	}
}

func TestMetricsService_GetONTTrafficTimeSeriesCustomRange_NoDataInRange(t *testing.T) {
	db := setupMetricsTestDB(t)
	service := NewMetricsService(db)

	siteID := uuid.New()
	oltID := uuid.New()
	ontID := uuid.New()

	db.Create(&models.OLT{ID: oltID, SiteID: siteID, Name: "olt1", IPAddress: "192.168.1.1", PreferredProtocol: "ssh", Username: "admin", Password: "pass"})
	db.Create(&models.ONT{ID: ontID, OLTID: oltID, SerialNumber: "ont1"})

	base := time.Now()
	db.Exec(`INSERT INTO ont_metrics (time, ont_id, rx_rate_mbps, tx_rate_mbps)
		VALUES (?, ?, ?, ?)`,
		base.Add(-100*time.Hour), ontID.String(), 1.5, 2.5)

	rows, err := service.GetONTTrafficTimeSeriesRange(ontID, base.Add(-24*time.Hour), base)

	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestMetricsService_GetOLTPollingStats_ReportsDiscoveryProgress(t *testing.T) {
	db := setupMetricsTestDB(t)
	service := NewMetricsService(db)

	siteID := uuid.New()
	oltID := uuid.New()
	require.NoError(t, db.Create(&models.OLT{ID: oltID, SiteID: siteID, Name: "olt-progress", IPAddress: "192.168.1.1", PreferredProtocol: "ssh", Username: "admin", Password: "pass"}).Error)

	columns := []string{
		"discovery_phase TEXT DEFAULT 'idle'",
		"discovery_total INTEGER DEFAULT 0",
		"discovery_registered INTEGER DEFAULT 0",
		"discovery_polled INTEGER DEFAULT 0",
		"discovery_error TEXT",
		"discovery_started_at DATETIME",
		"discovery_last_poll_at DATETIME",
	}
	for _, column := range columns {
		var name string
		parts := strings.SplitN(column, " ", 2)
		row := db.Raw("SELECT name FROM pragma_table_info('olts') WHERE name = ?", parts[0]).Row()
		if err := row.Scan(&name); err == sql.ErrNoRows {
			require.NoError(t, db.Exec("ALTER TABLE olts ADD COLUMN "+column).Error)
		}
	}
	require.NoError(t, db.Exec(`UPDATE olts SET discovery_phase = 'discovering', discovery_total = 197, discovery_registered = 64, discovery_polled = 51 WHERE id = ?`, oltID).Error)

	stats := service.GetOLTPollingStats(oltID)
	assert.Equal(t, "discovering", stats["phase"])
	assert.Equal(t, int64(197), stats["discovery_total"])
	assert.Equal(t, int64(64), stats["discovery_registered"])
	assert.Equal(t, int64(51), stats["discovery_polled"])
}

func TestTrafficBucketKeysUseUTC(t *testing.T) {
	wib := time.FixedZone("WIB", 7*60*60)
	value := time.Date(2026, time.August, 21, 6, 30, 0, 0, wib)

	assert.Equal(t, time.Date(2026, time.August, 20, 0, 0, 0, 0, time.UTC), truncateTrafficBucket(value, "day"))
	assert.Equal(t, time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC), truncateTrafficBucket(value, "month"))
	assert.Equal(t, time.Date(2026, time.August, 20, 23, 0, 0, 0, time.UTC), truncateTrafficBucket(value, "hour"))

	_, startBucket, endBucket, err := trafficBucketBounds(value, value.Add(48*time.Hour), "day")
	require.NoError(t, err)
	assert.Equal(t, time.UTC, startBucket.Location())
	assert.Equal(t, time.Date(2026, time.August, 20, 0, 0, 0, 0, time.UTC), startBucket)
	assert.Equal(t, time.Date(2026, time.August, 22, 0, 0, 0, 0, time.UTC), endBucket)
}

func TestMetricsService_GetONTTrafficTimeSeriesRangeBucket_MonthlyFillsEmptyMonths(t *testing.T) {
	db := setupMetricsTestDB(t)
	service := NewMetricsService(db)

	siteID := uuid.New()
	oltID := uuid.New()
	ontID := uuid.New()

	require.NoError(t, db.Create(&models.OLT{ID: oltID, SiteID: siteID, Name: "olt1", IPAddress: "192.168.1.1", PreferredProtocol: "ssh", Username: "admin", Password: "pass"}).Error)
	require.NoError(t, db.Create(&models.ONT{ID: ontID, OLTID: oltID, SerialNumber: "ont1"}).Error)

	january := time.Date(2026, time.January, 15, 12, 0, 0, 0, time.UTC)
	march := time.Date(2026, time.March, 20, 12, 0, 0, 0, time.UTC)
	require.NoError(t, db.Exec(`INSERT INTO ont_metrics (time, ont_id, rx_rate_mbps, tx_rate_mbps)
		VALUES (?, ?, ?, ?), (?, ?, ?, ?)`,
		january, ontID.String(), 1.0, 2.0,
		march, ontID.String(), 3.0, 4.0,
	).Error)

	start := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, time.December, 31, 23, 59, 59, 0, time.UTC)

	rows, err := service.GetONTTrafficTimeSeriesRangeBucket(ontID, start, end, "month")

	require.NoError(t, err)
	// Only 2 months have data - empty buckets are skipped
	require.Len(t, rows, 2)
	assert.Equal(t, time.January, rows[0].Time.Month())
	assert.Equal(t, time.March, rows[1].Time.Month())
	require.NotNil(t, rows[0].RxRateMbps)
	require.NotNil(t, rows[0].TxRateMbps)
	require.NotNil(t, rows[1].RxRateMbps)
	require.NotNil(t, rows[1].TxRateMbps)
	assert.Equal(t, 1.0, *rows[0].RxRateMbps)
	assert.Equal(t, 2.0, *rows[0].TxRateMbps)
	assert.Equal(t, 3.0, *rows[1].RxRateMbps)
	assert.Equal(t, 4.0, *rows[1].TxRateMbps)
}

func TestMetricsService_GetONTTrafficTimeSeriesRangeBucket_SkipsRowsWithoutRates(t *testing.T) {
	db := setupMetricsTestDB(t)
	service := NewMetricsService(db)

	siteID := uuid.New()
	oltID := uuid.New()
	ontID := uuid.New()

	require.NoError(t, db.Create(&models.OLT{ID: oltID, SiteID: siteID, Name: "olt1", IPAddress: "192.168.1.1", PreferredProtocol: "ssh", Username: "admin", Password: "pass"}).Error)
	require.NoError(t, db.Create(&models.ONT{ID: ontID, OLTID: oltID, SerialNumber: "ont1"}).Error)

	// Aug 18 predates the rate columns being collected: power is recorded but
	// rates are NULL. Aug 20 has real rates.
	require.NoError(t, db.Exec(`INSERT INTO ont_metrics (time, ont_id, rx_power, rx_rate_mbps, tx_rate_mbps)
		VALUES (?, ?, ?, NULL, NULL)`,
		time.Date(2026, time.August, 18, 10, 0, 0, 0, time.UTC), ontID.String(), -20.5,
	).Error)
	require.NoError(t, db.Exec(`INSERT INTO ont_metrics (time, ont_id, rx_rate_mbps, tx_rate_mbps)
		VALUES (?, ?, ?, ?)`,
		time.Date(2026, time.August, 20, 10, 0, 0, 0, time.UTC), ontID.String(), 1.0, 4.0,
	).Error)

	start := time.Date(2026, time.August, 17, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, time.August, 21, 0, 0, 0, 0, time.UTC)

	rows, err := service.GetONTTrafficTimeSeriesRangeBucket(ontID, start, end, "day")

	require.NoError(t, err)
	// The NULL-rate row must not become a 0 Mbps point.
	require.Len(t, rows, 1)
	assert.Equal(t, time.Date(2026, time.August, 20, 0, 0, 0, 0, time.UTC), rows[0].Time)
	assert.Equal(t, 1.0, *rows[0].RxRateMbps)
	assert.Equal(t, 4.0, *rows[0].TxRateMbps)
}

func TestMetricsService_GetONTTrafficTimeSeriesRangeBucket_TracksPeakPerBucket(t *testing.T) {
	db := setupMetricsTestDB(t)
	service := NewMetricsService(db)

	siteID := uuid.New()
	oltID := uuid.New()
	ontID := uuid.New()

	require.NoError(t, db.Create(&models.OLT{ID: oltID, SiteID: siteID, Name: "olt1", IPAddress: "192.168.1.1", PreferredProtocol: "ssh", Username: "admin", Password: "pass"}).Error)
	require.NoError(t, db.Create(&models.ONT{ID: ontID, OLTID: oltID, SerialNumber: "ont1"}).Error)

	// Two samples in the same day bucket: a low sample and a spike.
	morning := time.Date(2026, time.August, 20, 1, 0, 0, 0, time.UTC)
	noon := time.Date(2026, time.August, 20, 12, 0, 0, 0, time.UTC)
	require.NoError(t, db.Exec(`INSERT INTO ont_metrics (time, ont_id, rx_rate_mbps, tx_rate_mbps)
		VALUES (?, ?, ?, ?), (?, ?, ?, ?)`,
		morning, ontID.String(), 1.0, 2.0,
		noon, ontID.String(), 3.0, 12.0,
	).Error)

	start := time.Date(2026, time.August, 20, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, time.August, 20, 23, 59, 59, 0, time.UTC)

	rows, err := service.GetONTTrafficTimeSeriesRangeBucket(ontID, start, end, "day")

	require.NoError(t, err)
	require.Len(t, rows, 1)
	// Average stays the mean of the two samples.
	assert.Equal(t, 2.0, *rows[0].RxRateMbps)
	assert.Equal(t, 7.0, *rows[0].TxRateMbps)
	// Peak reflects the raw spike, not the average.
	require.NotNil(t, rows[0].RxMaxMbps)
	require.NotNil(t, rows[0].TxMaxMbps)
	assert.Equal(t, 3.0, *rows[0].RxMaxMbps)
	assert.Equal(t, 12.0, *rows[0].TxMaxMbps)
}
