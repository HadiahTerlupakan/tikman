package api

import (
	"github.com/gin-gonic/gin"
	"github.com/tikman/olt-provisioning/internal/middleware"
	"github.com/tikman/olt-provisioning/internal/models"
)

// registerSwitchRoutes covers the managed switches. Reads are open to every
// role; adding, editing and deleting are admin decisions, because editing the
// host would let a technician point the poller, and the credentials it sends,
// at a machine they control.
func (h *handlers) registerSwitchRoutes(api *gin.RouterGroup, authenticated gin.HandlerFunc) {
	admin := middleware.RequireRole(models.UserRoleAdmin)

	switches := api.Group("/switches")
	switches.Use(authenticated)
	{
		switches.GET("", h.switchHandler.List)
		switches.GET("/:id", h.switchHandler.Get)
		switches.POST("", admin, h.switchHandler.Create)
		switches.PUT("/:id", admin, h.switchHandler.Update)
		switches.DELETE("/:id", admin, h.switchHandler.Delete)
	}
}
