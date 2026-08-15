package api

import (
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
)

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
