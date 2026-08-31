package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"gorm.io/gorm"
)

// DiscoverNow brings an OLT's next inventory pass forward.
//
// Discovery runs every six hours, which is often enough for ONUs that are added
// a few times a day and cheap enough for a chassis where the pass itself takes
// minutes. This is the door out of that trade-off: a technician who has just
// installed an ONU asks for the pass rather than waiting for it.
//
// It schedules and returns. Running the discovery here would hold the request
// open for as long as the walk takes — over six minutes on the largest chassis
// here — and would reach the OLT's SNMP agent outside the lease that keeps two
// readers off it.
func (h *OLTHandler) DiscoverNow(c *gin.Context) {
	oltID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:  "INVALID_ID",
			Error: "Invalid OLT ID format",
		})
		return
	}

	err = h.pollJobService.RunNow(oltID, models.PollKindDiscovery)
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		c.JSON(http.StatusNotFound, ErrorResponse{
			Code:  "OLT_NOT_FOUND",
			Error: "No discovery job for this OLT",
		})
	case errors.Is(err, services.ErrJobRunning):
		c.JSON(http.StatusConflict, ErrorResponse{
			Code:  "DISCOVERY_RUNNING",
			Error: "Discovery is already running for this OLT",
		})
	case err != nil:
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:  "SCHEDULE_FAILED",
			Error: err.Error(),
		})
	default:
		c.JSON(http.StatusAccepted, gin.H{"status": "scheduled"})
	}
}
