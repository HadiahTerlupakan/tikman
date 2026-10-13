package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/middleware"
	"github.com/tikman/olt-provisioning/internal/models"
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

	if h.refuseRepoint(c, id, req) {
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

// refuseRepoint answers 403 when anyone but an admin asks to change where
// TikMan connects to the OLT. The CLI commander decrypts the stored password
// and sends it, and the pollers the SNMP community, to whatever address, port
// and protocol the row names, so moving them would hand those secrets to a
// host the caller controls - the reason editing a router is admin-only.
// Resending the stored values stays open: the edit form sends every field even
// when only the name or the pin changed.
func (h *OLTHandler) refuseRepoint(c *gin.Context, id uuid.UUID, req UpdateOLTRequest) bool {
	if role, _ := middleware.GetUserRole(c); role == models.UserRoleAdmin {
		return false
	}
	if req.IPAddress == nil && req.SSHPort == nil && req.TelnetPort == nil &&
		req.SNMPPort == nil && req.PreferredProtocol == nil {
		return false
	}

	olt, err := h.service.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error: "OLT not found",
			Code:  "NOT_FOUND",
		})
		return true
	}
	if !repointsOLT(olt, req) {
		return false
	}

	c.JSON(http.StatusForbidden, ErrorResponse{
		Error: "Only an admin may change an OLT's address, ports or protocol",
		Code:  "FORBIDDEN",
	})
	return true
}

func repointsOLT(olt *models.OLT, req UpdateOLTRequest) bool {
	return (req.IPAddress != nil && *req.IPAddress != olt.IPAddress) ||
		(req.SSHPort != nil && *req.SSHPort != olt.SSHPort) ||
		(req.TelnetPort != nil && *req.TelnetPort != olt.TelnetPort) ||
		(req.SNMPPort != nil && *req.SNMPPort != olt.SNMPPort) ||
		(req.PreferredProtocol != nil && *req.PreferredProtocol != olt.PreferredProtocol)
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
