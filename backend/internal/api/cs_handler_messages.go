package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	c.JSON(http.StatusOK, gin.H{"data": rows})
}

// Send queues a text reply on a thread the caller holds.
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
	if err := h.conversations.EnsureHolder(convID, userID); err != nil {
		h.refuseNotHolder(c, convID, err, "SEND_FAILED")
		return
	}

	msg, err := h.messages.Queue(convID, userID, models.MessageKindText, req.Body, nil)
	if err != nil {
		mapCSError(c, err, "SEND_FAILED")
		return
	}

	h.announce(c.Request.Context(), convID, msg.ID)
	c.JSON(http.StatusCreated, gin.H{"data": msg})
}

// SendMedia queues an attachment on a thread the caller holds. The file is
// stored under mediaRoot before the message row is written, the same way a
// media message arriving from WhatsApp is stored before its row.
func (h *CSHandler) SendMedia(c *gin.Context) {
	convID, ok := pathUUID(c, "id", "INVALID_CONVERSATION_ID")
	if !ok {
		return
	}

	userID, _ := middleware.GetUserID(c)
	if err := h.conversations.EnsureHolder(convID, userID); err != nil {
		h.refuseNotHolder(c, convID, err, "SEND_MEDIA_FAILED")
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "a file is required", Code: "MEDIA_REQUIRED"})
		return
	}

	media, kind, err := h.storeUpload(fileHeader)
	if err != nil {
		h.logger.Error("store outgoing attachment failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to store attachment", Code: "MEDIA_STORE_FAILED"})
		return
	}

	msg, err := h.messages.Queue(convID, userID, kind, c.PostForm("caption"), media)
	if err != nil {
		mapCSError(c, err, "SEND_MEDIA_FAILED")
		return
	}

	h.announce(c.Request.Context(), convID, msg.ID)
	c.JSON(http.StatusCreated, gin.H{"data": msg})
}

// refuseNotHolder answers 409 for anyone but the CS holding this thread,
// naming who does — otherwise the person reads "someone else" and has to ask
// around to find out who to actually go talk to.
func (h *CSHandler) refuseNotHolder(c *gin.Context, convID uuid.UUID, err error, code string) {
	if !errors.Is(err, services.ErrNotHolder) {
		mapCSError(c, err, code)
		return
	}

	holder := "orang lain"
	if conv, gerr := h.conversations.Get(convID); gerr == nil && conv.AssignedUserID != nil {
		holder = conv.AssignedUserID.String()
	}
	c.JSON(http.StatusConflict, ErrorResponse{
		Error: fmt.Sprintf("percakapan ini sedang dipegang oleh %s", holder),
		Code:  "NOT_HOLDER",
	})
}

// announce tells other CS browsers about a new reply and wakes the wa process
// to drain it now instead of waiting for its 30-second sweep. Redis carries no
// truth here — the message is already stored — so a failure to publish either
// is logged and never fails the request; the sweep still picks the reply up.
func (h *CSHandler) announce(ctx context.Context, convID, msgID uuid.UUID) {
	event := wa.Event{Type: wa.EventMessage, ConversationID: convID.String(), MessageID: msgID.String()}
	if err := h.publisher.Publish(ctx, event); err != nil {
		h.logger.Warn("publish cs event failed", zap.Error(err))
	}
	if err := h.redis.Publish(ctx, wa.OutboxChannel, "").Err(); err != nil {
		h.logger.Warn("publish cs outbox notice failed", zap.Error(err))
	}
}

// storeUpload writes an outgoing attachment to <mediaRoot>/<year>/<month>/<uuid><ext>
// and reports what kind of message it makes. The extension comes from the
// uploaded file's own name, but filepath.Ext never returns anything past the
// last path separator, so a crafted name cannot walk the write outside that
// directory.
func (h *CSHandler) storeUpload(header *multipart.FileHeader) (*services.MediaFile, models.MessageKind, error) {
	mime := header.Header.Get("Content-Type")
	rel := filepath.Join(time.Now().Format("2006"), time.Now().Format("01"), uuid.NewString()+filepath.Ext(header.Filename))
	full := filepath.Join(h.mediaRoot, rel)

	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		return nil, "", fmt.Errorf("create media directory: %w", err)
	}

	src, err := header.Open()
	if err != nil {
		return nil, "", fmt.Errorf("open upload: %w", err)
	}
	defer src.Close()

	dst, err := os.OpenFile(full, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		return nil, "", fmt.Errorf("create media file: %w", err)
	}
	defer dst.Close()

	written, err := io.Copy(dst, src)
	if err != nil {
		_ = os.Remove(full)
		return nil, "", fmt.Errorf("write media file: %w", err)
	}

	return &services.MediaFile{Path: rel, Mime: mime, Filename: header.Filename, Size: written}, kindForMime(mime), nil
}

// kindForMime buckets an uploaded file's declared type into what WhatsApp
// distinguishes; anything unrecognised goes as a document, the one form that
// carries any file.
func kindForMime(mime string) models.MessageKind {
	switch {
	case strings.HasPrefix(mime, "image/"):
		return models.MessageKindImage
	case strings.HasPrefix(mime, "video/"):
		return models.MessageKindVideo
	case strings.HasPrefix(mime, "audio/"):
		return models.MessageKindAudio
	default:
		return models.MessageKindDocument
	}
}

// ServeMedia streams a stored attachment. MediaPath comes out of the
// database, not the request, but a corrupted or tampered row must not become
// an arbitrary file read — so the joined path is checked against mediaRoot
// before anything is opened.
func (h *CSHandler) ServeMedia(c *gin.Context) {
	msgID, ok := pathUUID(c, "message_id", "INVALID_MESSAGE_ID")
	if !ok {
		return
	}

	msg, err := h.messages.Get(msgID)
	if err != nil {
		mapCSError(c, err, "MEDIA_NOT_FOUND")
		return
	}
	if msg.MediaPath == "" {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "message has no attachment", Code: "MEDIA_NOT_FOUND"})
		return
	}

	full, ok := mediaPathWithin(h.mediaRoot, msg.MediaPath)
	if !ok {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "attachment not found", Code: "MEDIA_NOT_FOUND"})
		return
	}

	c.File(full)
}

// mediaPathWithin joins a stored relative path onto the media root and
// reports whether the result is still inside it.
func mediaPathWithin(root, rel string) (string, bool) {
	cleanRoot := filepath.Clean(root)
	full := filepath.Join(cleanRoot, rel)
	if full != cleanRoot && !strings.HasPrefix(full, cleanRoot+string(filepath.Separator)) {
		return "", false
	}
	return full, true
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
