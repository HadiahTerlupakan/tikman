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
	cs.GET("/groups/:id/avatar", h.csGroupHandler.ServeGroupAvatar)
	cs.GET("/groups/:id/member-avatar", h.csGroupHandler.ServeMemberAvatar)
	cs.PUT("/groups/:id/pin", h.csGroupHandler.PinGroup)
	cs.POST("/groups/:id/typing", h.csGroupHandler.SetGroupTyping)
	cs.POST("/groups/:id/history", h.csGroupHandler.RequestGroupHistory)
	cs.POST("/groups/:id/polls", h.csGroupHandler.CreateGroupPoll)
	cs.GET("/groups/:id/starred", h.csGroupHandler.StarredGroupMessages)

	// Acting on one message is the team's, like writing one. Deleting someone
	// else's message is checked inside the handler: it is an admin's call.
	msg := "/groups/:id/messages/:message_id"
	cs.PUT(msg, h.csGroupHandler.EditGroupMessage)
	cs.DELETE(msg, h.csGroupHandler.DeleteGroupMessage)
	cs.POST(msg+"/reaction", h.csGroupHandler.ReactToMessage)
	cs.PUT(msg+"/star", h.csGroupHandler.StarGroupMessage)
	cs.POST(msg+"/forward", h.csGroupHandler.ForwardGroupMessage)
	cs.POST(msg+"/reply-private", h.csGroupHandler.ReplyPrivately)
	cs.GET(msg+"/receipts", h.csGroupHandler.MessageReceipts)
	cs.POST(msg+"/vote", h.csGroupHandler.VoteInPoll)

	cs.POST("/groups", admin, h.csGroupHandler.CreateGroup)
	cs.POST("/groups/:id/actions", admin, h.csGroupHandler.ActOnGroup)
	// Who may read an answer is decided inside: an admin any, anyone else
	// only their own, since an admin's answer can be the invite link.
	cs.GET("/group-actions/:id", h.csGroupHandler.GetGroupAction)
}
