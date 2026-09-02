package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func userCreateRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	db := TestDB(t)
	handler := NewUserHandler(services.NewUserService(db), services.NewAuditService(db, zap.NewNop()))

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/v1/users", handler.Create)
	return router, db
}

// The whole CS module authorises against models.UserRoleCS, so the role has
// to be assignable to a person. CreateUserRequest's oneof list is the only
// thing that decides that, and it left "cs" out — a CS had to be created as a
// technician for the inbox to let them in.
func TestCreateUserAcceptsTheCSRole(t *testing.T) {
	router, db := userCreateRouter(t)

	body := `{"username":"csbudi","email":"cs@example.com","password":"rahasiasekali12","role":"cs"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)

	var stored models.User
	require.NoError(t, db.Where("username = ?", "csbudi").First(&stored).Error)
	assert.Equal(t, models.UserRoleCS, stored.Role)
}

// Widening the list must not widen it to anything: a role nothing authorises
// against is still refused before it reaches the database, which has no check
// constraint on users.role to catch it later.
func TestCreateUserStillRefusesAnUnknownRole(t *testing.T) {
	router, _ := userCreateRouter(t)

	body := `{"username":"nobody","email":"nobody@example.com","password":"rahasiasekali12","role":"supervisor"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
