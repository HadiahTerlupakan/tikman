package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/middleware"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
)

// ONTHandler handles ONT HTTP requests
type ONTHandler struct {
	ontService     *services.ONTService
	metricsService *services.MetricsService
	auditService   *services.AuditService
}

// NewONTHandler creates a new ONT handler
func NewONTHandler(ontService *services.ONTService, metricsService *services.MetricsService, auditService *services.AuditService) *ONTHandler {
	return &ONTHandler{
		ontService:     ontService,
		metricsService: metricsService,
		auditService:   auditService,
	}
}

// List handles GET /api/v1/onts
func (h *ONTHandler) List(c *gin.Context) {
	var oltID *uuid.UUID
	var status *models.ONTStatus

	// Parse filters
	if oltIDStr := c.Query("olt_id"); oltIDStr != "" {
		id, err := uuid.Parse(oltIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Code:   "INVALID_OLT_ID",
				Error:  "Invalid OLT ID format",
				Details: map[string]string{"olt_id": oltIDStr},
			})
			return
		}
		oltID = &id
	}

	if statusStr := c.Query("status"); statusStr != "" {
		s := models.ONTStatus(statusStr)
		status = &s
	}

	// Parse pagination - Allow up to 500 items per request for frontend display
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	// Validate limit (max 500 to prevent performance issues)
	if limit < 1 {
		limit = 500  // Default to showing max items at once
	} else if limit > 500 {
		limit = 500  // Cap at 500 for safety
	}

	onts, total, err := h.ontService.List(oltID, status, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:  "LIST_FAILED",
			Error: err.Error(),
		})
		return
	}

	oltMap := make(map[uuid.UUID]string)
	var metricsMap map[uuid.UUID]*services.ONTMetricsRow
	
	if len(onts) > 0 {
		oltIDs := make([]uuid.UUID, 0)
		ontIDs := make([]uuid.UUID, len(onts))
		seenOLT := make(map[uuid.UUID]bool)
		
		for i, ont := range onts {
			ontIDs[i] = ont.ID
			if !seenOLT[ont.OLTID] {
				oltIDs = append(oltIDs, ont.OLTID)
				seenOLT[ont.OLTID] = true
			}
		}

		var olts []models.OLT
		if err := h.ontService.GetDB().Select("id, name").Where("id IN ?", oltIDs).Find(&olts).Error; err == nil {
			for _, olt := range olts {
				oltMap[olt.ID] = olt.Name
			}
		}

		metricsMap, _ = h.metricsService.GetLatestMetricsBatch(ontIDs)
	}

	responses := make([]ONTResponse, len(onts))
	for i, ont := range onts {
		metrics := metricsMap[ont.ID]
		resp := ToONTResponseWithMetrics(&ont, metrics)
		resp.OLTName = oltMap[ont.OLTID]
		responses[i] = resp
	}

	c.JSON(http.StatusOK, gin.H{
		"data":   responses,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// GetByID handles GET /api/v1/onts/:id
func (h *ONTHandler) GetByID(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:  "INVALID_ID",
			Error: "Invalid ONT ID format",
		})
		return
	}

	ont, err := h.ontService.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Code:  "NOT_FOUND",
			Error: err.Error(),
		})
		return
	}

	metrics, _ := h.metricsService.GetLatestMetrics(ont.ID)
	
	c.JSON(http.StatusOK, ToONTResponseWithMetrics(ont, metrics))
}

// Create handles POST /api/v1/onts
func (h *ONTHandler) Create(c *gin.Context) {
	var req CreateONTRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:  "VALIDATION_ERROR",
			Error: err.Error(),
		})
		return
	}

	ont := &models.ONT{
		OLTID:        req.OLTID,
		PortID:       req.PortID,
		ONTID:        req.ONTID,
		SerialNumber: req.SerialNumber,
		Description:  req.Description,
		Status:       req.Status,
	}

	if err := h.ontService.Create(ont); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:  "CREATE_FAILED",
			Error: err.Error(),
		})
		return
	}

	// Audit log
	actorID, _ := middleware.GetUserID(c)
	_ = h.auditService.Log(
		actorID,
		"create",
		"ont",
		ont.ID,
		nil,
		map[string]interface{}{
			"olt_id":        ont.OLTID,
			"port_id":       ont.PortID,
			"ont_id":        ont.ONTID,
			"serial_number": ont.SerialNumber,
		},
		c.ClientIP(),
		c.Request.UserAgent(),
	)

	c.JSON(http.StatusCreated, ToONTResponse(ont))
}

// Update handles PUT /api/v1/onts/:id
func (h *ONTHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:  "INVALID_ID",
			Error: "Invalid ONT ID format",
		})
		return
	}

	var req UpdateONTRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:  "VALIDATION_ERROR",
			Error: err.Error(),
		})
		return
	}

	// Get old state for audit
	oldONT, _ := h.ontService.GetByID(id)

	updates := make(map[string]interface{})
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}

	ont, err := h.ontService.Update(id, updates)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:  "UPDATE_FAILED",
			Error: err.Error(),
		})
		return
	}

	// Audit log
	actorID, _ := middleware.GetUserID(c)
	_ = h.auditService.Log(
		actorID,
		"update",
		"ont",
		ont.ID,
		map[string]interface{}{
			"description": oldONT.Description,
			"status":      oldONT.Status,
		},
		map[string]interface{}{
			"description": ont.Description,
			"status":      ont.Status,
		},
		c.ClientIP(),
		c.Request.UserAgent(),
	)

	c.JSON(http.StatusOK, ToONTResponse(ont))
}

// Delete handles DELETE /api/v1/onts/:id
func (h *ONTHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:  "INVALID_ID",
			Error: "Invalid ONT ID format",
		})
		return
	}

	// Get ONT for audit before deleting
	ont, err := h.ontService.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Code:  "NOT_FOUND",
			Error: err.Error(),
		})
		return
	}

	if err := h.ontService.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:  "DELETE_FAILED",
			Error: err.Error(),
		})
		return
	}

	// Audit log
	actorID, _ := middleware.GetUserID(c)
	_ = h.auditService.Log(
		actorID,
		"delete",
		"ont",
		id,
		map[string]interface{}{
			"serial_number": ont.SerialNumber,
			"olt_id":        ont.OLTID,
		},
		nil,
		c.ClientIP(),
		c.Request.UserAgent(),
	)

	c.JSON(http.StatusOK, gin.H{"message": "ONT deleted successfully"})
}
