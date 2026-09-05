package api

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/tikman/olt-provisioning/internal/auth"
	"github.com/tikman/olt-provisioning/internal/config"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/middleware"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"github.com/tikman/olt-provisioning/internal/wa"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// productionEnvironment is the ENVIRONMENT value that turns on cookie hardening.
const productionEnvironment = "production"

const (
	// The dashboard polls hard: ONT realtime metrics refetch every 3s and the
	// ONT list every 15s, and several operators can share one NAT address, so a
	// tight global ceiling would 429 normal use. Documented in docs/SECURITY.md.
	globalRequestsPerMinute = 600
	// Login is the only unauthenticated write, so it carries its own ceiling.
	// /auth/me is deliberately left on the global limit: the SPA refetches it on
	// every window focus and would trip a login-sized budget.
	loginRequestsPerMinute = 10
)

func Setup(ginEngine *gin.Engine, cfg *config.Config, db *gorm.DB, authStore *auth.Store, logger *zap.Logger, wgService *services.WireGuardService) (*gin.Engine, *services.PushNotifierService, *PushEventListener) {
	router := ginEngine

	router.Use(corsMiddleware(cfg.AllowedOrigins))
	router.Use(middleware.RateLimitMiddleware(globalRequestsPerMinute))

	router.GET("/health", NewHealthHandler(db, authStore).Check)

	// Created before the services because the OLT service uses it to read CLI
	// profile lists during discovery.
	commanderFactory := connectivity.NewCommanderFactoryWithEncryption(5*time.Second, cfg.EncryptionKey)

	userService := services.NewUserService(db)
	siteService := services.NewSiteService(db)
	oltService := services.NewOLTServiceWithCommander(db, cfg.EncryptionKey, commanderFactory)
	oltValidatorService := services.NewOLTValidatorService(db)
	ontService := services.NewONTServiceWithEncryption(db, cfg.EncryptionKey)
	metricsService := services.NewMetricsService(db)
	eventService := services.NewEventService(db)
	auditService := services.NewAuditService(db, logger)
	unconfiguredONUService := services.NewUnconfiguredONUService(db)
	configTemplateService := services.NewConfigTemplateService(db, auditService)
	settingService := services.NewSettingService(db, cfg.EncryptionKey)

	authHandler := NewAuthHandler(userService, authStore, cfg.Environment == productionEnvironment)
	userHandler := NewUserHandler(userService, auditService)
	siteHandler := NewSiteHandler(siteService, auditService)
	oltHandler := NewOLTHandler(oltService, oltValidatorService, auditService, ontService, services.NewPollJobService(db))
	onuRemovalService := services.NewZTEONURemovalService(db, ontService, commanderFactory)
	ontHandler := NewONTHandler(ontService, metricsService, auditService, onuRemovalService)
	metricsHandler := NewMetricsHandler(metricsService)
	eventHandler := NewEventHandler(eventService)
	unconfiguredONUHandler := NewUnconfiguredONUHandler(unconfiguredONUService)
	seedHandler := NewSeedHandler(db, cfg.EncryptionKey)
	configTemplateHandler := NewConfigTemplateHandler(configTemplateService)
	dashboardHandler := NewDashboardHandler(services.NewDashboardService(db))
	wireguardHandler := NewWireGuardHandler(wgService, auditService)
	settingHandler := NewSettingHandler(settingService, auditService)
	distributionHandler := NewDistributionHandler(services.NewDistributionService(db))

	// A dedicated connection: the CS inbox's presence and pub/sub traffic is
	// unrelated to the session store above, and giving it its own client keeps
	// one from being starved by the other's load.
	csRedisClient := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.RedisHost, cfg.RedisPort),
		Password: cfg.RedisPassword,
	})
	csConversationService := services.NewCSConversationService(db)
	csMessageService := services.NewCSMessageService(db, csConversationService)
	csQuickReplyService := services.NewCSQuickReplyService(db)
	csAccountService := services.NewCSAccountService(db)
	csChannelService := services.NewCSChannelService(db)
	csBroadcastPostService := services.NewCSBroadcastPostService(db)
	csPurgeService := services.NewCSPurgeService(db, cfg.WAMediaDir)
	csPresence := services.NewRedisPresence(csRedisClient)
	csAssignmentService := services.NewCSAssignmentService(db, csConversationService, csPresence)
	csPublisher := wa.NewPublisher(csRedisClient)
	csHandler := NewCSHandler(
		csConversationService, csMessageService, csQuickReplyService, csAccountService, csChannelService, csBroadcastPostService,
		csPurgeService, csAssignmentService, csPresence, auditService, ontService, userService, csPublisher, csRedisClient,
		logger, cfg.WAMediaDir,
	)

	pushService := services.NewPushService(db)
	pushHandler := NewPushHandler(pushService)
	// nil Sender for now: cmd/api is the one place that knows whether a real
	// Firebase client exists, and sets it via SetSender once Setup returns —
	// Setup only wires the notifier's other dependencies.
	pushNotifier := services.NewPushNotifierService(nil, pushService, csConversationService, csMessageService)
	pushListener := NewPushEventListener(csRedisClient, pushNotifier, logger)

	// Provisioning pipeline: the factory above creates per-OLT commanders since
	// each OLT has its own address and credentials.
	provisionJobService := services.NewJobService(db, auditService)
	snapshotService := services.NewSnapshotServiceWithCommander(db, connectivity.DriverFor, logger, commanderFactory)
	rollbackEngine := services.NewRollbackEngineForOLTs(commanderFactory, logger)
	ontProvisioningService := services.NewOntProvisioningServiceWithTemplates(db, provisionJobService, snapshotService, commanderFactory, rollbackEngine, auditService, logger, configTemplateService)
	provisionHandler := NewProvisionHandler(ontProvisioningService)
	zteProvisioner := services.NewZTEGPONRegisterService(db, provisionJobService, snapshotService, commanderFactory, rollbackEngine, logger).WithEncryptionKey(cfg.EncryptionKey)
	zteProvisionHandler := NewZTEProvisionHandler(zteProvisioner, provisionJobService)

	api := router.Group("/api/v1")

	{
		auth := api.Group("/auth")
		{
			auth.POST("/login", middleware.RateLimitMiddleware(loginRequestsPerMinute), authHandler.Login)
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

		// GET /browser is static while PUT/DELETE take :name. Gin keeps one
		// tree per method so these coexist — but a GET /settings/:name must
		// never be added here without moving /browser, or the router panics on
		// a wildcard conflict.
		settings := api.Group("/settings")
		settings.Use(middleware.AuthMiddleware(authStore, logger))
		{
			settings.GET("/browser", settingHandler.Browser)
			settings.GET("", middleware.RequireRole(models.UserRoleAdmin), settingHandler.List)
			settings.PUT("/:name", middleware.RequireRole(models.UserRoleAdmin), settingHandler.Set)
			settings.DELETE("/:name", middleware.RequireRole(models.UserRoleAdmin), settingHandler.Delete)
		}

		dashboard := api.Group("/dashboard")
		dashboard.Use(middleware.AuthMiddleware(authStore, logger))
		{
			dashboard.GET("/stats", dashboardHandler.GetStats)
		}

		olts := api.Group("/olts")
		olts.Use(middleware.AuthMiddleware(authStore, logger))
		{
			olts.GET("", oltHandler.List)
			olts.GET("/:id", oltHandler.GetByID)
			olts.POST("", middleware.RequireRole(models.UserRoleAdmin), oltHandler.Create)
			olts.PUT("/:id", middleware.RequireRole(models.UserRoleAdmin, models.UserRoleTechnician), oltHandler.Update)
			olts.DELETE("/:id", middleware.RequireRole(models.UserRoleAdmin), oltHandler.Delete)
			// No :id - the handler takes the candidate's address and credentials
			// from the body, because the point is to test a connection before the
			// OLT exists. Registering this under /:id/test left the create form's
			// Test Connection button hitting a 404.
			olts.POST("/test-connection", middleware.RequireRole(models.UserRoleAdmin), oltHandler.TestConnection)
			olts.GET("/:id/topology/cached", oltHandler.GetCachedTopology)
			olts.POST("/:id/topology", oltHandler.DiscoverOLTTopology)
			olts.POST("/:id/discover", oltHandler.DiscoverONTs)
			olts.POST("/:id/discover-and-register", oltHandler.DiscoverAndRegisterONTs)
			olts.POST("/:id/discover-now", middleware.RequireRole(models.UserRoleAdmin, models.UserRoleTechnician), oltHandler.DiscoverNow)
			olts.GET("/:id/stats", metricsHandler.GetOltsStats)
			olts.GET("/:id/unconfigured-onus", unconfiguredONUHandler.ListByOLT)
			olts.GET("/:id/vlans", oltHandler.ListVLANs)
			olts.GET("/:id/tcont-profiles", oltHandler.ListTCONTProfiles)
			olts.GET("/:id/vlan-profiles", oltHandler.ListVLANProfiles)
			olts.GET("/:id/onu-types", oltHandler.ListONUTypes)
			olts.GET("/:id/system", oltHandler.GetSystem)
			olts.GET("/:id/metrics/traffic", metricsHandler.GetOLTAggregateTraffic)
			olts.GET("/:id/pon-health", oltHandler.PonHealth)
			olts.POST("/:id/system/refresh", middleware.RequireRole(models.UserRoleAdmin, models.UserRoleTechnician), oltHandler.RefreshSystem)
			olts.POST("/:id/gpon/register", middleware.RequireRole(models.UserRoleAdmin, models.UserRoleTechnician), zteProvisionHandler.Register)
			olts.POST("/:id/gpon/preview", middleware.RequireRole(models.UserRoleAdmin, models.UserRoleTechnician), zteProvisionHandler.PreviewRegister)
		}

		onts := api.Group("/onts")
		onts.Use(middleware.AuthMiddleware(authStore, logger))
		{
			onts.GET("", ontHandler.List)
			onts.GET("/troubled", ontHandler.ListTroubled)
			onts.GET("/:id", ontHandler.GetByID)
			onts.POST("", middleware.RequireRole(models.UserRoleAdmin), ontHandler.Create)
			onts.PUT("/:id", middleware.RequireRole(models.UserRoleAdmin, models.UserRoleTechnician), ontHandler.Update)
			onts.DELETE("/:id", middleware.RequireRole(models.UserRoleAdmin), ontHandler.Delete)
			onts.GET("/:id/removal/preview", middleware.RequireRole(models.UserRoleAdmin), ontHandler.PreviewRemoval)

			onts.GET("/:id/metrics/realtime", metricsHandler.GetRealtime)
			onts.GET("/:id/metrics/timeseries", metricsHandler.GetTrafficTimeSeries)
			onts.GET("/:id/metrics/history", metricsHandler.GetHistory)
			onts.GET("/:id/metrics", metricsHandler.GetLatest)
			onts.GET("/:id/service-config", ontHandler.GetServiceConfig)
			onts.GET("/:id/events", eventHandler.GetEvents)
			onts.GET("/:id/availability", eventHandler.GetAvailability)
			onts.POST("/:id/gpon/configure", middleware.RequireRole(models.UserRoleAdmin, models.UserRoleTechnician), zteProvisionHandler.ConfigureExisting)
			onts.POST("/:id/gpon/preview", middleware.RequireRole(models.UserRoleAdmin, models.UserRoleTechnician), zteProvisionHandler.PreviewConfigure)

			// Which distribution box a drop lands in is field knowledge, so a
			// technician records it; only an admin may remove plant.
			onts.PUT("/:id/odp", middleware.RequireRole(models.UserRoleAdmin, models.UserRoleTechnician), distributionHandler.AssignONT)
			onts.DELETE("/:id/odp", middleware.RequireRole(models.UserRoleAdmin, models.UserRoleTechnician), distributionHandler.UnassignONT)
		}

		odcs := api.Group("/odcs")
		odcs.Use(middleware.AuthMiddleware(authStore, logger))
		{
			odcs.GET("", distributionHandler.ListODCs)
			odcs.POST("", middleware.RequireRole(models.UserRoleAdmin, models.UserRoleTechnician), distributionHandler.CreateODC)
			odcs.POST("/:id/feeds", middleware.RequireRole(models.UserRoleAdmin, models.UserRoleTechnician), distributionHandler.AddODCFeed)
		}

		// The CS inbox is read by the whole team on purpose: seeing each other's
		// threads is what keeps two agents off one customer. Only replying is
		// restricted, and that check lives in the handler because it depends on
		// who holds the thread rather than on a role.
		cs := api.Group("/cs")
		cs.Use(middleware.AuthMiddleware(authStore, logger))
		cs.Use(middleware.RequireRole(models.UserRoleAdmin, models.UserRoleCS, models.UserRoleTechnician))
		{
			cs.GET("/conversations", csHandler.ListConversations)
			cs.GET("/conversations/:id/messages", csHandler.History)
			cs.POST("/conversations/:id/messages", csHandler.Send)
			cs.POST("/conversations/:id/media", csHandler.SendMedia)
			cs.POST("/conversations/:id/typing", csHandler.SetTyping)
			cs.PUT("/conversations/:id/assign", csHandler.Assign)
			cs.PUT("/conversations/:id/status", csHandler.SetStatus)
			cs.PUT("/conversations/:id/ont", csHandler.LinkONT)
			cs.GET("/media/:message_id", csHandler.ServeMedia)
			cs.GET("/conversations/:id/avatar", csHandler.ServeAvatar)
			cs.GET("/messages/search", csHandler.SearchMessages)
			cs.DELETE("/messages/:id", csHandler.DeleteMessage)
			cs.DELETE("/conversations/:id/messages", csHandler.ClearConversation)
			// Emptying every thread on every number is the one purge with no
			// natural owner to gate it, so it is the admin's alone.
			cs.DELETE("/messages", middleware.RequireRole(models.UserRoleAdmin), csHandler.ClearInbox)
			cs.GET("/stream", csHandler.Stream)
			cs.GET("/online", csHandler.Online)

			cs.GET("/quick-replies", csHandler.ListQuickReplies)
			cs.POST("/quick-replies", middleware.RequireRole(models.UserRoleAdmin), csHandler.CreateQuickReply)
			cs.PUT("/quick-replies/:id", middleware.RequireRole(models.UserRoleAdmin), csHandler.UpdateQuickReply)
			cs.DELETE("/quick-replies/:id", middleware.RequireRole(models.UserRoleAdmin), csHandler.DeleteQuickReply)

			// Reading the number's state is not an admin matter the way pairing or
			// disconnecting it is: the whole team answering that number needs to know
			// whether their replies are actually going out.
			cs.GET("/wa-accounts", csHandler.ListAccounts)
			cs.POST("/wa-accounts", middleware.RequireRole(models.UserRoleAdmin), csHandler.CreateAccount)
			cs.POST("/wa-accounts/:id/connect", middleware.RequireRole(models.UserRoleAdmin), csHandler.Connect)
			cs.POST("/wa-accounts/:id/disconnect", middleware.RequireRole(models.UserRoleAdmin), csHandler.Disconnect)
			cs.DELETE("/wa-accounts/:id", middleware.RequireRole(models.UserRoleAdmin), csHandler.DeleteAccount)
			cs.DELETE("/wa-accounts/:id/messages", middleware.RequireRole(models.UserRoleAdmin), csHandler.ClearAccountMessages)

			cs.GET("/wa-channels", csHandler.ListChannels)
			cs.POST("/wa-channels/refresh", csHandler.RefreshChannels)
			cs.GET("/broadcasts", csHandler.ListBroadcasts)
			cs.POST("/broadcasts", csHandler.CreateBroadcast)
			cs.POST("/broadcasts/media", csHandler.CreateBroadcastMedia)
		}

		push := api.Group("/push")
		push.Use(middleware.AuthMiddleware(authStore, logger))
		{
			push.POST("/subscribe", pushHandler.Subscribe)
			push.DELETE("/subscribe", pushHandler.Unsubscribe)
		}

		odcFeeds := api.Group("/odc-feeds")
		odcFeeds.Use(middleware.AuthMiddleware(authStore, logger))
		{
			odcFeeds.GET("", distributionHandler.ListODCFeeds)
			odcFeeds.PUT("/:id/route", middleware.RequireRole(models.UserRoleAdmin, models.UserRoleTechnician), distributionHandler.SetODCFeedRoute)
		}

		odps := api.Group("/odps")
		odps.Use(middleware.AuthMiddleware(authStore, logger))
		{
			odps.GET("", distributionHandler.ListODPs)
			odps.POST("", middleware.RequireRole(models.UserRoleAdmin, models.UserRoleTechnician), distributionHandler.CreateODP)
			odps.GET("/:id/subscribers", distributionHandler.SubscribersOnODP)
			odps.PUT("/:id/route", middleware.RequireRole(models.UserRoleAdmin, models.UserRoleTechnician), distributionHandler.SetODPRoute)
		}

		configTemplates := api.Group("/config-templates")
		configTemplates.Use(middleware.AuthMiddleware(authStore, logger))
		{
			configTemplates.GET("", configTemplateHandler.List)
			configTemplates.GET("/:id", configTemplateHandler.GetByID)
			configTemplates.POST("", middleware.RequireRole(models.UserRoleAdmin, models.UserRoleTechnician), configTemplateHandler.Create)
			configTemplates.PUT("/:id", middleware.RequireRole(models.UserRoleAdmin, models.UserRoleTechnician), configTemplateHandler.Update)
			configTemplates.DELETE("/:id", middleware.RequireRole(models.UserRoleAdmin), configTemplateHandler.Delete)
		}

		wireguard := api.Group("/wireguard")
		wireguard.Use(middleware.AuthMiddleware(authStore, logger))
		{
			wireguard.GET("/server", wireguardHandler.GetServer)
			wireguard.PUT("/server", middleware.RequireRole(models.UserRoleAdmin), wireguardHandler.SaveServer)
			wireguard.GET("/peers", wireguardHandler.ListPeers)
			wireguard.POST("/peers", middleware.RequireRole(models.UserRoleAdmin), wireguardHandler.CreatePeer)
			wireguard.PUT("/peers/:id", middleware.RequireRole(models.UserRoleAdmin), wireguardHandler.UpdatePeer)
			wireguard.DELETE("/peers/:id", middleware.RequireRole(models.UserRoleAdmin), wireguardHandler.DeletePeer)
			wireguard.GET("/peers/:id/config", middleware.RequireRole(models.UserRoleAdmin), wireguardHandler.GetPeerConfig)
			wireguard.POST("/peers/:id/test", middleware.RequireRole(models.UserRoleAdmin, models.UserRoleTechnician), wireguardHandler.TestReachability)
			wireguard.GET("/sites/:site_id/suggested-subnets", wireguardHandler.SuggestSubnets)
		}

		provisionJobs := api.Group("/provision-jobs")
		provisionJobs.Use(middleware.AuthMiddleware(authStore, logger))
		{
			provisionJobs.GET("/:id", provisionHandler.GetProvisionJob)
		}

		onts.POST("/:id/provision", middleware.AuthMiddleware(authStore, logger), middleware.RequireRole(models.UserRoleAdmin, models.UserRoleTechnician), provisionHandler.ProvisionOnt)
		onts.GET("/:id/provision-jobs", middleware.AuthMiddleware(authStore, logger), provisionHandler.ListProvisionJobsByONT)

		admin := api.Group("/admin")
		admin.Use(middleware.AuthMiddleware(authStore, logger))
		admin.Use(middleware.RequireRole(models.UserRoleAdmin))
		{
			admin.POST("/seed-events", seedHandler.SeedEventHistory)
		}
	}

	return router, pushNotifier, pushListener
}
