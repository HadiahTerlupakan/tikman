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

	// The same defaults the create form applies, so testing a connection and
	// then saving it reach the chassis the same way.
	in := createOLTInput(CreateOLTRequest{
		IPAddress:         req.IPAddress,
		Username:          req.Username,
		Password:          req.Password,
		SSHPort:           req.SSHPort,
		TelnetPort:        req.TelnetPort,
		SNMPPort:          req.SNMPPort,
		SNMPCommunity:     req.SNMPCommunity,
		PreferredProtocol: req.PreferredProtocol,
	})

	result, err := h.validatorService.ValidateCreate(
		in.IPAddress,
		in.Username,
		in.Password,
		in.SSHPort,
		in.TelnetPort,
		in.SNMPPort,
		in.SNMPCommunity,
		in.PreferredProtocol,
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
