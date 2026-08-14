package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/auth"
	"github.com/tikman/olt-provisioning/internal/models"
	"go.uber.org/zap"
)

func AuthMiddleware(store *auth.Store, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie("session_token")
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "No session token provided",
				"code":    "UNAUTHORIZED",
				"details": gin.H{},
			})
			c.Abort()
			return
		}

		sessionData, err := store.Get(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "Invalid or expired session",
				"code":    "UNAUTHORIZED",
				"details": gin.H{},
			})
			c.Abort()
			return
		}

		if err := store.Refresh(token); err != nil {
			logger.Error("Failed to refresh session TTL",
				zap.String("token", token),
				zap.Error(err),
			)
		}

		c.Set("user_id", sessionData.UserID)
		c.Set("user_role", sessionData.Role)
		c.Set("session_token", token)

		c.Next()
	}
}

func GetUserID(c *gin.Context) (uuid.UUID, bool) {
	userID, exists := c.Get("user_id")
	if !exists {
		return uuid.Nil, false
	}
	uid, ok := userID.(uuid.UUID)
	return uid, ok
}

func GetUserRole(c *gin.Context) (models.UserRole, bool) {
	role, exists := c.Get("user_role")
	if !exists {
		return "", false
	}
	r, ok := role.(models.UserRole)
	return r, ok
}
