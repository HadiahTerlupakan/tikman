package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/middleware"
	"github.com/tikman/olt-provisioning/internal/services"
)

type SiteHandler struct {
	service      *services.SiteService
	auditService *services.AuditService
}

func NewSiteHandler(service *services.SiteService, auditService *services.AuditService) *SiteHandler {
	return &SiteHandler{
		service:      service,
		auditService: auditService,
	}
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

	// Audit log
	actorID, _ := middleware.GetUserID(c)
	if h.auditService != nil {
		_ = h.auditService.Log(
			actorID,
			"create",
			"site",
			site.ID,
			nil,
			map[string]interface{}{
				"name":        site.Name,
				"location":    site.Location,
				"description": site.Description,
			},
			c.ClientIP(),
			c.Request.UserAgent(),
		)
	}

	oltCount, _ := h.service.CountOLTsBySite(site.ID)
	c.JSON(http.StatusCreated, ToSiteResponse(oltCount, site))
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
		oltCount, _ := h.service.CountOLTsBySite(site.ID)
		responses[i] = ToSiteResponse(oltCount, &site)
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
		if strings.Contains(err.Error(), "database error") {
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error: "Database error",
				Code:  "DATABASE_ERROR",
			})
			return
		}
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error: "Site not found",
			Code:  "NOT_FOUND",
		})
		return
	}

	oltCount, _ := h.service.CountOLTsBySite(site.ID)
	c.JSON(http.StatusOK, ToSiteResponse(oltCount, site))
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

	// Get old state for audit log
	oldSite, err := h.service.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error: "Site not found",
			Code:  "NOT_FOUND",
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

	site, err := h.service.GetByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to retrieve updated site",
			Code:  "RETRIEVE_FAILED",
		})
		return
	}

	// Audit log
	actorID, _ := middleware.GetUserID(c)
	oldState := map[string]interface{}{
		"name":        oldSite.Name,
		"location":    oldSite.Location,
		"description": oldSite.Description,
	}
	newState := map[string]interface{}{
		"name":        site.Name,
		"location":    site.Location,
		"description": site.Description,
	}
	if h.auditService != nil {
		_ = h.auditService.Log(
			actorID,
			"update",
			"site",
			site.ID,
			oldState,
			newState,
			c.ClientIP(),
			c.Request.UserAgent(),
		)
	}

	oltCount, _ := h.service.CountOLTsBySite(site.ID)
	c.JSON(http.StatusOK, ToSiteResponse(oltCount, site))
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
		if errors.Is(err, services.ErrSiteHasTunnel) {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "Site still has a VPN tunnel",
				Code:    "SITE_HAS_TUNNEL",
				Details: err.Error(),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to delete site",
			Code:  "DELETE_FAILED",
		})
		return
	}

	// Audit log
	actorID, _ := middleware.GetUserID(c)
	if h.auditService != nil {
		_ = h.auditService.Log(
			actorID,
			"delete",
			"site",
			id,
			nil,
			nil,
			c.ClientIP(),
			c.Request.UserAgent(),
		)
	}

	c.JSON(http.StatusNoContent, nil)
}
