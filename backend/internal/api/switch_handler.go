package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/middleware"
	"github.com/tikman/olt-provisioning/internal/services"
	"gorm.io/gorm"
)

const switchAuditResource = "switch"

// SwitchHandler serves the switch pages.
type SwitchHandler struct {
	switches *services.SwitchService
	audit    *services.AuditService
}

// NewSwitchHandler creates a switch handler.
func NewSwitchHandler(switches *services.SwitchService, audit *services.AuditService) *SwitchHandler {
	return &SwitchHandler{switches: switches, audit: audit}
}

// List returns every switch.
func (h *SwitchHandler) List(c *gin.Context) {
	switches, err := h.switches.List()
	if err != nil {
		respondSwitchError(c, err)
		return
	}
	c.JSON(http.StatusOK, switches)
}

// Get returns one switch.
func (h *SwitchHandler) Get(c *gin.Context) {
	id, ok := routerParam(c, "id")
	if !ok {
		return
	}
	sw, err := h.switches.Get(id)
	if err != nil {
		respondSwitchError(c, err)
		return
	}
	c.JSON(http.StatusOK, sw)
}

// Create adds a switch.
func (h *SwitchHandler) Create(c *gin.Context) {
	var req CreateSwitchRequest
	if !bindRouterJSON(c, &req) {
		return
	}
	sw, err := h.switches.Create(req.toInput())
	if err != nil {
		respondSwitchError(c, err)
		return
	}
	h.record(c, "create", sw.ID, map[string]interface{}{"name": sw.Name, "host": sw.Host,
		"vendor": sw.Vendor, "protocol": sw.Protocol})
	c.JSON(http.StatusCreated, sw)
}

// Update changes a switch.
func (h *SwitchHandler) Update(c *gin.Context) {
	id, ok := routerParam(c, "id")
	if !ok {
		return
	}
	var req UpdateSwitchRequest
	if !bindRouterJSON(c, &req) {
		return
	}
	sw, err := h.switches.Update(id, req.toUpdate())
	if err != nil {
		respondSwitchError(c, err)
		return
	}
	// Which fields changed, never the secret values themselves.
	h.record(c, "update", id, map[string]interface{}{"name": sw.Name, "host": sw.Host, "protocol": sw.Protocol,
		"community_changed": req.SNMPCommunity != nil && *req.SNMPCommunity != "",
		"password_changed":  req.Password != nil && *req.Password != ""})
	c.JSON(http.StatusOK, sw)
}

// Delete removes a switch.
func (h *SwitchHandler) Delete(c *gin.Context) {
	id, ok := routerParam(c, "id")
	if !ok {
		return
	}
	if err := h.switches.Delete(id); err != nil {
		respondSwitchError(c, err)
		return
	}
	h.record(c, "delete", id, nil)
	c.Status(http.StatusNoContent)
}

func (h *SwitchHandler) record(c *gin.Context, action string, id uuid.UUID, values map[string]interface{}) {
	if h.audit == nil {
		return
	}
	actorID, _ := middleware.GetUserID(c)
	_ = h.audit.Log(actorID, action, switchAuditResource, id, nil, values, c.ClientIP(), c.Request.UserAgent())
}

func respondSwitchError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, services.ErrValidation):
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error(), Code: "INVALID_SWITCH"})
	case errors.Is(err, gorm.ErrRecordNotFound):
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "Switch not found", Code: "NOT_FOUND"})
	case errors.Is(err, services.ErrHostKeyNotPresented):
		c.JSON(http.StatusConflict, ErrorResponse{Error: "Switch kini menunjukkan fingerprint lain",
			Code: "HOST_KEY_NOT_PRESENTED"})
	default:
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Switch request failed", Code: "SWITCH_FAILED"})
	}
}
