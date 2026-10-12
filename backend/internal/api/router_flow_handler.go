package api

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/netflow"
	"github.com/tikman/olt-provisioning/internal/services"
)

const (
	// maxFlowRange bounds one flow read. The daily rollup is kept forever;
	// five years covers planning and still bounds what a request scans.
	maxFlowRange     = 5 * 366 * 24 * time.Hour
	defaultFlowLimit = 10
	maxFlowLimit     = 20
	// flowPOPOther selects the traffic of addresses in no POP's blocks.
	flowPOPOther = "other"
)

// RouterFlowHandler serves a router's traffic destinations.
type RouterFlowHandler struct {
	routers *services.RouterService
	flows   *services.FlowQueryService
}

// NewRouterFlowHandler creates a flow handler.
func NewRouterFlowHandler(routers *services.RouterService, flows *services.FlowQueryService) *RouterFlowHandler {
	return &RouterFlowHandler{routers: routers, flows: flows}
}

// Destinations is the stacked chart of the largest destinations.
func (h *RouterFlowHandler) Destinations(c *gin.Context) {
	id, filter, ok := h.flowRequest(c)
	if !ok {
		return
	}
	limit, ok := flowLimit(c)
	if !ok {
		return
	}
	out, err := h.flows.Destinations(id, filter, limit)
	respondFlow(c, out, err)
}

// Summary is the destination table.
func (h *RouterFlowHandler) Summary(c *gin.Context) {
	id, filter, ok := h.flowRequest(c)
	if !ok {
		return
	}
	out, err := h.flows.Summary(id, filter)
	respondFlow(c, out, err)
}

// POPs is the per-POP table; it is filtered by direction only, since its
// rows are what the POP filter is chosen from.
func (h *RouterFlowHandler) POPs(c *gin.Context) {
	id, filter, ok := h.flowRequest(c)
	if !ok {
		return
	}
	out, err := h.flows.POPs(id, services.FlowFilter{From: filter.From, To: filter.To, Direction: filter.Direction})
	respondFlow(c, out, err)
}

// Peering splits one destination ASN over the upstreams that carried it.
func (h *RouterFlowHandler) Peering(c *gin.Context) {
	id, filter, ok := h.flowRequest(c)
	if !ok {
		return
	}
	asn, err := strconv.ParseUint(c.Query("asn"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "asn must be an AS number", Code: "INVALID_FILTER"})
		return
	}
	out, err := h.flows.Peering(id, filter, int64(asn))
	respondFlow(c, out, err)
}

// TopIPs lists the largest destination addresses of the last 7 days.
func (h *RouterFlowHandler) TopIPs(c *gin.Context) {
	id, filter, ok := h.flowRequest(c)
	if !ok {
		return
	}
	out, err := h.flows.TopIPs(id, filter, time.Now())
	respondFlow(c, out, err)
}

// Status describes the ASN dataset in use.
func (h *RouterFlowHandler) Status(c *gin.Context) {
	if _, ok := h.existingRouter(c); !ok {
		return
	}
	out, err := h.flows.Status()
	respondFlow(c, out, err)
}

// flowRequest reads what every range read shares. Range and filters are
// checked before the router is looked up, so a malformed request is a 400
// whichever router it names.
func (h *RouterFlowHandler) flowRequest(c *gin.Context) (uuid.UUID, services.FlowFilter, bool) {
	from, to, ok := timeRangeWithin(c, maxFlowRange)
	if !ok {
		return uuid.Nil, services.FlowFilter{}, false
	}
	filter, err := parseFlowFilter(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error(), Code: "INVALID_FILTER"})
		return uuid.Nil, services.FlowFilter{}, false
	}
	filter.From, filter.To = from, to
	id, ok := h.existingRouter(c)
	return id, filter, ok
}

func (h *RouterFlowHandler) existingRouter(c *gin.Context) (uuid.UUID, bool) {
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

func parseFlowFilter(c *gin.Context) (services.FlowFilter, error) {
	var f services.FlowFilter
	if err := parsePOP(c.Query("pop"), &f); err != nil {
		return f, err
	}
	if upstream := c.Query("upstream"); upstream != "" {
		id, err := uuid.Parse(upstream)
		if err != nil {
			return f, errors.New("upstream must be an interface ID")
		}
		f.Upstream = &id
	}
	switch direction := c.Query("direction"); direction {
	case "", string(netflow.Download), string(netflow.Upload):
		f.Direction = direction
	default:
		return f, errors.New("direction must be download or upload")
	}
	return f, nil
}

func parsePOP(pop string, f *services.FlowFilter) error {
	switch pop {
	case "":
		return nil
	case flowPOPOther:
		f.OtherPOP = true
		return nil
	}
	id, err := uuid.Parse(pop)
	if err != nil {
		return errors.New(`pop must be an interface ID or "other"`)
	}
	f.POP = &id
	return nil
}

func flowLimit(c *gin.Context) (int, bool) {
	text := c.Query("limit")
	if text == "" {
		return defaultFlowLimit, true
	}
	limit, err := strconv.Atoi(text)
	if err != nil || limit < 1 || limit > maxFlowLimit {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "limit must be between 1 and 20", Code: "INVALID_FILTER"})
		return 0, false
	}
	return limit, true
}

func respondFlow(c *gin.Context, out interface{}, err error) {
	switch {
	case errors.Is(err, services.ErrTopIPsRange):
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error(), Code: "TOP_IPS_RANGE"})
	case err != nil:
		respondRouterError(c, err)
	default:
		c.JSON(http.StatusOK, out)
	}
}
