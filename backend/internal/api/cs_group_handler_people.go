package api

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/wa"
	"go.uber.org/zap"
)

// ServeGroupAvatar streams a group's photo.
func (h *CSGroupHandler) ServeGroupAvatar(c *gin.Context) {
	group, ok := h.loadGroup(c, "AVATAR_NOT_FOUND")
	if !ok {
		return
	}
	h.serveAvatar(c, group.WAAccountID, group.JID)
}

// ServeMemberAvatar streams a member's photo as this group's number sees it.
func (h *CSGroupHandler) ServeMemberAvatar(c *gin.Context) {
	group, ok := h.loadGroup(c, "AVATAR_NOT_FOUND")
	if !ok {
		return
	}
	h.serveAvatar(c, group.WAAccountID, c.Query("jid"))
}

// serveAvatar answers one photo with the guards ServeAvatar uses: the path
// stays inside the media root and the type is one an image may be.
func (h *CSGroupHandler) serveAvatar(c *gin.Context, accountID uuid.UUID, jid string) {
	path, err := h.avatars.Path(accountID, jid)
	if err != nil {
		mapCSError(c, err, "AVATAR_NOT_FOUND")
		return
	}
	mime := wa.AvatarMime(path)
	full, within := mediaPathWithin(h.inbox.mediaRoot, path)
	if path == "" || mime == "" || !within {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "no profile photo", Code: "AVATAR_NOT_FOUND"})
		return
	}
	c.Header("Content-Type", mime)
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Security-Policy", "default-src 'none'; sandbox")
	// A face changes rarely and the list draws dozens; an hour spares the
	// API every one of those requests on each refetch.
	c.Header("Cache-Control", "private, max-age=3600")
	c.File(full)
}

// PinRequest pins a group to the top of the list or lets it go.
type PinRequest struct {
	Pinned bool `json:"pinned"`
}

// PinGroup pins or unpins a group for the whole team.
func (h *CSGroupHandler) PinGroup(c *gin.Context) {
	group, ok := h.loadGroup(c, "GROUP_PIN_FAILED")
	if !ok {
		return
	}
	var req PinRequest
	if !bindJSON(c, &req) {
		return
	}
	pinned, err := h.groups.SetPinned(group.ID, req.Pinned)
	if err != nil {
		mapCSError(c, err, "GROUP_PIN_FAILED")
		return
	}
	h.inbox.announceEvent(c.Request.Context(), wa.Event{Type: wa.EventGroup, GroupID: group.ID.String()})
	c.JSON(http.StatusOK, gin.H{"data": pinned})
}

// SetGroupTyping shows the group that someone on the team is writing. Like
// the inbox's, a lost update costs a line that was worth nothing a moment
// later, so a failure to publish is logged and the request still succeeds.
func (h *CSGroupHandler) SetGroupTyping(c *gin.Context) {
	group, ok := h.sendableGroup(c, "TYPING_FAILED")
	if !ok {
		return
	}
	var req TypingRequest
	if !bindJSON(c, &req) {
		return
	}
	payload, err := json.Marshal(wa.PresenceMessage{GroupID: group.ID.String(), Typing: req.Typing})
	if err != nil {
		mapCSError(c, err, "TYPING_FAILED")
		return
	}
	if err := h.inbox.redis.Publish(c.Request.Context(), wa.PresenceChannel, payload).Err(); err != nil {
		h.logger.Warn("publish group typing update failed", zap.String("group_id", group.ID.String()), zap.Error(err))
	}
	c.Status(http.StatusNoContent)
}
