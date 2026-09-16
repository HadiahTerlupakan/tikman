package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/middleware"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
)

// CSPerformanceHandler serves the CS response-time report: the figures, and the
// waits they are made of.
type CSPerformanceHandler struct {
	performance *services.CSPerformanceService
}

// NewCSPerformanceHandler constructs a CSPerformanceHandler.
func NewCSPerformanceHandler(performance *services.CSPerformanceService) *CSPerformanceHandler {
	return &CSPerformanceHandler{performance: performance}
}

// Summary answers the report for the dates asked for. An admin sees every CS's
// row; anyone else sees the team's figures and their own row.
func (h *CSPerformanceHandler) Summary(c *gin.Context) {
	period, ok := reportRange(c)
	if !ok {
		return
	}
	summary, err := h.performance.Summary(period, reportViewer(c), time.Now())
	if err != nil {
		mapCSError(c, err, "CS_PERFORMANCE_FAILED")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": summary})
}

// Waits answers one page of the waits behind the report, so a CS can check the
// figure they were given and open the thread it came from.
func (h *CSPerformanceHandler) Waits(c *gin.Context) {
	period, ok := reportRange(c)
	if !ok {
		return
	}
	endedBy, ok := waitListOwner(c)
	if !ok {
		return
	}
	limit, offset := paginationParams(c)
	list, err := h.performance.Waits(period, endedBy, limit, offset)
	if err != nil {
		mapCSError(c, err, "CS_PERFORMANCE_WAITS_FAILED")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": list.Items, "total": list.Total})
}

// reportRange reads the two dates a report covers, answering the request itself
// when they cannot be read.
func reportRange(c *gin.Context) (services.ReportRange, bool) {
	period, err := services.ReportRangeFromDates(c.Query("from"), c.Query("to"))
	if err != nil {
		mapCSError(c, err, "INVALID_REPORT_RANGE")
		return services.ReportRange{}, false
	}
	return period, true
}

// reportViewer is nil for an admin, who sees every CS, and the caller's own id
// for anyone else.
func reportViewer(c *gin.Context) *uuid.UUID {
	if role, _ := middleware.GetUserRole(c); role == models.UserRoleAdmin {
		return nil
	}
	id, _ := middleware.GetUserID(c)
	return &id
}

// waitListOwner decides whose waits a list may show. An admin may name anyone,
// or nobody and see them all; everyone else is pinned to themselves, whatever
// the query says.
func waitListOwner(c *gin.Context) (*uuid.UUID, bool) {
	if viewer := reportViewer(c); viewer != nil {
		return viewer, true
	}
	named := c.Query("user_id")
	if named == "" {
		return nil, true
	}
	id, err := uuid.Parse(named)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid ID format", Code: "INVALID_USER_ID"})
		return nil, false
	}
	return &id, true
}
