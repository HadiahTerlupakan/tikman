package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// corsMiddleware echoes back only the origins listed in ALLOWED_ORIGINS. The
// origin has to be echoed rather than wildcarded because the session cookie
// needs Access-Control-Allow-Credentials, which browsers refuse alongside "*".
func corsMiddleware(allowedOrigins string) gin.HandlerFunc {
	allowed := make(map[string]struct{})
	for _, origin := range strings.Split(allowedOrigins, ",") {
		if trimmed := strings.TrimSpace(origin); trimmed != "" {
			allowed[trimmed] = struct{}{}
		}
	}

	return func(c *gin.Context) {
		// Without this a shared cache keyed on the URL alone would hand one
		// origin's Allow-Origin header to another.
		c.Header("Vary", "Origin")

		origin := c.GetHeader("Origin")
		if _, ok := allowed[origin]; ok {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, Cookie, X-CSRF-Token, X-Request-ID")
			c.Header("Access-Control-Allow-Credentials", "true")
		}

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
