package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *OLTHandler) TestConnection(c *gin.Context) {
	var req TestConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request body",
			Code:    "INVALID_REQUEST",
			Details: err.Error(),
		})
		return
	}

	// Apply defaults
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

	// Run validation tests
	result, err := h.validatorService.ValidateCreate(
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
			Error: "Failed to test connection",
			Code:  "TEST_FAILED",
		})
		return
	}

	c.JSON(http.StatusOK, TestConnectionResponse{
		Success:      result.Success,
		PassedTests:  result.PassedTests,
		FailedTest:   result.FailedTest,
		FailedReason: result.FailedReason,
	})
}
