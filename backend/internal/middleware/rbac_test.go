package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/tikman/olt-provisioning/internal/models"
)

func setupRBACRouter(userRole *models.UserRole, allowedRoles ...models.UserRole) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Mock middleware to set user_role in context (simulating auth middleware)
	router.Use(func(c *gin.Context) {
		if userRole != nil {
			c.Set("user_id", uuid.New())
			c.Set("user_role", *userRole)
		}
		c.Next()
	})

	router.Use(RequireRole(allowedRoles...))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "ok"})
	})
	return router
}

func TestRequireRole_Allowed(t *testing.T) {
	role := models.UserRoleAdmin
	router := setupRBACRouter(&role, models.UserRoleAdmin, models.UserRoleTechnician)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequireRole_Forbidden(t *testing.T) {
	role := models.UserRoleViewer
	router := setupRBACRouter(&role, models.UserRoleAdmin)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestRequireRole_NoRole(t *testing.T) {
	router := setupRBACRouter(nil, models.UserRoleAdmin)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestRequireRole_TechnicianAllowed(t *testing.T) {
	role := models.UserRoleTechnician
	router := setupRBACRouter(&role, models.UserRoleAdmin, models.UserRoleTechnician)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequireRole_SingleRoleMatch(t *testing.T) {
	role := models.UserRoleViewer
	router := setupRBACRouter(&role, models.UserRoleViewer)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
