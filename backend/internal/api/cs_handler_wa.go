package api

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/utils"
	"github.com/tikman/olt-provisioning/internal/wa"
	"go.uber.org/zap"
)

// ConnectAccountRequest is the phone number an admin typed in to pair a
// WhatsApp number, in whatever form they wrote it.
type ConnectAccountRequest struct {
	Phone string `json:"phone" binding:"required"`
}

// waControlMessage is one admin action on wa.ControlChannel. The wa process
// is the only thing subscribed to it — the API never touches WhatsApp itself.
type waControlMessage struct {
	Action    string `json:"action"`
	AccountID string `json:"account_id"`
	Phone     string `json:"phone,omitempty"`
}

// ListAccounts answers every WhatsApp number the team can answer from.
func (h *CSHandler) ListAccounts(c *gin.Context) {
	rows, err := h.accounts.List()
	if err != nil {
		mapCSError(c, err, "ACCOUNT_LIST_FAILED")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows})
}

// Connect starts pairing a number by phone. The account is marked "pairing"
// immediately, before the wa process even sees the request, so a browser
// polling the list shows the change without delay; the eight-character
// linking code the admin types into WhatsApp arrives moments later over the
// SSE stream the browser already listens to. PairPhone on the wa side
// rejects a number written with a leading zero, so it is normalized to
// 628... form here rather than left to fail on the other side of Redis.
func (h *CSHandler) Connect(c *gin.Context) {
	id, ok := pathUUID(c, "id", "INVALID_ACCOUNT_ID")
	if !ok {
		return
	}
	var req ConnectAccountRequest
	if !bindJSON(c, &req) {
		return
	}

	phone, err := utils.NormalizePhone(req.Phone)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error(), Code: "INVALID_PHONE"})
		return
	}

	if err := h.accounts.MarkPairing(id); err != nil {
		mapCSError(c, err, "CONNECT_FAILED")
		return
	}

	h.publishControl(c, waControlMessage{Action: "connect", AccountID: id.String(), Phone: phone})
	c.JSON(http.StatusAccepted, gin.H{"data": gin.H{"status": string(models.WAAccountPairing)}})
}

// Disconnect asks the wa process to give up the pairing. The wa process
// records the account as disconnected once the session is actually logged
// out — this endpoint only asks, it never touches the WhatsApp connection.
func (h *CSHandler) Disconnect(c *gin.Context) {
	id, ok := pathUUID(c, "id", "INVALID_ACCOUNT_ID")
	if !ok {
		return
	}
	if _, err := h.accounts.Get(id); err != nil {
		mapCSError(c, err, "DISCONNECT_FAILED")
		return
	}

	h.publishControl(c, waControlMessage{Action: "disconnect", AccountID: id.String()})
	c.JSON(http.StatusAccepted, gin.H{"data": gin.H{"status": "disconnect_requested"}})
}

// publishControl sends one action to the wa process. Redis here is a nudge,
// not the truth — a failed publish is logged and never fails the request,
// same as every other announcement in this module.
func (h *CSHandler) publishControl(c *gin.Context, msg waControlMessage) {
	payload, err := json.Marshal(msg)
	if err != nil {
		h.logger.Warn("Could not encode a WhatsApp control message", zap.Error(err))
		return
	}
	if err := h.redis.Publish(c.Request.Context(), wa.ControlChannel, payload).Err(); err != nil {
		h.logger.Warn("Could not publish a WhatsApp control message", zap.Error(err))
	}
}
