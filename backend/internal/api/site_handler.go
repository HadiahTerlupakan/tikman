package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/middleware"
	"github.com/tikman/olt-provisioning/internal/models"
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

	site, err := h.service.CreateWithCoordinates(req.Name, req.Location, req.Description, req.Latitude, req.Longitude)
	if err != nil {
		if errors.Is(err, services.ErrValidation) {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error: strings.TrimPrefix(err.Error(), services.ErrValidation.Error()+": "),
				Code:  "INVALID_COORDINATES",
			})
			return
		}
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

// applyCoordinateUpdate writes the coordinate columns the request asked for.
func applyCoordinateUpdate(req UpdateSiteRequest, updates map[string]interface{}) {
	if req.ClearCoordinates {
		updates["latitude"] = (*float64)(nil)
		updates["longitude"] = (*float64)(nil)
		return
	}
	if req.Latitude != nil {
		updates["latitude"] = req.Latitude
	}
	if req.Longitude != nil {
		updates["longitude"] = req.Longitude
	}
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

	// Read before the write, so the audit entry can say what changed.
	oldSite, err := h.service.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error: "Site not found",
			Code:  "NOT_FOUND",
		})
		return
	}

	if err := h.service.Update(id, siteUpdates(req)); err != nil {
		refuseSiteUpdate(c, err)
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

	h.logSiteUpdate(c, oldSite, site)

	oltCount, _ := h.service.CountOLTsBySite(site.ID)
	c.JSON(http.StatusOK, ToSiteResponse(oltCount, site))
}

// refuseSiteUpdate separates coordinates the operator can correct from a
// failure that is ours.
func refuseSiteUpdate(c *gin.Context, err error) {
	if errors.Is(err, services.ErrValidation) {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: strings.TrimPrefix(err.Error(), services.ErrValidation.Error()+": "),
			Code:  "INVALID_COORDINATES",
		})
		return
	}
	c.JSON(http.StatusInternalServerError, ErrorResponse{
		Error: "Failed to update site",
		Code:  "UPDATE_FAILED",
	})
}

// siteUpdates turns the supplied fields into the columns to write, leaving
// absent ones alone.
func siteUpdates(req UpdateSiteRequest) map[string]interface{} {
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
	applyCoordinateUpdate(req, updates)
	return updates
}

func (h *SiteHandler) logSiteUpdate(c *gin.Context, before, after *models.Site) {
	if h.auditService == nil {
		return
	}
	actorID, _ := middleware.GetUserID(c)
	_ = h.auditService.Log(
		actorID,
		"update",
		"site",
		after.ID,
		map[string]interface{}{
			"name":        before.Name,
			"location":    before.Location,
			"description": before.Description,
		},
		map[string]interface{}{
			"name":        after.Name,
			"location":    after.Location,
			"description": after.Description,
		},
		c.ClientIP(),
		c.Request.UserAgent(),
	)
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
