package api

import (
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
)

type CreateUserRequest struct {
	Username string          `json:"username" binding:"required,min=3,max=50"`
	Email    string          `json:"email" binding:"required,email"`
	Password string          `json:"password" binding:"required,min=8"`
	Role     models.UserRole `json:"role" binding:"required,oneof=admin technician viewer"`
}

type UpdateUserRequest struct {
	Email    *string          `json:"email" binding:"omitempty,email"`
	Password *string          `json:"password" binding:"omitempty,min=8"`
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
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
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
		OLTCount:    len(site.OLTs),
		CreatedAt:   site.CreatedAt,
		UpdatedAt:   site.UpdatedAt,
	}
}
