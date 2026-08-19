package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tikman/olt-provisioning/internal/auth"
	"github.com/tikman/olt-provisioning/internal/middleware"
	"github.com/tikman/olt-provisioning/internal/services"
)

type AuthHandler struct {
	userService  *services.UserService
	sessionStore *auth.Store
}

func NewAuthHandler(userService *services.UserService, sessionStore *auth.Store) *AuthHandler {
	return &AuthHandler{
		userService:  userService,
		sessionStore: sessionStore,
	}
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Invalid request body",
			Code:  "INVALID_REQUEST",
		})
		return
	}

	user, err := h.userService.GetByUsername(req.Username)
	if err != nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error: "Invalid credentials",
			Code:  "INVALID_CREDENTIALS",
		})
		return
	}

	if err := h.userService.VerifyPassword(user, req.Password); err != nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error: "Invalid credentials",
			Code:  "INVALID_CREDENTIALS",
		})
		return
	}

	token, err := h.sessionStore.Create(user.ID, user.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to create session",
			Code:  "SESSION_FAILED",
		})
		return
	}

	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(
		"session_token",
		token,
		86400, // 24 hours
		"/api",
		"",
		false, // Secure (set true in production with HTTPS)
		true,  // HttpOnly
	)

	c.JSON(http.StatusOK, LoginResponse{
		User:  ToUserResponse(user),
		Token: token,
	})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	token, err := c.Cookie("session_token")
	if err == nil {
		_ = h.sessionStore.Delete(token)
	}

	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(
		"session_token",
		"",
		-1,
		"/api",
		"",
		false,
		true,
	)

	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

func (h *AuthHandler) Me(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error: "User not authenticated",
			Code:  "UNAUTHORIZED",
		})
		return
	}

	user, err := h.userService.GetByID(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error: "User not found",
			Code:  "NOT_FOUND",
		})
		return
	}

	c.JSON(http.StatusOK, ToUserResponse(user))
}
