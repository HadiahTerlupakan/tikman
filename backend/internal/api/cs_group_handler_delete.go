package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tikman/olt-provisioning/internal/middleware"
	"github.com/tikman/olt-provisioning/internal/services"
	"github.com/tikman/olt-provisioning/internal/wa"
	"go.uber.org/zap"
)

// DeleteGroup removes a group the number has already left from TikMan, with
// its history and files. Only the api's rows and the media root change, never
// WhatsApp, so the wa process and its sessions are left alone.
func (h *CSGroupHandler) DeleteGroup(c *gin.Context) {
	group, ok := h.loadGroup(c, "GROUP_DELETE_FAILED")
	if !ok {
		return
	}

	err := h.groups.DeleteLeft(group.ID, h.inbox.mediaRoot)
	switch {
	case errors.Is(err, services.ErrGroupNotLeft):
		c.JSON(http.StatusConflict, ErrorResponse{Error: err.Error(), Code: "GROUP_NOT_LEFT"})
		return
	case errors.Is(err, services.ErrGroupMediaLeftBehind):
		// The rows are gone and cannot come back; a few stray files are worth a
		// log line, not telling the admin a delete failed that did not.
		h.logger.Warn("deleted group left media on disk", zap.String("group_id", group.ID.String()), zap.Error(err))
	case err != nil:
		mapCSError(c, err, "GROUP_DELETE_FAILED")
		return
	}

	actorID, _ := middleware.GetUserID(c)
	old := map[string]interface{}{"name": group.Name, "jid": group.JID}
	if err := h.inbox.audit.Log(actorID, "delete", "wa_group", group.ID, old, nil,
		c.ClientIP(), c.Request.UserAgent()); err != nil {
		h.logger.Warn("audit log for wa group deletion failed", zap.Error(err))
	}

	h.inbox.announceEvent(c.Request.Context(), wa.Event{Type: wa.EventGroup, GroupID: group.ID.String()})
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"deleted": true}})
}
