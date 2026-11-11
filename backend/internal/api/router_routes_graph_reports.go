package api

import (
	"github.com/gin-gonic/gin"
	"github.com/tikman/olt-provisioning/internal/middleware"
	"github.com/tikman/olt-provisioning/internal/models"
)

// registerGraphReportRoutes covers the daily LibreNMS graph reports. All of it
// is an admin's: a report speaks for the number in a customer's group, and the
// LibreNMS reads go out with the installation's token.
func (h *handlers) registerGraphReportRoutes(api *gin.RouterGroup, authenticated gin.HandlerFunc) {
	reports := api.Group("/graph-reports")
	reports.Use(authenticated, middleware.RequireRole(models.UserRoleAdmin))
	{
		reports.GET("", h.graphReportHandler.List)
		reports.POST("", h.graphReportHandler.Create)
		reports.PUT("/:id", h.graphReportHandler.Update)
		reports.DELETE("/:id", h.graphReportHandler.Delete)
		reports.POST("/:id/send", h.graphReportHandler.Send)
		reports.POST("/preview", h.graphReportHandler.Preview)

		reports.GET("/librenms/devices", h.graphReportHandler.LibreNMSDevices)
		reports.GET("/librenms/devices/:deviceId/ports", h.graphReportHandler.LibreNMSPorts)
		reports.GET("/librenms/graph", h.graphReportHandler.LibreNMSGraph)

		reports.GET("/telegram/chats", h.graphReportHandler.TelegramChats)
	}
}
