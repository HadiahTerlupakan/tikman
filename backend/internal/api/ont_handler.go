package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

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
	removalService *services.ZTEONURemovalService
}

// NewONTHandler creates a new ONT handler
func NewONTHandler(ontService *services.ONTService, metricsService *services.MetricsService, auditService *services.AuditService, removalService *services.ZTEONURemovalService) *ONTHandler {
	return &ONTHandler{
		ontService:     ontService,
		metricsService: metricsService,
		auditService:   auditService,
		removalService: removalService,
	}
}

// List handles GET /api/v1/onts
// maxONTPageSize bounds one page of ONTs. The old cap of 500 sat below a single
// populated chassis — Cariu carries 651 — so a caller asking for 1000 was
// silently answered with 500, and every page that counted the rows it received
// understated the network without saying so. A full ZTE C320 fits inside this.
//
// It is a ceiling, not a page size to aim for: the overview reads
// /dashboard/stats instead, because counting rows in the browser is what made a
// cap load-bearing in the first place.
const maxONTPageSize = 5000

// optionalInt reads a query parameter that narrows the list, returning nil when
// it is absent or not a number.
func optionalInt(raw string) *int {
	if raw == "" {
		return nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return nil
	}
	return &value
}

func (h *ONTHandler) List(c *gin.Context) {
	var oltID *uuid.UUID
	var status *models.ONTStatus
	var startTime, endTime *time.Time

	// Parse filters
	if oltIDStr := c.Query("olt_id"); oltIDStr != "" {
		id, err := uuid.Parse(oltIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Code:    "INVALID_OLT_ID",
				Error:   "Invalid OLT ID format",
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

	// Parse optional time range filter
	if startTimeStr := c.Query("start_time"); startTimeStr != "" {
		t, err := time.Parse(time.RFC3339, startTimeStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Code:  "INVALID_TIME_RANGE",
				Error: fmt.Sprintf("Invalid start_time format: %v", err),
			})
			return
		}
		startTime = &t
	}

	if endTimeStr := c.Query("end_time"); endTimeStr != "" {
		t, err := time.Parse(time.RFC3339, endTimeStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Code:  "INVALID_TIME_RANGE",
				Error: fmt.Sprintf("Invalid end_time format: %v", err),
			})
			return
		}
		endTime = &t
	}

	// A card and port narrow a position. Anything unparseable is left unset
	// rather than rejected: a stray query parameter should widen the answer, not
	// fail the page an operator is looking at.
	slot := optionalInt(c.Query("slot"))
	portID := optionalInt(c.Query("port_id"))

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	if limit < 1 {
		limit = maxONTPageSize
	} else if limit > maxONTPageSize {
		limit = maxONTPageSize
	}

	onts, total, err := h.ontService.ListFiltered(services.ONTListFilter{
		OLTID:     oltID,
		Status:    status,
		Slot:      slot,
		PortID:    portID,
		Search:    strings.TrimSpace(c.Query("search")),
		StartTime: startTime,
		EndTime:   endTime,
		Limit:     limit,
		Offset:    offset,
	})
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

		if olts, err := h.ontService.GetONTOlts(oltIDs); err == nil {
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

	// The OLT is cleared first. Deleting TikMan's rows before the device would
	// leave an ONU configured on the OLT that nothing tracks any more, and the
	// next discovery poll would simply register it again.
	if c.Query("remove_from_olt") == "true" {
		if h.removalService == nil {
			c.JSON(http.StatusServiceUnavailable, ErrorResponse{
				Code:  "OLT_REMOVAL_UNAVAILABLE",
				Error: "This deployment has no CLI access configured for OLT removal",
			})
			return
		}
		if err := h.removalService.RemoveFromOLT(c.Request.Context(), id); err != nil {
			c.JSON(http.StatusBadGateway, ErrorResponse{
				Code:  "OLT_REMOVAL_FAILED",
				Error: err.Error(),
			})
			return
		}
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
