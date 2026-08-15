package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/middleware"
	"github.com/tikman/olt-provisioning/internal/services"
)

type OLTHandler struct {
	service      *services.OLTService
	auditService *services.AuditService
}

func NewOLTHandler(service *services.OLTService, auditService *services.AuditService) *OLTHandler {
	return &OLTHandler{
		service:      service,
		auditService: auditService,
	}
}

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

	olt, err := h.service.Create(
		req.SiteID,
		req.Name,
		req.IPAddress,
		req.Username,
		req.Password,
		sshPort,
		telnetPort,
		snmpPort,
		snmpCommunity,
		req.PreferredProtocol,
	)
	if err != nil {
		// Check for specific error types
		errMsg := err.Error()

		// Site not found
		if errMsg == "site not found: record not found" {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error: "Site not found",
				Code:  "INVALID_SITE_ID",
			})
			return
		}

		// Duplicate IP
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

		// Validation failure
		if len(errMsg) > 21 && errMsg[:21] == "OLT validation failed" {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error: "OLT validation failed",
				Code:  "VALIDATION_FAILED",
				Details: errMsg,
			})
			return
		}

		// Generic error
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to create OLT",
			Code:  "CREATE_FAILED",
		})
		return
	}

	// Audit log
	if h.auditService != nil {
		actorID, _ := middleware.GetUserID(c)
		h.auditService.Log(
			actorID,
			"create",
			"olt",
			olt.ID,
			nil,
			map[string]interface{}{
				"site_id":            olt.SiteID,
				"name":               olt.Name,
				"ip_address":         olt.IPAddress,
				"preferred_protocol": olt.PreferredProtocol,
			},
			c.ClientIP(),
			c.Request.UserAgent(),
		)
	}

	c.JSON(http.StatusCreated, ToOLTResponse(olt))
}

func (h *OLTHandler) List(c *gin.Context) {
	olts, err := h.service.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to list OLTs",
			Code:  "LIST_FAILED",
		})
		return
	}

	responses := make([]OLTResponse, len(olts))
	for i, olt := range olts {
		responses[i] = ToOLTResponse(&olt)
	}

	c.JSON(http.StatusOK, responses)
}

func (h *OLTHandler) GetByID(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Invalid OLT ID",
			Code:  "INVALID_ID",
		})
		return
	}

	olt, err := h.service.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error: "OLT not found",
			Code:  "NOT_FOUND",
		})
		return
	}

	c.JSON(http.StatusOK, ToOLTResponse(olt))
}

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

	// Get old state for audit log
	oldOLT, err := h.service.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error: "OLT not found",
			Code:  "NOT_FOUND",
		})
		return
	}

	updates := make(map[string]interface{})
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
	if req.Username != nil {
		updates["username"] = *req.Username
	}
	if req.Password != nil {
		updates["password"] = *req.Password
	}

	if err := h.service.Update(id, updates); err != nil {
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

	// Audit log
	actorID, _ := middleware.GetUserID(c)
	oldState := map[string]interface{}{
		"name":               oldOLT.Name,
		"ip_address":         oldOLT.IPAddress,
		"preferred_protocol": oldOLT.PreferredProtocol,
	}
	newState := map[string]interface{}{
		"name":               olt.Name,
		"ip_address":         olt.IPAddress,
		"preferred_protocol": olt.PreferredProtocol,
	}
	if h.auditService != nil {
		h.auditService.Log(
			actorID,
			"update",
			"olt",
			olt.ID,
			oldState,
			newState,
			c.ClientIP(),
			c.Request.UserAgent(),
		)
	}

	c.JSON(http.StatusOK, ToOLTResponse(olt))
}

func (h *OLTHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Invalid OLT ID",
			Code:  "INVALID_ID",
		})
		return
	}

	if err := h.service.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to delete OLT",
			Code:  "DELETE_FAILED",
		})
		return
	}

	// Audit log
	if h.auditService != nil {
		actorID, _ := middleware.GetUserID(c)
		h.auditService.Log(
			actorID,
			"delete",
			"olt",
			id,
			nil,
			nil,
			c.ClientIP(),
			c.Request.UserAgent(),
		)
	}

	c.JSON(http.StatusNoContent, nil)
}
