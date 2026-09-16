package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
)

// bindJSON reads a request body, answering the caller and reporting false when
// it cannot.
func bindJSON(c *gin.Context, target interface{}) bool {
	if err := c.ShouldBindJSON(target); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request body",
			Code:    "INVALID_REQUEST",
			Details: err.Error(),
		})
		return false
	}
	return true
}

// pathUUID reads a UUID out of the path, answering the caller and reporting
// false when it is not one.
func pathUUID(c *gin.Context, param, code string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(param))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Invalid ID format", Code: code,
		})
		return uuid.Nil, false
	}
	return id, true
}

// badRequest answers a validation failure with the operator's own sentence.
func badRequest(c *gin.Context, err error, code string) bool {
	if !errors.Is(err, services.ErrValidation) {
		return false
	}
	c.JSON(http.StatusBadRequest, ErrorResponse{
		Error: strings.TrimPrefix(err.Error(), services.ErrValidation.Error()+": "),
		Code:  code,
	})
	return true
}

// User DTOs
type CreateUserRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50,alphanum"`
	Email    string `json:"email" binding:"required,email,max=255"`
	Password string `json:"password" binding:"required,min=12,max=100"`
	// Left empty, this derives from Username (see deriveInitials) — most
	// admins never need to set it.
	Initials string          `json:"initials" binding:"omitempty,max=4,alphanum"`
	Role     models.UserRole `json:"role" binding:"required,oneof=admin technician viewer cs"`
}

type UpdateUserRequest struct {
	Email    *string `json:"email" binding:"omitempty,email,max=255"`
	Password *string `json:"password" binding:"omitempty,min=12,max=100"`
	// A present-but-empty value ("initials": "") clears an explicit mark and
	// falls back to deriveInitials on the username already on file; omitting
	// the field entirely leaves the stored initials untouched.
	Initials *string          `json:"initials" binding:"omitempty,max=4,alphanum"`
	Role     *models.UserRole `json:"role" binding:"omitempty,oneof=admin technician viewer cs"`
}

type UserResponse struct {
	ID        uuid.UUID       `json:"id"`
	Username  string          `json:"username"`
	Email     string          `json:"email"`
	Initials  string          `json:"initials"`
	Role      models.UserRole `json:"role"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

func ToUserResponse(user *models.User) UserResponse {
	return UserResponse{
		ID:        user.ID,
		Username:  user.Username,
		Email:     user.Email,
		Initials:  user.Initials,
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

// LoginResponse carries no token: the session travels in an HttpOnly cookie,
// and echoing it here would hand the same value to any script on the page.
type LoginResponse struct {
	User UserResponse `json:"user"`
}

type ErrorResponse struct {
	Error   string      `json:"error"`
	Code    string      `json:"code"`
	Details interface{} `json:"details,omitempty"`
}
