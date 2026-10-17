package api

import (
	"github.com/gin-gonic/gin"
	"github.com/tikman/olt-provisioning/internal/middleware"
	"github.com/tikman/olt-provisioning/internal/models"
)

// registerCSGroupRoutes covers the WhatsApp groups. They hang off the CS
// group, so reading and writing in them is open to the same team as the
// inbox; managing a group — who is in it, what it is called, leaving it — is
// an admin's call, because it speaks for the number to everyone in the group.
func (h *handlers) registerCSGroupRoutes(cs *gin.RouterGroup) {
	admin := middleware.RequireRole(models.UserRoleAdmin)

	cs.GET("/groups", h.csGroupHandler.ListGroups)
	cs.POST("/groups/refresh", h.csGroupHandler.RefreshGroups)
	cs.GET("/groups/:id", h.csGroupHandler.GetGroup)
	cs.GET("/groups/:id/messages", h.csGroupHandler.GroupHistory)
	cs.POST("/groups/:id/messages", h.csGroupHandler.SendGroupMessage)
	cs.POST("/groups/:id/media", h.csGroupHandler.SendGroupMedia)
	cs.GET("/group-media/:message_id", h.csGroupHandler.ServeGroupMedia)

	cs.POST("/groups", admin, h.csGroupHandler.CreateGroup)
	cs.POST("/groups/:id/actions", admin, h.csGroupHandler.ActOnGroup)
	// Admin only like the requests themselves: an answer can be the group's
	// invite link, and anyone holding it can join the group.
	cs.GET("/group-actions/:id", admin, h.csGroupHandler.GetGroupAction)
}
