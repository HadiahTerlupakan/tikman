package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/tikman/olt-provisioning/internal/models"
	"go.uber.org/zap"
)

func firebaseTokenRouter(t *testing.T, h *FirebaseTokenHandler, id uuid.UUID, role models.UserRole) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user_id", id)
		c.Set("user_role", role)
		c.Next()
	})
	r.GET("/api/v1/auth/firebase-token", h.Token)
	return r
}

// The whole app is built to run without a Firebase project. An unconfigured
// deployment must say so plainly rather than fail as if something broke.
func TestFirebaseTokenSaysSoWhenFirebaseIsNotConfigured(t *testing.T) {
	h := NewFirebaseTokenHandler(nil, zap.NewNop())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/firebase-token", nil)
	rec := httptest.NewRecorder()
	firebaseTokenRouter(t, h, uuid.New(), models.UserRoleCS).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Contains(t, rec.Body.String(), "FIREBASE_NOT_CONFIGURED")
}

// The id comes from the session and nowhere else. A token minted for an id in
// the request would let any agent write another agent's presence node.
func TestFirebaseTokenRefusesWithoutASession(t *testing.T) {
	h := NewFirebaseTokenHandler(nil, zap.NewNop())

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/v1/auth/firebase-token", h.Token)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/firebase-token", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}
