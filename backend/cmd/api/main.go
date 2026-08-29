package main

// TikMan API Server - Main entry point

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/tikman/olt-provisioning/internal/api"
	"github.com/tikman/olt-provisioning/internal/auth"
	"github.com/tikman/olt-provisioning/internal/config"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/database"
	"github.com/tikman/olt-provisioning/internal/logger"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"go.uber.org/zap"
)

// wireguardStatusInterval is comfortably below the three-minute liveness grace,
// so a peer that drops is noticed within one cycle.
const wireguardStatusInterval = 30 * time.Second

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(fmt.Sprintf("Failed to load config: %v", err))
	}

	log, err := logger.New(cfg.LogLevel)
	if err != nil {
		panic(fmt.Sprintf("Failed to create logger: %v", err))
	}
	defer func() {
		_ = log.Sync()
	}()

	log.Info("Starting API server", zap.Int("port", cfg.APIPort))

	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatal("Failed to connect to database", zap.Error(err))
	}
	log.Info("Database connected successfully")

	if err := models.AutoMigrate(db); err != nil {
		log.Fatal("Failed to run migrations", zap.Error(err))
	}
	log.Info("GORM AutoMigrate completed")

	// SQL migrations (TimescaleDB hypertables, FKs, indexes) run after
	// AutoMigrate and are version-tracked so they apply exactly once per
	// database regardless of volume lifecycle.
	migrationsDir := os.Getenv("MIGRATIONS_DIR")
	if migrationsDir == "" {
		migrationsDir = "/app/migrations"
	}
	if err := database.RunSQLMigrations(db, migrationsDir); err != nil {
		log.Fatal("Failed to run SQL migrations", zap.Error(err))
	}
	log.Info("SQL migrations completed")

	if err := services.CreateDefaultAdmin(db, log); err != nil {
		log.Fatal("Failed to seed default admin", zap.Error(err))
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.RedisHost, cfg.RedisPort),
		Password: cfg.RedisPassword,
		DB:       0,
	})

	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatal("Failed to connect to Redis", zap.Error(err))
	}
	log.Info("Redis connected successfully")

	sessionStore := auth.NewStore(redisClient, 24*time.Hour)

	wgService := services.NewWireGuardService(db, cfg.EncryptionKey, connectivity.NewWireGuardDevice())
	// A tunnel that cannot come up must not stop the API: the operator needs the
	// UI precisely to fix it.
	if err := wgService.Reconcile(); err != nil {
		log.Warn("Failed to apply WireGuard configuration at startup", zap.Error(err))
	}
	go wgService.RunStatusRefresher(context.Background(), wireguardStatusInterval, log)

	// Without this Gin stays in debug mode whatever ENVIRONMENT says: it prints
	// a route dump and a per-request debug line, and warns about it on every
	// boot.
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.Default()
	// Do not widen: gin's default trusts every proxy, making c.ClientIP() equal
	// to any X-Forwarded-For the caller sends, which bypasses the per-IP rate
	// limiter and forges audit-log IPs. This is the docker bridge range nginx
	// sits in. Requests reaching the published host port keep their real source
	// address and fall outside it, except when docker-proxy relays them from the
	// host itself.
	if err := engine.SetTrustedProxies([]string{"172.16.0.0/12"}); err != nil {
		log.Fatal("Failed to set trusted proxies", zap.Error(err))
	}

	router := api.Setup(engine, cfg, db, sessionStore, log, wgService)

	addr := fmt.Sprintf(":%d", cfg.APIPort)
	log.Info("Server starting", zap.String("address", addr))

	// Graceful shutdown
	go func() {
		if err := router.Run(addr); err != nil {
			log.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down server...")
	log.Info("Server stopped gracefully")
}
