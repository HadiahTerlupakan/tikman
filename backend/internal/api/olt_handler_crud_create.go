package api

import (
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func (h *OLTHandler) Create(c *gin.Context) {
	var req CreateOLTRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request body",
			Code:    "INVALID_REQUEST",
			Details: err.Error(),
		})
		return
	}

	sshPort := req.SSHPort
	if sshPort == 0 {
		sshPort = 22
	}
	telnetPort := req.TelnetPort
	if telnetPort == 0 {
		telnetPort = 23
	}
	snmpPort := req.SNMPPort
	if snmpPort == 0 {
		snmpPort = 161
	}
	snmpCommunity := req.SNMPCommunity
	if snmpCommunity == "" {
		snmpCommunity = "public"
	}

	rack := 0
	shelf := 0
	slot := 0

	olt, err := h.service.Create(
		req.SiteID,
		req.Name,
		req.IPAddress,
		snmpCommunity,
		req.Username,
		req.Password,
		req.Model,
		rack,
		shelf,
		slot,
		sshPort,
		telnetPort,
		snmpPort,
		req.PreferredProtocol,
	)
	if err != nil {
		errMsg := err.Error()

		if errMsg == "site not found: record not found" {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error: "Site not found",
				Code:  "INVALID_SITE_ID",
			})
			return
		}

		if errMsg == "IP address already exists" {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error: "IP address already exists",
				Code:  "DUPLICATE_IP",
				Details: map[string]interface{}{
					"ip_address": req.IPAddress,
				},
			})
			return
		}

		if len(errMsg) > 21 && errMsg[:21] == "OLT validation failed" {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "OLT validation failed",
				Code:    "VALIDATION_FAILED",
				Details: errMsg,
			})
			return
		}

		// An unreachable OLT is the operator's input to correct, not a server
		// fault, and the reason has to travel: "Failed to create OLT" with a 500
		// gave no hint that SNMP was the problem.
		if strings.HasPrefix(errMsg, "SNMP connection test failed") {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "SNMP connection test failed",
				Code:    "SNMP_TEST_FAILED",
				Details: errMsg,
			})
			return
		}

		log.Printf("[OLT Create] Failed to create OLT %s (%s): %v", req.Name, req.IPAddress, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to create OLT",
			Code:  "CREATE_FAILED",
		})
		return
	}

	// Start the first discovery immediately. The service-level database claim
	// prevents the worker fallback from starting a duplicate SNMP walk.
	go h.service.AutoDiscoverONTMetrics(olt)

	siteName := h.service.SiteNameForOLT(olt.SiteID)
	ontCount, _ := h.ontService.CountONTsByOLT(olt.ID)
	response := ToOLTResponse(siteName, ontCount, olt)
	c.JSON(http.StatusCreated, response)
}
