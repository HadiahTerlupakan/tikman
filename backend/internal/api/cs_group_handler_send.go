package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/middleware"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"github.com/tikman/olt-provisioning/internal/wa"
	"go.uber.org/zap"
)

// SendGroupMessageRequest is a message to a group. Mentions are member JIDs,
// each written into Body as "@<address>" the way WhatsApp carries them.
type SendGroupMessageRequest struct {
	Body      string   `json:"body" binding:"required"`
	ReplyToID string   `json:"reply_to_id"`
	Mentions  []string `json:"mentions"`
}

// SendGroupMessage queues a text message to a group.
func (h *CSGroupHandler) SendGroupMessage(c *gin.Context) {
	group, ok := h.sendableGroup(c, "GROUP_SEND_FAILED")
	if !ok {
		return
	}
	var req SendGroupMessageRequest
	if !bindJSON(c, &req) {
		return
	}
	replyTo, ok := h.quoteTarget(c, group.ID, req.ReplyToID, "GROUP_SEND_FAILED")
	if !ok {
		return
	}
	if err := h.groups.CheckMentions(group.ID, req.Mentions); err != nil {
		mapCSError(c, err, "GROUP_SEND_FAILED")
		return
	}

	userID, _ := middleware.GetUserID(c)
	msg, err := h.messages.Queue(services.GroupOutgoing{
		GroupID: group.ID, SenderUserID: userID, SenderName: h.senderName(userID),
		Kind: models.MessageKindText, Body: h.inbox.sign(userID, req.Body), ReplyToID: replyTo,
		Mentions: req.Mentions,
	})
	if err != nil {
		mapCSError(c, err, "GROUP_SEND_FAILED")
		return
	}
	h.queued(c.Request.Context(), msg)
	c.JSON(http.StatusCreated, gin.H{"data": msg})
}

// SendGroupMedia queues an attachment to a group, through the same upload
// checks the customer inbox uses.
func (h *CSGroupHandler) SendGroupMedia(c *gin.Context) {
	group, ok := h.sendableGroup(c, "GROUP_SEND_MEDIA_FAILED")
	if !ok {
		return
	}
	replyTo, ok := h.quoteTarget(c, group.ID, c.Query("reply_to_id"), "GROUP_SEND_MEDIA_FAILED")
	if !ok {
		return
	}
	media, mime, ok := h.inbox.acceptUpload(c)
	if !ok {
		return
	}
	// Read after the upload: the form is only parsed once the body is bounded.
	mentions := c.PostFormArray("mentions")
	if err := h.groups.CheckMentions(group.ID, mentions); err != nil {
		h.inbox.removeOrphanedUpload(media.Path)
		mapCSError(c, err, "GROUP_SEND_MEDIA_FAILED")
		return
	}

	userID, _ := middleware.GetUserID(c)
	caption := strings.TrimSpace(c.PostForm("caption"))
	if caption != "" {
		caption = h.inbox.sign(userID, caption)
	}
	msg, err := h.messages.Queue(services.GroupOutgoing{
		GroupID: group.ID, SenderUserID: userID, SenderName: h.senderName(userID),
		Kind: kindForMime(mime), Body: caption, Media: media, ReplyToID: replyTo,
		Mentions: mentions,
	})
	if err != nil {
		h.inbox.removeOrphanedUpload(media.Path)
		mapCSError(c, err, "GROUP_SEND_MEDIA_FAILED")
		return
	}
	h.queued(c.Request.Context(), msg)
	c.JSON(http.StatusCreated, gin.H{"data": msg})
}

// sendableGroup refuses a group the number cannot post in: one it has left,
// and an admins-only group it does not administer. WhatsApp would refuse both
// anyway, but only after the message sat in the queue looking sent.
func (h *CSGroupHandler) sendableGroup(c *gin.Context, code string) (*models.WAGroup, bool) {
	group, ok := h.loadGroup(c, code)
	if !ok {
		return nil, false
	}
	switch {
	case group.Left:
		c.JSON(http.StatusConflict, ErrorResponse{
			Error: "Nomor ini sudah tidak menjadi anggota grup", Code: "GROUP_LEFT",
		})
		return nil, false
	case group.Announce && !group.IsAdmin:
		c.JSON(http.StatusConflict, ErrorResponse{
			Error: "Hanya admin grup yang boleh mengirim pesan di grup ini", Code: "GROUP_ANNOUNCE",
		})
		return nil, false
	}
	return group, true
}

// quoteTarget checks the message a reply quotes, answering nil for none.
func (h *CSGroupHandler) quoteTarget(c *gin.Context, groupID uuid.UUID, raw, code string) (*uuid.UUID, bool) {
	if raw == "" {
		return nil, true
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid ID format", Code: "INVALID_REPLY_TO_ID"})
		return nil, false
	}
	target, err := h.messages.QuoteTarget(groupID, id)
	if err != nil {
		mapCSError(c, err, code)
		return nil, false
	}
	return &target.ID, true
}

// senderName is the name a group message from TikMan is shown under. A user
// this cannot resolve leaves it empty; the browser falls back to "Anda".
func (h *CSGroupHandler) senderName(userID uuid.UUID) string {
	user, err := h.inbox.users.GetByID(userID)
	if err != nil {
		h.logger.Warn("Could not read the sender's name for a group message",
			zap.String("user_id", userID.String()), zap.Error(err))
		return ""
	}
	return user.Username
}

// queued tells the other browsers and wakes the wa process to send now.
func (h *CSGroupHandler) queued(ctx context.Context, msg *models.WAGroupMessage) {
	h.inbox.announceEvent(ctx, wa.Event{
		Type: wa.EventGroupMessage, GroupID: msg.GroupID.String(), MessageID: msg.ID.String(),
	})
	h.wake(ctx)
}

// wake asks the wa process to drain its queues; the periodic sweep is the net
// behind a lost one.
func (h *CSGroupHandler) wake(ctx context.Context) {
	if err := h.inbox.redis.Publish(ctx, wa.OutboxChannel, "").Err(); err != nil {
		h.logger.Warn("publish group outbox notice failed", zap.Error(err))
	}
}

// ServeGroupMedia streams a group attachment with the same guards as
// ServeMedia: the path is kept inside the media root and the type is the one
// the allowlist accepts.
func (h *CSGroupHandler) ServeGroupMedia(c *gin.Context) {
	msgID, ok := pathUUID(c, "message_id", "INVALID_MESSAGE_ID")
	if !ok {
		return
	}
	msg, err := h.messages.Get(msgID)
	if err != nil {
		mapCSError(c, err, "MEDIA_NOT_FOUND")
		return
	}
	full, ok := mediaPathWithin(h.inbox.mediaRoot, msg.MediaPath)
	if msg.MediaPath == "" || !ok {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "attachment not found", Code: "MEDIA_NOT_FOUND"})
		return
	}
	setMediaResponseHeaders(c, msg.MediaMime, msg.MediaFilename)
	c.File(full)
}
