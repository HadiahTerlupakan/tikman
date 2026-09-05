package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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

// rbacTestRouter builds the real route tree, and hands back the session store
// so a test can hold a role rather than assert on middleware in isolation:
// which role reaches a route is a property of router.go, not of the handler.
func rbacTestRouter(t *testing.T) (*gin.Engine, *auth.Store) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, models.AutoMigrate(db))

	store := auth.NewMemoryStore(24 * time.Hour)
	cfg := &config.Config{
		LogLevel:       "release",
		EncryptionKey:  testEncryptionKey,
		Environment:    "development",
		AllowedOrigins: "http://localhost:3000",
	}

	router, _, _, _ := Setup(gin.New(), cfg, db, store, zap.NewNop(),
		services.NewWireGuardService(db, testEncryptionKey, &connectivity.MemoryTunnelDevice{}), nil)
	return router, store
}

func postAs(t *testing.T, router *gin.Engine, store *auth.Store, role models.UserRole, path string) int {
	t.Helper()

	token, err := store.Create(uuid.New(), role)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, path, nil)
	req.AddCookie(&http.Cookie{Name: "session_token", Value: token})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec.Code
}

// discoveryWrites are the OLT routes that reach out over SNMP, and in the last
// case write ONT rows. /discover-now next to them in router.go already carries
// RequireRole; these three were left on bare authentication, which put a write
// in reach of the two read-only roles.
var discoveryWrites = []string{"topology", "discover", "discover-and-register"}

func TestDiscoveryIsClosedToReadOnlyRoles(t *testing.T) {
	for _, role := range []models.UserRole{models.UserRoleViewer, models.UserRoleCS} {
		for _, action := range discoveryWrites {
			t.Run(fmt.Sprintf("%s/%s", role, action), func(t *testing.T) {
				router, store := rbacTestRouter(t)
				path := fmt.Sprintf("/api/v1/olts/%s/%s", uuid.New(), action)

				assert.Equal(t, http.StatusForbidden, postAs(t, router, store, role, path))
			})
		}
	}
}

// The same routes must stay open to the roles that run discovery, or closing
// them to a viewer would have cost the operators the feature.
func TestDiscoveryStaysOpenToTechnicians(t *testing.T) {
	for _, action := range discoveryWrites {
		t.Run(action, func(t *testing.T) {
			router, store := rbacTestRouter(t)
			path := fmt.Sprintf("/api/v1/olts/%s/%s", uuid.New(), action)

			// The OLT does not exist, so the handler answers 404 or 500. What
			// matters is that the route was entered at all.
			assert.NotEqual(t, http.StatusForbidden,
				postAs(t, router, store, models.UserRoleTechnician, path))
		})
	}
}
