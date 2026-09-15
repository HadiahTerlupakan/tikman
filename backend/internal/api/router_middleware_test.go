package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/auth"
	"github.com/tikman/olt-provisioning/internal/config"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newMiddlewareTestRouter(t *testing.T, allowedOrigins string) *gin.Engine {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, models.AutoMigrate(db))

	logger, _ := zap.NewDevelopment()
	cfg := &config.Config{
		LogLevel:       "release",
		EncryptionKey:  "0123456789abcdef0123456789abcdef",
		Environment:    "development",
		AllowedOrigins: allowedOrigins,
	}

	router, _, _ := Setup(gin.New(), cfg, db, auth.NewMemoryStore(24*time.Hour), logger,
		services.NewWireGuardService(db, testEncryptionKey, &connectivity.MemoryTunnelDevice{}), nil)
	return router
}

func TestCORSEchoesConfiguredOrigin(t *testing.T) {
	router := newMiddlewareTestRouter(t, "https://tikman.example.com, http://localhost:3000")

	for _, origin := range []string{"https://tikman.example.com", "http://localhost:3000"} {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		req.Header.Set("Origin", origin)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Echoed rather than wildcarded, because the session cookie rides on
		// Allow-Credentials and browsers reject that next to "*".
		assert.Equal(t, origin, w.Header().Get("Access-Control-Allow-Origin"))
		assert.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))
	}
}

func TestCORSRejectsUnconfiguredOrigin(t *testing.T) {
	router := newMiddlewareTestRouter(t, "https://tikman.example.com")

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Origin", "https://attacker.example.com")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "Origin", w.Header().Get("Vary"))
}

func TestLoginIsRateLimitedPerIP(t *testing.T) {
	router := newMiddlewareTestRouter(t, "http://localhost:3000")

	body := `{"username":"nobody","password":"wrongpassword"}`
	postLogin := func() int {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w.Code
	}

	for i := 0; i < loginRequestsPerMinute; i++ {
		assert.Equal(t, http.StatusUnauthorized, postLogin(), "attempt %d should still be answered", i+1)
	}

	assert.Equal(t, http.StatusTooManyRequests, postLogin(),
		"password guessing past %d attempts a minute must be refused", loginRequestsPerMinute)
}

func TestHealthIsNotRateLimitedByTheLoginBudget(t *testing.T) {
	router := newMiddlewareTestRouter(t, "http://localhost:3000")

	// The uptime checker and the SPA's own polling share this path, so the
	// login ceiling must not reach it.
	for i := 0; i < loginRequestsPerMinute*2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		require.NotEqual(t, http.StatusTooManyRequests, w.Code, "request %d was throttled", i+1)
	}
}
