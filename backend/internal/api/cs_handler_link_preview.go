package api

import (
	"encoding/base64"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tikman/olt-provisioning/internal/wa/linkpreview"
)

// linkPreviewResponse is the card the composer draws while a CS types.
type linkPreviewResponse struct {
	URL         string `json:"url"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	// Thumbnail is a base64 JPEG, empty when the page named no usable image.
	Thumbnail string `json:"thumbnail,omitempty"`
}

// LinkPreview resolves the first link in a draft so the composer can show what
// the customer will receive.
//
// It takes the whole draft rather than a URL so that "what counts as a link"
// has one definition, shared with the send path.
//
// What it answers is display only. The send path in the wa process resolves
// the page again for itself and ignores anything the browser reports, so a
// forged card here cannot change the message a customer receives — it would
// only mislead the CS who typed it.
//
// This runs in the api process, which carries wg0 and can therefore reach
// every OLT. linkpreview's address guard is what stops a typed plant address
// from becoming a probe of the operator's own network; see
// TestTheTunnelAndThePlantAreRefused, which pins the deployed ranges.
func (h *CSHandler) LinkPreview(c *gin.Context) {
	preview := linkpreview.Resolve(c.Request.Context(), c.Query("text"))
	if preview == nil {
		// Not an error: no link, an unreachable one, or a page with nothing
		// worth showing all mean the composer draws no card.
		c.JSON(http.StatusOK, gin.H{"data": nil})
		return
	}

	out := linkPreviewResponse{
		URL:         preview.URL,
		Title:       preview.Title,
		Description: preview.Description,
	}
	if len(preview.Thumbnail) > 0 {
		out.Thumbnail = base64.StdEncoding.EncodeToString(preview.Thumbnail)
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}
