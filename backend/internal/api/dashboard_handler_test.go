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

func setupDashboardTest(t *testing.T) (*gorm.DB, *DashboardHandler) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.OLT{}, &models.ONT{}))

	return db, NewDashboardHandler(services.NewDashboardService(db))
}

func probeDashboard(t *testing.T, handler *DashboardHandler) (int, map[string]any) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/stats", nil)

	handler.GetStats(c)

	var body map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	return recorder.Code, body
}

func TestDashboardStatsReportsTheWholeNetwork(t *testing.T) {
	db, handler := setupDashboardTest(t)

	oltID := uuid.New()
	require.NoError(t, db.Create(&models.OLT{
		ID: oltID, SiteID: uuid.New(), Name: "Cariu", IPAddress: "172.30.30.3",
		Model: models.OLTModelZTEC320, Username: "admin", Password: "pass",
		Status: models.OLTStatusOnline,
	}).Error)
	for i := 0; i < 3; i++ {
		require.NoError(t, db.Create(&models.ONT{
			ID: uuid.New(), OLTID: oltID, PortID: 1, ONTID: i,
			SerialNumber: "SN" + uuid.New().String()[:8], Status: models.ONTStatusOnline,
		}).Error)
	}

	code, body := probeDashboard(t, handler)
	assert.Equal(t, http.StatusOK, code)

	onts, ok := body["onts"].(map[string]any)
	require.True(t, ok, "the response must carry ONT counts")
	assert.Equal(t, float64(3), onts["total"])
	assert.Equal(t, float64(3), onts["online"])

	olts, ok := body["olts"].([]any)
	require.True(t, ok)
	require.Len(t, olts, 1)
	assert.Equal(t, "Cariu", olts[0].(map[string]any)["olt_name"])
}

func TestDashboardStatsOnAnEmptyInstallationIsStillWellFormed(t *testing.T) {
	// The page reads every key on first load, before any OLT is added. A null
	// where a list belongs would break it rather than show an empty board.
	_, handler := setupDashboardTest(t)

	code, body := probeDashboard(t, handler)
	assert.Equal(t, http.StatusOK, code)

	assert.NotNil(t, body["olts"])
	assert.NotNil(t, body["weakest_signals"])
	assert.Empty(t, body["olts"])
	assert.Empty(t, body["weakest_signals"])
}
