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

	router, _, _ := Setup(gin.New(), cfg, db, store, zap.NewNop(),
		services.NewWireGuardService(db, testEncryptionKey, &connectivity.MemoryTunnelDevice{}), nil)
	return router, store
}

// requestAs holds the given role and fires a bodyless request of the given
// method at path, answering with the status code the route chain produced.
func requestAs(t *testing.T, router *gin.Engine, store *auth.Store, role models.UserRole, method, path string) int {
	t.Helper()

	token, err := store.Create(uuid.New(), role)
	require.NoError(t, err)

	req := httptest.NewRequest(method, path, nil)
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

				assert.Equal(t, http.StatusForbidden, requestAs(t, router, store, role, http.MethodPost, path))
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
				requestAs(t, router, store, models.UserRoleTechnician, http.MethodPost, path))
		})
	}
}

// mappingWrites are the routes that place, move or remove something on the
// map, including where an ONT's drop is patched in; a read-only role must not
// reach any of them. Unlike discoveryWrites these are not all POST, so each
// entry carries its own method.
var mappingWrites = []struct {
	method string
	path   string
}{
	{http.MethodPost, "/api/v1/mapping/nodes"},
	{http.MethodPut, "/api/v1/mapping/nodes/ODP-01"},
	{http.MethodDelete, "/api/v1/mapping/nodes/ODP-01"},
	{http.MethodPost, "/api/v1/mapping/edges"},
	{http.MethodPut, "/api/v1/mapping/edges/E-1"},
	{http.MethodDelete, "/api/v1/mapping/edges/E-1"},
	{http.MethodPut, "/api/v1/onts/11111111-1111-1111-1111-111111111111/odp"},
	{http.MethodDelete, "/api/v1/onts/11111111-1111-1111-1111-111111111111/odp"},
}

func TestMappingWritesAreClosedToReadOnlyRoles(t *testing.T) {
	for _, role := range []models.UserRole{models.UserRoleViewer, models.UserRoleCS} {
		for _, w := range mappingWrites {
			t.Run(fmt.Sprintf("%s/%s %s", role, w.method, w.path), func(t *testing.T) {
				router, store := rbacTestRouter(t)

				assert.Equal(t, http.StatusForbidden, requestAs(t, router, store, role, w.method, w.path))
			})
		}
	}
}

// The same routes must stay open to the role that does field work, or closing
// them to a viewer would have cost technicians the map entirely.
func TestMappingWritesStayOpenToTechnicians(t *testing.T) {
	for _, w := range mappingWrites {
		t.Run(fmt.Sprintf("%s %s", w.method, w.path), func(t *testing.T) {
			router, store := rbacTestRouter(t)

			// The node/edge does not exist and the POSTs carry no body, so the
			// handler answers 400 or 404. What matters is that the role was
			// let past the gate at all.
			assert.NotEqual(t, http.StatusForbidden,
				requestAs(t, router, store, models.UserRoleTechnician, w.method, w.path))
		})
	}
}

// The KMZ export is a read like ListNodes/ListEdges, not a write - a viewer
// who can already see the map in the browser must be able to download it too.
func TestMappingExportIsOpenToEveryRole(t *testing.T) {
	for _, role := range []models.UserRole{models.UserRoleViewer, models.UserRoleCS, models.UserRoleTechnician, models.UserRoleAdmin} {
		t.Run(string(role), func(t *testing.T) {
			router, store := rbacTestRouter(t)

			assert.Equal(t, http.StatusOK,
				requestAs(t, router, store, role, http.MethodGet, "/api/v1/mapping/export"))
		})
	}
}
