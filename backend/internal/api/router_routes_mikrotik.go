package api

import (
	"github.com/gin-gonic/gin"
	"github.com/tikman/olt-provisioning/internal/middleware"
	"github.com/tikman/olt-provisioning/internal/models"
)

// registerMikrotikRoutes covers the MikroTik border routers. Reads are open to
// every role; adding, deleting and trusting a certificate are admin decisions.
func (h *handlers) registerMikrotikRoutes(api *gin.RouterGroup, authenticated gin.HandlerFunc) {
	admin := middleware.RequireRole(models.UserRoleAdmin)
	operator := middleware.RequireRole(models.UserRoleAdmin, models.UserRoleTechnician)

	routers := api.Group("/routers")
	routers.Use(authenticated)
	{
		routers.GET("", h.routerHandler.List)
		routers.POST("", admin, h.routerHandler.Create)
		routers.POST("/test-connection", admin, h.routerHandler.TestConnection)
		routers.PUT("/:id", operator, h.routerHandler.Update)
		routers.DELETE("/:id", admin, h.routerHandler.Delete)
		routers.POST("/:id/refresh", operator, h.routerHandler.Refresh)
		routers.POST("/:id/trust-certificate", admin, h.routerHandler.TrustCertificate)
		routers.GET("/:id/interfaces", h.routerHandler.Interfaces)
		routers.GET("/:id/interfaces/:ifId/traffic", h.routerHandler.Traffic)
		routers.GET("/:id/bgp/sessions", h.routerHandler.Sessions)
		routers.GET("/:id/bgp/events", h.routerHandler.Events)
	}
}
