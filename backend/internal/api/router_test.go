package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/tikman/olt-provisioning/internal/auth"
	"github.com/tikman/olt-provisioning/internal/config"
	"github.com/tikman/olt-provisioning/internal/models"
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
		LogLevel:      "debug",
		EncryptionKey: "0123456789abcdef0123456789abcdef",
	}

	sessionStore := auth.NewMemoryStore(24 * time.Hour)

	router := Setup(cfg, db, sessionStore, logger)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "healthy", response["status"])
	assert.NotNil(t, response["time"])
}

func TestRouterSetup(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	cfg := &config.Config{
		LogLevel:      "release",
		EncryptionKey: "0123456789abcdef0123456789abcdef",
	}

	sessionStore := auth.NewMemoryStore(24 * time.Hour)

	router := Setup(cfg, db, sessionStore, logger)

	assert.NotNil(t, router)
}
