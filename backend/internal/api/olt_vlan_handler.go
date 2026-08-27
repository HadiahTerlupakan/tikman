package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ListVLANs handles GET /api/v1/olts/:id/vlans. The list comes from the
// discovery poll's cache, not a live walk, so opening the provisioning form
// costs no SNMP traffic.
func (h *OLTHandler) ListVLANs(c *gin.Context) {
	oltID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:  "INVALID_ID",
			Error: "Invalid OLT ID format",
		})
		return
	}

	vlans, updatedAt, err := h.service.ListVLANs(oltID)
	if err != nil {
		status, code := http.StatusInternalServerError, "VLAN_LOOKUP_FAILED"
		if strings.Contains(err.Error(), "OLT not found") {
			status, code = http.StatusNotFound, "NOT_FOUND"
		}
		c.JSON(status, ErrorResponse{Code: code, Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"olt_id":     oltID,
		"data":       vlans,
		"total":      len(vlans),
		"updated_at": updatedAt,
	})
}
