package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/tikman/olt-provisioning/internal/auth"
	"github.com/tikman/olt-provisioning/internal/config"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestHealthEndpoint(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	err = models.AutoMigrate(db)
	assert.NoError(t, err)

	cfg := &config.Config{
		LogLevel:       "debug",
		EncryptionKey:  "0123456789abcdef0123456789abcdef",
		Environment:    "development",
		AllowedOrigins: "http://localhost:3000",
	}

	sessionStore := auth.NewMemoryStore(24 * time.Hour)

	router, _, _ := Setup(gin.New(), cfg, db, sessionStore, logger,
		services.NewWireGuardService(db, testEncryptionKey, &connectivity.MemoryTunnelDevice{}), nil)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "healthy", response["status"])
	assert.NotNil(t, response["time"])

	// The dashboard renders one row per dependency, so each has to report its own
	// state. A memory-backed session store means Redis was never configured,
	// which is not the same as Redis being unreachable.
	deps, ok := response["dependencies"].(map[string]interface{})
	assert.True(t, ok, "expected a dependencies object, got %v", response["dependencies"])
	assert.Equal(t, "up", deps["database"])
	assert.Equal(t, "disabled", deps["redis"])
}

func TestHealthEndpoint_ReportsDatabaseDown(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	cfg := &config.Config{
		LogLevel:       "debug",
		EncryptionKey:  "0123456789abcdef0123456789abcdef",
		Environment:    "development",
		AllowedOrigins: "http://localhost:3000",
	}

	router, _, _ := Setup(gin.New(), cfg, db, auth.NewMemoryStore(24*time.Hour), logger,
		services.NewWireGuardService(db, testEncryptionKey, &connectivity.MemoryTunnelDevice{}), nil)

	// Closing the pool makes every query fail, standing in for an unreachable
	// Postgres without needing one in the test environment.
	sqlDB, err := db.DB()
	assert.NoError(t, err)
	assert.NoError(t, sqlDB.Close())

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// A failing dependency must not read as healthy to an uptime checker.
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var response map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, "degraded", response["status"])

	deps, ok := response["dependencies"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "down", deps["database"])
}

func TestRouterSetup(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	cfg := &config.Config{
		LogLevel:       "release",
		EncryptionKey:  "0123456789abcdef0123456789abcdef",
		Environment:    "development",
		AllowedOrigins: "http://localhost:3000",
	}

	sessionStore := auth.NewMemoryStore(24 * time.Hour)

	router, _, _ := Setup(gin.New(), cfg, db, sessionStore, logger,
		services.NewWireGuardService(db, testEncryptionKey, &connectivity.MemoryTunnelDevice{}), nil)

	assert.NotNil(t, router)
}

// The create form tests a connection before the OLT exists, so this route must
// carry no :id. It previously lived at /olts/:id/test, which the frontend never
// calls, making Test Connection a silent 404. A 404 here means the path and its
// only caller have drifted apart again; 401 means the route exists and merely
// wants a session, which is all this test needs to prove.
func TestTestConnectionRouteIsRegisteredWithoutID(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)
	assert.NoError(t, models.AutoMigrate(db))

	cfg := &config.Config{
		LogLevel:       "release",
		EncryptionKey:  "0123456789abcdef0123456789abcdef",
		Environment:    "development",
		AllowedOrigins: "http://localhost:3000",
	}

	router, _, _ := Setup(gin.New(), cfg, db, auth.NewMemoryStore(24*time.Hour), logger,
		services.NewWireGuardService(db, testEncryptionKey, &connectivity.MemoryTunnelDevice{}), nil)

	req := httptest.NewRequest("POST", "/api/v1/olts/test-connection", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.NotEqual(t, http.StatusNotFound, w.Code,
		"POST /api/v1/olts/test-connection is unrouted; the frontend's Test Connection button calls exactly this path")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
