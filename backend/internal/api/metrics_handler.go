package api

import (
	"log"
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

// GetPollingStats handles GET /api/v1/polling/stats - returns current polling statistics
func (h *MetricsHandler) GetPollingStats(c *gin.Context) {
	stats := h.metricsService.GetPollingStats()
	c.JSON(http.StatusOK, stats)
}

// GetOltsStats handles GET /api/v1/olts/:id/stats - returns polling stats for a specific OLT
func (h *MetricsHandler) GetOltsStats(c *gin.Context) {
	oltID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:  "INVALID_ID",
			Error: "Invalid OLT ID format",
		})
		return
	}

	stats := h.metricsService.GetOLTPollingStats(oltID)
	c.JSON(http.StatusOK, stats)
}

// GetRealtime queries the OLT live via SNMP for the freshest metrics,
// including the octet-rate gauges — so the UI's 3-second polling reflects
// actual current traffic rather than the worker's last stored row.
func (h *MetricsHandler) GetRealtime(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:  "INVALID_ID",
			Error: "Invalid ONT ID format",
		})
		return
	}

	metrics, err := h.metricsService.GetRealtimeMetrics(id)
	if err != nil {
		// Fall back to the latest stored row when live polling fails
		// (e.g. slot not yet discovered).
		if stored, storedErr := h.metricsService.GetLatestMetrics(id); storedErr == nil {
			c.JSON(http.StatusOK, ToONTMetricsResponse(stored))
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:  "QUERY_FAILED",
			Error: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, ToONTMetricsResponse(metrics))
}

// GetTrafficTimeSeries returns bucketed traffic rates for a period. Rates are
// the worker-collected octet-rate gauges averaged per bucket.
func (h *MetricsHandler) GetTrafficTimeSeries(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:  "INVALID_ID",
			Error: "Invalid ONT ID format",
		})
		return
	}

	period := c.DefaultQuery("period", "3h")

	startStr, endStr := c.Query("start"), c.Query("end")
	var timeSeries []services.ONTMetricsRow
	if startStr != "" && endStr != "" {
		startTime, errStart := time.Parse(time.RFC3339, startStr)
		endTime, errEnd := time.Parse(time.RFC3339, endStr)
		if errStart != nil || errEnd != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Code:  "INVALID_RANGE",
				Error: "start and end must be RFC3339 timestamps",
			})
			return
		}
		if !startTime.Before(endTime) {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Code:  "INVALID_RANGE",
				Error: "start must be before end",
			})
			return
		}
		timeSeries, err = h.metricsService.GetONTTrafficTimeSeriesRange(id, startTime, endTime)
	} else {
		timeSeries, err = h.metricsService.GetONTTrafficTimeSeries(id, period)
	}
	if err != nil {
		log.Printf("[ERROR] GetTrafficTimeSeries failed for ONT %s: %v", id, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:  "QUERY_FAILED",
			Error: err.Error(),
		})
		return
	}

	response := make([]ONTTrafficTimeSeriesResponse, len(timeSeries))
	for i, point := range timeSeries {
		var rxMbps, txMbps float64
		if point.RxRateMbps != nil {
			rxMbps = *point.RxRateMbps
		}
		if point.TxRateMbps != nil {
			txMbps = *point.TxRateMbps
		}
		response[i] = ONTTrafficTimeSeriesResponse{
			Time:    point.Time,
			RxBytes: point.RxBytes,
			TxBytes: point.TxBytes,
			RxMbps:  rxMbps,
			TxMbps:  txMbps,
		}
	}

	c.JSON(http.StatusOK, response)
}
