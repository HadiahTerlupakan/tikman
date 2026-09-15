package api

import (
	"context"
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
)

// History returns one page of a thread, newest first.
func (h *CSHandler) History(c *gin.Context) {
	convID, ok := pathUUID(c, "id", "INVALID_CONVERSATION_ID")
	if !ok {
		return
	}

	rows, err := h.messages.History(convID, queryInt(c, "limit"), queryInt(c, "offset"))
	if err != nil {
		mapCSError(c, err, "HISTORY_FAILED")
		return
	}

	h.markThreadRead(c, convID)
	c.JSON(http.StatusOK, gin.H{"data": rows})
}

// markThreadRead drops the unread badge on a thread a CS just opened, and
// tells the other browsers so their lists drop it too. Nothing else in the
// module clears the count, so without this the badge grows for the life of
// the thread and stops meaning anything.
//
// The nudge goes out only when the count actually changed. History is what a
// browser calls in answer to a nudge naming this thread, so announcing a read
// that changed nothing would keep feeding itself. A failure clears nothing
// and is logged: the CS asked for the history, not for the badge.
func (h *CSHandler) markThreadRead(c *gin.Context, convID uuid.UUID) {
	cleared, err := h.conversations.MarkRead(convID)
	if err != nil {
		h.logger.Warn("clear unread count failed",
			zap.String("conversation_id", convID.String()), zap.Error(err))
		return
	}
	if !cleared {
		return
	}
	h.announceEvent(c.Request.Context(), wa.Event{
		Type:           wa.EventStatus,
		ConversationID: convID.String(),
	})
}

// sign puts the sender's initials on a reply. A username this cannot resolve
// leaves the body untouched: a missing signature is a smaller problem than a
// refused reply, and the CS would have no way to act on the failure anyway.
func (h *CSHandler) sign(userID uuid.UUID, body string) string {
	user, err := h.users.GetByID(userID)
	if err != nil {
		h.logger.Warn("Could not read the sender's name to sign a reply",
			zap.String("user_id", userID.String()), zap.Error(err))
		return body
	}
	return signReply(body, user.Initials)
}

// Send queues a text reply on a thread the caller holds, or claims one nobody
// holds (see holdForReply).
func (h *CSHandler) Send(c *gin.Context) {
	convID, ok := pathUUID(c, "id", "INVALID_CONVERSATION_ID")
	if !ok {
		return
	}
	var req SendMessageRequest
	if !bindJSON(c, &req) {
		return
	}

	userID, _ := middleware.GetUserID(c)
	quoted, ok := h.quoteTarget(c, convID, req.ReplyToID, "SEND_FAILED")
	if !ok {
		return
	}
	if !h.holdForReply(c, convID, userID, "SEND_FAILED") {
		return
	}

	msg, err := h.messages.Queue(convID, userID, models.MessageKindText, h.sign(userID, req.Body), nil, quotedID(quoted))
	if err != nil {
		mapCSError(c, err, "SEND_FAILED")
		return
	}
	attachQuote(msg, quoted)

	h.announce(c.Request.Context(), convID, msg.ID)
	c.JSON(http.StatusCreated, gin.H{"data": msg})
}

// SendMedia queues an attachment on a thread the caller holds, or claims one
// nobody holds. The file is stored under mediaRoot before the message row is
// written, the same way a media message arriving from WhatsApp is stored
// before its row.
func (h *CSHandler) SendMedia(c *gin.Context) {
	convID, ok := pathUUID(c, "id", "INVALID_CONVERSATION_ID")
	if !ok {
		return
	}

	userID, _ := middleware.GetUserID(c)
	quoted, ok := h.quoteTarget(c, convID, c.Query("reply_to_id"), "SEND_MEDIA_FAILED")
	if !ok {
		return
	}

	media, mime, ok := h.acceptUpload(c)
	if !ok {
		return
	}
	if !h.holdForReply(c, convID, userID, "SEND_MEDIA_FAILED") {
		h.removeOrphanedUpload(media.Path)
		return
	}

	// An empty caption stays empty: a photo whose whole caption is "~BS" tells
	// the customer nothing and reads as a stray character.
	caption := strings.TrimSpace(c.PostForm("caption"))
	if caption != "" {
		caption = h.sign(userID, caption)
	}

	msg, err := h.messages.Queue(convID, userID, kindForMime(mime), caption, media, quotedID(quoted))
	if err != nil {
		h.removeOrphanedUpload(media.Path)
		mapCSError(c, err, "SEND_MEDIA_FAILED")
		return
	}
	attachQuote(msg, quoted)

	h.announce(c.Request.Context(), convID, msg.ID)
	c.JSON(http.StatusCreated, gin.H{"data": msg})
}

// acceptUpload takes the attachment off the request and stores it, answering
// the request itself on every refusal.
//
// The allowlist check happens on the declared type before anything is written,
// not after: html/svg/xhtml/xml are deliberately absent from it, because
// ServeMedia later hands this file back from the API's own origin, and a CS
// must not be able to put a script in front of a customer there.
func (h *CSHandler) acceptUpload(c *gin.Context) (*services.MediaFile, string, bool) {
	// The bound is applied to the body itself, not to what the multipart
	// header claims: everything downstream — storeUpload's copy to disk, and
	// wa.Client.SendMedia reading the file whole to hand it to whatsmeow —
	// works on however many bytes actually arrive.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadBytes)

	fileHeader, err := c.FormFile("file")
	if err != nil {
		refuseUpload(c, err)
		return nil, "", false
	}

	mime := wa.NormalizeMime(fileHeader.Header.Get("Content-Type"))
	ext, allowed := wa.AllowedExtension(mime)
	if !allowed {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: fmt.Sprintf("attachment type %q is not accepted", mime),
			Code:  "MEDIA_TYPE_NOT_ALLOWED",
		})
		return nil, "", false
	}

	media, err := h.storeUpload(fileHeader, mime, ext)
	if err != nil {
		h.logger.Error("store outgoing attachment failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to store attachment", Code: "MEDIA_STORE_FAILED"})
		return nil, "", false
	}
	return media, mime, true
}

// refuseUpload answers the two ways a multipart body yields no file: one that
// ran past maxUploadBytes, and one that never carried a file at all. The first
// needs its own sentence — "a file is required" for a photo the CS can plainly
// see they attached reads as a broken page.
func refuseUpload(c *gin.Context, err error) {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: fmt.Sprintf("lampiran melebihi batas %d MB", maxUploadBytes>>20),
			Code:  "MEDIA_TOO_LARGE",
		})
		return
	}
	c.JSON(http.StatusBadRequest, ErrorResponse{Error: "a file is required", Code: "MEDIA_REQUIRED"})
}

// refuseNotHolder answers 409 for anyone but the CS holding this thread,
// naming who does — otherwise the person reads "someone else" and has to ask
// around to find out who to actually go talk to.
func (h *CSHandler) refuseNotHolder(c *gin.Context, convID uuid.UUID, err error, code string) {
	if !errors.Is(err, services.ErrNotHolder) {
		mapCSError(c, err, code)
		return
	}

	// The name the inbox shows, so a CS who lost the race to answer knows who
	// won it.
	holder := "orang lain"
	if conv, gerr := h.conversations.Get(convID); gerr == nil && conv.AssignedUserID != nil {
		if user, uerr := h.users.GetByID(*conv.AssignedUserID); uerr == nil {
			holder = user.Username
		}
	}
	c.JSON(http.StatusConflict, ErrorResponse{
		Error: fmt.Sprintf("Percakapan ini sedang dilayani %s", holder),
		Code:  "NOT_HOLDER",
	})
}

// holdForReply lets the sender answer: it claims a thread nobody holds, or
// confirms they already hold it, and answers the request itself on a refusal.
// Opening a thread never assigns it; answering is what makes a CS the one who
// serves the customer. A claim is audited and announced like any other
// handover, so the other agents' screens stop offering the thread.
func (h *CSHandler) holdForReply(c *gin.Context, convID, userID uuid.UUID, code string) bool {
	claimed, err := h.conversations.ClaimForReply(convID, userID)
	if err != nil {
		h.refuseNotHolder(c, convID, err, code)
		return false
	}
	if claimed {
		h.auditHandover(c, convID, nil, userID)
		h.announceEvent(c.Request.Context(), wa.Event{
			Type:           wa.EventAssignment,
			ConversationID: convID.String(),
		})
	}
	return true
}

// announce tells other CS browsers about a new reply and wakes the wa process
// to drain it now instead of waiting for its 30-second sweep. Redis carries no
// truth here — the message is already stored — so a failure to publish either
// is logged and never fails the request; the sweep still picks the reply up.
func (h *CSHandler) announce(ctx context.Context, convID, msgID uuid.UUID) {
	h.announceEvent(ctx, wa.Event{
		Type:           wa.EventMessage,
		ConversationID: convID.String(),
		MessageID:      msgID.String(),
	})
	if err := h.redis.Publish(ctx, wa.OutboxChannel, "").Err(); err != nil {
		h.logger.Warn("publish cs outbox notice failed", zap.Error(err))
	}
}

// announceEvent nudges the other browsers and stops there. A change that puts
// nothing in the outbox — a takeover, a close, an ONT link, a thread being
// read — must not also wake the wa process to drain a queue with nothing new
// in it.
func (h *CSHandler) announceEvent(ctx context.Context, event wa.Event) {
	if err := h.publisher.Publish(ctx, event); err != nil {
		h.logger.Warn("publish cs event failed", zap.Error(err))
	}
}

// SearchMessages finds messages by their words, across every thread.
func (h *CSHandler) SearchMessages(c *gin.Context) {
	term := c.Query("q")
	if term == "" {
		c.JSON(http.StatusOK, gin.H{"data": []models.CSMessage{}})
		return
	}

	rows, err := h.messages.Search(term, queryInt(c, "limit"))
	if err != nil {
		mapCSError(c, err, "SEARCH_FAILED")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows})
}
