package api

import (
	"github.com/gin-gonic/gin"
	"github.com/tikman/olt-provisioning/internal/auth"
	"github.com/tikman/olt-provisioning/internal/config"
	"github.com/tikman/olt-provisioning/internal/middleware"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"net/http"
)

func Setup(ginEngine *gin.Engine, cfg *config.Config, db *gorm.DB, authStore *auth.Store, logger *zap.Logger) *gin.Engine {
	router := ginEngine

	// CORS Configuration - allow requests from frontend
	router.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "http://localhost:3000")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, Cookie, X-CSRF-Token, X-Request-ID")
		c.Header("Access-Control-Allow-Credentials", "true")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	})

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "healthy",
			"time":   c.GetHeader("Date"),
		})
	})

	userService := services.NewUserService(db)
	siteService := services.NewSiteService(db)
	oltService := services.NewOLTService(db, cfg.EncryptionKey)
	oltValidatorService := services.NewOLTValidatorService(db)
	ontService := services.NewONTService(db)
	metricsService := services.NewMetricsService(db)
	eventService := services.NewEventService(db)
	auditService := services.NewAuditService(db, logger)

	authHandler := NewAuthHandler(userService, authStore)
	userHandler := NewUserHandler(userService, auditService)
	siteHandler := NewSiteHandler(siteService, auditService)
	oltHandler := NewOLTHandler(oltService, oltValidatorService, auditService)
	ontHandler := NewONTHandler(ontService, metricsService, auditService)
	metricsHandler := NewMetricsHandler(metricsService)
	eventHandler := NewEventHandler(eventService)
	seedHandler := NewSeedHandler(db, cfg.EncryptionKey)

	api := router.Group("/api/v1")
	{
		auth := api.Group("/auth")
		{
			auth.POST("/login", authHandler.Login)
			auth.POST("/logout", authHandler.Logout)
			auth.GET("/me", middleware.AuthMiddleware(authStore, logger), authHandler.Me)
		}

		users := api.Group("/users")
		users.Use(middleware.AuthMiddleware(authStore, logger))
		{
			users.GET("", userHandler.List)
			users.POST("", middleware.RequireRole(models.UserRoleAdmin), userHandler.Create)
			users.PUT("/:id", middleware.RequireRole(models.UserRoleAdmin, models.UserRoleTechnician), userHandler.Update)
			users.DELETE("/:id", middleware.RequireRole(models.UserRoleAdmin), userHandler.Delete)
		}

		sites := api.Group("/sites")
		sites.Use(middleware.AuthMiddleware(authStore, logger))
		{
			sites.GET("", siteHandler.List)
			sites.POST("", middleware.RequireRole(models.UserRoleAdmin), siteHandler.Create)
			sites.PUT("/:id", middleware.RequireRole(models.UserRoleAdmin, models.UserRoleTechnician), siteHandler.Update)
			sites.DELETE("/:id", middleware.RequireRole(models.UserRoleAdmin), siteHandler.Delete)
		}

		olts := api.Group("/olts")
		olts.Use(middleware.AuthMiddleware(authStore, logger))
		{
			olts.GET("", oltHandler.List)
			olts.GET("/:id", oltHandler.GetByID)
			olts.POST("", middleware.RequireRole(models.UserRoleAdmin), oltHandler.Create)
			olts.PUT("/:id", middleware.RequireRole(models.UserRoleAdmin, models.UserRoleTechnician), oltHandler.Update)
			olts.DELETE("/:id", middleware.RequireRole(models.UserRoleAdmin), oltHandler.Delete)
			olts.POST("/:id/test", middleware.RequireRole(models.UserRoleAdmin), oltHandler.TestConnection)
			olts.GET("/:id/topology/cached", oltHandler.GetCachedTopology)
			olts.POST("/:id/topology", oltHandler.DiscoverOLTTopology)
			olts.POST("/:id/discover", oltHandler.DiscoverONTs)
			olts.POST("/:id/discover-and-register", oltHandler.DiscoverAndRegisterONTs)
			olts.GET("/:id/stats", metricsHandler.GetOltsStats)
		}

		onts := api.Group("/onts")
		onts.Use(middleware.AuthMiddleware(authStore, logger))
		{
			onts.GET("", ontHandler.List)
			onts.GET("/:id", ontHandler.GetByID)
			onts.POST("", middleware.RequireRole(models.UserRoleAdmin), ontHandler.Create)
			onts.PUT("/:id", middleware.RequireRole(models.UserRoleAdmin, models.UserRoleTechnician), ontHandler.Update)
		onts.DELETE("/:id", middleware.RequireRole(models.UserRoleAdmin), ontHandler.Delete)

		onts.GET("/:id/metrics/realtime", metricsHandler.GetRealtime)
		onts.GET("/:id/metrics/timeseries", metricsHandler.GetTrafficTimeSeries)
		onts.GET("/:id/metrics/history", metricsHandler.GetHistory)
		onts.GET("/:id/metrics", metricsHandler.GetLatest)
		onts.GET("/:id/events", eventHandler.GetEvents)
		onts.GET("/:id/availability", eventHandler.GetAvailability)
		}

		admin := api.Group("/admin")
		admin.Use(middleware.AuthMiddleware(authStore, logger))
		admin.Use(middleware.RequireRole(models.UserRoleAdmin))
		{
			admin.POST("/seed-events", seedHandler.SeedEventHistory)
		}
	}

	return router
}
