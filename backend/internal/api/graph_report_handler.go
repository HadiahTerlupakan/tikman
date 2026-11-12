package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/middleware"
	"github.com/tikman/olt-provisioning/internal/services"
	"github.com/tikman/olt-provisioning/internal/telegram"
)

const graphReportResource = "graph_report"

// graphReportSendTimeout covers ten graphs at LibreNMS's 30 seconds each, with
// room to store and queue them.
const graphReportSendTimeout = 6 * time.Minute

// GraphReportPortRequest is one port of a report, in the order it is sent.
type GraphReportPortRequest struct {
	DeviceID   int    `json:"device_id"`
	DeviceName string `json:"device_name"`
	IfName     string `json:"if_name"`
	Label      string `json:"label"`
	// NotifyStatus sends this port's down/up notices to the report's
	// destinations; false when left out.
	NotifyStatus bool `json:"notify_status"`
}

// GraphReportRequest creates or rewrites a report. Enabled defaults to true
// when left out, which is what registering a report is for. Either
// destination may be null or left out, but not both. A null or absent
// TelegramThreadID posts to General, or to a chat that is no forum.
type GraphReportRequest struct {
	WAGroupID        *uuid.UUID               `json:"wa_group_id"`
	TelegramChatID   *uuid.UUID               `json:"telegram_chat_id"`
	TelegramThreadID *int64                   `json:"telegram_thread_id"`
	CustomerName     string                   `json:"customer_name"`
	Message          string                   `json:"message"`
	Enabled          *bool                    `json:"enabled"`
	MentionAll       bool                     `json:"mention_all"`
	SendTime         string                   `json:"send_time"`
	Ports            []GraphReportPortRequest `json:"ports"`
}

func (r GraphReportRequest) input() services.GraphReportInput {
	return services.GraphReportInput{
		WAGroupID: r.WAGroupID, TelegramChatID: r.TelegramChatID, TelegramThreadID: r.TelegramThreadID,
		CustomerName: r.CustomerName, Message: r.Message,
		Enabled: r.Enabled == nil || *r.Enabled, MentionAll: r.MentionAll,
		SendTime: r.SendTime, Ports: portInputs(r.Ports),
	}
}

func portInputs(ports []GraphReportPortRequest) []services.GraphReportPortInput {
	in := make([]services.GraphReportPortInput, len(ports))
	for i, port := range ports {
		in[i] = services.GraphReportPortInput(port)
	}
	return in
}

// GraphReportHandler manages the daily LibreNMS graph reports.
type GraphReportHandler struct {
	reports *services.GraphReportService
	sender  *services.GraphReportSender
	chats   *services.TelegramChatService
	audit   *services.AuditService
}

// NewGraphReportHandler constructs a GraphReportHandler.
func NewGraphReportHandler(
	reports *services.GraphReportService, sender *services.GraphReportSender,
	chats *services.TelegramChatService, audit *services.AuditService,
) *GraphReportHandler {
	return &GraphReportHandler{reports: reports, sender: sender, chats: chats, audit: audit}
}

// TelegramChats answers the chats the bot has been seen in, by title, the
// ones it has left included so a report bound to one still names it, each
// with the forum topics seen in it.
func (h *GraphReportHandler) TelegramChats(c *gin.Context) {
	chats, err := h.chats.List()
	if err != nil {
		graphReportError(c, err, "TELEGRAM_CHAT_LIST_FAILED")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": chats})
}

// List answers every report.
func (h *GraphReportHandler) List(c *gin.Context) {
	reports, err := h.reports.List()
	if err != nil {
		graphReportError(c, err, "GRAPH_REPORT_LIST_FAILED")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": reports})
}

// Create registers a report for a group.
func (h *GraphReportHandler) Create(c *gin.Context) {
	var req GraphReportRequest
	if !bindJSON(c, &req) {
		return
	}
	actor, _ := middleware.GetUserID(c)
	report, err := h.reports.Create(req.input(), actor)
	if err != nil {
		graphReportError(c, err, "GRAPH_REPORT_CREATE_FAILED")
		return
	}
	h.record(c, "create", report.ID, report.CustomerName)
	c.JSON(http.StatusCreated, report)
}

// Update rewrites a report, replacing its ports.
func (h *GraphReportHandler) Update(c *gin.Context) {
	id, ok := pathUUID(c, "id", "INVALID_GRAPH_REPORT_ID")
	if !ok {
		return
	}
	var req GraphReportRequest
	if !bindJSON(c, &req) {
		return
	}
	report, err := h.reports.Update(id, req.input())
	if err != nil {
		graphReportError(c, err, "GRAPH_REPORT_UPDATE_FAILED")
		return
	}
	h.record(c, "update", report.ID, report.CustomerName)
	c.JSON(http.StatusOK, report)
}

// Delete removes a report.
func (h *GraphReportHandler) Delete(c *gin.Context) {
	id, ok := pathUUID(c, "id", "INVALID_GRAPH_REPORT_ID")
	if !ok {
		return
	}
	if err := h.reports.Delete(id); err != nil {
		graphReportError(c, err, "GRAPH_REPORT_DELETE_FAILED")
		return
	}
	h.record(c, "delete", id, "")
	c.Status(http.StatusNoContent)
}

// Send sends a report to its destinations now, answering only once every graph has
// been fetched, so the admin learns of a LibreNMS failure on the spot. The
// send outlives the request: a browser giving up halfway would otherwise
// cancel the fetch in flight, or the wake-up after the messages are queued.
func (h *GraphReportHandler) Send(c *gin.Context) {
	id, ok := pathUUID(c, "id", "INVALID_GRAPH_REPORT_ID")
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), graphReportSendTimeout)
	defer cancel()
	if err := h.sender.SendNow(ctx, id); err != nil {
		graphReportError(c, err, "GRAPH_REPORT_SEND_FAILED")
		return
	}
	h.record(c, "send", id, "")
	c.JSON(http.StatusOK, gin.H{"message": "Laporan dikirim"})
}

func (h *GraphReportHandler) record(c *gin.Context, action string, id uuid.UUID, customer string) {
	if h.audit == nil {
		return
	}
	actor, _ := middleware.GetUserID(c)
	var detail map[string]interface{}
	if customer != "" {
		detail = map[string]interface{}{"customer_name": customer}
	}
	_ = h.audit.Log(actor, action, graphReportResource, id, nil, detail, c.ClientIP(), c.Request.UserAgent())
}

// graphReportError answers the statuses the page tells apart: an unset
// LibreNMS is the admin's to fix (400), a group that cannot be posted in or
// already has a report, through this number or another, is a conflict (409),
// and LibreNMS or Telegram failing is upstream (502). An unset or malformed
// Telegram token is the admin's like LibreNMS's, and a chat the bot left a conflict like a
// group the number left.
func graphReportError(c *gin.Context, err error, code string) {
	status := 0
	switch {
	case errors.Is(err, services.ErrGraphReportGroupTaken):
		// Its own code so the form can point at the number picker.
		status, code = http.StatusConflict, "GRAPH_REPORT_GROUP_TAKEN"
	case errors.Is(err, services.ErrLibreNMSNotConfigured), errors.Is(err, services.ErrTelegramNotConfigured),
		errors.Is(err, telegram.ErrInvalidToken):
		status = http.StatusBadRequest
	case errors.Is(err, services.ErrGraphReportDuplicate),
		errors.Is(err, services.ErrGroupLeft), errors.Is(err, services.ErrGroupAnnounceOnly),
		errors.Is(err, services.ErrGraphReportTooManyMentions), errors.Is(err, services.ErrTelegramChatInactive):
		status = http.StatusConflict
	case errors.Is(err, services.ErrGraphFetch), errors.Is(err, services.ErrTelegramSend):
		status = http.StatusBadGateway
	}
	if status == 0 {
		mapCSError(c, err, code)
		return
	}
	c.JSON(status, ErrorResponse{Error: err.Error(), Code: code})
}
