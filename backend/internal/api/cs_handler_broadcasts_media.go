package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tikman/olt-provisioning/internal/middleware"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/wa"
	"go.uber.org/zap"
)

// CreateBroadcastMedia queues an announcement carrying an attachment, aimed at
// one or more destinations named as repeated query parameters: channel_id for
// a channel, status_account_id for a status.
//
// Destinations are resolved once the MIME is known but before storeUpload
// runs, so a document aimed at a status is refused with no file ever written
// — the same guarantee the text path gets from resolving before Queue, just
// moved earlier here because storing the upload is itself the side effect to
// avoid.
func (h *CSHandler) CreateBroadcastMedia(c *gin.Context) {
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
	kind := kindForMime(mime)

	targets, ok := h.resolveDestinations(c, broadcastMediaDestinations(c), kind)
	if !ok {
		return
	}

	media, err := h.storeUpload(fileHeader, mime, ext)
	if err != nil {
		h.logger.Error("store broadcast attachment failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "gagal menyimpan lampiran", Code: "MEDIA_STORE_FAILED",
		})
		return
	}

	userID, _ := middleware.GetUserID(c)
	body := strings.TrimSpace(c.PostForm("caption"))
	queued, err := h.queueAll(targets, userID, kind, body, media)
	if err != nil {
		h.removeOrphanedUpload(media.Path)
		mapCSError(c, err, "BROADCAST_FAILED")
		return
	}

	h.announceBroadcast(c)
	c.JSON(http.StatusCreated, gin.H{"data": queued})
}

// broadcastMediaDestinations reads the media form's destinations from
// repeated query parameters, since a multipart body carries no JSON array.
func broadcastMediaDestinations(c *gin.Context) []BroadcastDestinationRequest {
	var out []BroadcastDestinationRequest
	for _, id := range c.QueryArray("channel_id") {
		out = append(out, BroadcastDestinationRequest{Type: string(models.DestinationChannel), ChannelID: id})
	}
	for _, id := range c.QueryArray("status_account_id") {
		out = append(out, BroadcastDestinationRequest{Type: string(models.DestinationStatus), WAAccountID: id})
	}
	return out
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
