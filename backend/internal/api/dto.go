package api

import (
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
)

// User DTOs
type CreateUserRequest struct {
	Username string          `json:"username" binding:"required,min=3,max=50,alphanum"`
	Email    string          `json:"email" binding:"required,email,max=255"`
	Password string          `json:"password" binding:"required,min=12,max=100"`
	Role     models.UserRole `json:"role" binding:"required,oneof=admin technician viewer cs"`
}

type UpdateUserRequest struct {
	Email    *string          `json:"email" binding:"omitempty,email,max=255"`
	Password *string          `json:"password" binding:"omitempty,min=12,max=100"`
	Role     *models.UserRole `json:"role" binding:"omitempty,oneof=admin technician viewer cs"`
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
