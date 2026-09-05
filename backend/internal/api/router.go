package api

import (
	firebase "firebase.google.com/go/v4"
	"github.com/gin-gonic/gin"
	"github.com/tikman/olt-provisioning/internal/auth"
	"github.com/tikman/olt-provisioning/internal/config"
	"github.com/tikman/olt-provisioning/internal/middleware"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
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

func Setup(ginEngine *gin.Engine, cfg *config.Config, db *gorm.DB, authStore *auth.Store, logger *zap.Logger, wgService *services.WireGuardService, firebaseApp *firebase.App) (*gin.Engine, *services.PushNotifierService, *PushEventListener, *services.RedisPresence) {
	router := ginEngine

	router.Use(corsMiddleware(cfg.AllowedOrigins))
	router.Use(middleware.RateLimitMiddleware(globalRequestsPerMinute))

	router.GET("/health", NewHealthHandler(db, authStore).Check)

	h, pushNotifier, pushListener, csPresence := newHandlers(cfg, db, authStore, logger, wgService, firebaseApp)

	api := router.Group("/api/v1")
	authenticated := middleware.AuthMiddleware(authStore, logger)

	h.registerAccountRoutes(api, authenticated)
	h.registerPlantRoutes(api, authenticated)
	h.registerONTRoutes(api, authenticated)
	h.registerCSRoutes(api, authenticated)
	h.registerOperationsRoutes(api, authenticated)
	h.registerVPNRoutes(api, authenticated)

	return router, pushNotifier, pushListener, csPresence
}

// registerAccountRoutes covers who is signed in and what the installation is
// configured with.
func (h *handlers) registerAccountRoutes(api *gin.RouterGroup, authenticated gin.HandlerFunc) {
	auth := api.Group("/auth")
	{
		auth.POST("/login", middleware.RateLimitMiddleware(loginRequestsPerMinute), h.authHandler.Login)
		auth.POST("/logout", h.authHandler.Logout)
		auth.GET("/me", authenticated, h.authHandler.Me)
		auth.GET("/firebase-token",
			authenticated,
			middleware.RequireRole(models.UserRoleAdmin, models.UserRoleCS, models.UserRoleTechnician),
			h.firebaseTokenHandler.Token)
	}

	users := api.Group("/users")
	users.Use(authenticated)
	{
		users.GET("", h.userHandler.List)
		users.POST("", middleware.RequireRole(models.UserRoleAdmin), h.userHandler.Create)
		users.PUT("/:id", middleware.RequireRole(models.UserRoleAdmin, models.UserRoleTechnician), h.userHandler.Update)
		users.DELETE("/:id", middleware.RequireRole(models.UserRoleAdmin), h.userHandler.Delete)
	}

	sites := api.Group("/sites")
	sites.Use(authenticated)
	{
		sites.GET("", h.siteHandler.List)
		sites.POST("", middleware.RequireRole(models.UserRoleAdmin), h.siteHandler.Create)
		sites.PUT("/:id", middleware.RequireRole(models.UserRoleAdmin, models.UserRoleTechnician), h.siteHandler.Update)
		sites.DELETE("/:id", middleware.RequireRole(models.UserRoleAdmin), h.siteHandler.Delete)
	}

	// GET /browser is static while PUT/DELETE take :name. Gin keeps one
	// tree per method so these coexist — but a GET /settings/:name must
	// never be added here without moving /browser, or the router panics on
	// a wildcard conflict.
	settings := api.Group("/settings")
	settings.Use(authenticated)
	{
		settings.GET("/browser", h.settingHandler.Browser)
		settings.GET("", middleware.RequireRole(models.UserRoleAdmin), h.settingHandler.List)
		settings.PUT("/:name", middleware.RequireRole(models.UserRoleAdmin), h.settingHandler.Set)
		settings.DELETE("/:name", middleware.RequireRole(models.UserRoleAdmin), h.settingHandler.Delete)
	}

	dashboard := api.Group("/dashboard")
	dashboard.Use(authenticated)
	{
		dashboard.GET("/stats", h.dashboardHandler.GetStats)
	}

}

// registerPlantRoutes covers the chassis: the OLT rows and everything read off
// or pushed to one.
func (h *handlers) registerPlantRoutes(api *gin.RouterGroup, authenticated gin.HandlerFunc) {
	olts := api.Group("/olts")
	olts.Use(authenticated)
	{
		olts.GET("", h.oltHandler.List)
		olts.GET("/:id", h.oltHandler.GetByID)
		olts.POST("", middleware.RequireRole(models.UserRoleAdmin), h.oltHandler.Create)
		olts.PUT("/:id", middleware.RequireRole(models.UserRoleAdmin, models.UserRoleTechnician), h.oltHandler.Update)
		olts.DELETE("/:id", middleware.RequireRole(models.UserRoleAdmin), h.oltHandler.Delete)
		// No :id - the handler takes the candidate's address and credentials
		// from the body, because the point is to test a connection before the
		// OLT exists. Registering this under /:id/test left the create form's
		// Test Connection button hitting a 404.
		olts.POST("/test-connection", middleware.RequireRole(models.UserRoleAdmin), h.oltHandler.TestConnection)
		olts.GET("/:id/topology/cached", h.oltHandler.GetCachedTopology)
		// Discovery talks to the chassis over SNMP, and the last of these
		// writes the ONT rows it finds, so it carries the same gate as
		// /discover-now rather than bare authentication.
		olts.POST("/:id/topology", middleware.RequireRole(models.UserRoleAdmin, models.UserRoleTechnician), h.oltHandler.DiscoverOLTTopology)
		olts.POST("/:id/discover", middleware.RequireRole(models.UserRoleAdmin, models.UserRoleTechnician), h.oltHandler.DiscoverONTs)
		olts.POST("/:id/discover-and-register", middleware.RequireRole(models.UserRoleAdmin, models.UserRoleTechnician), h.oltHandler.DiscoverAndRegisterONTs)
		olts.POST("/:id/discover-now", middleware.RequireRole(models.UserRoleAdmin, models.UserRoleTechnician), h.oltHandler.DiscoverNow)
		olts.GET("/:id/stats", h.metricsHandler.GetOltsStats)
		olts.GET("/:id/unconfigured-onus", h.unconfiguredONUHandler.ListByOLT)
		olts.GET("/:id/vlans", h.oltHandler.ListVLANs)
		olts.GET("/:id/tcont-profiles", h.oltHandler.ListTCONTProfiles)
		olts.GET("/:id/vlan-profiles", h.oltHandler.ListVLANProfiles)
		olts.GET("/:id/onu-types", h.oltHandler.ListONUTypes)
		olts.GET("/:id/system", h.oltHandler.GetSystem)
		olts.GET("/:id/metrics/traffic", h.metricsHandler.GetOLTAggregateTraffic)
		olts.GET("/:id/pon-health", h.oltHandler.PonHealth)
		olts.POST("/:id/system/refresh", middleware.RequireRole(models.UserRoleAdmin, models.UserRoleTechnician), h.oltHandler.RefreshSystem)
		olts.POST("/:id/gpon/register", middleware.RequireRole(models.UserRoleAdmin, models.UserRoleTechnician), h.zteProvisionHandler.Register)
		olts.POST("/:id/gpon/preview", middleware.RequireRole(models.UserRoleAdmin, models.UserRoleTechnician), h.zteProvisionHandler.PreviewRegister)
	}

}

// registerONTRoutes covers the subscriber side: the ONTs themselves and the
// cabinets their drops land in.
func (h *handlers) registerONTRoutes(api *gin.RouterGroup, authenticated gin.HandlerFunc) {
	onts := api.Group("/onts")
	onts.Use(authenticated)
	{
		onts.GET("", h.ontHandler.List)
		onts.GET("/troubled", h.ontHandler.ListTroubled)
		onts.GET("/:id", h.ontHandler.GetByID)
		onts.POST("", middleware.RequireRole(models.UserRoleAdmin), h.ontHandler.Create)
		onts.PUT("/:id", middleware.RequireRole(models.UserRoleAdmin, models.UserRoleTechnician), h.ontHandler.Update)
		onts.DELETE("/:id", middleware.RequireRole(models.UserRoleAdmin), h.ontHandler.Delete)
		onts.GET("/:id/removal/preview", middleware.RequireRole(models.UserRoleAdmin), h.ontHandler.PreviewRemoval)

		onts.GET("/:id/metrics/realtime", h.metricsHandler.GetRealtime)
		onts.GET("/:id/metrics/timeseries", h.metricsHandler.GetTrafficTimeSeries)
		onts.GET("/:id/metrics/history", h.metricsHandler.GetHistory)
		onts.GET("/:id/metrics", h.metricsHandler.GetLatest)
		onts.GET("/:id/service-config", h.ontHandler.GetServiceConfig)
		onts.GET("/:id/events", h.eventHandler.GetEvents)
		onts.GET("/:id/availability", h.eventHandler.GetAvailability)
		onts.POST("/:id/gpon/configure", middleware.RequireRole(models.UserRoleAdmin, models.UserRoleTechnician), h.zteProvisionHandler.ConfigureExisting)
		onts.POST("/:id/gpon/preview", middleware.RequireRole(models.UserRoleAdmin, models.UserRoleTechnician), h.zteProvisionHandler.PreviewConfigure)

		// Which distribution box a drop lands in is field knowledge, so a
		// technician records it; only an admin may remove plant.
		onts.PUT("/:id/odp", middleware.RequireRole(models.UserRoleAdmin, models.UserRoleTechnician), h.distributionHandler.AssignONT)
		onts.DELETE("/:id/odp", middleware.RequireRole(models.UserRoleAdmin, models.UserRoleTechnician), h.distributionHandler.UnassignONT)

		onts.POST("/:id/provision", middleware.RequireRole(models.UserRoleAdmin, models.UserRoleTechnician), h.provisionHandler.ProvisionOnt)
		onts.GET("/:id/provision-jobs", h.provisionHandler.ListProvisionJobsByONT)
	}

	odcs := api.Group("/odcs")
	odcs.Use(authenticated)
	{
		odcs.GET("", h.distributionHandler.ListODCs)
		odcs.POST("", middleware.RequireRole(models.UserRoleAdmin, models.UserRoleTechnician), h.distributionHandler.CreateODC)
		odcs.POST("/:id/feeds", middleware.RequireRole(models.UserRoleAdmin, models.UserRoleTechnician), h.distributionHandler.AddODCFeed)
	}

}

// registerCSRoutes covers the WhatsApp inbox and the pushes that announce it.
func (h *handlers) registerCSRoutes(api *gin.RouterGroup, authenticated gin.HandlerFunc) {
	// The CS inbox is read by the whole team on purpose: seeing each other's
	// threads is what keeps two agents off one customer. Only replying is
	// restricted, and that check lives in the handler because it depends on
	// who holds the thread rather than on a role.
	cs := api.Group("/cs")
	cs.Use(authenticated)
	cs.Use(middleware.RequireRole(models.UserRoleAdmin, models.UserRoleCS, models.UserRoleTechnician))
	{
		cs.GET("/conversations", h.csHandler.ListConversations)
		cs.GET("/conversations/:id/messages", h.csHandler.History)
		cs.POST("/conversations/:id/messages", h.csHandler.Send)
		cs.POST("/conversations/:id/media", h.csHandler.SendMedia)
		cs.POST("/conversations/:id/typing", h.csHandler.SetTyping)
		cs.PUT("/conversations/:id/assign", h.csHandler.Assign)
		cs.PUT("/conversations/:id/status", h.csHandler.SetStatus)
		cs.PUT("/conversations/:id/ont", h.csHandler.LinkONT)
		cs.GET("/media/:message_id", h.csHandler.ServeMedia)
		cs.GET("/conversations/:id/avatar", h.csHandler.ServeAvatar)
		cs.GET("/messages/search", h.csHandler.SearchMessages)
		cs.DELETE("/messages/:id", h.csHandler.DeleteMessage)
		cs.DELETE("/conversations/:id/messages", h.csHandler.ClearConversation)
		// Emptying every thread on every number is the one purge with no
		// natural owner to gate it, so it is the admin's alone.
		cs.DELETE("/messages", middleware.RequireRole(models.UserRoleAdmin), h.csHandler.ClearInbox)
		cs.GET("/stream", h.csHandler.Stream)
		cs.GET("/link-preview", h.csHandler.LinkPreview)

	}

	push := api.Group("/push")
	push.Use(authenticated)
	{
		push.POST("/subscribe", h.pushHandler.Subscribe)
		push.DELETE("/subscribe", h.pushHandler.Unsubscribe)
	}

	h.registerCSAdminRoutes(cs)
}

// registerCSAdminRoutes covers the canned replies, the WhatsApp numbers behind
// the inbox, and the channels broadcast to.
func (h *handlers) registerCSAdminRoutes(cs *gin.RouterGroup) {
	{
		cs.GET("/quick-replies", h.csHandler.ListQuickReplies)
		cs.POST("/quick-replies", middleware.RequireRole(models.UserRoleAdmin), h.csHandler.CreateQuickReply)
		cs.PUT("/quick-replies/:id", middleware.RequireRole(models.UserRoleAdmin), h.csHandler.UpdateQuickReply)
		cs.DELETE("/quick-replies/:id", middleware.RequireRole(models.UserRoleAdmin), h.csHandler.DeleteQuickReply)

		// Reading the number's state is not an admin matter the way pairing or
		// disconnecting it is: the whole team answering that number needs to know
		// whether their replies are actually going out.
		cs.GET("/wa-accounts", h.csHandler.ListAccounts)
		cs.POST("/wa-accounts", middleware.RequireRole(models.UserRoleAdmin), h.csHandler.CreateAccount)
		cs.POST("/wa-accounts/:id/connect", middleware.RequireRole(models.UserRoleAdmin), h.csHandler.Connect)
		cs.POST("/wa-accounts/:id/disconnect", middleware.RequireRole(models.UserRoleAdmin), h.csHandler.Disconnect)
		cs.DELETE("/wa-accounts/:id", middleware.RequireRole(models.UserRoleAdmin), h.csHandler.DeleteAccount)
		cs.DELETE("/wa-accounts/:id/messages", middleware.RequireRole(models.UserRoleAdmin), h.csHandler.ClearAccountMessages)

		cs.GET("/wa-channels", h.csHandler.ListChannels)
		cs.POST("/wa-channels/refresh", h.csHandler.RefreshChannels)
		cs.GET("/broadcasts", h.csHandler.ListBroadcasts)
		cs.POST("/broadcasts", h.csHandler.CreateBroadcast)
		cs.POST("/broadcasts/media", h.csHandler.CreateBroadcastMedia)
	}

}

// registerOperationsRoutes covers the plant records between an OLT and a
// subscriber, and the templates a provision is built from.
func (h *handlers) registerOperationsRoutes(api *gin.RouterGroup, authenticated gin.HandlerFunc) {
	odcFeeds := api.Group("/odc-feeds")
	odcFeeds.Use(authenticated)
	{
		odcFeeds.GET("", h.distributionHandler.ListODCFeeds)
		odcFeeds.PUT("/:id/route", middleware.RequireRole(models.UserRoleAdmin, models.UserRoleTechnician), h.distributionHandler.SetODCFeedRoute)
	}

	odps := api.Group("/odps")
	odps.Use(authenticated)
	{
		odps.GET("", h.distributionHandler.ListODPs)
		odps.POST("", middleware.RequireRole(models.UserRoleAdmin, models.UserRoleTechnician), h.distributionHandler.CreateODP)
		odps.GET("/:id/subscribers", h.distributionHandler.SubscribersOnODP)
		odps.PUT("/:id/route", middleware.RequireRole(models.UserRoleAdmin, models.UserRoleTechnician), h.distributionHandler.SetODPRoute)
	}

	configTemplates := api.Group("/config-templates")
	configTemplates.Use(authenticated)
	{
		configTemplates.GET("", h.configTemplateHandler.List)
		configTemplates.GET("/:id", h.configTemplateHandler.GetByID)
		configTemplates.POST("", middleware.RequireRole(models.UserRoleAdmin, models.UserRoleTechnician), h.configTemplateHandler.Create)
		configTemplates.PUT("/:id", middleware.RequireRole(models.UserRoleAdmin, models.UserRoleTechnician), h.configTemplateHandler.Update)
		configTemplates.DELETE("/:id", middleware.RequireRole(models.UserRoleAdmin), h.configTemplateHandler.Delete)
	}

}

// registerVPNRoutes covers the tunnel to a remote site and the provisioning
// jobs that run across it.
func (h *handlers) registerVPNRoutes(api *gin.RouterGroup, authenticated gin.HandlerFunc) {
	wireguard := api.Group("/wireguard")
	wireguard.Use(authenticated)
	{
		wireguard.GET("/server", h.wireguardHandler.GetServer)
		wireguard.PUT("/server", middleware.RequireRole(models.UserRoleAdmin), h.wireguardHandler.SaveServer)
		wireguard.GET("/peers", h.wireguardHandler.ListPeers)
		wireguard.POST("/peers", middleware.RequireRole(models.UserRoleAdmin), h.wireguardHandler.CreatePeer)
		wireguard.PUT("/peers/:id", middleware.RequireRole(models.UserRoleAdmin), h.wireguardHandler.UpdatePeer)
		wireguard.DELETE("/peers/:id", middleware.RequireRole(models.UserRoleAdmin), h.wireguardHandler.DeletePeer)
		wireguard.GET("/peers/:id/config", middleware.RequireRole(models.UserRoleAdmin), h.wireguardHandler.GetPeerConfig)
		wireguard.POST("/peers/:id/test", middleware.RequireRole(models.UserRoleAdmin, models.UserRoleTechnician), h.wireguardHandler.TestReachability)
		wireguard.GET("/sites/:site_id/suggested-subnets", h.wireguardHandler.SuggestSubnets)
	}

	provisionJobs := api.Group("/provision-jobs")
	provisionJobs.Use(authenticated)
	{
		provisionJobs.GET("/:id", h.provisionHandler.GetProvisionJob)
	}

	admin := api.Group("/admin")
	admin.Use(authenticated)
	admin.Use(middleware.RequireRole(models.UserRoleAdmin))
	{
		admin.POST("/seed-events", h.seedHandler.SeedEventHistory)
	}
}
