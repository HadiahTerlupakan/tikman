package api

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tikman/olt-provisioning/internal/auth"
	"gorm.io/gorm"
)

// healthProbeTimeout keeps a hung dependency from stalling the probe, which
// uptime checkers poll frequently.
const healthProbeTimeout = 2 * time.Second

// HealthHandler reports whether the API and its dependencies are usable.
type HealthHandler struct {
	db        *gorm.DB
	authStore *auth.Store
}

// NewHealthHandler creates a health handler.
func NewHealthHandler(db *gorm.DB, authStore *auth.Store) *HealthHandler {
	return &HealthHandler{db: db, authStore: authStore}
}

// Check handles GET /health. It returns 503 when a dependency the API needs to
// serve requests is unreachable, so the response is usable as a readiness probe
// rather than only proving the process is running.
func (h *HealthHandler) Check(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), healthProbeTimeout)
	defer cancel()

	database := h.databaseStatus(ctx)
	redis := h.authStore.Backend(ctx)

	status, code := "healthy", http.StatusOK
	if database != string(auth.BackendUp) || redis == auth.BackendDown {
		status, code = "degraded", http.StatusServiceUnavailable
	}

	c.JSON(code, gin.H{
		"status": status,
		"time":   time.Now().UTC().Format(time.RFC3339),
		"dependencies": gin.H{
			"database": database,
			"redis":    redis,
		},
	})
}

func (h *HealthHandler) databaseStatus(ctx context.Context) string {
	sqlDB, err := h.db.DB()
	if err != nil {
		return string(auth.BackendDown)
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		return string(auth.BackendDown)
	}
	return string(auth.BackendUp)
}
