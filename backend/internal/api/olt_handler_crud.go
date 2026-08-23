package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/middleware"
)

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
		responses[i] = ToOLTResponse(siteName, &olt)
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
	response := ToOLTResponse(siteName, olt)
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
