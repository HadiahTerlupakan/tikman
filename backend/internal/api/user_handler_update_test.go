package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// userUpdateRouter runs Update behind the context the real middleware leaves:
// the actor's own id and role, which is what the handler authorises against.
func userUpdateRouter(db *gorm.DB, actor *models.User) *gin.Engine {
	handler := NewUserHandler(services.NewUserService(db), services.NewAuditService(db, zap.NewNop()))

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.PUT("/api/v1/users/:id", func(c *gin.Context) {
		c.Set("user_id", actor.ID)
		c.Set("user_role", actor.Role)
	}, handler.Update)
	return router
}

func createUser(t *testing.T, db *gorm.DB, username string, role models.UserRole) *models.User {
	t.Helper()
	user, err := services.NewUserService(db).Create(
		username, username+"@example.com", "rahasiasekali12", "", role)
	require.NoError(t, err)
	return user
}

func putUser(router *gin.Engine, id uuid.UUID, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPut,
		fmt.Sprintf("/api/v1/users/%s", id), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// A technician is on PUT /users/:id so they can maintain their own account.
// Letting the role travel in that same body turns the route into a one-request
// promotion to admin, which every admin-only route then honours.
func TestTechnicianCannotPromoteThemselves(t *testing.T) {
	db := TestDB(t)
	technician := createUser(t, db, "teknisi", models.UserRoleTechnician)

	rec := putUser(userUpdateRouter(db, technician), technician.ID, `{"role":"admin"}`)

	assert.Equal(t, http.StatusForbidden, rec.Code)

	var stored models.User
	require.NoError(t, db.First(&stored, "id = ?", technician.ID).Error)
	assert.Equal(t, models.UserRoleTechnician, stored.Role)
}

// The other half of the same hole: without touching any role, a technician
// could set the admin's password and simply log in as them.
func TestTechnicianCannotChangeAnotherAccount(t *testing.T) {
	db := TestDB(t)
	technician := createUser(t, db, "teknisi", models.UserRoleTechnician)
	admin := createUser(t, db, "admin", models.UserRoleAdmin)

	rec := putUser(userUpdateRouter(db, technician), admin.ID, `{"password":"passwordbaru123"}`)

	assert.Equal(t, http.StatusForbidden, rec.Code)

	var stored models.User
	require.NoError(t, db.First(&stored, "id = ?", admin.ID).Error)
	assert.Equal(t, admin.PasswordHash, stored.PasswordHash)
}

// Maintaining your own account is why a technician is on this route at all, so
// that has to keep working.
func TestTechnicianCanUpdateTheirOwnAccount(t *testing.T) {
	db := TestDB(t)
	technician := createUser(t, db, "teknisi", models.UserRoleTechnician)

	rec := putUser(userUpdateRouter(db, technician), technician.ID, `{"email":"baru@example.com"}`)

	assert.Equal(t, http.StatusOK, rec.Code)

	var stored models.User
	require.NoError(t, db.First(&stored, "id = ?", technician.ID).Error)
	assert.Equal(t, "baru@example.com", stored.Email)
}

// Role changes remain an admin's to make, on anyone.
func TestAdminCanStillChangeARole(t *testing.T) {
	db := TestDB(t)
	admin := createUser(t, db, "admin", models.UserRoleAdmin)
	viewer := createUser(t, db, "pengamat", models.UserRoleViewer)

	rec := putUser(userUpdateRouter(db, admin), viewer.ID, `{"role":"technician"}`)

	assert.Equal(t, http.StatusOK, rec.Code)

	var stored models.User
	require.NoError(t, db.First(&stored, "id = ?", viewer.ID).Error)
	assert.Equal(t, models.UserRoleTechnician, stored.Role)
}
