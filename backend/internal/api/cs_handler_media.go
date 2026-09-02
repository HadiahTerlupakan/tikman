package api

import (
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
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"github.com/tikman/olt-provisioning/internal/wa"
	"go.uber.org/zap"
)

// maxUploadBytes is what one outbound attachment may weigh. The whole file is
// held in memory twice on its way out — storeUpload copies it to disk, and
// wa.Client.SendMedia reads it back whole to hand to whatsmeow — so this is
// the only thing standing between a CS's upload and the wa container's RAM.
// 16 MiB is also roughly where WhatsApp itself stops accepting a photo.
const maxUploadBytes = 16 << 20

// storeUpload writes an outgoing attachment to <mediaRoot>/<year>/<month>/<uuid><ext>.
// mime and ext are the caller's already-allowlisted values (see SendMedia) —
// this never derives either from the uploader's declared Content-Type or
// filename, which is what let a mislabelled upload come back as HTML before.
// The display filename goes through wa.ClampFilename, the same guard the
// inbound path uses: media_filename is varchar(255), and an over-long name
// here would otherwise fail the insert on Postgres — a 500 the SQLite tests
// cannot catch — where a graceful truncation should do instead.
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

	return &services.MediaFile{Path: rel, Mime: mime, Filename: wa.ClampFilename(header.Filename), Size: written}, nil
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
// possibly disagree with it.
//
// The stored MediaMime itself is only trusted when wa.AllowedExtension
// accepts it. An outbound upload always satisfies that — SendMedia refuses
// anything else before it is stored — but an inbound message is stored with
// whatever the customer's client declared: NormalizeMime truncates and caps
// that value, it does not allowlist it. Echoing an unchecked inbound mime
// straight into the response would let the storing side's necessary leniency
// (a message that already arrived must be stored somehow) become the serving
// side's problem too — the same asymmetry that made the upload path
// exploitable before SendMedia was gated. Content-Disposition only changes
// how a direct fetch of this URL is offered to save; it is ignored for a
// subresource fetch like <img src>, so the inbox still renders photos inline.
func setMediaResponseHeaders(c *gin.Context, msg *models.CSMessage) {
	mime := "application/octet-stream"
	if _, allowed := wa.AllowedExtension(msg.MediaMime); allowed {
		mime = msg.MediaMime
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
