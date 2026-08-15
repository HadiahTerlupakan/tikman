package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/services"
)

// MetricsHandler handles ONT metrics HTTP requests
type MetricsHandler struct {
	metricsService *services.MetricsService
}

// NewMetricsHandler creates a new metrics handler
func NewMetricsHandler(metricsService *services.MetricsService) *MetricsHandler {
	return &MetricsHandler{
		metricsService: metricsService,
	}
}

// GetLatest handles GET /api/v1/onts/:id/metrics
func (h *MetricsHandler) GetLatest(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:  "INVALID_ID",
			Error: "Invalid ONT ID format",
		})
		return
	}

	metrics, err := h.metricsService.GetLatestMetrics(id)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Code:  "NOT_FOUND",
			Error: "No metrics found for this ONT",
		})
		return
	}

	c.JSON(http.StatusOK, ToONTMetricsResponse(metrics))
}

// GetHistory handles GET /api/v1/onts/:id/metrics/history
func (h *MetricsHandler) GetHistory(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:  "INVALID_ID",
			Error: "Invalid ONT ID format",
		})
		return
	}

	// Parse time range (default: last 24 hours)
	endTime := time.Now()
	startTime := endTime.Add(-24 * time.Hour)

	if start := c.Query("start"); start != "" {
		if parsed, err := time.Parse(time.RFC3339, start); err == nil {
			startTime = parsed
		}
	}

	if end := c.Query("end"); end != "" {
		if parsed, err := time.Parse(time.RFC3339, end); err == nil {
			endTime = parsed
		}
	}

	metrics, err := h.metricsService.GetMetricsHistory(id, startTime, endTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:  "QUERY_FAILED",
			Error: err.Error(),
		})
		return
	}

	responses := make([]ONTMetricsResponse, len(metrics))
	for i, m := range metrics {
		responses[i] = ToONTMetricsResponse(&m)
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  responses,
		"start": startTime,
		"end":   endTime,
		"count": len(responses),
	})
}
