package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/middleware"
	"github.com/tikman/olt-provisioning/internal/services"
)

// SettingHandler manages credentials for external integrations.
type SettingHandler struct {
	service      *services.SettingService
	auditService *services.AuditService
}

// NewSettingHandler creates a setting handler.
func NewSettingHandler(service *services.SettingService, auditService *services.AuditService) *SettingHandler {
	return &SettingHandler{service: service, auditService: auditService}
}

// List handles GET /api/v1/settings. It returns status only, never values.
func (h *SettingHandler) List(c *gin.Context) {
	statuses, err := h.service.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:  "LIST_FAILED",
			Error: "Failed to read settings",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": statuses})
}

// Browser handles GET /api/v1/settings/browser. Any authenticated user may
// call it: the values it returns drive features that only run client-side.
func (h *SettingHandler) Browser(c *gin.Context) {
	values, err := h.service.BrowserValues()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:  "READ_FAILED",
			Error: "Failed to read settings",
		})
		return
	}
	c.JSON(http.StatusOK, values)
}

// Set handles PUT /api/v1/settings/:name.
func (h *SettingHandler) Set(c *gin.Context) {
	name := c.Param("name")

	var req SetSettingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:  "INVALID_REQUEST",
			Error: "A value is required",
		})
		return
	}

	actorID, _ := middleware.GetUserID(c)
	if err := h.service.Set(name, req.Value, actorID); err != nil {
		h.reportFailure(c, err)
		return
	}

	h.audit(c, actorID, "update", name)

	statuses, err := h.service.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:  "LIST_FAILED",
			Error: "Saved, but the status could not be read back",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": statuses})
}

// Delete handles DELETE /api/v1/settings/:name.
func (h *SettingHandler) Delete(c *gin.Context) {
	name := c.Param("name")

	if err := h.service.Delete(name); err != nil {
		h.reportFailure(c, err)
		return
	}

	actorID, _ := middleware.GetUserID(c)
	h.audit(c, actorID, "delete", name)

	// c.Status alone never flushes headers unless the gin engine's request
	// lifecycle runs afterward; c.JSON does, and matches every other DELETE
	// handler in this package (site, user, wireguard, olt).
	c.JSON(http.StatusNoContent, nil)
}

func (h *SettingHandler) reportFailure(c *gin.Context, err error) {
	switch {
	case errors.Is(err, services.ErrUnknownSetting):
		c.JSON(http.StatusNotFound, ErrorResponse{
			Code:  "UNKNOWN_SETTING",
			Error: "No such setting",
		})
	case errors.Is(err, services.ErrValidation):
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:  "INVALID_VALUE",
			Error: "A value is required",
		})
	default:
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:  "SAVE_FAILED",
			Error: "Failed to save the setting",
		})
	}
}

// audit records who changed which setting. The value is deliberately absent:
// an audit trail holding the credential defeats the encryption it audits.
// Settings are keyed by name rather than UUID, so the resource id is uuid.Nil.
func (h *SettingHandler) audit(c *gin.Context, actorID uuid.UUID, action, name string) {
	if h.auditService == nil {
		return
	}
	_ = h.auditService.Log(
		actorID,
		action,
		"setting",
		uuid.Nil,
		nil,
		map[string]interface{}{"name": name},
		c.ClientIP(),
		c.Request.UserAgent(),
	)
}
