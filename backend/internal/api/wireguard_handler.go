package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/middleware"
	"github.com/tikman/olt-provisioning/internal/services"
)

const auditResourceWireguardPeer = "wireguard_peer"

// WireGuardHandler exposes the WireGuard VPN server and its peers over HTTP.
type WireGuardHandler struct {
	service      *services.WireGuardService
	auditService *services.AuditService
}

// NewWireGuardHandler wires the handler to the WireGuard service and the
// (optionally nil, for tests) audit service.
func NewWireGuardHandler(service *services.WireGuardService, auditService *services.AuditService) *WireGuardHandler {
	return &WireGuardHandler{service: service, auditService: auditService}
}

// GetServer returns the current server configuration, or 404 with code
// NOT_CONFIGURED before the one-time setup has run.
func (h *WireGuardHandler) GetServer(c *gin.Context) {
	server, err := h.service.GetServer()
	if err != nil {
		if errors.Is(err, services.ErrServerNotConfigured) {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error: "WireGuard server is not configured yet",
				Code:  "NOT_CONFIGURED",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to load server", Code: "LOAD_FAILED"})
		return
	}
	c.JSON(http.StatusOK, ToWireguardServerResponse(server))
}

// SaveServer performs the one-time setup and later edits with the same call, so
// the UI needs a single form rather than a create/update distinction.
func (h *WireGuardHandler) SaveServer(c *gin.Context) {
	var req SaveWireguardServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid request body", Code: "INVALID_REQUEST", Details: err.Error()})
		return
	}

	if _, err := h.service.EnsureServer(req.EndpointHost); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to initialise server", Code: "SETUP_FAILED", Details: err.Error()})
		return
	}
	server, err := h.service.UpdateServer(req.EndpointHost, req.ListenPort)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to save server", Code: "SAVE_FAILED", Details: err.Error()})
		return
	}

	h.audit(c, "update", "wireguard_server", server.ID, map[string]interface{}{
		"endpoint_host": server.EndpointHost,
		"listen_port":   server.ListenPort,
	})
	c.JSON(http.StatusOK, ToWireguardServerResponse(server))
}

// ListPeers returns every peer with its live connection state.
func (h *WireGuardHandler) ListPeers(c *gin.Context) {
	peers, err := h.service.ListPeers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to list peers", Code: "LIST_FAILED"})
		return
	}

	now := time.Now()
	responses := make([]WireguardPeerResponse, len(peers))
	for i := range peers {
		responses[i] = ToWireguardPeerResponse(&peers[i], now)
	}
	c.JSON(http.StatusOK, responses)
}

// CreatePeer registers a site as a WireGuard peer. The name is supplied by the
// frontend from the site record; the response carries no site name of its own.
func (h *WireGuardHandler) CreatePeer(c *gin.Context) {
	var req CreateWireguardPeerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid request body", Code: "INVALID_REQUEST", Details: err.Error()})
		return
	}
	siteID, err := uuid.Parse(req.SiteID)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid site ID", Code: "INVALID_ID"})
		return
	}

	peer, err := h.service.CreatePeer(siteID, req.Name, req.AllowedIPs, req.TunnelAddress)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Failed to create peer", Code: "CREATE_FAILED", Details: err.Error()})
		return
	}

	h.audit(c, "create", auditResourceWireguardPeer, peer.ID, map[string]interface{}{
		"site_id":        peer.SiteID.String(),
		"tunnel_address": peer.TunnelAddress,
	})
	c.JSON(http.StatusCreated, ToWireguardPeerResponse(peer, time.Now()))
}

// UpdatePeer edits an existing peer's name, allowed IPs, or enabled state.
func (h *WireGuardHandler) UpdatePeer(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid peer ID", Code: "INVALID_ID"})
		return
	}

	var req UpdateWireguardPeerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid request body", Code: "INVALID_REQUEST", Details: err.Error()})
		return
	}

	peer, err := h.service.UpdatePeer(id, req.Name, req.AllowedIPs, req.Enabled)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Failed to update peer", Code: "UPDATE_FAILED", Details: err.Error()})
		return
	}

	h.audit(c, "update", auditResourceWireguardPeer, peer.ID, map[string]interface{}{
		"enabled": peer.Enabled,
	})
	c.JSON(http.StatusOK, ToWireguardPeerResponse(peer, time.Now()))
}

// DeletePeer removes a peer.
func (h *WireGuardHandler) DeletePeer(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid peer ID", Code: "INVALID_ID"})
		return
	}

	if err := h.service.DeletePeer(id); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to delete peer", Code: "DELETE_FAILED"})
		return
	}

	h.audit(c, "delete", auditResourceWireguardPeer, id, nil)
	c.JSON(http.StatusNoContent, nil)
}

// GetPeerConfig renders a peer's configuration in the requested format
// (defaulting to wg-quick). This is the only endpoint that returns key
// material, so every call is audited.
func (h *WireGuardHandler) GetPeerConfig(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid peer ID", Code: "INVALID_ID"})
		return
	}

	format := c.DefaultQuery("format", services.ConfigFormatWGQuick)
	config, err := h.service.PeerConfig(id, format)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Failed to render config", Code: "CONFIG_FAILED", Details: err.Error()})
		return
	}

	h.audit(c, "download_config", auditResourceWireguardPeer, id, map[string]interface{}{"format": format})
	c.JSON(http.StatusOK, WireguardPeerConfigResponse{Format: format, Config: config})
}

// SuggestSubnets proposes allowed-IP subnets for a site based on its
// registered OLTs.
func (h *WireGuardHandler) SuggestSubnets(c *gin.Context) {
	siteID, err := uuid.Parse(c.Param("site_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid site ID", Code: "INVALID_ID"})
		return
	}

	subnets, err := h.service.SuggestAllowedIPsForSite(siteID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to read OLT addresses", Code: "SUGGEST_FAILED"})
		return
	}
	c.JSON(http.StatusOK, SuggestedSubnetsResponse{Subnets: subnets})
}

func (h *WireGuardHandler) audit(c *gin.Context, action, resource string, id uuid.UUID, newValue map[string]interface{}) {
	if h.auditService == nil {
		return
	}
	actorID, _ := middleware.GetUserID(c)
	_ = h.auditService.Log(actorID, action, resource, id, nil, newValue, c.ClientIP(), c.Request.UserAgent())
}
