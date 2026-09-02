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

	// The allowlist check happens on the declared type before anything is
	// written, not after: html/svg/xhtml/xml are deliberately absent from it,
	// because ServeMedia later hands this file back from the API's own origin,
	// and a CS must not be able to put a script in front of a customer there.
	mime := wa.NormalizeMime(fileHeader.Header.Get("Content-Type"))
	ext, allowed := wa.AllowedExtension(mime)
	if !allowed {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: fmt.Sprintf("attachment type %q is not accepted", mime),
			Code:  "MEDIA_TYPE_NOT_ALLOWED",
		})
		return
	}

	media, err := h.storeUpload(fileHeader, mime, ext)
	if err != nil {
		h.logger.Error("store outgoing attachment failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to store attachment", Code: "MEDIA_STORE_FAILED"})
		return
	}

	msg, err := h.messages.Queue(convID, userID, kindForMime(mime), c.PostForm("caption"), media)
	if err != nil {
		h.removeOrphanedUpload(media.Path)
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

// storeUpload writes an outgoing attachment to <mediaRoot>/<year>/<month>/<uuid><ext>.
// mime and ext are the caller's already-allowlisted values (see SendMedia) —
// this never derives either from the uploader's declared Content-Type or
// filename, which is what let a mislabelled upload come back as HTML before.
func (h *CSHandler) storeUpload(header *multipart.FileHeader, mime, ext string) (*services.MediaFile, error) {
	rel := filepath.Join(time.Now().Format("2006"), time.Now().Format("01"), uuid.NewString()+ext)
	full := filepath.Join(h.mediaRoot, rel)

	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		return nil, fmt.Errorf("create media directory: %w", err)
	}

	src, err := header.Open()
	if err != nil {
		return nil, fmt.Errorf("open upload: %w", err)
	}
	defer src.Close()

	dst, err := os.OpenFile(full, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		return nil, fmt.Errorf("create media file: %w", err)
	}
	defer dst.Close()

	written, err := io.Copy(dst, src)
	if err != nil {
		_ = os.Remove(full)
		return nil, fmt.Errorf("write media file: %w", err)
	}

	return &services.MediaFile{Path: rel, Mime: mime, Filename: header.Filename, Size: written}, nil
}

// removeOrphanedUpload deletes a file storeUpload just wrote when the message
// row that was supposed to own it never got created — the same class of leak
// Task 10 closed for inbound media, now on the outbound side too.
func (h *CSHandler) removeOrphanedUpload(rel string) {
	full := filepath.Join(h.mediaRoot, rel)
	if err := os.Remove(full); err != nil && !os.IsNotExist(err) {
		h.logger.Warn("remove orphaned upload failed", zap.String("path", rel), zap.Error(err))
	}
}

// kindForMime buckets an uploaded file's declared type into what WhatsApp
// distinguishes; anything unrecognised goes as a document, the one form that
// carries any file. Called only after wa.AllowedExtension has already
// accepted the mime, so default here is unreachable in practice — it exists
// because the switch has to be exhaustive over more than three kinds.
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
// before anything is opened. The response headers are equally deliberate: the
// browser is told the type this file was actually stored as, not left to
// guess one from the extension or the bytes.
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

	setMediaResponseHeaders(c, msg)
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

// setMediaResponseHeaders locks the response to exactly the type a message
// was stored with. Without this, gin's c.File delegates to http.ServeFile,
// which infers Content-Type from the served path's extension (or by sniffing
// its bytes) — and since an upload's extension came from an allowlisted mime
// (see SendMedia), setting it explicitly here is what actually makes the
// allowlist binding, rather than leaving a second, independent inference to
// possibly disagree with it. Content-Disposition only changes how a direct
// fetch of this URL is offered to save; it is ignored for a subresource
// fetch like <img src>, so the inbox still renders photos inline.
func setMediaResponseHeaders(c *gin.Context, msg *models.CSMessage) {
	mime := msg.MediaMime
	if mime == "" {
		mime = "application/octet-stream"
	}
	c.Header("Content-Type", mime)
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", sanitizeContentDispositionFilename(msg.MediaFilename)))
	c.Header("Content-Security-Policy", "default-src 'none'; sandbox")
}

// sanitizeContentDispositionFilename keeps a stored filename safe to place
// inside a quoted header value: no quote to end the string early, no control
// character (a CR/LF pair among them) to inject a second header line.
func sanitizeContentDispositionFilename(name string) string {
	var b strings.Builder
	for _, r := range name {
		if r == '"' || r == '\\' || r < 0x20 {
			continue
		}
		b.WriteRune(r)
	}
	if b.Len() == 0 {
		return "attachment"
	}
	return b.String()
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
