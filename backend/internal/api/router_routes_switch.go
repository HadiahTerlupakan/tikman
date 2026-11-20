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
	operator := middleware.RequireRole(models.UserRoleAdmin, models.UserRoleTechnician)

	switches := api.Group("/switches")
	switches.Use(authenticated)
	{
		switches.GET("", h.switchHandler.List)
		switches.GET("/:id", h.switchHandler.Get)
		switches.POST("", admin, h.switchHandler.Create)
		switches.PUT("/:id", admin, h.switchHandler.Update)
		switches.DELETE("/:id", admin, h.switchHandler.Delete)
		switches.POST("/test-connection", admin, h.switchReadsHandler.TestConnection)
		switches.POST("/:id/refresh", operator, h.switchReadsHandler.Refresh)
		switches.POST("/:id/backup", operator, h.switchReadsHandler.BackupNow)
		switches.POST("/:id/trust-host-key", admin, h.switchReadsHandler.TrustHostKey)
		switches.GET("/:id/interfaces", h.switchReadsHandler.Interfaces)
		switches.GET("/:id/vlans", h.switchReadsHandler.VLANs)
		switches.GET("/:id/neighbors", h.switchReadsHandler.Neighbors)
		switches.GET("/:id/backups", h.switchReadsHandler.Backups)
		switches.GET("/:id/backups/diff", h.switchReadsHandler.BackupDiff)
		switches.GET("/:id/backups/:bid", h.switchReadsHandler.Backup)
	}
}
