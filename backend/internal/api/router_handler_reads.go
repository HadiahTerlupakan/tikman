package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// maxRouterRange is the longest range a graph or event list may ask for: the
// hourly rollup's year of retention, with room for the month it started in.
const maxRouterRange = 400 * 24 * time.Hour

// Interfaces lists a router's interfaces.
func (h *RouterHandler) Interfaces(c *gin.Context) {
	id, ok := h.existingRouter(c)
	if !ok {
		return
	}
	ifaces, err := h.queries.ListInterfaces(id)
	if err != nil {
		respondRouterError(c, err)
		return
	}
	c.JSON(http.StatusOK, ifaces)
}

// Sessions lists a router's BGP sessions.
func (h *RouterHandler) Sessions(c *gin.Context) {
	id, ok := h.existingRouter(c)
	if !ok {
		return
	}
	sessions, err := h.queries.ListSessions(id)
	if err != nil {
		respondRouterError(c, err)
		return
	}
	c.JSON(http.StatusOK, sessions)
}

// Events lists a router's BGP events in a range.
func (h *RouterHandler) Events(c *gin.Context) {
	id, ok := h.existingRouter(c)
	if !ok {
		return
	}
	from, to, ok := routerTimeRange(c)
	if !ok {
		return
	}
	events, err := h.queries.ListEvents(id, from, to)
	if err != nil {
		respondRouterError(c, err)
		return
	}
	c.JSON(http.StatusOK, events)
}

// Traffic returns one interface's traffic in a range.
func (h *RouterHandler) Traffic(c *gin.Context) {
	routerID, ok := routerParam(c, "id")
	if !ok {
		return
	}
	interfaceID, ok := routerParam(c, "ifId")
	if !ok {
		return
	}
	from, to, ok := routerTimeRange(c)
	if !ok {
		return
	}
	points, err := h.queries.InterfaceTraffic(routerID, interfaceID, from, to)
	if err != nil {
		respondRouterError(c, err)
		return
	}
	c.JSON(http.StatusOK, points)
}

func (h *RouterHandler) existingRouter(c *gin.Context) (uuid.UUID, bool) {
	id, ok := routerParam(c, "id")
	if !ok {
		return uuid.Nil, false
	}
	if _, err := h.routers.Get(id); err != nil {
		respondRouterError(c, err)
		return uuid.Nil, false
	}
	return id, true
}

func routerTimeRange(c *gin.Context) (time.Time, time.Time, bool) {
	return timeRangeWithin(c, maxRouterRange)
}

// timeRangeWithin reads from and to, refusing a range longer than maxSpan or
// one that starts in the future (it can hold no data and only wastes a scan).
func timeRangeWithin(c *gin.Context, maxSpan time.Duration) (time.Time, time.Time, bool) {
	from, errFrom := time.Parse(time.RFC3339, c.Query("from"))
	to, errTo := time.Parse(time.RFC3339, c.Query("to"))
	if errFrom != nil || errTo != nil || !to.After(from) || to.Sub(from) > maxSpan || from.After(time.Now()) {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: fmt.Sprintf("from and to must be RFC3339, with to after from, from not in the future, and at most %d days apart",
				int(maxSpan.Hours()/24)),
			Code: "INVALID_RANGE",
		})
		return time.Time{}, time.Time{}, false
	}
	return from, to, true
}
