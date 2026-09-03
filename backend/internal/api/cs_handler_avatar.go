package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tikman/olt-provisioning/internal/wa"
)

// ServeAvatar hands back a customer's profile photo.
//
// The same shape as ServeMedia and for the same reason: the stored path comes
// out of a database row, so it is checked against the media root before
// anything is read, and the type is named from our own allowlist rather than
// left for the browser to guess at.
//
// Unlike an attachment it is served inline — it is drawn as a face in a list,
// not offered as a download — which is why wa.AvatarMime is a narrower list
// than the attachment one, with no shape that can carry script.
func (h *CSHandler) ServeAvatar(c *gin.Context) {
	convID, ok := pathUUID(c, "id", "INVALID_CONVERSATION_ID")
	if !ok {
		return
	}

	conv, err := h.conversations.Get(convID)
	if err != nil {
		mapCSError(c, err, "AVATAR_NOT_FOUND")
		return
	}

	mime := wa.AvatarMime(conv.AvatarPath)
	full, within := mediaPathWithin(h.mediaRoot, conv.AvatarPath)
	if conv.AvatarPath == "" || mime == "" || !within {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error: "conversation has no profile photo",
			Code:  "AVATAR_NOT_FOUND",
		})
		return
	}

	c.Header("Content-Type", mime)
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Security-Policy", "default-src 'none'; sandbox")
	c.File(full)
}
