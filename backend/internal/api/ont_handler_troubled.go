package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Bounds on what a request may ask the ranking for. The window is capped by the
// service as well; the page size is capped here because the ranking's whole
// point is the top of the list.
const (
	defaultTroubledHours = 24
	maxTroubledHours     = 24 * 7
	defaultTroubledLimit = 50
	maxTroubledLimit     = 200
)

// troubledQuery reads the window and page size out of a request, clamped.
func troubledQuery(c *gin.Context) (time.Duration, int) {
	hours, err := strconv.Atoi(c.DefaultQuery("hours", strconv.Itoa(defaultTroubledHours)))
	if err != nil || hours < 1 {
		hours = defaultTroubledHours
	} else if hours > maxTroubledHours {
		hours = maxTroubledHours
	}

	limit, err := strconv.Atoi(c.DefaultQuery("limit", strconv.Itoa(defaultTroubledLimit)))
	if err != nil || limit < 1 {
		limit = defaultTroubledLimit
	} else if limit > maxTroubledLimit {
		limit = maxTroubledLimit
	}

	return time.Duration(hours) * time.Hour, limit
}

// ListTroubled ranks the subscribers in most trouble.
//
// The ONT list answers "is this subscriber up", which an ONU that drops and
// returns every few seconds passes every time it is asked. This answers the
// question nobody could ask before: which subscribers are failing repeatedly,
// whatever they happen to read at this instant.
func (h *ONTHandler) ListTroubled(c *gin.Context) {
	window, limit := troubledQuery(c)

	var oltID *uuid.UUID
	if raw := c.Query("olt_id"); raw != "" {
		parsed, err := uuid.Parse(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Code:  "INVALID_OLT_ID",
				Error: "Invalid OLT ID format",
			})
			return
		}
		oltID = &parsed
	}

	troubled, summary, err := h.ontService.TroubledONTs(window, limit, oltID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:  "TROUBLED_QUERY_FAILED",
			Error: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":    troubled,
		"summary": summary,
		"hours":   int(window.Hours()),
	})
}
