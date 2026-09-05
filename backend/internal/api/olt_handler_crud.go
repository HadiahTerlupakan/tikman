package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/middleware"
	"github.com/tikman/olt-provisioning/internal/models"
)

// redactCredentials clears the fields of an OLT response that are credentials
// for the chassis rather than facts about it. The community string is one: an
// SNMP agent authenticates on it, so it goes only to the roles that may manage
// the OLT — the same ones that can reach the edit form which reads it back.
func redactCredentials(c *gin.Context, response *OLTResponse) {
	role, ok := middleware.GetUserRole(c)
	if ok && (role == models.UserRoleAdmin || role == models.UserRoleTechnician) {
		return
	}
	response.SNMPCommunity = ""
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
		siteName := h.service.SiteNameForOLT(olt.SiteID)
		ontCount, _ := h.ontService.CountONTsByOLT(olt.ID)
		responses[i] = ToOLTResponse(siteName, ontCount, &olt)
		redactCredentials(c, &responses[i])
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

	siteName := h.service.SiteNameForOLT(olt.SiteID)
	ontCount, _ := h.ontService.CountONTsByOLT(olt.ID)
	response := ToOLTResponse(siteName, ontCount, olt)
	redactCredentials(c, &response)
	c.JSON(http.StatusOK, response)
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

	if h.auditService != nil {
		actorID, _ := middleware.GetUserID(c)
		_ = h.auditService.Log(
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
