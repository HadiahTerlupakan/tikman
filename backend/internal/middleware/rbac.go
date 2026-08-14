package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tikman/olt-provisioning/internal/models"
)

func RequireRole(allowedRoles ...models.UserRole) gin.HandlerFunc {
	return func(c *gin.Context) {
		roleValue, exists := c.Get("user_role")
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{
				"error":   "User role not found in context",
				"code":    "FORBIDDEN",
				"details": gin.H{},
			})
			c.Abort()
			return
		}

		userRole, ok := roleValue.(models.UserRole)
		if !ok {
			c.JSON(http.StatusForbidden, gin.H{
				"error":   "Invalid user role type",
				"code":    "FORBIDDEN",
				"details": gin.H{},
			})
			c.Abort()
			return
		}

		for _, role := range allowedRoles {
			if userRole == role {
				c.Next()
				return
			}
		}

		c.JSON(http.StatusForbidden, gin.H{
			"error":   "Insufficient permissions",
			"code":    "FORBIDDEN",
			"details": gin.H{},
		})
		c.Abort()
	}
}
