package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/services"
)

type OLTHandler struct {
	service *services.OLTService
}

func NewOLTHandler(service *services.OLTService) *OLTHandler {
	return &OLTHandler{service: service}
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
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to create OLT",
			Code:  "CREATE_FAILED",
		})
		return
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

	olt, _ := h.service.GetByID(id)
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

	c.JSON(http.StatusNoContent, nil)
}
