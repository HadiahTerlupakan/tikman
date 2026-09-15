package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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

func newWireguardRouter(t *testing.T) (*gin.Engine, *auth.Store) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, models.AutoMigrate(db))

	logger, _ := zap.NewDevelopment()
	cfg := &config.Config{
		LogLevel:       "release",
		EncryptionKey:  testEncryptionKey,
		Environment:    "development",
		AllowedOrigins: "http://localhost:3000",
	}
	sessionStore := auth.NewMemoryStore(24 * time.Hour)
	wgService := services.NewWireGuardService(db, testEncryptionKey, &connectivity.MemoryTunnelDevice{})

	router, _, _ := Setup(gin.New(), cfg, db, sessionStore, logger, wgService, nil)
	return router, sessionStore
}

func wireguardSessionCookie(t *testing.T, store *auth.Store, role models.UserRole) *http.Cookie {
	t.Helper()
	token, err := store.Create(uuid.New(), role)
	require.NoError(t, err)
	return &http.Cookie{Name: "session_token", Value: token}
}

// A 404 here would mean the path and its only caller have drifted apart; 401
// proves the route exists and merely wants a session.
func TestWireguardRoutesRequireAuthentication(t *testing.T) {
	router, _ := newWireguardRouter(t)

	for _, path := range []string{"/api/v1/wireguard/server", "/api/v1/wireguard/peers"} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, http.StatusUnauthorized, w.Code, path)
	}
}

func TestWireguardMutationsRequireAdmin(t *testing.T) {
	router, store := newWireguardRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/wireguard/peers", nil)
	req.AddCookie(wireguardSessionCookie(t, store, models.UserRoleTechnician))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusForbidden, w.Code,
		"a wrong subnet on a peer can break routing for other sites, so technicians may only read")
}

// The config endpoint is the only way key material leaves the system, and the
// spec restricts it to Admin even though every other read is open to all roles.
func TestWireguardPeerConfigIsAdminOnly(t *testing.T) {
	router, store := newWireguardRouter(t)

	path := "/api/v1/wireguard/peers/" + uuid.New().String() + "/config"
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.AddCookie(wireguardSessionCookie(t, store, models.UserRoleTechnician))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusForbidden, w.Code,
		"a technician must not be able to read a peer's private key")
}

func TestWireguardListIsReadableByTechnician(t *testing.T) {
	router, store := newWireguardRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/wireguard/peers", nil)
	req.AddCookie(wireguardSessionCookie(t, store, models.UserRoleTechnician))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}
