package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tikman/olt-provisioning/internal/middleware"
)

// QuickReplyRequest is a canned answer's title and body.
type QuickReplyRequest struct {
	Title string `json:"title" binding:"required"`
	Body  string `json:"body" binding:"required"`
}

// ListQuickReplies answers every canned reply. Reading them is not an admin
// matter — a CS who cannot read the templates cannot use them.
func (h *CSHandler) ListQuickReplies(c *gin.Context) {
	rows, err := h.quickReplies.List()
	if err != nil {
		mapCSError(c, err, "QUICK_REPLY_LIST_FAILED")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows})
}

// CreateQuickReply records a new template.
func (h *CSHandler) CreateQuickReply(c *gin.Context) {
	var req QuickReplyRequest
	if !bindJSON(c, &req) {
		return
	}
	userID, _ := middleware.GetUserID(c)

	row, err := h.quickReplies.Create(req.Title, req.Body, userID)
	if err != nil {
		mapCSError(c, err, "QUICK_REPLY_CREATE_FAILED")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": row})
}

// UpdateQuickReply rewrites a template.
func (h *CSHandler) UpdateQuickReply(c *gin.Context) {
	id, ok := pathUUID(c, "id", "INVALID_QUICK_REPLY_ID")
	if !ok {
		return
	}
	var req QuickReplyRequest
	if !bindJSON(c, &req) {
		return
	}

	row, err := h.quickReplies.Update(id, req.Title, req.Body)
	if err != nil {
		mapCSError(c, err, "QUICK_REPLY_UPDATE_FAILED")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": row})
}

// DeleteQuickReply removes a template.
func (h *CSHandler) DeleteQuickReply(c *gin.Context) {
	id, ok := pathUUID(c, "id", "INVALID_QUICK_REPLY_ID")
	if !ok {
		return
	}
	if err := h.quickReplies.Delete(id); err != nil {
		mapCSError(c, err, "QUICK_REPLY_DELETE_FAILED")
		return
	}
	c.Status(http.StatusNoContent)
}
