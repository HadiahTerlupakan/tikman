package api

import (
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
)

// SaveWireguardServerRequest configures the VPS side of the tunnel. It is used
// both for the one-time setup and later edits.
type SaveWireguardServerRequest struct {
	EndpointHost string `json:"endpoint_host" binding:"required,min=1,max=255"`
	ListenPort   int    `json:"listen_port" binding:"required,min=1,max=65535"`
}

// CreateWireguardPeerRequest registers a site as a WireGuard peer.
type CreateWireguardPeerRequest struct {
	SiteID        string   `json:"site_id" binding:"required,uuid"`
	Name          string   `json:"name" binding:"required,min=2,max=255"`
	AllowedIPs    []string `json:"allowed_ips" binding:"required,min=1"`
	TunnelAddress string   `json:"tunnel_address"`
}

// UpdateWireguardPeerRequest edits an existing peer. Fields left nil or empty
// are unchanged by the service.
type UpdateWireguardPeerRequest struct {
	Name       *string  `json:"name" binding:"omitempty,min=2,max=255"`
	AllowedIPs []string `json:"allowed_ips"`
	Enabled    *bool    `json:"enabled"`
}

// WireguardServerResponse deliberately omits the private key. The only way out
// for key material is the peer config endpoint.
type WireguardServerResponse struct {
	ID            uuid.UUID `json:"id"`
	InterfaceName string    `json:"interface_name"`
	ListenPort    int       `json:"listen_port"`
	PublicKey     string    `json:"public_key"`
	EndpointHost  string    `json:"endpoint_host"`
	TunnelSubnet  string    `json:"tunnel_subnet"`
	Address       string    `json:"address"`
}

// WireguardPeerResponse is the peer as exposed over HTTP. It deliberately
// omits PrivateKey and PresharedKey - the model carries no json tag guarding
// them, so mapping to this DTO is the only thing keeping key material out of
// list/create/update responses.
type WireguardPeerResponse struct {
	ID                  uuid.UUID  `json:"id"`
	SiteID              uuid.UUID  `json:"site_id"`
	Name                string     `json:"name"`
	TunnelAddress       string     `json:"tunnel_address"`
	AllowedIPs          []string   `json:"allowed_ips"`
	PersistentKeepalive int        `json:"persistent_keepalive"`
	Enabled             bool       `json:"enabled"`
	Connected           bool       `json:"connected"`
	LastHandshakeAt     *time.Time `json:"last_handshake_at"`
	RxBytes             int64      `json:"rx_bytes"`
	TxBytes             int64      `json:"tx_bytes"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

// WireguardPeerConfigResponse carries a rendered peer configuration. This is
// the only response in the package that contains key material.
// TestReachabilityRequest asks whether one address can be reached through a
// site's tunnel. It is the operator's way of telling a mistyped subnet apart
// from a device that is simply not answering.
type TestReachabilityRequest struct {
	Address string `json:"address" binding:"required,max=45"`
}

type ReachabilityResponse struct {
	Reachable bool   `json:"reachable"`
	Routed    bool   `json:"routed"`
	Message   string `json:"message"`
}

type WireguardPeerConfigResponse struct {
	Format string `json:"format"`
	Config string `json:"config"`
}

// SuggestedSubnetsResponse lists candidate allowed-IP subnets derived from a
// site's registered OLTs.
type SuggestedSubnetsResponse struct {
	Subnets []string `json:"subnets"`
}

// ToWireguardServerResponse maps the server model to its HTTP response,
// stripping the private key.
func ToWireguardServerResponse(server *models.WireGuardServer) WireguardServerResponse {
	return WireguardServerResponse{
		ID:            server.ID,
		InterfaceName: server.InterfaceName,
		ListenPort:    server.ListenPort,
		PublicKey:     server.PublicKey,
		EndpointHost:  server.EndpointHost,
		TunnelSubnet:  server.TunnelSubnet,
		Address:       server.Address,
	}
}

// ToWireguardPeerResponse maps the peer model to its HTTP response, stripping
// key material and computing the connection state as of now.
func ToWireguardPeerResponse(peer *models.WireGuardPeer, now time.Time) WireguardPeerResponse {
	allowedIPs, err := peer.AllowedIPsList()
	if err != nil {
		allowedIPs = nil
	}
	return WireguardPeerResponse{
		ID:                  peer.ID,
		SiteID:              peer.SiteID,
		Name:                peer.Name,
		TunnelAddress:       peer.TunnelAddress,
		AllowedIPs:          allowedIPs,
		PersistentKeepalive: peer.PersistentKeepalive,
		Enabled:             peer.Enabled,
		Connected:           peer.Enabled && services.PeerConnected(peer.LastHandshakeAt, now),
		LastHandshakeAt:     peer.LastHandshakeAt,
		RxBytes:             peer.RxBytes,
		TxBytes:             peer.TxBytes,
		CreatedAt:           peer.CreatedAt,
		UpdatedAt:           peer.UpdatedAt,
	}
}
