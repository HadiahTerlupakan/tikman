package api

import (
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
)

// User DTOs
type CreateUserRequest struct {
	Username string          `json:"username" binding:"required,min=3,max=50,alphanum"`
	Email    string          `json:"email" binding:"required,email,max=255"`
	Password string          `json:"password" binding:"required,min=12,max=100"`
	Role     models.UserRole `json:"role" binding:"required,oneof=admin technician viewer"`
}

type UpdateUserRequest struct {
	Email    *string          `json:"email" binding:"omitempty,email,max=255"`
	Password *string          `json:"password" binding:"omitempty,min=12,max=100"`
	Role     *models.UserRole `json:"role" binding:"omitempty,oneof=admin technician viewer"`
}

type UserResponse struct {
	ID        uuid.UUID       `json:"id"`
	Username  string          `json:"username"`
	Email     string          `json:"email"`
	Role      models.UserRole `json:"role"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

func ToUserResponse(user *models.User) UserResponse {
	return UserResponse{
		ID:        user.ID,
		Username:  user.Username,
		Email:     user.Email,
		Role:      user.Role,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
}

// Auth DTOs
type LoginRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50,alphanum"`
	Password string `json:"password" binding:"required,min=8,max=100"`
}

type LoginResponse struct {
	User  UserResponse `json:"user"`
	Token string       `json:"token"`
}

type ErrorResponse struct {
	Error   string      `json:"error"`
	Code    string      `json:"code"`
	Details interface{} `json:"details,omitempty"`
}

type CreateSiteRequest struct {
	Name        string `json:"name" binding:"required,min=2,max=255"`
	Location    string `json:"location"`
	Description string `json:"description"`
}

type UpdateSiteRequest struct {
	Name        *string `json:"name" binding:"omitempty,min=2,max=255"`
	Location    *string `json:"location"`
	Description *string `json:"description"`
}

type SiteResponse struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Location    string    `json:"location"`
	Description string    `json:"description"`
	OLTCount    int       `json:"olt_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func ToSiteResponse(site *models.Site) SiteResponse {
	return SiteResponse{
		ID:          site.ID,
		Name:        site.Name,
		Location:    site.Location,
		Description: site.Description,
		OLTCount:    0, // TODO: query count separately if needed
		CreatedAt:   site.CreatedAt,
		UpdatedAt:   site.UpdatedAt,
	}
}

type CreateOLTRequest struct {
	SiteID            uuid.UUID          `json:"site_id" binding:"required"`
	Name              string             `json:"name" binding:"required,min=2,max=255"`
	IPAddress         string             `json:"ip_address" binding:"required,ip"`
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

func ToOLTResponse(olt *models.OLT) OLTResponse {
	return OLTResponse{
		ID:                olt.ID,
		SiteID:            olt.SiteID,
		SiteName:          "", // TODO: join with Site table if needed
		Name:              olt.Name,
		IPAddress:         olt.IPAddress,
		SSHPort:           olt.SSHPort,
		TelnetPort:        olt.TelnetPort,
		SNMPPort:          olt.SNMPPort,
		SNMPCommunity:     olt.SNMPCommunity,
		PreferredProtocol: olt.PreferredProtocol,
		Username:          olt.Username,
		Status:            olt.Status,
		LastSeen:          olt.LastSeen,
		ONTCount:          0, // TODO: query count separately if needed
		CreatedAt:         olt.CreatedAt,
		UpdatedAt:         olt.UpdatedAt,
	}
}

// ONT DTOs
type CreateONTRequest struct {
	OLTID        uuid.UUID         `json:"olt_id" binding:"required"`
	PortID       int               `json:"port_id" binding:"required,min=0,max=15"`
	ONTID        int               `json:"ont_id" binding:"required,min=0,max=127"`
	SerialNumber string            `json:"serial_number" binding:"required,min=1,max=20"`
	Description  string            `json:"description" binding:"omitempty,max=255"`
	Status       models.ONTStatus  `json:"status" binding:"omitempty,oneof=online offline los unknown"`
}

type UpdateONTRequest struct {
	Description *string           `json:"description" binding:"omitempty,max=255"`
	Status      *models.ONTStatus `json:"status" binding:"omitempty,oneof=online offline los unknown"`
}

type ONTResponse struct {
	ID           uuid.UUID         `json:"id"`
	OLTID        uuid.UUID         `json:"olt_id"`
	OLTName      string            `json:"olt_name"`
	PortID       int               `json:"port_id"`
	ONTID        int               `json:"ont_id"`
	SerialNumber string            `json:"serial_number"`
	Description  string            `json:"description"`
	Status       models.ONTStatus  `json:"status"`
	LastSeenAt   *time.Time        `json:"last_seen_at"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

func ToONTResponse(ont *models.ONT) ONTResponse {
	return ONTResponse{
		ID:           ont.ID,
		OLTID:        ont.OLTID,
		OLTName:      "", // TODO: join with OLT table if needed
		PortID:       ont.PortID,
		ONTID:        ont.ONTID,
		SerialNumber: ont.SerialNumber,
		Description:  ont.Description,
		Status:       ont.Status,
		LastSeenAt:   ont.LastSeenAt,
		CreatedAt:    ont.CreatedAt,
		UpdatedAt:    ont.UpdatedAt,
	}
}

// ONT Metrics DTOs
type ONTMetricsResponse struct {
	Time        time.Time `json:"time"`
	RxPower     float64   `json:"rx_power"`
	TxPower     float64   `json:"tx_power"`
	Temperature float64   `json:"temperature"`
	Voltage     float64   `json:"voltage"`
	Distance    int       `json:"distance"`
	RxBytes     uint64    `json:"rx_bytes"`
	TxBytes     uint64    `json:"tx_bytes"`
}

func ToONTMetricsResponse(metrics *services.ONTMetricsRow) ONTMetricsResponse {
	return ONTMetricsResponse{
		Time:        metrics.Time,
		RxPower:     metrics.RxPower,
		TxPower:     metrics.TxPower,
		Temperature: metrics.Temperature,
		Voltage:     metrics.Voltage,
		Distance:    metrics.Distance,
		RxBytes:     metrics.RxBytes,
		TxBytes:     metrics.TxBytes,
	}
}
