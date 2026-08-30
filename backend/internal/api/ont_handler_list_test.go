package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupONTListHandler(t *testing.T) (*gorm.DB, *ONTHandler, uuid.UUID) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.OLT{}, &models.ONT{}))

	// The handler enriches each row from ont_metrics. Without the table the
	// filtering still tests true, but every case logs an error and the test stops
	// resembling the path it is meant to cover.
	require.NoError(t, db.Exec(`CREATE TABLE IF NOT EXISTS ont_metrics (
		time DATETIME NOT NULL, ont_id TEXT NOT NULL,
		rx_power REAL, tx_power REAL, temperature REAL, voltage REAL, distance INTEGER,
		rx_bytes INTEGER, tx_bytes INTEGER, rx_packets INTEGER, tx_packets INTEGER,
		rx_rate_mbps REAL, tx_rate_mbps REAL)`).Error)

	oltID := uuid.New()
	require.NoError(t, db.Create(&models.OLT{
		ID: oltID, SiteID: uuid.New(), Name: "Cariu", IPAddress: "172.30.30.3",
		Model: models.OLTModelZTEC320, Username: "admin", Password: "pass",
	}).Error)

	handler := NewONTHandler(services.NewONTService(db), services.NewMetricsService(db), nil, nil)
	return db, handler, oltID
}

func addONT(t *testing.T, db *gorm.DB, oltID uuid.UUID, slot *int, port, ontID int, serial string) {
	t.Helper()
	require.NoError(t, db.Create(&models.ONT{
		ID: uuid.New(), OLTID: oltID, Slot: slot, PortID: port, ONTID: ontID,
		SerialNumber: serial, Status: models.ONTStatusOnline,
	}).Error)
}

func listONTs(t *testing.T, handler *ONTHandler, query string) map[string]any {
	t.Helper()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/onts?"+query, nil)

	handler.List(c)
	require.Equal(t, http.StatusOK, recorder.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	return body
}

func TestListNarrowsByCardAndPort(t *testing.T) {
	db, handler, oltID := setupONTListHandler(t)
	eight, nine := 8, 9
	addONT(t, db, oltID, &eight, 1, 1, "SNCARD08")
	addONT(t, db, oltID, &nine, 1, 1, "SNCARD09")

	body := listONTs(t, handler, "slot=9&port_id=1")

	assert.Equal(t, float64(1), body["total"])
	rows := body["data"].([]any)
	require.Len(t, rows, 1)
	assert.Equal(t, "SNCARD09", rows[0].(map[string]any)["serial_number"])
}

func TestListSearchesTheSerial(t *testing.T) {
	db, handler, oltID := setupONTListHandler(t)
	addONT(t, db, oltID, nil, 1, 1, "RTEGC6090CD5")
	addONT(t, db, oltID, nil, 1, 2, "ZTEGCACC2F40")

	body := listONTs(t, handler, "search=acc2f")

	assert.Equal(t, float64(1), body["total"])
}

func TestListTotalDescribesTheWholeMatchNotThePage(t *testing.T) {
	// The pager is driven by this number. Reporting the page's own size would
	// tell an operator the network ends where their screen does.
	db, handler, oltID := setupONTListHandler(t)
	for i := 0; i < 25; i++ {
		addONT(t, db, oltID, nil, 1, i, "SN"+uuid.New().String()[:8])
	}

	body := listONTs(t, handler, "limit=10&offset=0")

	assert.Equal(t, float64(25), body["total"])
	assert.Len(t, body["data"].([]any), 10)
}

func TestListIgnoresAnUnparseableNarrowingParameter(t *testing.T) {
	// A stray parameter should widen the answer, not fail the page an operator
	// is looking at.
	db, handler, oltID := setupONTListHandler(t)
	addONT(t, db, oltID, nil, 1, 1, "SNONLYONE")

	body := listONTs(t, handler, "slot=abc&port_id=")

	assert.Equal(t, float64(1), body["total"])
}
