package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tikman/olt-provisioning/internal/auth"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// healthProbeTimeout keeps a hung dependency from stalling the probe, which
// uptime checkers poll frequently.
const healthProbeTimeout = 2 * time.Second

// workerStaleAfter is three of the worker's one-minute cycles. A single slow
// cycle — one OLT answering SNMP at its timeout — must not be reported as a
// dead worker, and three missed cycles is no longer explainable that way.
const workerStaleAfter = 3 * time.Minute

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
	worker, lastBeat := h.workerStatus(ctx, time.Now())

	// The worker is reported but deliberately does not affect the status code.
	// This endpoint is the api container's Docker healthcheck: answering 503
	// because a separate process died would have Docker restart the API, which
	// is serving fine and is not what broke.
	status, code := "healthy", http.StatusOK
	if database != string(auth.BackendUp) || redis == auth.BackendDown {
		status, code = "degraded", http.StatusServiceUnavailable
	}

	body := gin.H{
		"status": status,
		"time":   time.Now().UTC().Format(time.RFC3339),
		"dependencies": gin.H{
			"database": database,
			"redis":    redis,
			"worker":   worker,
		},
	}
	if lastBeat != nil {
		body["worker_last_beat"] = lastBeat.UTC().Format(time.RFC3339)
	}

	c.JSON(code, body)
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

// workerStatus reports whether the polling worker finished a cycle recently.
// "unknown" covers the two cases where the answer is genuinely not known: a
// worker that has never run, and a database that could not be asked.
func (h *HealthHandler) workerStatus(ctx context.Context, now time.Time) (string, *time.Time) {
	var beat models.WorkerHeartbeat

	err := h.db.WithContext(ctx).
		First(&beat, "name = ?", models.WorkerHeartbeatPoller).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "unknown", nil
	}
	if err != nil {
		return "unknown", nil
	}

	if now.Sub(beat.BeatAt) > workerStaleAfter {
		return string(auth.BackendDown), &beat.BeatAt
	}
	return string(auth.BackendUp), &beat.BeatAt
}
