package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tikman/olt-provisioning/internal/middleware"
	"github.com/tikman/olt-provisioning/internal/services"
)

// PushTokenRequest is the body both push endpoints share — subscribing and
// unsubscribing name exactly the same thing, the caller's own device token.
type PushTokenRequest struct {
	FCMToken string `json:"fcm_token" binding:"required"`
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

// Subscribe registers the caller's device token.
func (h *PushHandler) Subscribe(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthenticated", Code: "UNAUTHENTICATED"})
		return
	}
	var req PushTokenRequest
	if !bindJSON(c, &req) {
		return
	}
	if err := h.push.Subscribe(userID, req.FCMToken); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to subscribe", Code: "PUSH_SUBSCRIBE_FAILED"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"status": "subscribed"}})
}

// Unsubscribe removes the caller's device token. Removing a token the caller
// does not own succeeds silently rather than erroring — the same non-answer
// either way keeps the endpoint from confirming whether some other token
// exists at all.
func (h *PushHandler) Unsubscribe(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthenticated", Code: "UNAUTHENTICATED"})
		return
	}
	var req PushTokenRequest
	if !bindJSON(c, &req) {
		return
	}
	if err := h.push.Unsubscribe(userID, req.FCMToken); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to unsubscribe", Code: "PUSH_UNSUBSCRIBE_FAILED"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"status": "unsubscribed"}})
}
