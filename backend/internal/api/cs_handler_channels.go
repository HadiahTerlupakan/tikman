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

// CreateChannelPostRequest is one text update as the composer sends it.
type CreateChannelPostRequest struct {
	ChannelID string `json:"channel_id" binding:"required"`
	Body      string `json:"body" binding:"required"`
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

// ListChannelPosts answers the broadcast history, newest first.
func (h *CSHandler) ListChannelPosts(c *gin.Context) {
	_, ok := h.channelFromQuery(c, "CHANNEL_HISTORY_FAILED")
	if !ok {
		return
	}

	rows, err := h.broadcasts.ListRecent(queryInt(c, "limit"))
	if err != nil {
		mapCSError(c, err, "CHANNEL_HISTORY_FAILED")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows})
}

// CreateChannelPost queues a text update.
func (h *CSHandler) CreateChannelPost(c *gin.Context) {
	var req CreateChannelPostRequest
	if !bindJSON(c, &req) {
		return
	}

	body := strings.TrimSpace(req.Body)
	if body == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "pembaruan tidak boleh kosong", Code: "EMPTY_UPDATE",
		})
		return
	}

	channel, ok := h.channelByID(c, req.ChannelID, "CHANNEL_POST_FAILED")
	if !ok {
		return
	}

	userID, _ := middleware.GetUserID(c)
	post, err := h.broadcasts.Queue(services.BroadcastPost{
		WAAccountID:  channel.WAAccountID,
		Destination:  models.DestinationChannel,
		ChannelJID:   channel.JID,
		SenderUserID: userID,
		Kind:         models.MessageKindText,
		Body:         body,
	})
	if err != nil {
		mapCSError(c, err, "CHANNEL_POST_FAILED")
		return
	}

	h.announceChannelPost(c)
	c.JSON(http.StatusCreated, gin.H{"data": post})
}

// CreateChannelPostMedia queues an update carrying an attachment. The file is
// stored before the row is written, the same order the chat path uses.
func (h *CSHandler) CreateChannelPostMedia(c *gin.Context) {
	channel, ok := h.channelFromQuery(c, "CHANNEL_POST_FAILED")
	if !ok {
		return
	}

	// The bound is applied to the body itself, not to what the multipart
	// header claims: everything downstream works on however many bytes
	// actually arrive.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadBytes)

	fileHeader, err := c.FormFile("file")
	if err != nil {
		refuseUpload(c, err)
		return
	}

	mime := wa.NormalizeMime(fileHeader.Header.Get("Content-Type"))
	ext, ok := validateAttachmentMime(c, mime)
	if !ok {
		return
	}

	media, err := h.storeUpload(fileHeader, mime, ext)
	if err != nil {
		h.logger.Error("store channel attachment failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "gagal menyimpan lampiran", Code: "MEDIA_STORE_FAILED",
		})
		return
	}

	userID, _ := middleware.GetUserID(c)
	post, err := h.broadcasts.Queue(services.BroadcastPost{
		WAAccountID:  channel.WAAccountID,
		Destination:  models.DestinationChannel,
		ChannelJID:   channel.JID,
		SenderUserID: userID,
		Kind:         kindForMime(mime),
		Body:         strings.TrimSpace(c.PostForm("caption")),
		Media:        media,
	})
	if err != nil {
		h.removeOrphanedUpload(media.Path)
		mapCSError(c, err, "CHANNEL_POST_FAILED")
		return
	}

	h.announceChannelPost(c)
	c.JSON(http.StatusCreated, gin.H{"data": post})
}

// validateAttachmentMime refuses an upload whose declared type is not on the
// allowlist before anything is written to disk. The check runs on the type
// alone, ahead of storeUpload, because html/svg/xml are deliberately absent
// from the allowlist — ServeMedia later hands attachments back from the API's
// own origin, and a rejected file must never have touched disk in the first
// place for that guarantee to hold.
func validateAttachmentMime(c *gin.Context, mime string) (string, bool) {
	ext, allowed := wa.AllowedExtension(mime)
	if !allowed {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: fmt.Sprintf("tipe lampiran %q tidak diterima", mime),
			Code:  "MEDIA_TYPE_NOT_ALLOWED",
		})
		return "", false
	}
	return ext, true
}

// channelFromQuery resolves the channel_id query parameter.
func (h *CSHandler) channelFromQuery(c *gin.Context, code string) (*models.WAChannel, bool) {
	return h.channelByID(c, c.Query("channel_id"), code)
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

// announceChannelPost wakes the wa process to drain now instead of waiting for
// its sweep, and tells the other browsers a row appeared. Redis carries no
// truth here — the row is already stored — so neither failure fails the
// request.
func (h *CSHandler) announceChannelPost(c *gin.Context) {
	ctx := c.Request.Context()
	h.announceEvent(ctx, wa.Event{Type: wa.EventBroadcastPost})
	if err := h.redis.Publish(ctx, wa.OutboxChannel, "").Err(); err != nil {
		h.logger.Warn("publish cs outbox notice failed", zap.Error(err))
	}
}
