package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/middleware"
	"github.com/tikman/olt-provisioning/internal/models"
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
		refuseUserCreate(c, err)
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

// refuseUserCreate names the field already taken, so the form can point at it
// rather than reporting a server fault the operator cannot act on.
func refuseUserCreate(c *gin.Context, err error) {
	switch {
	case isDuplicateError(err, "username"):
		c.JSON(http.StatusConflict, ErrorResponse{
			Error: "Username already exists",
			Code:  "USERNAME_EXISTS",
		})
	case isDuplicateError(err, "email"):
		c.JSON(http.StatusConflict, ErrorResponse{
			Error: "Email already exists",
			Code:  "EMAIL_EXISTS",
		})
	default:
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "Failed to create user",
			Code:  "CREATE_FAILED",
		})
	}
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

// mayUpdateUser decides who PUT /users/:id is for. A technician is on the
// route to maintain their own account, so anything wider is refused: someone
// else's row, or a role on any row at all. Without the second half the route
// was a one-request promotion to admin, which every admin-only route behind
// RequireRole then honoured.
func mayUpdateUser(c *gin.Context, targetID uuid.UUID, role *models.UserRole) bool {
	if actorRole, ok := middleware.GetUserRole(c); ok && actorRole == models.UserRoleAdmin {
		return true
	}
	if role != nil {
		return false
	}
	actorID, ok := middleware.GetUserID(c)
	return ok && actorID == targetID
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

	if !mayUpdateUser(c, id, req.Role) {
		c.JSON(http.StatusForbidden, ErrorResponse{
			Error: "You may only update your own account, and only an admin may change a role",
			Code:  "FORBIDDEN",
		})
		return
	}

	// Read before the write, so the audit entry can say what changed.
	oldUser, err := h.service.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "User not found", Code: "NOT_FOUND"})
		return
	}

	if err := h.service.Update(id, userUpdates(req)); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to update user", Code: "UPDATE_FAILED"})
		return
	}

	user, err := h.service.GetByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "User updated but failed to retrieve", Code: "FETCH_FAILED"})
		return
	}

	h.logUserUpdate(c, oldUser, user)

	c.JSON(http.StatusOK, ToUserResponse(user))
}

// userUpdates turns the supplied fields into the columns to write. The
// password arrives in plain text and the service hashes it; nothing here
// writes it as given.
func userUpdates(req UpdateUserRequest) map[string]interface{} {
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
	return updates
}

func (h *UserHandler) logUserUpdate(c *gin.Context, before, after *models.User) {
	actorID, _ := middleware.GetUserID(c)
	_ = h.auditService.Log(
		actorID,
		"update",
		"user",
		after.ID,
		map[string]interface{}{"email": before.Email, "role": before.Role},
		map[string]interface{}{"email": after.Email, "role": after.Role},
		c.ClientIP(),
		c.Request.UserAgent(),
	)
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
