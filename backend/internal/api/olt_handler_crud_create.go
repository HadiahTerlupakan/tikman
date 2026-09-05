package api

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tikman/olt-provisioning/internal/services"
)

// What a ZTE chassis answers on when the form leaves a port or the community
// empty. They are defaults for the form, not limits: an OLT reachable on
// another port is configured with it.
const (
	defaultSSHPort       = 22
	defaultTelnetPort    = 23
	defaultSNMPPort      = 161
	defaultSNMPCommunity = "public"
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

	olt, err := h.service.Create(createOLTInput(req))
	if err != nil {
		refuseOLTCreate(c, req, err)
		return
	}

	// Start the first discovery immediately. The service-level database claim
	// prevents the worker fallback from starting a duplicate SNMP walk.
	go h.service.AutoDiscoverONTMetrics(olt)

	h.respondWithOLT(c, http.StatusCreated, olt)
}

// createOLTInput fills in the ports and community a ZTE chassis answers on
// when the form left them empty.
func createOLTInput(req CreateOLTRequest) services.CreateOLTInput {
	in := services.CreateOLTInput{
		SiteID:            req.SiteID,
		Name:              req.Name,
		IPAddress:         req.IPAddress,
		SNMPCommunity:     req.SNMPCommunity,
		Username:          req.Username,
		Password:          req.Password,
		Model:             req.Model,
		SSHPort:           req.SSHPort,
		TelnetPort:        req.TelnetPort,
		SNMPPort:          req.SNMPPort,
		PreferredProtocol: req.PreferredProtocol,
		Latitude:          req.Latitude,
		Longitude:         req.Longitude,
	}
	if in.SSHPort == 0 {
		in.SSHPort = defaultSSHPort
	}
	if in.TelnetPort == 0 {
		in.TelnetPort = defaultTelnetPort
	}
	if in.SNMPPort == 0 {
		in.SNMPPort = defaultSNMPPort
	}
	if in.SNMPCommunity == "" {
		in.SNMPCommunity = defaultSNMPCommunity
	}
	return in
}

// refuseOLTCreate turns a create failure into the answer the form can act on.
// Everything here except the last case is the operator's input to correct, so
// the reason has to travel: "Failed to create OLT" with a 500 gave no hint
// that SNMP was the problem.
func refuseOLTCreate(c *gin.Context, req CreateOLTRequest, err error) {
	errMsg := err.Error()

	switch {
	case errors.Is(err, services.ErrValidation):
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: strings.TrimPrefix(errMsg, services.ErrValidation.Error()+": "),
			Code:  "INVALID_COORDINATES",
		})
	case errMsg == "site not found: record not found":
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Site not found",
			Code:  "INVALID_SITE_ID",
		})
	case errMsg == "IP address already exists":
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "IP address already exists",
			Code:    "DUPLICATE_IP",
			Details: map[string]interface{}{"ip_address": req.IPAddress},
		})
	case strings.HasPrefix(errMsg, "OLT validation failed"):
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "OLT validation failed",
			Code:    "VALIDATION_FAILED",
			Details: errMsg,
		})
	case strings.HasPrefix(errMsg, "SNMP connection test failed"):
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "SNMP connection test failed",
			Code:    "SNMP_TEST_FAILED",
			Details: errMsg,
		})
	default:
		log.Printf("[OLT Create] Failed to create OLT %s (%s): %v", req.Name, req.IPAddress, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to create OLT",
			Code:  "CREATE_FAILED",
		})
	}
}
