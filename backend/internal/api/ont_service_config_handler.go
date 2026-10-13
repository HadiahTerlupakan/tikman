package api

import (
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/middleware"
	"github.com/tikman/olt-provisioning/internal/models"
)

// GetServiceConfig handles GET /api/v1/onts/:id/service-config. It returns the
// ONU's provisioned service as the last discovery poll read it from the OLT,
// so the configure form can open showing what is actually running.
//
// The subscriber's PPPoE password, decrypted from store, is included only for
// the roles that may reconfigure the ONU: reconfiguring has to resend it, and
// those operators can read it off the OLT's own running config anyway.
func (h *ONTHandler) GetServiceConfig(c *gin.Context) {
	ontID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Code: "INVALID_ID", Error: "Invalid ONT ID format"})
		return
	}

	service, updatedAt, err := h.ontService.GetServiceConfig(ontID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Code: "NOT_FOUND", Error: err.Error()})
		return
	}

	// An ONT the poll has not covered yet answers with no data rather than an
	// error: the form simply opens empty, as it did before.
	var payload gin.H
	if service != nil {
		redactPPPoEPassword(c, service)
		payload = serviceConfigPayload(*service)
	}

	c.JSON(http.StatusOK, gin.H{
		"ont_id":     ontID,
		"data":       payload,
		"updated_at": updatedAt,
	})
}

// redactPPPoEPassword clears the subscriber's PPPoE password for every role
// that cannot reach the configure form. The password logs anyone in to the
// BRAS as that subscriber, so it goes no further than the operators who must
// resend it, as redactCredentials does for an OLT's SNMP community.
func redactPPPoEPassword(c *gin.Context, service *connectivity.ZTEONUService) {
	role, ok := middleware.GetUserRole(c)
	if ok && (role == models.UserRoleAdmin || role == models.UserRoleTechnician) {
		return
	}
	service.PPPoEPassword = ""
}

// serviceConfigPayload is the shape the configure form reads back.
func serviceConfigPayload(service connectivity.ZTEONUService) gin.H {
	return gin.H{
		"onu_type":      service.ONUType,
		"vlan_id":       service.VLANID,
		"vlan_mode":     service.VLANMode,
		"service_type":  service.ServiceType,
		"tcont_profile": service.TCONTProfile,
		"wan_mode":      service.WANMode,
		"wan_ip_mode":   service.WANIPMode,
		"vlan_profile":  service.VLANProfile,
		// Left out, the form could only ever open with the VEIP toggle off, and
		// saving from there dropped it on an ONU that had it.
		"use_veip":       service.UseVEIP,
		"pppoe_username": service.PPPoEUsername,
		"pppoe_password": service.PPPoEPassword,
	}
}
