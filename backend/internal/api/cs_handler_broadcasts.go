package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/middleware"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"github.com/tikman/olt-provisioning/internal/wa"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// BroadcastDestinationRequest names one place an announcement should go.
// Exactly one of the two id fields is used, decided by Type.
type BroadcastDestinationRequest struct {
	Type        string `json:"type" binding:"required"`
	ChannelID   string `json:"channel_id"`
	WAAccountID string `json:"wa_account_id"`
}

// CreateBroadcastRequest is one announcement as the composer sends it.
type CreateBroadcastRequest struct {
	Body         string                        `json:"body" binding:"required"`
	Destinations []BroadcastDestinationRequest `json:"destinations" binding:"required"`
}

// ListChannels answers every channel the team may post to. It reads the
// mirror the wa process keeps; no WhatsApp connection is involved.
func (h *CSHandler) ListChannels(c *gin.Context) {
	rows, err := h.channels.List()
	if err != nil {
		mapCSError(c, err, "CHANNEL_LIST_FAILED")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows})
}

// RefreshChannels asks the wa process to re-read every number's channel list.
//
// One control message per number, because a session is per number and the
// process routes the action by account id. A publish that fails is logged and
// swallowed, unlike the other control actions, which hand theirs back to the
// caller: the hourly sweep re-reads the list anyway, so the worst a lost
// request costs is the wait the button was meant to skip.
func (h *CSHandler) RefreshChannels(c *gin.Context) {
	accounts, err := h.accounts.List()
	if err != nil {
		mapCSError(c, err, "CHANNEL_REFRESH_FAILED")
		return
	}

	for _, account := range accounts {
		msg := wa.ControlMessage{Action: wa.ControlSyncChannels, AccountID: account.ID.String()}
		if err := h.publishControl(c, msg); err != nil {
			h.logger.Warn("publish channel sync request failed",
				zap.String("account_id", account.ID.String()), zap.Error(err))
		}
	}
	c.JSON(http.StatusAccepted, gin.H{"data": gin.H{"accounts": len(accounts)}})
}

// ListBroadcasts answers the most recent announcements across every
// destination, newest first.
func (h *CSHandler) ListBroadcasts(c *gin.Context) {
	rows, err := h.broadcasts.ListRecent(queryInt(c, "limit"))
	if err != nil {
		mapCSError(c, err, "BROADCAST_HISTORY_FAILED")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows})
}

// CreateBroadcast queues one text announcement per destination.
//
// Every destination is resolved before any row is written, so a request naming
// one good and one bad destination queues nothing — half an announcement is
// worse than none, because the sender would believe both went.
func (h *CSHandler) CreateBroadcast(c *gin.Context) {
	var req CreateBroadcastRequest
	if !bindJSON(c, &req) {
		return
	}

	body := strings.TrimSpace(req.Body)
	if body == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "pengumuman tidak boleh kosong", Code: "EMPTY_BROADCAST",
		})
		return
	}

	targets, ok := h.resolveDestinations(c, req.Destinations, models.MessageKindText)
	if !ok {
		return
	}

	userID, _ := middleware.GetUserID(c)
	queued, err := h.queueAll(targets, userID, models.MessageKindText, body, nil)
	if err != nil {
		mapCSError(c, err, "BROADCAST_FAILED")
		return
	}

	h.announceBroadcast(c)
	c.JSON(http.StatusCreated, gin.H{"data": queued})
}

// broadcastTarget is one resolved destination, ready to become a row.
type broadcastTarget struct {
	accountID   uuid.UUID
	destination models.BroadcastDestination
	channelJID  string
}

// resolveDestinations turns the request's destinations into targets, refusing
// the whole request if any one of them cannot be honoured — including a
// document aimed at a status, which WhatsApp does not accept.
func (h *CSHandler) resolveDestinations(
	c *gin.Context, requested []BroadcastDestinationRequest, kind models.MessageKind,
) ([]broadcastTarget, bool) {
	if len(requested) == 0 {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "pilih setidaknya satu tujuan", Code: "NO_DESTINATION",
		})
		return nil, false
	}

	targets := make([]broadcastTarget, 0, len(requested))
	for _, want := range requested {
		switch want.Type {
		case string(models.DestinationChannel):
			channel, ok := h.channelByID(c, want.ChannelID, "BROADCAST_FAILED")
			if !ok {
				return nil, false
			}
			targets = append(targets, broadcastTarget{
				accountID: channel.WAAccountID, destination: models.DestinationChannel,
				channelJID: channel.JID,
			})
		case string(models.DestinationStatus):
			if !statusAccepts(kind) {
				c.JSON(http.StatusBadRequest, ErrorResponse{
					Error: "status hanya menerima teks, gambar, dan video",
					Code:  "STATUS_KIND_NOT_ALLOWED",
				})
				return nil, false
			}
			account, ok := h.accountByID(c, want.WAAccountID)
			if !ok {
				return nil, false
			}
			targets = append(targets, broadcastTarget{
				accountID: account.ID, destination: models.DestinationStatus,
			})
		default:
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error: fmt.Sprintf("tujuan tidak dikenal %q", want.Type),
				Code:  "UNKNOWN_DESTINATION",
			})
			return nil, false
		}
	}
	return targets, true
}

// statusAccepts answers whether WhatsApp will take this kind as a status. A
// document will not go up, and refusing here is what stops one being stored.
func statusAccepts(kind models.MessageKind) bool {
	switch kind {
	case models.MessageKindText, models.MessageKindImage, models.MessageKindVideo:
		return true
	default:
		return false
	}
}

// queueAll turns each resolved destination into a services.BroadcastPost and
// writes them together through QueueAll, which is transactional: a request
// with several destinations lands as all of its rows or none of them.
func (h *CSHandler) queueAll(
	targets []broadcastTarget, userID uuid.UUID,
	kind models.MessageKind, body string, media *services.MediaFile,
) ([]models.WABroadcastPost, error) {
	posts := make([]services.BroadcastPost, len(targets))
	for i, target := range targets {
		posts[i] = services.BroadcastPost{
			WAAccountID:  target.accountID,
			Destination:  target.destination,
			ChannelJID:   target.channelJID,
			SenderUserID: userID,
			Kind:         kind,
			Body:         body,
			Media:        media,
		}
	}
	return h.broadcasts.QueueAll(posts)
}

// channelByID turns a picked id into the channel it names, refusing one that
// is no longer in the mirror — which is what stops an update being queued to a
// channel this number may no longer post to.
func (h *CSHandler) channelByID(c *gin.Context, raw, code string) (*models.WAChannel, bool) {
	id, err := uuid.Parse(raw)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "saluran tidak valid", Code: "INVALID_CHANNEL_ID",
		})
		return nil, false
	}

	channel, err := h.channels.Get(id)
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error: "saluran tidak ditemukan", Code: code,
		})
		return nil, false
	case err != nil:
		// Anything else is the database failing, not the channel being gone.
		// Answering 404 for it would tell the sender their admin right was
		// revoked and send them chasing one they still have.
		mapCSError(c, err, code)
		return nil, false
	}
	return channel, true
}

// accountByID mirrors channelByID: an account id the mirror no longer knows is
// refused the same way a stale channel id is, so a request naming it queues
// nothing rather than failing hours later on the wa side.
func (h *CSHandler) accountByID(c *gin.Context, raw string) (*models.WAAccount, bool) {
	id, err := uuid.Parse(raw)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "akun WhatsApp tidak valid", Code: "INVALID_ACCOUNT_ID",
		})
		return nil, false
	}

	account, err := h.accounts.Get(id)
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error: "akun WhatsApp tidak ditemukan", Code: "BROADCAST_FAILED",
		})
		return nil, false
	case err != nil:
		mapCSError(c, err, "BROADCAST_FAILED")
		return nil, false
	}
	return account, true
}

// announceBroadcast wakes the wa process to drain now instead of waiting for
// its sweep, and tells the other browsers a row appeared. Redis carries no
// truth here — the row is already stored — so neither failure fails the
// request.
func (h *CSHandler) announceBroadcast(c *gin.Context) {
	ctx := c.Request.Context()
	h.announceEvent(ctx, wa.Event{Type: wa.EventBroadcastPost})
	if err := h.redis.Publish(ctx, wa.OutboxChannel, "").Err(); err != nil {
		h.logger.Warn("publish cs outbox notice failed", zap.Error(err))
	}
}
