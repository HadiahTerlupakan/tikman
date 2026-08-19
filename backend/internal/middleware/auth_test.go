package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/auth"
	"github.com/tikman/olt-provisioning/internal/models"
	"go.uber.org/zap"
)

func setupTestRedis(t *testing.T) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available, skipping test")
	}

	return client
}

func setupTestRouter(middleware gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware)
	router.GET("/test", func(c *gin.Context) {
		userID, _ := c.Get("user_id")
		role, _ := c.Get("user_role")
		c.JSON(200, gin.H{
			"user_id": userID,
			"role":    role,
		})
	})
	return router
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	client := setupTestRedis(t)
	defer func() { _ = client.Close() }()

	store := auth.NewStore(client, 24*time.Hour)
	logger, _ := zap.NewDevelopment()
	userID := uuid.New()
	token, err := store.Create(userID, models.UserRoleAdmin)
	require.NoError(t, err)

	router := setupTestRouter(AuthMiddleware(store, logger))

	req := httptest.NewRequest("GET", "/test", nil)
	req.AddCookie(&http.Cookie{
		Name:  "session_token",
		Value: token,
	})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuthMiddleware_NoToken(t *testing.T) {
	client := setupTestRedis(t)
	defer func() { _ = client.Close() }()

	store := auth.NewStore(client, 24*time.Hour)
	logger, _ := zap.NewDevelopment()
	router := setupTestRouter(AuthMiddleware(store, logger))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	client := setupTestRedis(t)
	defer func() { _ = client.Close() }()

	store := auth.NewStore(client, 24*time.Hour)
	logger, _ := zap.NewDevelopment()
	router := setupTestRouter(AuthMiddleware(store, logger))

	req := httptest.NewRequest("GET", "/test", nil)
	req.AddCookie(&http.Cookie{
		Name:  "session_token",
		Value: "invalid-token",
	})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
