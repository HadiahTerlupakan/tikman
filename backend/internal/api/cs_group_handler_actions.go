package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/middleware"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
)

// CreateGroupRequest asks a number to make a new group.
type CreateGroupRequest struct {
	WAAccountID  string   `json:"wa_account_id" binding:"required"`
	Name         string   `json:"name" binding:"required"`
	Participants []string `json:"participants" binding:"required"`
}

// GroupActionRequest asks a number to change a group it is in. Which fields
// matter depends on Action (see services.validateGroupAction).
type GroupActionRequest struct {
	Action       models.GroupActionKind `json:"action" binding:"required"`
	Name         string                 `json:"name"`
	Topic        string                 `json:"topic"`
	Participants []string               `json:"participants"`
	Enabled      bool                   `json:"enabled"`
}

// CreateGroup queues the creation of a group. The answer is the request, not
// the group: only the wa process can talk to WhatsApp, and a group_action
// event follows once it has.
func (h *CSGroupHandler) CreateGroup(c *gin.Context) {
	var req CreateGroupRequest
	if !bindJSON(c, &req) {
		return
	}
	accountID, err := uuid.Parse(req.WAAccountID)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid ID format", Code: "INVALID_ACCOUNT_ID"})
		return
	}
	if _, err := h.inbox.accounts.Get(accountID); err != nil {
		mapCSError(c, err, "GROUP_CREATE_FAILED")
		return
	}

	userID, _ := middleware.GetUserID(c)
	h.queueAction(c, services.GroupActionRequest{
		AccountID: accountID,
		Action:    models.GroupActionCreate,
		Args:      services.GroupActionArgs{Name: req.Name, Participants: req.Participants},
		UserID:    userID,
	}, "GROUP_CREATE_FAILED")
}

// ActOnGroup queues one management request on an existing group.
func (h *CSGroupHandler) ActOnGroup(c *gin.Context) {
	group, ok := h.loadGroup(c, "GROUP_ACTION_FAILED")
	if !ok {
		return
	}
	var req GroupActionRequest
	if !bindJSON(c, &req) {
		return
	}
	if req.Action == models.GroupActionCreate {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Gunakan POST /groups untuk membuat grup", Code: "GROUP_ACTION_FAILED"})
		return
	}

	userID, _ := middleware.GetUserID(c)
	h.queueAction(c, services.GroupActionRequest{
		AccountID: group.WAAccountID,
		GroupID:   &group.ID,
		Action:    req.Action,
		Args: services.GroupActionArgs{
			Name: req.Name, Topic: req.Topic, Participants: req.Participants, Enabled: req.Enabled,
		},
		UserID: userID,
	}, "GROUP_ACTION_FAILED")
}

func (h *CSGroupHandler) queueAction(c *gin.Context, req services.GroupActionRequest, code string) {
	action, err := h.actions.Queue(req)
	if err != nil {
		mapCSError(c, err, code)
		return
	}
	h.wake(c.Request.Context())
	c.JSON(http.StatusAccepted, gin.H{"data": action})
}

// GetGroupAction answers how far a management request got.
func (h *CSGroupHandler) GetGroupAction(c *gin.Context) {
	id, ok := pathUUID(c, "id", "INVALID_ACTION_ID")
	if !ok {
		return
	}
	action, err := h.actions.Get(id)
	if err != nil {
		mapCSError(c, err, "GROUP_ACTION_NOT_FOUND")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": action})
}
