package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/services"
)

// UnconfiguredONUHandler handles unconfigured (autofind) ONU HTTP requests.
type UnconfiguredONUHandler struct {
	service *services.UnconfiguredONUService
}

// NewUnconfiguredONUHandler creates a new unconfigured ONU handler.
func NewUnconfiguredONUHandler(service *services.UnconfiguredONUService) *UnconfiguredONUHandler {
	return &UnconfiguredONUHandler{service: service}
}

// ListByOLT handles GET /api/v1/olts/:id/unconfigured-onus
func (h *UnconfiguredONUHandler) ListByOLT(c *gin.Context) {
	oltID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:  "INVALID_ID",
			Error: "Invalid OLT ID format",
		})
		return
	}

	onus, err := h.service.ListByOLT(oltID)
	if err != nil {
		status, code := http.StatusInternalServerError, "SCAN_FAILED"
		switch {
		case strings.Contains(err.Error(), "OLT not found"):
			status, code = http.StatusNotFound, "NOT_FOUND"
		case strings.Contains(err.Error(), "SNMP community not configured"):
			status, code = http.StatusBadRequest, "CONFIG_ERROR"
		}
		c.JSON(status, ErrorResponse{Code: code, Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"olt_id": oltID,
		"data":   onus,
		"total":  len(onus),
	})
}
