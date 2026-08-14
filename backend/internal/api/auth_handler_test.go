package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/tikman/olt-provisioning/internal/auth"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupAuthTest(t *testing.T) (*gorm.DB, *services.UserService, *auth.Store, *AuthHandler) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	err = db.AutoMigrate(&models.User{})
	assert.NoError(t, err)

	userService := services.NewUserService(db)
	sessionStore := auth.NewMemoryStore(24 * time.Hour)
	handler := NewAuthHandler(userService, sessionStore)

	return db, userService, sessionStore, handler
}

func TestAuthHandler_Login_Success(t *testing.T) {
	_, userService, _, handler := setupAuthTest(t)

	// Create test user
	_, err := userService.Create("testuser", "test@example.com", "password123", models.UserRoleAdmin)
	assert.NoError(t, err)

	// Create login request
	loginReq := LoginRequest{
		Username: "testuser",
		Password: "password123",
	}
	body, _ := json.Marshal(loginReq)

	// Create request
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	// Execute
	handler.Login(c)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)

	var response LoginResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "testuser", response.User.Username)
	assert.NotEmpty(t, response.Token)

	// Check cookie
	cookies := w.Result().Cookies()
	assert.Len(t, cookies, 1)
	assert.Equal(t, "session_token", cookies[0].Name)
	assert.Equal(t, response.Token, cookies[0].Value)
}

func TestAuthHandler_Login_InvalidCredentials(t *testing.T) {
	_, userService, _, handler := setupAuthTest(t)

	// Create test user
	_, err := userService.Create("testuser", "test@example.com", "password123", models.UserRoleAdmin)
	assert.NoError(t, err)

	// Create login request with wrong password
	loginReq := LoginRequest{
		Username: "testuser",
		Password: "wrongpassword",
	}
	body, _ := json.Marshal(loginReq)

	// Create request
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	// Execute
	handler.Login(c)

	// Assert
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var response ErrorResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "Invalid credentials", response.Error)
	assert.Equal(t, "INVALID_CREDENTIALS", response.Code)
}

func TestAuthHandler_Login_UserNotFound(t *testing.T) {
	_, _, _, handler := setupAuthTest(t)

	// Create login request for non-existent user
	loginReq := LoginRequest{
		Username: "nonexistent",
		Password: "password123",
	}
	body, _ := json.Marshal(loginReq)

	// Create request
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	// Execute
	handler.Login(c)

	// Assert
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "Invalid credentials", response.Error)
	assert.Equal(t, "INVALID_CREDENTIALS", response.Code)
}

func TestAuthHandler_Logout(t *testing.T) {
	_, userService, sessionStore, handler := setupAuthTest(t)

	// Create test user and session
	user, err := userService.Create("testuser", "test@example.com", "password123", models.UserRoleAdmin)
	assert.NoError(t, err)

	token, err := sessionStore.Create(user.ID, user.Role)
	assert.NoError(t, err)

	// Create request with cookie
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	c.Request.AddCookie(&http.Cookie{
		Name:  "session_token",
		Value: token,
	})

	// Execute
	handler.Logout(c)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)

	// Check session is deleted
	_, err = sessionStore.Get(token)
	assert.Error(t, err)

	// Check cookie is cleared
	cookies := w.Result().Cookies()
	assert.Len(t, cookies, 1)
	assert.Equal(t, "session_token", cookies[0].Name)
	assert.Equal(t, "", cookies[0].Value)
	assert.Equal(t, -1, cookies[0].MaxAge)
}

func TestAuthHandler_Me_Success(t *testing.T) {
	_, userService, _, handler := setupAuthTest(t)

	// Create test user
	user, err := userService.Create("testuser", "test@example.com", "password123", models.UserRoleAdmin)
	assert.NoError(t, err)

	// Create request with user context
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	c.Set("user_id", user.ID)
	c.Set("user_role", user.Role)

	// Execute
	handler.Me(c)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)

	var response UserResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, user.ID, response.ID)
	assert.Equal(t, "testuser", response.Username)
	assert.Equal(t, "test@example.com", response.Email)
	assert.Equal(t, models.UserRoleAdmin, response.Role)
}

func TestAuthHandler_Me_NoUserContext(t *testing.T) {
	_, _, _, handler := setupAuthTest(t)

	// Create request without user context
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/auth/me", nil)

	// Execute
	handler.Me(c)

	// Assert
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "User not authenticated", response.Error)
	assert.Equal(t, "UNAUTHORIZED", response.Code)
}

func TestAuthHandler_Me_UserNotFound(t *testing.T) {
	_, _, _, handler := setupAuthTest(t)

	// Create request with invalid user ID
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	c.Set("user_id", uuid.New())
	c.Set("user_role", models.UserRoleAdmin)

	// Execute
	handler.Me(c)

	// Assert
	assert.Equal(t, http.StatusNotFound, w.Code)

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "User not found", response.Error)
	assert.Equal(t, "NOT_FOUND", response.Code)
}
