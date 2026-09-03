package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tikman/olt-provisioning/internal/middleware"
	"github.com/tikman/olt-provisioning/internal/services"
)

// PushFIDRequest is the body both push endpoints share — subscribing and
// unsubscribing name exactly the same thing, the caller's own Firebase
// Installation ID.
type PushFIDRequest struct {
	FID string `json:"fid" binding:"required"`
}

// PushHandler registers and removes the devices push notifications are sent
// to. Registering your own device is not a CS-specific action, so unlike the
// rest of /api/v1/cs/*, no role is required beyond being logged in.
type PushHandler struct {
	push *services.PushService
}

func NewPushHandler(push *services.PushService) *PushHandler {
	return &PushHandler{push: push}
}

// Subscribe registers the caller's installation ID.
func (h *PushHandler) Subscribe(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthenticated", Code: "UNAUTHENTICATED"})
		return
	}
	var req PushFIDRequest
	if !bindJSON(c, &req) {
		return
	}
	if err := h.push.Subscribe(userID, req.FID); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to subscribe", Code: "PUSH_SUBSCRIBE_FAILED"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"status": "subscribed"}})
}

// Unsubscribe removes the caller's installation ID. Removing an FID the
// caller does not own succeeds silently rather than erroring — the same
// non-answer either way keeps the endpoint from confirming whether some other
// FID exists at all.
func (h *PushHandler) Unsubscribe(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthenticated", Code: "UNAUTHENTICATED"})
		return
	}
	var req PushFIDRequest
	if !bindJSON(c, &req) {
		return
	}
	if err := h.push.Unsubscribe(userID, req.FID); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to unsubscribe", Code: "PUSH_UNSUBSCRIBE_FAILED"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"status": "unsubscribed"}})
}
