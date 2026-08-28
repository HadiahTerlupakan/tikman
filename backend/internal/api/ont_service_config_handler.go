package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// GetServiceConfig handles GET /api/v1/onts/:id/service-config. It returns the
// ONU's provisioned service as the last discovery poll read it from the OLT,
// so the configure form can open showing what is actually running.
//
// The response includes the subscriber's PPPoE password, decrypted from store.
// Reconfiguring the service has to resend it, and the same operator can read
// it off the OLT's own running config anyway; it is stored encrypted and only
// leaves here over an authenticated session.
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
		payload = gin.H{
			"onu_type":       service.ONUType,
			"vlan_id":        service.VLANID,
			"vlan_mode":      service.VLANMode,
			"service_type":   service.ServiceType,
			"tcont_profile":  service.TCONTProfile,
			"wan_mode":       service.WANMode,
			"wan_ip_mode":    service.WANIPMode,
			"vlan_profile":   service.VLANProfile,
			"pppoe_username": service.PPPoEUsername,
			"pppoe_password": service.PPPoEPassword,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"ont_id":     ontID,
		"data":       payload,
		"updated_at": updatedAt,
	})
}
