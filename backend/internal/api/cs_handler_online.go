package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Online lists the agents currently holding the inbox open.
//
// Presence has been written on every stream heartbeat since the round-robin
// needed it; this only opens a window onto the set it already keeps. What it
// answers is narrower than "logged in": a browser claims presence only while
// the CS Inbox route itself is open, so someone reading the OLT map is absent
// from this list by design.
func (h *CSHandler) Online(c *gin.Context) {
	online, err := h.presence.Online(c.Request.Context())
	if err != nil {
		h.logger.Error("list online CS", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to list online agents",
			"code":  "PRESENCE_UNAVAILABLE",
		})
		return
	}
	// An empty inbox is a normal state, and the browser renders a list either
	// way — a nil slice would reach it as null and have to be guarded there.
	if online == nil {
		online = []uuid.UUID{}
	}
	c.JSON(http.StatusOK, gin.H{"data": online})
}
