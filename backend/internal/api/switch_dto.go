package api

import (
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
)

// CreateSwitchRequest adds a switch. The vendor and protocol are checked here
// for shape and by the service for what a driver exists for.
type CreateSwitchRequest struct {
	Name          string     `json:"name" binding:"required,min=2,max=255"`
	Host          string     `json:"host" binding:"required,max=255"`
	SiteID        *uuid.UUID `json:"site_id"`
	Vendor        string     `json:"vendor" binding:"required,oneof=crs huawei_ce fiberhome"`
	SNMPCommunity string     `json:"snmp_community" binding:"required"`
	SNMPPort      int        `json:"snmp_port" binding:"omitempty,min=1,max=65535"`
	Protocol      string     `json:"protocol" binding:"required,oneof=telnet ssh rest"`
	Port          int        `json:"port" binding:"omitempty,min=1,max=65535"`
	Username      string     `json:"username" binding:"required,max=64"`
	Password      string     `json:"password" binding:"required"`
	// HostKeyFingerprint is the SSH key confirmed in the connection test.
	HostKeyFingerprint string `json:"host_key_fingerprint" binding:"max=64"`
}

func (r CreateSwitchRequest) toInput() services.SwitchInput {
	return services.SwitchInput{
		Name: r.Name, Host: r.Host, SiteID: r.SiteID, Vendor: models.SwitchVendor(r.Vendor),
		SNMPCommunity: r.SNMPCommunity, SNMPPort: r.SNMPPort, Protocol: models.SwitchProtocol(r.Protocol),
		Port: r.Port, Username: r.Username, Password: r.Password, HostKeyFingerprint: r.HostKeyFingerprint,
	}
}

// TrustSwitchHostKeyRequest carries the fingerprint the admin was shown, so
// a key the poller recorded after the page loaded is never the one pinned.
type TrustSwitchHostKeyRequest struct {
	Fingerprint string `json:"fingerprint" binding:"required,max=64"`
}

// UpdateSwitchRequest changes a switch. An omitted or empty secret keeps the
// stored one.
type UpdateSwitchRequest struct {
	Name          *string    `json:"name" binding:"omitempty,min=2,max=255"`
	Host          *string    `json:"host" binding:"omitempty,max=255"`
	SiteID        *uuid.UUID `json:"site_id"`
	ClearSite     bool       `json:"clear_site"`
	SNMPCommunity *string    `json:"snmp_community"`
	SNMPPort      *int       `json:"snmp_port" binding:"omitempty,min=1,max=65535"`
	Protocol      *string    `json:"protocol" binding:"omitempty,oneof=telnet ssh rest"`
	Port          *int       `json:"port" binding:"omitempty,min=1,max=65535"`
	Username      *string    `json:"username" binding:"omitempty,max=64"`
	Password      *string    `json:"password"`
	Enabled       *bool      `json:"enabled"`
	RackPosition  *int       `json:"rack_position" binding:"omitempty,min=1,max=99"`
	// ClearRackPosition unsets the slot; RackPosition alone cannot say null.
	ClearRackPosition bool `json:"clear_rack_position"`
}

func (r UpdateSwitchRequest) toUpdate() services.SwitchUpdate {
	upd := services.SwitchUpdate{
		Name: r.Name, Host: r.Host, SiteID: r.SiteID, ClearSite: r.ClearSite, SNMPCommunity: r.SNMPCommunity,
		SNMPPort: r.SNMPPort, Port: r.Port, Username: r.Username, Password: r.Password, Enabled: r.Enabled,
		RackPosition: r.RackPosition, ClearRackPosition: r.ClearRackPosition,
	}
	if r.Protocol != nil {
		protocol := models.SwitchProtocol(*r.Protocol)
		upd.Protocol = &protocol
	}
	return upd
}
