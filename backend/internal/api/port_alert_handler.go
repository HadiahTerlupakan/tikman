package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/middleware"
	"github.com/tikman/olt-provisioning/internal/services"
)

// libreNMSTokenHeader carries the webhook secret; LibreNMS's API transport
// sets it from its "key=value" header lines.
const libreNMSTokenHeader = "X-TikMan-Token"

// maxHookBodyBytes bounds a webhook body; the one field read is a number.
const maxHookBodyBytes = 64 << 10

const portAlertDestinationResource = "port_alert_destination"

// PortAlertDestinationRequest replaces the NOC destination. Enabled defaults
// to true when left out; both destinations null means no NOC notices.
type PortAlertDestinationRequest struct {
	WAGroupID        *uuid.UUID `json:"wa_group_id"`
	TelegramChatID   *uuid.UUID `json:"telegram_chat_id"`
	TelegramThreadID *int64     `json:"telegram_thread_id"`
	Enabled          *bool      `json:"enabled"`
}

// PortAlertHandler takes LibreNMS's alerts and manages the NOC destination.
type PortAlertHandler struct {
	authorized   func(token string) bool
	trigger      func(deviceID int)
	destinations *services.PortAlertDestinationService
	audit        *services.AuditService
}

// NewPortAlertHandler constructs a PortAlertHandler.
func NewPortAlertHandler(
	alerts *services.PortAlertService, destinations *services.PortAlertDestinationService, audit *services.AuditService,
) *PortAlertHandler {
	return &PortAlertHandler{
		authorized: alerts.WebhookAuthorized, trigger: alerts.Trigger, destinations: destinations, audit: audit,
	}
}

// LibreNMSHook takes an alert from LibreNMS's API transport. The body says
// only which device; the ports are read back from LibreNMS in the background,
// so the answer is 202 whether or not the device is one TikMan watches.
// LibreNMS sends the body as text/plain unless told otherwise, so it is read
// as JSON whatever its Content-Type.
func (h *PortAlertHandler) LibreNMSHook(c *gin.Context) {
	if !h.authorized(c.GetHeader(libreNMSTokenHeader)) {
		c.JSON(http.StatusForbidden, ErrorResponse{Error: "token webhook tidak valid", Code: "WEBHOOK_FORBIDDEN"})
		return
	}
	raw, err := io.ReadAll(io.LimitReader(c.Request.Body, maxHookBodyBytes))
	deviceID, ok := hookDeviceID(raw)
	if err != nil || !ok {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "device_id tidak valid", Code: "INVALID_DEVICE_ID"})
		return
	}
	h.trigger(deviceID)
	c.JSON(http.StatusAccepted, gin.H{"message": "Diterima"})
}

// hookDeviceID reads device_id as a number or, as LibreNMS's template quotes
// it, a string holding one.
func hookDeviceID(raw []byte) (int, bool) {
	var body struct {
		DeviceID json.RawMessage `json:"device_id"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return 0, false
	}
	id, err := strconv.Atoi(strings.TrimSpace(strings.Trim(string(body.DeviceID), `"`)))
	return id, err == nil && id > 0
}

// Destination answers the NOC destination.
func (h *PortAlertHandler) Destination(c *gin.Context) {
	view, err := h.destinations.Get()
	if err != nil {
		graphReportError(c, err, "PORT_ALERT_DESTINATION_FAILED")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": view})
}

// SetDestination replaces the NOC destination.
func (h *PortAlertHandler) SetDestination(c *gin.Context) {
	var req PortAlertDestinationRequest
	if !bindJSON(c, &req) {
		return
	}
	view, err := h.destinations.Set(services.PortAlertDestinationInput{
		WAGroupID: req.WAGroupID, TelegramChatID: req.TelegramChatID, TelegramThreadID: req.TelegramThreadID,
		Enabled: req.Enabled == nil || *req.Enabled,
	})
	if err != nil {
		graphReportError(c, err, "PORT_ALERT_DESTINATION_UPDATE_FAILED")
		return
	}
	if h.audit != nil {
		actor, _ := middleware.GetUserID(c)
		_ = h.audit.Log(actor, "update", portAlertDestinationResource, uuid.Nil, nil, nil, c.ClientIP(), c.Request.UserAgent())
	}
	c.JSON(http.StatusOK, gin.H{"data": view})
}
