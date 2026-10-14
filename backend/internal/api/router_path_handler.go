package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
)

// maxASNamesQuery bounds one names request: a path diagram of the busiest
// destinations has a few hundred ASes at most.
const maxASNamesQuery = 500

// routeLookupper asks a router, live, for the BGP candidates of a target.
type routeLookupper interface {
	LookupRoutes(router *models.MikrotikRouter, target string, now time.Time) (services.RouteLookup, error)
}

// RouterPathHandler serves the AS path views: a live lookup and the names
// that label the ASes on a path.
type RouterPathHandler struct {
	routers *services.RouterService
	lookup  routeLookupper
	flows   *services.FlowQueryService
}

// NewRouterPathHandler creates a path handler.
func NewRouterPathHandler(routers *services.RouterService, lookup routeLookupper, flows *services.FlowQueryService) *RouterPathHandler {
	return &RouterPathHandler{routers: routers, lookup: lookup, flows: flows}
}

// Lookup asks the router now for the candidates of ?target, an IPv4 address
// or prefix.
func (h *RouterPathHandler) Lookup(c *gin.Context) {
	id, ok := routerParam(c, "id")
	if !ok {
		return
	}
	router, err := h.routers.Get(id)
	if err != nil {
		respondRouterError(c, err)
		return
	}
	out, err := h.lookup.LookupRoutes(router, c.Query("target"), time.Now())
	switch {
	case errors.Is(err, services.ErrLookupTarget):
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error(), Code: "LOOKUP_TARGET"})
	case errors.Is(err, services.ErrLookupBusy):
		c.JSON(http.StatusConflict, ErrorResponse{Error: err.Error(), Code: "LOOKUP_BUSY"})
	case err != nil:
		respondRouterError(c, err)
	default:
		c.JSON(http.StatusOK, out)
	}
}

// ASNames answers the names of the ASes in ?asn, a comma-separated list;
// an AS the dataset has no name for is left out.
func (h *RouterPathHandler) ASNames(c *gin.Context) {
	if _, ok := routerParam(c, "id"); !ok {
		return
	}
	asns, ok := parseASNList(c.Query("asn"))
	if !ok {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "asn must be 1 to " + strconv.Itoa(maxASNamesQuery) + " AS numbers, comma-separated", Code: "INVALID_ASN"})
		return
	}
	names, err := h.flows.ASNNames(asns)
	if err != nil {
		respondRouterError(c, err)
		return
	}
	c.JSON(http.StatusOK, names)
}

func parseASNList(raw string) ([]int64, bool) {
	parts := strings.Split(raw, ",")
	if raw == "" || len(parts) > maxASNamesQuery {
		return nil, false
	}
	asns := make([]int64, 0, len(parts))
	for _, p := range parts {
		asn, err := strconv.ParseInt(strings.TrimSpace(p), 10, 64)
		if err != nil || asn < 0 {
			return nil, false
		}
		asns = append(asns, asn)
	}
	return asns, true
}
