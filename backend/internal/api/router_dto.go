package api

import (
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
)

// CreateRouterRequest adds a router. TLSFingerprint is the one the connection
// test returned; plain api has none.
type CreateRouterRequest struct {
	Name           string     `json:"name" binding:"required,min=2,max=255"`
	Host           string     `json:"host" binding:"required,max=255"`
	SiteID         *uuid.UUID `json:"site_id"`
	SNMPCommunity  string     `json:"snmp_community" binding:"required"`
	SNMPPort       int        `json:"snmp_port" binding:"omitempty,min=1,max=65535"`
	APIUsername    string     `json:"api_username" binding:"required,max=64"`
	APIPassword    string     `json:"api_password" binding:"required"`
	APIPort        int        `json:"api_port" binding:"omitempty,min=1,max=65535"`
	APIProtocol    string     `json:"api_protocol" binding:"omitempty,oneof=rest api-ssl api"`
	TLSFingerprint string     `json:"tls_fingerprint" binding:"omitempty,len=64,hexadecimal"`
	Role           *string    `json:"role" binding:"omitempty,oneof=border bras other"`
}

func (r CreateRouterRequest) toInput() services.RouterInput {
	in := services.RouterInput{
		Name: r.Name, Host: r.Host, SiteID: r.SiteID, SNMPCommunity: r.SNMPCommunity, SNMPPort: r.SNMPPort,
		APIUsername: r.APIUsername, APIPassword: r.APIPassword, APIPort: r.APIPort, APIProtocol: r.APIProtocol, TLSFingerprint: r.TLSFingerprint,
	}
	if r.Role != nil {
		in.Role = models.RouterRole(*r.Role)
	}
	return in
}

// UpdateRouterRequest changes a router. An omitted or empty secret keeps the
// stored one.
type UpdateRouterRequest struct {
	Name          *string    `json:"name" binding:"omitempty,min=2,max=255"`
	Host          *string    `json:"host" binding:"omitempty,max=255"`
	SiteID        *uuid.UUID `json:"site_id"`
	ClearSite     bool       `json:"clear_site"`
	SNMPCommunity *string    `json:"snmp_community"`
	SNMPPort      *int       `json:"snmp_port" binding:"omitempty,min=1,max=65535"`
	APIUsername   *string    `json:"api_username" binding:"omitempty,max=64"`
	APIPassword   *string    `json:"api_password"`
	APIPort       *int       `json:"api_port" binding:"omitempty,min=1,max=65535"`
	APIProtocol   *string    `json:"api_protocol" binding:"omitempty,oneof=rest api-ssl api"`
	Enabled       *bool      `json:"enabled"`
	FlowEnabled   *bool      `json:"flow_enabled"`
	FlowSampling  *int       `json:"flow_sampling" binding:"omitempty,min=1,max=10000"`
	RackPosition  *int       `json:"rack_position" binding:"omitempty,min=1,max=99"`
	// ClearRackPosition unsets the slot; RackPosition alone cannot say null.
	ClearRackPosition bool    `json:"clear_rack_position"`
	Role              *string `json:"role" binding:"omitempty,oneof=border bras other"`
	// ResetRole returns the role to discovery's guess.
	ResetRole bool `json:"reset_role"`
}

func (r UpdateRouterRequest) toUpdate() services.RouterUpdate {
	upd := services.RouterUpdate{
		Name: r.Name, Host: r.Host, SiteID: r.SiteID, ClearSite: r.ClearSite, SNMPCommunity: r.SNMPCommunity,
		SNMPPort: r.SNMPPort, APIUsername: r.APIUsername, APIPassword: r.APIPassword, APIPort: r.APIPort,
		APIProtocol: r.APIProtocol, Enabled: r.Enabled, FlowEnabled: r.FlowEnabled, FlowSampling: r.FlowSampling,
		RackPosition: r.RackPosition, ClearRackPosition: r.ClearRackPosition,
		ResetRole: r.ResetRole,
	}
	if r.Role != nil {
		role := models.RouterRole(*r.Role)
		upd.Role = &role
	}
	return upd
}

// TestRouterConnectionRequest is the add-router form before anything is
// stored. For the TLS protocols, without TLSFingerprint only SNMP runs and the
// router's certificate fingerprint comes back; with the fingerprint the operator
// confirmed, the API credentials are sent to a router presenting that
// certificate.
type TestRouterConnectionRequest struct {
	Host           string `json:"host" binding:"required,max=255"`
	SNMPCommunity  string `json:"snmp_community" binding:"required"`
	SNMPPort       int    `json:"snmp_port" binding:"omitempty,min=1,max=65535"`
	APIUsername    string `json:"api_username" binding:"required"`
	APIPassword    string `json:"api_password" binding:"required"`
	APIPort        int    `json:"api_port" binding:"omitempty,min=1,max=65535"`
	APIProtocol    string `json:"api_protocol" binding:"omitempty,oneof=rest api-ssl api"`
	TLSFingerprint string `json:"tls_fingerprint" binding:"omitempty,len=64,hexadecimal"`
}

func (r TestRouterConnectionRequest) toInput() services.ConnectionTestInput {
	return services.ConnectionTestInput{
		Host: r.Host, SNMPPort: r.SNMPPort, SNMPCommunity: r.SNMPCommunity,
		APIPort: r.APIPort, APIUsername: r.APIUsername, APIPassword: r.APIPassword,
		APIProtocol: r.APIProtocol, TLSFingerprint: r.TLSFingerprint,
	}
}

// TrustCertificateRequest accepts the certificate the router now presents.
type TrustCertificateRequest struct {
	Fingerprint string `json:"fingerprint" binding:"required,len=64,hexadecimal"`
}

// RouterResponse is a router without its secrets, with its BGP tally.
type RouterResponse struct {
	models.MikrotikRouter
	BGPEstablished int `json:"bgp_established"`
	BGPTotal       int `json:"bgp_total"`
	// SilentInterfaces is how many enabled interfaces receive nothing although
	// they carried traffic before: a POP gone behind a VLAN that stays up.
	SilentInterfaces int `json:"silent_interfaces"`
}
