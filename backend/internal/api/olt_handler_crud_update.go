package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/services"
)

func (h *OLTHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Invalid OLT ID",
			Code:  "INVALID_ID",
		})
		return
	}

	var req UpdateOLTRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request body",
			Code:    "INVALID_REQUEST",
			Details: err.Error(),
		})
		return
	}

	_, _ = h.service.GetByID(id)

	updates := make(map[string]interface{})
	// Clearing wins over a supplied value: an operator who ticks "remove the
	// pin" and leaves stale numbers in the fields means the removal.
	if req.ClearCoordinates {
		updates["latitude"] = (*float64)(nil)
		updates["longitude"] = (*float64)(nil)
	} else {
		if req.Latitude != nil {
			updates["latitude"] = req.Latitude
		}
		if req.Longitude != nil {
			updates["longitude"] = req.Longitude
		}
	}

	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.IPAddress != nil {
		updates["ip_address"] = *req.IPAddress
	}
	if req.SSHPort != nil {
		updates["ssh_port"] = *req.SSHPort
	}
	if req.TelnetPort != nil {
		updates["telnet_port"] = *req.TelnetPort
	}
	if req.SNMPPort != nil {
		updates["snmp_port"] = *req.SNMPPort
	}
	if req.SNMPCommunity != nil {
		updates["snmp_community"] = *req.SNMPCommunity
	}
	if req.PreferredProtocol != nil {
		updates["preferred_protocol"] = *req.PreferredProtocol
	}
	if req.Model != nil {
		updates["model"] = *req.Model
	}
	if req.Username != nil {
		updates["username"] = *req.Username
	}
	if req.Password != nil {
		updates["password"] = *req.Password
	}

	if err := h.service.Update(id, updates); err != nil {
		if errors.Is(err, services.ErrValidation) {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error: strings.TrimPrefix(err.Error(), services.ErrValidation.Error()+": "),
				Code:  "INVALID_COORDINATES",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to update OLT",
			Code:  "UPDATE_FAILED",
		})
		return
	}

	olt, err := h.service.GetByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "OLT updated but failed to retrieve",
			Code:  "RETRIEVAL_FAILED",
		})
		return
	}

	// Discovery runs on create and worker cycles; avoid starting a second
	// long-running SNMP walk on every metadata-only update.

	siteName := h.service.SiteNameForOLT(olt.SiteID)
	ontCount, _ := h.ontService.CountONTsByOLT(olt.ID)
	response := ToOLTResponse(siteName, ontCount, olt)
	c.JSON(http.StatusOK, response)
}
