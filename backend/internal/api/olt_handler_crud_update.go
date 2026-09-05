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

	if err := h.service.Update(id, oltUpdates(req)); err != nil {
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

	// Discovery runs on create and worker cycles; a metadata-only update must
	// not start a second long-running SNMP walk.
	h.respondWithOLT(c, http.StatusOK, olt)
}

// oltUpdates turns the supplied fields into the columns to write. Absent
// fields are left alone, which is what makes the form able to send only what
// the operator touched.
func oltUpdates(req UpdateOLTRequest) map[string]interface{} {
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

	setString := map[string]*string{
		"name": req.Name, "ip_address": req.IPAddress,
		"username": req.Username, "password": req.Password,
	}
	for column, value := range setString {
		if value != nil {
			updates[column] = *value
		}
	}

	setInt := map[string]*int{
		"ssh_port": req.SSHPort, "telnet_port": req.TelnetPort, "snmp_port": req.SNMPPort,
	}
	for column, value := range setInt {
		if value != nil {
			updates[column] = *value
		}
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
	return updates
}
