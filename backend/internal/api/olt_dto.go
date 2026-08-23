package api

import (
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
)

type CreateOLTRequest struct {
	SiteID            uuid.UUID          `json:"site_id" binding:"required"`
	Name              string             `json:"name" binding:"required,min=2,max=255"`
	IPAddress         string             `json:"ip_address" binding:"required,ip"`
	Model             models.OLTModel    `json:"model" binding:"required,oneof=zte_c300 zte_c320 hsgq"`
	SSHPort           int                `json:"ssh_port" binding:"omitempty,min=1,max=65535"`
	TelnetPort        int                `json:"telnet_port" binding:"omitempty,min=1,max=65535"`
	SNMPPort          int                `json:"snmp_port" binding:"omitempty,min=1,max=65535"`
	SNMPCommunity     string             `json:"snmp_community" binding:"omitempty,max=100"`
	PreferredProtocol models.OLTProtocol `json:"preferred_protocol" binding:"required,oneof=ssh telnet"`
	Username          string             `json:"username" binding:"required,min=1,max=100"`
	Password          string             `json:"password" binding:"required,min=1"`
}

type TestConnectionRequest struct {
	IPAddress         string             `json:"ip_address" binding:"required,ip"`
	SSHPort           int                `json:"ssh_port" binding:"omitempty,min=1,max=65535"`
	TelnetPort        int                `json:"telnet_port" binding:"omitempty,min=1,max=65535"`
	SNMPPort          int                `json:"snmp_port" binding:"omitempty,min=1,max=65535"`
	SNMPCommunity     string             `json:"snmp_community" binding:"omitempty,max=100"`
	PreferredProtocol models.OLTProtocol `json:"preferred_protocol" binding:"required,oneof=ssh telnet"`
	Username          string             `json:"username" binding:"required,min=1,max=100"`
	Password          string             `json:"password" binding:"required,min=1"`
}

type TestConnectionResponse struct {
	Success      bool     `json:"success"`
	PassedTests  []string `json:"passed_tests"`
	FailedTest   string   `json:"failed_test,omitempty"`
	FailedReason string   `json:"failed_reason,omitempty"`
}

type UpdateOLTRequest struct {
	Name              *string             `json:"name" binding:"omitempty,min=2,max=255"`
	IPAddress         *string             `json:"ip_address" binding:"omitempty,ip"`
	Model             *models.OLTModel    `json:"model" binding:"omitempty,oneof=zte_c300 zte_c320 hsgq"`
	SSHPort           *int                `json:"ssh_port" binding:"omitempty,min=1,max=65535"`
	TelnetPort        *int                `json:"telnet_port" binding:"omitempty,min=1,max=65535"`
	SNMPPort          *int                `json:"snmp_port" binding:"omitempty,min=1,max=65535"`
	SNMPCommunity     *string             `json:"snmp_community" binding:"omitempty,max=100"`
	PreferredProtocol *models.OLTProtocol `json:"preferred_protocol" binding:"omitempty,oneof=ssh telnet"`
	Username          *string             `json:"username" binding:"omitempty,min=1,max=100"`
	Password          *string             `json:"password" binding:"omitempty,min=1"`
}

type OLTResponse struct {
	ID                uuid.UUID          `json:"id"`
	SiteID            uuid.UUID          `json:"site_id"`
	SiteName          string             `json:"site_name"`
	Name              string             `json:"name"`
	IPAddress         string             `json:"ip_address"`
	Model             models.OLTModel    `json:"model"`
	SSHPort           int                `json:"ssh_port"`
	TelnetPort        int                `json:"telnet_port"`
	SNMPPort          int                `json:"snmp_port"`
	SNMPCommunity     string             `json:"snmp_community"`
	PreferredProtocol models.OLTProtocol `json:"preferred_protocol"`
	Username          string             `json:"username"`
	Status            models.OLTStatus   `json:"status"`
	LastSeen          *time.Time         `json:"last_seen"`
	ONTCount          int                `json:"ont_count"`
	CreatedAt         time.Time          `json:"created_at"`
	UpdatedAt         time.Time          `json:"updated_at"`
}

// ToOLTResponse renders an OLT row for the API. The site name is resolved by
// the caller via the OLT service; a row whose site is missing or unset simply
// renders an empty name.
func ToOLTResponse(siteName string, olt *models.OLT) OLTResponse {
	return OLTResponse{
		ID:                olt.ID,
		SiteID:            olt.SiteID,
		SiteName:          siteName,
		Name:              olt.Name,
		IPAddress:         olt.IPAddress,
		Model:             olt.Model,
		SSHPort:           olt.SSHPort,
		TelnetPort:        olt.TelnetPort,
		SNMPPort:          olt.SNMPPort,
		SNMPCommunity:     olt.SNMPCommunity,
		PreferredProtocol: olt.PreferredProtocol,
		Username:          olt.Username,
		Status:            olt.Status,
		LastSeen:          olt.LastSeen,
		ONTCount:          0,
		CreatedAt:         olt.CreatedAt,
		UpdatedAt:         olt.UpdatedAt,
	}
}
