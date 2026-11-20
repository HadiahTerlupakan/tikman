package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/connectivity/shell"
	"github.com/tikman/olt-provisioning/internal/middleware"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
)

// switchProbeTimeout bounds a test connection, login included.
const switchProbeTimeout = 30 * time.Second

// SwitchReadsHandler serves what was read from a switch, the host key trust
// and the actions that only schedule a read. None of it writes to a switch.
type SwitchReadsHandler struct {
	switches *services.SwitchService
	queries  *services.SwitchQueryService
	jobs     *services.SwitchPollJobService
	prober   *services.SwitchProber
	audit    *services.AuditService
}

// NewSwitchReadsHandler creates a switch reads handler.
func NewSwitchReadsHandler(switches *services.SwitchService, queries *services.SwitchQueryService,
	jobs *services.SwitchPollJobService, prober *services.SwitchProber, audit *services.AuditService) *SwitchReadsHandler {
	return &SwitchReadsHandler{switches: switches, queries: queries, jobs: jobs, prober: prober, audit: audit}
}

// existingSwitch answers 404 for a switch that is not there, so a read of a
// missing switch is not an empty list.
func (h *SwitchReadsHandler) existingSwitch(c *gin.Context) (uuid.UUID, bool) {
	id, ok := routerParam(c, "id")
	if !ok {
		return uuid.Nil, false
	}
	if _, err := h.switches.Get(id); err != nil {
		respondSwitchError(c, err)
		return uuid.Nil, false
	}
	return id, true
}

// Interfaces returns a switch's ports.
func (h *SwitchReadsHandler) Interfaces(c *gin.Context) {
	h.respondWithSwitch(c, func(id uuid.UUID) (interface{}, error) { return h.queries.Interfaces(id) })
}

// VLANs returns a switch's VLANs.
func (h *SwitchReadsHandler) VLANs(c *gin.Context) {
	h.respondWithSwitch(c, func(id uuid.UUID) (interface{}, error) { return h.queries.VLANs(id) })
}

// Neighbors returns what was seen behind a switch's ports.
func (h *SwitchReadsHandler) Neighbors(c *gin.Context) {
	h.respondWithSwitch(c, func(id uuid.UUID) (interface{}, error) { return h.queries.Neighbors(id) })
}

// Backups lists a switch's config backups without their text.
func (h *SwitchReadsHandler) Backups(c *gin.Context) {
	h.respondWithSwitch(c, func(id uuid.UUID) (interface{}, error) { return h.queries.Backups(id) })
}

func (h *SwitchReadsHandler) respondWithSwitch(c *gin.Context, read func(uuid.UUID) (interface{}, error)) {
	id, ok := h.existingSwitch(c)
	if !ok {
		return
	}
	rows, err := read(id)
	if err != nil {
		respondSwitchError(c, err)
		return
	}
	c.JSON(http.StatusOK, rows)
}

// Backup returns one backup with its masked text.
func (h *SwitchReadsHandler) Backup(c *gin.Context) {
	id, ok := routerParam(c, "id")
	if !ok {
		return
	}
	backupID, ok := routerParam(c, "bid")
	if !ok {
		return
	}
	backup, err := h.queries.Backup(id, backupID)
	if err != nil {
		respondSwitchError(c, err)
		return
	}
	c.JSON(http.StatusOK, backup)
}

// BackupDiff returns the unified diff between two backups.
func (h *SwitchReadsHandler) BackupDiff(c *gin.Context) {
	id, ok := routerParam(c, "id")
	if !ok {
		return
	}
	from, err1 := uuid.Parse(c.Query("from"))
	to, err2 := uuid.Parse(c.Query("to"))
	if err := errors.Join(err1, err2); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "from and to must be backup ids", Code: "INVALID_ID"})
		return
	}
	diff, err := h.queries.Diff(id, from, to)
	if err != nil {
		respondSwitchError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"diff": diff})
}

// BackupNow makes the backup job due, for the worker to run as a manual
// backup.
func (h *SwitchReadsHandler) BackupNow(c *gin.Context) {
	h.schedule(c, "backup_now", h.jobs.RequestBackup)
}

// Refresh makes the interface and inventory jobs due.
func (h *SwitchReadsHandler) Refresh(c *gin.Context) {
	h.schedule(c, "refresh", func(id uuid.UUID) error {
		return h.jobs.RunNow(id, models.SwitchPollKindInterfaces, models.SwitchPollKindInventory)
	})
}

func (h *SwitchReadsHandler) schedule(c *gin.Context, action string, run func(uuid.UUID) error) {
	id, ok := h.existingSwitch(c)
	if !ok {
		return
	}
	if err := run(id); err != nil {
		respondSwitchError(c, err)
		return
	}
	h.record(c, action, id, nil)
	c.JSON(http.StatusAccepted, gin.H{"status": "scheduled"})
}

// TrustHostKey pins the fingerprint the admin confirmed, if the switch still
// presents it.
func (h *SwitchReadsHandler) TrustHostKey(c *gin.Context) {
	id, ok := routerParam(c, "id")
	if !ok {
		return
	}
	var req TrustSwitchHostKeyRequest
	if !bindRouterJSON(c, &req) {
		return
	}
	sw, err := h.switches.TrustHostKey(id, req.Fingerprint)
	if err != nil {
		respondSwitchError(c, err)
		return
	}
	h.record(c, "trust_host_key", id, map[string]interface{}{"fingerprint": sw.HostKeyFingerprint})
	c.JSON(http.StatusOK, sw)
}

// TestConnection opens the switch from the form's values, read-only, and says
// what it is. A fingerprint nobody has pinned yet answers 409 with the
// fingerprint, for the form to show.
func (h *SwitchReadsHandler) TestConnection(c *gin.Context) {
	var req CreateSwitchRequest
	if !bindRouterJSON(c, &req) {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), switchProbeTimeout)
	defer cancel()
	identity, err := h.prober.Probe(ctx, req.toInput())
	var unpinned shell.ErrHostKeyUnpinned
	switch {
	case err == nil:
		c.JSON(http.StatusOK, gin.H{"model": identity.Model, "version": identity.Version})
	case errors.As(err, &unpinned):
		c.JSON(http.StatusConflict, gin.H{"error": "Host key belum dipercaya", "code": "HOST_KEY_UNPINNED",
			"presented_fingerprint": unpinned.Presented})
	case errors.Is(err, services.ErrValidation):
		respondSwitchError(c, err)
	default:
		c.JSON(http.StatusBadGateway, ErrorResponse{Error: "Switch tidak dapat dihubungi", Code: "SWITCH_UNREACHABLE"})
	}
}

func (h *SwitchReadsHandler) record(c *gin.Context, action string, id uuid.UUID, values map[string]interface{}) {
	if h.audit == nil {
		return
	}
	actorID, _ := middleware.GetUserID(c)
	_ = h.audit.Log(actorID, action, switchAuditResource, id, nil, values, c.ClientIP(), c.Request.UserAgent())
}
