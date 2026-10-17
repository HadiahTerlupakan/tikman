package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"github.com/tikman/olt-provisioning/internal/wa"
	"go.uber.org/zap"
)

// CSGroupHandler serves the WhatsApp groups, apart from the customer inbox.
//
// It borrows the inbox handler for what the two genuinely share — storing an
// upload, signing a reply, publishing to the wa process — rather than copying
// those paths and letting the copies drift.
type CSGroupHandler struct {
	inbox    *CSHandler
	groups   *services.WAGroupService
	messages *services.WAGroupMessageService
	actions  *services.WAGroupActionService
	logger   *zap.Logger
}

// NewCSGroupHandler constructs a CSGroupHandler.
func NewCSGroupHandler(
	inbox *CSHandler,
	groups *services.WAGroupService,
	messages *services.WAGroupMessageService,
	actions *services.WAGroupActionService,
	logger *zap.Logger,
) *CSGroupHandler {
	return &CSGroupHandler{inbox: inbox, groups: groups, messages: messages, actions: actions, logger: logger}
}

// GroupDetail is one group with its members.
type GroupDetail struct {
	Group        *models.WAGroup             `json:"group"`
	Participants []models.WAGroupParticipant `json:"participants"`
}

// ListGroups answers every group, most recently active first.
func (h *CSGroupHandler) ListGroups(c *gin.Context) {
	filter := services.GroupFilter{Search: c.Query("search")}
	if raw := c.Query("wa_account_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid ID format", Code: "INVALID_ACCOUNT_ID"})
			return
		}
		filter.AccountID = &id
	}

	groups, err := h.groups.List(filter)
	if err != nil {
		mapCSError(c, err, "GROUP_LIST_FAILED")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": groups})
}

// GetGroup answers one group and who is in it.
func (h *CSGroupHandler) GetGroup(c *gin.Context) {
	group, ok := h.loadGroup(c, "GROUP_NOT_FOUND")
	if !ok {
		return
	}
	members, err := h.groups.Participants(group.ID)
	if err != nil {
		mapCSError(c, err, "GROUP_NOT_FOUND")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": GroupDetail{Group: group, Participants: members}})
}

// GroupHistory answers one page of a group, newest first, and clears its
// unread count the way opening a customer thread does.
func (h *CSGroupHandler) GroupHistory(c *gin.Context) {
	groupID, ok := pathUUID(c, "id", "INVALID_GROUP_ID")
	if !ok {
		return
	}
	rows, err := h.messages.History(groupID, queryInt(c, "limit"), queryInt(c, "offset"))
	if err != nil {
		mapCSError(c, err, "GROUP_HISTORY_FAILED")
		return
	}

	// Announced only when the count changed: History is what a browser calls
	// in answer to a nudge, and announcing a read that changed nothing would
	// keep feeding itself.
	cleared, err := h.groups.MarkRead(groupID)
	if err != nil {
		h.logger.Warn("clear group unread count failed", zap.String("group_id", groupID.String()), zap.Error(err))
	} else if cleared {
		h.inbox.announceEvent(c.Request.Context(), wa.Event{Type: wa.EventGroup, GroupID: groupID.String()})
	}
	c.JSON(http.StatusOK, gin.H{"data": rows})
}

// RefreshGroupsRequest names the number to re-read, or none for all of them.
type RefreshGroupsRequest struct {
	WAAccountID string `json:"wa_account_id"`
}

// RefreshGroups asks the wa process to re-read the group lists now. A lost
// request costs only the wait the button skips: the hourly sync re-reads them
// anyway, so publish failures are logged like RefreshChannels does.
func (h *CSGroupHandler) RefreshGroups(c *gin.Context) {
	var req RefreshGroupsRequest
	if c.Request.ContentLength > 0 && !bindJSON(c, &req) {
		return
	}
	accounts, err := h.inbox.accounts.List()
	if err != nil {
		mapCSError(c, err, "GROUP_REFRESH_FAILED")
		return
	}
	for _, account := range accounts {
		if req.WAAccountID != "" && req.WAAccountID != account.ID.String() {
			continue
		}
		msg := wa.ControlMessage{Action: wa.ControlSyncGroups, AccountID: account.ID.String()}
		if err := h.inbox.publishControl(c, msg); err != nil {
			h.logger.Warn("publish group sync request failed",
				zap.String("account_id", account.ID.String()), zap.Error(err))
		}
	}
	c.JSON(http.StatusAccepted, gin.H{"data": gin.H{"queued": true}})
}

// loadGroup reads the group the path names, answering the request itself
// when there is none.
func (h *CSGroupHandler) loadGroup(c *gin.Context, code string) (*models.WAGroup, bool) {
	groupID, ok := pathUUID(c, "id", "INVALID_GROUP_ID")
	if !ok {
		return nil, false
	}
	group, err := h.groups.Get(groupID)
	if err != nil {
		mapCSError(c, err, code)
		return nil, false
	}
	return group, true
}
