package api

import (
	"fmt"
	"time"

	firebase "firebase.google.com/go/v4"
	"github.com/redis/go-redis/v9"
	"github.com/tikman/olt-provisioning/internal/auth"
	"github.com/tikman/olt-provisioning/internal/config"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/services"
	"github.com/tikman/olt-provisioning/internal/wa"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// handlers is every HTTP handler the router registers. It exists so that
// registering routes reads as routes: the construction below is linear
// dependency wiring, and interleaving the two made the one function nobody
// could scan.
type handlers struct {
	authHandler            *AuthHandler
	userHandler            *UserHandler
	siteHandler            *SiteHandler
	settingHandler         *SettingHandler
	dashboardHandler       *DashboardHandler
	oltHandler             *OLTHandler
	ontHandler             *ONTHandler
	metricsHandler         *MetricsHandler
	eventHandler           *EventHandler
	unconfiguredONUHandler *UnconfiguredONUHandler
	seedHandler            *SeedHandler
	configTemplateHandler  *ConfigTemplateHandler
	wireguardHandler       *WireGuardHandler
	distributionHandler    *DistributionHandler
	firebaseTokenHandler   *FirebaseTokenHandler
	csHandler              *CSHandler
	pushHandler            *PushHandler
	provisionHandler       *ProvisionHandler
	zteProvisionHandler    *ZTEProvisionHandler
}

// newHandlers builds the services and handlers one API process needs, and
// returns alongside them the three things cmd/api drives itself: the push
// notifier whose Firebase sender it sets, the listener it starts, and the
// presence store it shares with the CS surface.
func newHandlers(cfg *config.Config, db *gorm.DB, authStore *auth.Store, logger *zap.Logger,
	wgService *services.WireGuardService, firebaseApp *firebase.App) (
	*handlers, *services.PushNotifierService, *PushEventListener, *services.RedisPresence) {

	// Created before the services because the OLT service uses it to read CLI
	// profile lists during discovery.
	commanderFactory := connectivity.NewCommanderFactoryWithEncryption(5*time.Second, cfg.EncryptionKey)

	userService := services.NewUserService(db)
	ontService := services.NewONTServiceWithEncryption(db, cfg.EncryptionKey)
	auditService := services.NewAuditService(db, logger)
	metricsService := services.NewMetricsService(db)
	configTemplateService := services.NewConfigTemplateService(db, auditService)

	cs := newCSStack(cfg, db, logger, auditService, ontService, userService)
	provisionHandler, zteProvisionHandler := newProvisioningHandlers(cfg, db, logger, auditService, configTemplateService, commanderFactory)

	return &handlers{
		authHandler:      NewAuthHandler(userService, authStore, cfg.Environment == productionEnvironment),
		userHandler:      NewUserHandler(userService, auditService),
		siteHandler:      NewSiteHandler(services.NewSiteService(db), auditService),
		settingHandler:   NewSettingHandler(services.NewSettingService(db, cfg.EncryptionKey), auditService),
		dashboardHandler: NewDashboardHandler(services.NewDashboardService(db)),
		oltHandler: NewOLTHandler(services.NewOLTServiceWithCommander(db, cfg.EncryptionKey, commanderFactory),
			services.NewOLTValidatorService(db), auditService, ontService, services.NewPollJobService(db)),
		ontHandler: NewONTHandler(ontService, metricsService, auditService,
			services.NewZTEONURemovalService(db, ontService, commanderFactory)),
		metricsHandler:         NewMetricsHandler(metricsService),
		eventHandler:           NewEventHandler(services.NewEventService(db)),
		unconfiguredONUHandler: NewUnconfiguredONUHandler(services.NewUnconfiguredONUService(db)),
		seedHandler:            NewSeedHandler(db, cfg.EncryptionKey),
		configTemplateHandler:  NewConfigTemplateHandler(configTemplateService),
		wireguardHandler:       NewWireGuardHandler(wgService, auditService),
		distributionHandler:    NewDistributionHandler(services.NewDistributionService(db)),
		firebaseTokenHandler:   NewFirebaseTokenHandler(firebaseApp, logger),
		csHandler:              cs.handler,
		pushHandler:            cs.push,
		provisionHandler:       provisionHandler,
		zteProvisionHandler:    zteProvisionHandler,
	}, cs.notifier, cs.listener, cs.presence
}

// csStack is the CS inbox's half of the wiring: its handler, and the three
// pieces cmd/api drives itself once Setup returns.
type csStack struct {
	handler  *CSHandler
	push     *PushHandler
	notifier *services.PushNotifierService
	listener *PushEventListener
	presence *services.RedisPresence
}

func newCSStack(cfg *config.Config, db *gorm.DB, logger *zap.Logger, auditService *services.AuditService,
	ontService *services.ONTService, userService *services.UserService) csStack {

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
		csPurgeService, csAssignmentService, auditService, ontService, userService, csPublisher, csRedisClient,
		logger, cfg.WAMediaDir,
	)

	pushService := services.NewPushService(db)
	pushHandler := NewPushHandler(pushService)
	// nil Sender for now: cmd/api is the one place that knows whether a real
	// Firebase client exists, and sets it via SetSender once Setup returns —
	// Setup only wires the notifier's other dependencies.
	pushNotifier := services.NewPushNotifierService(nil, pushService, csConversationService, csMessageService)
	pushListener := NewPushEventListener(csRedisClient, pushNotifier, logger)

	return csStack{handler: csHandler, push: pushHandler, notifier: pushNotifier, listener: pushListener, presence: csPresence}
}

func newProvisioningHandlers(cfg *config.Config, db *gorm.DB, logger *zap.Logger,
	auditService *services.AuditService, configTemplateService *services.ConfigTemplateService,
	commanderFactory services.CommanderFactory) (*ProvisionHandler, *ZTEProvisionHandler) {

	// Provisioning pipeline: the factory above creates per-OLT commanders since
	// each OLT has its own address and credentials.
	provisionJobService := services.NewJobService(db, auditService)
	snapshotService := services.NewSnapshotServiceWithCommander(db, connectivity.DriverFor, logger, commanderFactory)
	rollbackEngine := services.NewRollbackEngineForOLTs(commanderFactory, logger)
	ontProvisioningService := services.NewOntProvisioningServiceWithTemplates(db, provisionJobService, snapshotService, commanderFactory, rollbackEngine, auditService, logger, configTemplateService)
	provisionHandler := NewProvisionHandler(ontProvisioningService)
	zteProvisioner := services.NewZTEGPONRegisterService(db, provisionJobService, snapshotService, commanderFactory, rollbackEngine, logger).WithEncryptionKey(cfg.EncryptionKey)
	zteProvisionHandler := NewZTEProvisionHandler(zteProvisioner, provisionJobService)

	return provisionHandler, zteProvisionHandler
}
