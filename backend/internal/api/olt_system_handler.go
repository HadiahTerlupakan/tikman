package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RefreshSystem handles POST /api/v1/olts/:id/system/refresh. It re-reads the
// OLT over SNMP straight away rather than waiting for the discovery poll, which
// an operator needs after changing something on the device.
func (h *OLTHandler) RefreshSystem(c *gin.Context) {
	oltID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:  "INVALID_ID",
			Error: "Invalid OLT ID format",
		})
		return
	}

	snapshot, err := h.service.RefreshSystem(oltID)
	if err != nil {
		status, code := http.StatusInternalServerError, "SYSTEM_REFRESH_FAILED"
		if strings.Contains(err.Error(), "OLT not found") {
			status, code = http.StatusNotFound, "NOT_FOUND"
		}
		c.JSON(status, ErrorResponse{Code: code, Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"olt_id":     oltID,
		"data":       snapshot,
		"updated_at": snapshot.UpdatedAt,
	})
}

// GetSystem handles GET /api/v1/olts/:id/system. It answers from the discovery
// poll's cache, so opening the OLT configuration page costs the device nothing.
func (h *OLTHandler) GetSystem(c *gin.Context) {
	oltID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:  "INVALID_ID",
			Error: "Invalid OLT ID format",
		})
		return
	}

	snapshot, err := h.service.GetSystemSnapshot(oltID)
	if err != nil {
		status, code := http.StatusInternalServerError, "SYSTEM_LOOKUP_FAILED"
		if strings.Contains(err.Error(), "OLT not found") {
			status, code = http.StatusNotFound, "NOT_FOUND"
		}
		c.JSON(status, ErrorResponse{Code: code, Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"olt_id":     oltID,
		"data":       snapshot,
		"updated_at": snapshot.UpdatedAt,
	})
}
