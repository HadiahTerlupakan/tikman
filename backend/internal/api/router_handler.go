package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/middleware"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"gorm.io/gorm"
)

const routerAuditResource = "mikrotik_router"

// RouterHandler serves the MikroTik router pages.
type RouterHandler struct {
	routers *services.RouterService
	jobs    *services.RouterPollJobService
	queries *services.RouterQueryService
	audit   *services.AuditService
}

// NewRouterHandler creates a router handler.
func NewRouterHandler(routers *services.RouterService, jobs *services.RouterPollJobService,
	queries *services.RouterQueryService, audit *services.AuditService) *RouterHandler {
	return &RouterHandler{routers: routers, jobs: jobs, queries: queries, audit: audit}
}

// List returns every router with its BGP tally.
func (h *RouterHandler) List(c *gin.Context) {
	routers, err := h.routers.List()
	if err != nil {
		respondRouterError(c, err)
		return
	}
	summary, err := h.queries.BGPSummary()
	if err != nil {
		respondRouterError(c, err)
		return
	}
	silent, err := h.queries.SilentInterfaces()
	if err != nil {
		respondRouterError(c, err)
		return
	}
	out := make([]RouterResponse, len(routers))
	for i := range routers {
		tally := summary[routers[i].ID]
		out[i] = RouterResponse{MikrotikRouter: routers[i], BGPEstablished: tally.Established, BGPTotal: tally.Total,
			SilentInterfaces: silent[routers[i].ID]}
	}
	c.JSON(http.StatusOK, out)
}

// Create adds a router.
func (h *RouterHandler) Create(c *gin.Context) {
	var req CreateRouterRequest
	if !bindRouterJSON(c, &req) {
		return
	}
	router, err := h.routers.Create(req.toInput())
	if err != nil {
		respondRouterError(c, err)
		return
	}
	h.record(c, "create", router.ID, map[string]interface{}{"name": router.Name, "host": router.Host,
		"api_protocol": router.APIProtocol})
	c.JSON(http.StatusCreated, RouterResponse{MikrotikRouter: *router})
}

// Update changes a router.
func (h *RouterHandler) Update(c *gin.Context) {
	id, ok := routerParam(c, "id")
	if !ok {
		return
	}
	var req UpdateRouterRequest
	if !bindRouterJSON(c, &req) {
		return
	}
	router, err := h.routers.Update(id, req.toUpdate())
	if err != nil {
		respondRouterError(c, err)
		return
	}
	// Which fields changed, never the secret values themselves.
	h.record(c, "update", id, map[string]interface{}{"name": router.Name, "host": router.Host, "api_protocol": router.APIProtocol,
		"flow_enabled": router.FlowEnabled, "flow_sampling": router.FlowSampling,
		"community_changed": req.SNMPCommunity != nil && *req.SNMPCommunity != "",
		"password_changed":  req.APIPassword != nil && *req.APIPassword != ""})
	c.JSON(http.StatusOK, RouterResponse{MikrotikRouter: *router})
}

// Delete removes a router.
func (h *RouterHandler) Delete(c *gin.Context) {
	id, ok := routerParam(c, "id")
	if !ok {
		return
	}
	if err := h.routers.Delete(id); err != nil {
		respondRouterError(c, err)
		return
	}
	h.record(c, "delete", id, nil)
	c.Status(http.StatusNoContent)
}

// TestConnection probes a router in two steps: SNMP plus the certificate
// fingerprint, then the REST API once the operator has confirmed that
// fingerprint.
func (h *RouterHandler) TestConnection(c *gin.Context) {
	var req TestRouterConnectionRequest
	if !bindRouterJSON(c, &req) {
		return
	}
	c.JSON(http.StatusOK, h.routers.TestConnection(req.toInput()))
}

// Refresh makes discovery due now. It answers before discovery runs.
func (h *RouterHandler) Refresh(c *gin.Context) {
	id, ok := routerParam(c, "id")
	if !ok {
		return
	}
	if _, err := h.routers.Get(id); err != nil {
		respondRouterError(c, err)
		return
	}
	if err := h.jobs.RunNow(id, models.RouterPollKindDiscovery); err != nil {
		respondRouterError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"status": "scheduled"})
}

// TrustCertificate pins the certificate the router now presents and resumes
// REST polling.
func (h *RouterHandler) TrustCertificate(c *gin.Context) {
	id, ok := routerParam(c, "id")
	if !ok {
		return
	}
	var req TrustCertificateRequest
	if !bindRouterJSON(c, &req) {
		return
	}
	router, err := h.routers.TrustCertificate(id, req.Fingerprint)
	if err != nil {
		respondRouterError(c, err)
		return
	}
	// The pin has changed by now, so it is audited even if scheduling fails.
	h.record(c, "trust_certificate", id, map[string]interface{}{"fingerprint": router.TLSFingerprint})
	if err := h.jobs.RunNow(id, models.RouterPollKindBGP, models.RouterPollKindDiscovery); err != nil {
		respondRouterError(c, err)
		return
	}
	c.JSON(http.StatusOK, RouterResponse{MikrotikRouter: *router})
}

func (h *RouterHandler) record(c *gin.Context, action string, id uuid.UUID, values map[string]interface{}) {
	if h.audit == nil {
		return
	}
	actorID, _ := middleware.GetUserID(c)
	_ = h.audit.Log(actorID, action, routerAuditResource, id, nil, values, c.ClientIP(), c.Request.UserAgent())
}

func routerParam(c *gin.Context, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid " + name, Code: "INVALID_ID"})
		return uuid.Nil, false
	}
	return id, true
}

func bindRouterJSON(c *gin.Context, out interface{}) bool {
	if err := c.ShouldBindJSON(out); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid request body", Code: "INVALID_REQUEST", Details: err.Error()})
		return false
	}
	return true
}

func respondRouterError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, services.ErrValidation):
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error(), Code: "INVALID_ROUTER"})
	case errors.Is(err, gorm.ErrRecordNotFound):
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "Router not found", Code: "NOT_FOUND"})
	case errors.Is(err, services.ErrFingerprintMismatch):
		c.JSON(http.StatusConflict, ErrorResponse{Error: err.Error(), Code: "FINGERPRINT_MISMATCH"})
	default:
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Router request failed", Code: "ROUTER_FAILED"})
	}
}
