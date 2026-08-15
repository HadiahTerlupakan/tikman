package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/tikman/olt-provisioning/internal/auth"
	"github.com/tikman/olt-provisioning/internal/config"
	"github.com/tikman/olt-provisioning/internal/middleware"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func Setup(cfg *config.Config, db *gorm.DB, sessionStore *auth.Store, logger *zap.Logger) *gin.Engine {
	if cfg.LogLevel == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	router.Use(gin.Recovery())

	// Security headers
	router.Use(middleware.SecureHeaders(cfg.Environment))

	// HTTPS redirect for production
	if cfg.Environment == "production" {
		router.Use(middleware.HTTPSRedirect())
	}

	router.Use(middleware.RateLimitMiddleware(100))

	// CORS with environment-based origins
	origins := strings.Split(cfg.AllowedOrigins, ",")
	router.Use(cors.New(cors.Config{
		AllowOrigins:     origins,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	router.Use(func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		c.Next()
		duration := time.Since(start)

		logger.Info("Request",
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("duration", duration),
			zap.String("ip", c.ClientIP()),
		)
	})

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "healthy",
			"time":   time.Now().UTC(),
		})
	})

	userService := services.NewUserService(db)
	siteService := services.NewSiteService(db)
	oltService := services.NewOLTService(db, cfg.EncryptionKey)
	oltValidatorService := services.NewOLTValidatorService(db)
	ontService := services.NewONTService(db)
	metricsService := services.NewMetricsService(db)
	auditService := services.NewAuditService(db, logger)

	authHandler := NewAuthHandler(userService, sessionStore)
	userHandler := NewUserHandler(userService, auditService)
	siteHandler := NewSiteHandler(siteService, auditService)
	oltHandler := NewOLTHandler(oltService, oltValidatorService, auditService)
	ontHandler := NewONTHandler(ontService, auditService)
	metricsHandler := NewMetricsHandler(metricsService)

	api := router.Group("/api/v1")
	{
		auth := api.Group("/auth")
		{
			auth.POST("/login", authHandler.Login)
			auth.POST("/logout", authHandler.Logout)
			auth.GET("/me", middleware.AuthMiddleware(sessionStore, logger), authHandler.Me)
		}

		users := api.Group("/users")
		users.Use(middleware.AuthMiddleware(sessionStore, logger))
		{
			users.GET("", userHandler.List)
			users.POST("", middleware.RequireRole(models.UserRoleAdmin), userHandler.Create)
			users.GET("/:id", userHandler.GetByID)
			users.PUT("/:id", middleware.RequireRole(models.UserRoleAdmin), userHandler.Update)
			users.DELETE("/:id", middleware.RequireRole(models.UserRoleAdmin), userHandler.Delete)
		}

		sites := api.Group("/sites")
		sites.Use(middleware.AuthMiddleware(sessionStore, logger))
		{
			sites.GET("", siteHandler.List)
			sites.POST("", middleware.RequireRole(models.UserRoleAdmin, models.UserRoleTechnician), siteHandler.Create)
			sites.GET("/:id", siteHandler.GetByID)
			sites.PUT("/:id", middleware.RequireRole(models.UserRoleAdmin, models.UserRoleTechnician), siteHandler.Update)
			sites.DELETE("/:id", middleware.RequireRole(models.UserRoleAdmin), siteHandler.Delete)
		}

		olts := api.Group("/olts")
		olts.Use(middleware.AuthMiddleware(sessionStore, logger))
		{
			olts.GET("", oltHandler.List)
			olts.POST("", middleware.RequireRole(models.UserRoleAdmin, models.UserRoleTechnician), oltHandler.Create)
			olts.POST("/test-connection", middleware.RequireRole(models.UserRoleAdmin, models.UserRoleTechnician), oltHandler.TestConnection)
			olts.GET("/:id", oltHandler.GetByID)
			olts.PUT("/:id", middleware.RequireRole(models.UserRoleAdmin, models.UserRoleTechnician), oltHandler.Update)
			olts.DELETE("/:id", middleware.RequireRole(models.UserRoleAdmin), oltHandler.Delete)
		}

		onts := api.Group("/onts")
		onts.Use(middleware.AuthMiddleware(sessionStore, logger))
		{
			onts.GET("", ontHandler.List)
			onts.POST("", middleware.RequireRole(models.UserRoleAdmin, models.UserRoleTechnician), ontHandler.Create)
			onts.GET("/:id", ontHandler.GetByID)
			onts.PUT("/:id", middleware.RequireRole(models.UserRoleAdmin, models.UserRoleTechnician), ontHandler.Update)
			onts.DELETE("/:id", middleware.RequireRole(models.UserRoleAdmin), ontHandler.Delete)

			// Metrics routes
			onts.GET("/:id/metrics", metricsHandler.GetLatest)
			onts.GET("/:id/metrics/history", metricsHandler.GetHistory)
		}
	}

	return router
}
