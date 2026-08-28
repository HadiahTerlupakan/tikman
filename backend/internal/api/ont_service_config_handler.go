package api

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/connectivity"
)

// GetServiceConfig handles GET /api/v1/onts/:id/service-config. It returns the
// ONU's provisioned service as the last discovery poll read it from the OLT,
// so the configure form can open showing what is actually running.
//
// The response carries no PPPoE password. The OLT holds one in clear text and
// the operator retypes it rather than having it travel to a browser.
func (h *ONTHandler) GetServiceConfig(c *gin.Context) {
	ontID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Code: "INVALID_ID", Error: "Invalid ONT ID format"})
		return
	}

	ont, err := h.ontService.GetByID(ontID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Code: "NOT_FOUND", Error: "ONT not found"})
		return
	}

	// An ONT the poll has not covered yet answers with no data rather than an
	// error: the form simply opens empty, as it did before.
	var service *connectivity.ZTEONUService
	if len(ont.ServiceConfig) > 0 {
		var decoded connectivity.ZTEONUService
		if err := json.Unmarshal(ont.ServiceConfig, &decoded); err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Code:  "SERVICE_CONFIG_UNREADABLE",
				Error: "stored service config is unreadable",
			})
			return
		}
		service = &decoded
	}

	c.JSON(http.StatusOK, gin.H{
		"ont_id":     ontID,
		"data":       service,
		"updated_at": ont.ServiceConfigAt,
	})
}
