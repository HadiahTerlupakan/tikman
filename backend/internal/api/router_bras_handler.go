package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// defaultBRASRange is what the BRAS page shows when it asks for no range.
const defaultBRASRange = 24 * time.Hour

// BRAS returns a BRAS router's snapshot, samples, drops and hourly RADIUS
// counts. Unlike the other range reads, a missing from or to is filled in and
// a long range is clamped by the service rather than refused.
func (h *RouterHandler) BRAS(c *gin.Context) {
	id, ok := routerParam(c, "id")
	if !ok {
		return
	}
	from, to, ok := brasRange(c, time.Now())
	if !ok {
		return
	}
	view, err := h.routers.BRASView(id, from, to)
	if err != nil {
		respondRouterError(c, err)
		return
	}
	c.JSON(http.StatusOK, view)
}

func brasRange(c *gin.Context, now time.Time) (time.Time, time.Time, bool) {
	to, ok := optionalTime(c, "to", now)
	if !ok {
		return time.Time{}, time.Time{}, false
	}
	from, ok := optionalTime(c, "from", to.Add(-defaultBRASRange))
	if !ok {
		return time.Time{}, time.Time{}, false
	}
	if !to.After(from) {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "to must be after from", Code: "INVALID_RANGE"})
		return time.Time{}, time.Time{}, false
	}
	return from, to, true
}

func optionalTime(c *gin.Context, key string, fallback time.Time) (time.Time, bool) {
	raw := c.Query(key)
	if raw == "" {
		return fallback, true
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: key + " must be RFC3339", Code: "INVALID_RANGE"})
		return time.Time{}, false
	}
	return t, true
}
