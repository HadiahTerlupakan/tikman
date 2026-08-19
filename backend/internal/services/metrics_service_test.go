package services

import (
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
			tx_errors INTEGER
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

	ontID := uuid.New()
	metrics, err := service.GetLatestMetrics(ontID)

	require.NoError(t, err)
	assert.Nil(t, metrics.RxPower)
	assert.Nil(t, metrics.TxPower)
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

	err := service.StoreMetrics(ontID, metrics)
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

	err := service.StoreMetrics(ontID, metrics)
	require.NoError(t, err)

	retrieved, err := service.GetLatestMetrics(ontID)
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	assert.Nil(t, retrieved.RxPower)
	assert.Nil(t, retrieved.TxPower)
	assert.Equal(t, temp, retrieved.Temperature)
}
