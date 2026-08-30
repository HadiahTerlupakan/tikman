package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tikman/olt-provisioning/internal/services"
)

// DashboardHandler serves the overview page's figures.
type DashboardHandler struct {
	dashboardService *services.DashboardService
}

// NewDashboardHandler creates a new dashboard handler.
func NewDashboardHandler(dashboardService *services.DashboardService) *DashboardHandler {
	return &DashboardHandler{dashboardService: dashboardService}
}

// GetStats returns ONT counts by state, the per-OLT breakdown and the weakest
// optical readings, counted in the database rather than in the browser.
func (h *DashboardHandler) GetStats(c *gin.Context) {
	stats, err := h.dashboardService.Stats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:  "STATS_FAILED",
			Error: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, stats)
}
