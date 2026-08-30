package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/auth"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupHealthTest(t *testing.T) (*gorm.DB, *HealthHandler) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.WorkerHeartbeat{}))

	return db, NewHealthHandler(db, auth.NewMemoryStore(time.Hour))
}

func probeHealth(t *testing.T, handler *HealthHandler) (int, map[string]any) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/health", nil)

	handler.Check(c)

	var body map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	return recorder.Code, body
}

func stampHeartbeat(t *testing.T, db *gorm.DB, at time.Time) {
	t.Helper()
	require.NoError(t, db.Create(&models.WorkerHeartbeat{
		Name:   models.WorkerHeartbeatPoller,
		BeatAt: at,
	}).Error)
}

func dependencies(body map[string]any) map[string]any {
	deps, _ := body["dependencies"].(map[string]any)
	return deps
}

func TestHealthReportsAWorkerThatJustFinishedACycle(t *testing.T) {
	db, handler := setupHealthTest(t)
	stampHeartbeat(t, db, time.Now().Add(-20*time.Second))

	code, body := probeHealth(t, handler)

	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, "up", dependencies(body)["worker"])
	assert.NotEmpty(t, body["worker_last_beat"])
}

func TestHealthReportsAWorkerThatStoppedPolling(t *testing.T) {
	db, handler := setupHealthTest(t)
	stampHeartbeat(t, db, time.Now().Add(-10*time.Minute))

	code, body := probeHealth(t, handler)

	assert.Equal(t, "down", dependencies(body)["worker"])
	// The status code is the api container's Docker healthcheck. A dead worker
	// is a separate process failing; answering 503 here would have Docker
	// restart an API that is serving correctly.
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, "healthy", body["status"])
}

func TestHealthToleratesASingleSlowCycle(t *testing.T) {
	// One cycle running long — an OLT answering SNMP at its timeout — must not
	// be reported as a dead worker.
	db, handler := setupHealthTest(t)
	stampHeartbeat(t, db, time.Now().Add(-2*time.Minute))

	_, body := probeHealth(t, handler)

	assert.Equal(t, "up", dependencies(body)["worker"])
}

func TestHealthSaysUnknownBeforeTheWorkerHasEverRun(t *testing.T) {
	// A fresh install has no heartbeat yet. Calling that "down" would raise an
	// alarm about a worker that simply has not had its first cycle.
	_, handler := setupHealthTest(t)

	_, body := probeHealth(t, handler)

	assert.Equal(t, "unknown", dependencies(body)["worker"])
	assert.NotContains(t, body, "worker_last_beat")
}

func TestHealthStillFailsReadinessWhenTheDatabaseIsGone(t *testing.T) {
	db, handler := setupHealthTest(t)
	stampHeartbeat(t, db, time.Now())

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	code, body := probeHealth(t, handler)

	assert.Equal(t, http.StatusServiceUnavailable, code)
	assert.Equal(t, "degraded", body["status"])
	assert.Equal(t, "down", dependencies(body)["database"])
	// The worker cannot be assessed without the database it is read from.
	assert.Equal(t, "unknown", dependencies(body)["worker"])
}
