package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/middleware"
	"github.com/tikman/olt-provisioning/internal/services"
)

type UserHandler struct {
	service      *services.UserService
	auditService *services.AuditService
}

func NewUserHandler(service *services.UserService, auditService *services.AuditService) *UserHandler {
	return &UserHandler{
		service:      service,
		auditService: auditService,
	}
}

// isDuplicateError checks if the error is a unique constraint violation for the given field
func isDuplicateError(err error, field string) bool {
	errMsg := err.Error()
	return strings.Contains(errMsg, "duplicate") ||
		strings.Contains(errMsg, "UNIQUE constraint failed") ||
		(strings.Contains(errMsg, "unique") && strings.Contains(strings.ToLower(errMsg), field))
}

func (h *UserHandler) Create(c *gin.Context) {
	var req CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request body",
			Code:    "INVALID_REQUEST",
			Details: err.Error(),
		})
		return
	}

	user, err := h.service.Create(req.Username, req.Email, req.Password, req.Initials, req.Role)
	if err != nil {
		// Check for duplicate username/email
		if isDuplicateError(err, "username") {
			c.JSON(http.StatusConflict, ErrorResponse{
				Error: "Username already exists",
				Code:  "USERNAME_EXISTS",
			})
			return
		}
		if isDuplicateError(err, "email") {
			c.JSON(http.StatusConflict, ErrorResponse{
				Error: "Email already exists",
				Code:  "EMAIL_EXISTS",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to create user",
			Code:  "CREATE_FAILED",
		})
		return
	}

	// Audit log
	actorID, _ := middleware.GetUserID(c)
	_ = h.auditService.Log(
		actorID,
		"create",
		"user",
		user.ID,
		nil,
		map[string]interface{}{
			"username": user.Username,
			"email":    user.Email,
			"role":     user.Role,
		},
		c.ClientIP(),
		c.Request.UserAgent(),
	)

	c.JSON(http.StatusCreated, ToUserResponse(user))
}

func (h *UserHandler) List(c *gin.Context) {
	users, err := h.service.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to list users",
			Code:  "LIST_FAILED",
		})
		return
	}

	responses := make([]UserResponse, len(users))
	for i, user := range users {
		responses[i] = ToUserResponse(&user)
	}

	c.JSON(http.StatusOK, responses)
}

func (h *UserHandler) GetByID(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Invalid user ID",
			Code:  "INVALID_ID",
		})
		return
	}

	user, err := h.service.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error: "User not found",
			Code:  "NOT_FOUND",
		})
		return
	}

	c.JSON(http.StatusOK, ToUserResponse(user))
}

func (h *UserHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Invalid user ID",
			Code:  "INVALID_ID",
		})
		return
	}

	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request body",
			Code:    "INVALID_REQUEST",
			Details: err.Error(),
		})
		return
	}

	// Get old state for audit log
	oldUser, err := h.service.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error: "User not found",
			Code:  "NOT_FOUND",
		})
		return
	}

	updates := make(map[string]interface{})
	if req.Email != nil {
		updates["email"] = *req.Email
	}
	if req.Password != nil {
		updates["password"] = *req.Password
	}
	if req.Initials != nil {
		updates["initials"] = *req.Initials
	}
	if req.Role != nil {
		updates["role"] = *req.Role
	}

	if err := h.service.Update(id, updates); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to update user",
			Code:  "UPDATE_FAILED",
		})
		return
	}

	user, err := h.service.GetByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to retrieve updated user",
			Code:  "FETCH_FAILED",
		})
		return
	}

	// Audit log
	actorID, _ := middleware.GetUserID(c)
	oldState := map[string]interface{}{
		"email": oldUser.Email,
		"role":  oldUser.Role,
	}
	newState := map[string]interface{}{
		"email": user.Email,
		"role":  user.Role,
	}
	_ = h.auditService.Log(
		actorID,
		"update",
		"user",
		user.ID,
		oldState,
		newState,
		c.ClientIP(),
		c.Request.UserAgent(),
	)

	c.JSON(http.StatusOK, ToUserResponse(user))
}

func (h *UserHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Invalid user ID",
			Code:  "INVALID_ID",
		})
		return
	}

	userID, _ := middleware.GetUserID(c)
	if userID == id {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Cannot delete your own account",
			Code:  "SELF_DELETE",
		})
		return
	}

	if err := h.service.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to delete user",
			Code:  "DELETE_FAILED",
		})
		return
	}

	// Audit log
	actorID, _ := middleware.GetUserID(c)
	_ = h.auditService.Log(
		actorID,
		"delete",
		"user",
		id,
		nil,
		nil,
		c.ClientIP(),
		c.Request.UserAgent(),
	)

	c.JSON(http.StatusNoContent, nil)
}
