package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/services"
)

type SiteHandler struct {
	service *services.SiteService
}

func NewSiteHandler(service *services.SiteService) *SiteHandler {
	return &SiteHandler{service: service}
}

func (h *SiteHandler) Create(c *gin.Context) {
	var req CreateSiteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request body",
			Code:    "INVALID_REQUEST",
			Details: err.Error(),
		})
		return
	}

	site, err := h.service.Create(req.Name, req.Location, req.Description)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to create site",
			Code:  "CREATE_FAILED",
		})
		return
	}

	c.JSON(http.StatusCreated, ToSiteResponse(site))
}

func (h *SiteHandler) List(c *gin.Context) {
	sites, err := h.service.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to list sites",
			Code:  "LIST_FAILED",
		})
		return
	}

	responses := make([]SiteResponse, len(sites))
	for i, site := range sites {
		responses[i] = ToSiteResponse(&site)
	}

	c.JSON(http.StatusOK, responses)
}

func (h *SiteHandler) GetByID(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Invalid site ID",
			Code:  "INVALID_ID",
		})
		return
	}

	site, err := h.service.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error: "Site not found",
			Code:  "NOT_FOUND",
		})
		return
	}

	c.JSON(http.StatusOK, ToSiteResponse(site))
}

func (h *SiteHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Invalid site ID",
			Code:  "INVALID_ID",
		})
		return
	}

	var req UpdateSiteRequest
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
	if req.Location != nil {
		updates["location"] = *req.Location
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}

	if err := h.service.Update(id, updates); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to update site",
			Code:  "UPDATE_FAILED",
		})
		return
	}

	site, _ := h.service.GetByID(id)
	c.JSON(http.StatusOK, ToSiteResponse(site))
}

func (h *SiteHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Invalid site ID",
			Code:  "INVALID_ID",
		})
		return
	}

	if err := h.service.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to delete site",
			Code:  "DELETE_FAILED",
		})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}
