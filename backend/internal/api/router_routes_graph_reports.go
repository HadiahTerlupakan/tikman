package api

import (
	"github.com/gin-gonic/gin"
	"github.com/tikman/olt-provisioning/internal/middleware"
	"github.com/tikman/olt-provisioning/internal/models"
)

// registerGraphReportRoutes covers the daily LibreNMS graph reports and the
// port notices. All of it but LibreNMS's webhook is an admin's: a report
// speaks for the number in a customer's group, and the LibreNMS reads go out
// with the installation's token.
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

		reports.GET("/port-alert-destination", h.portAlertHandler.Destination)
		reports.PUT("/port-alert-destination", h.portAlertHandler.SetDestination)
	}

	// LibreNMS has no session: the webhook is authenticated by its token, and
	// held to the global rate limit like every other route.
	api.POST("/hooks/librenms", h.portAlertHandler.LibreNMSHook)
}
