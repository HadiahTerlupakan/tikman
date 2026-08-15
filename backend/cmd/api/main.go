package main

// TikMan API Server - Main entry point

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/tikman/olt-provisioning/internal/api"
	"github.com/tikman/olt-provisioning/internal/auth"
	"github.com/tikman/olt-provisioning/internal/config"
	"github.com/tikman/olt-provisioning/internal/database"
	"github.com/tikman/olt-provisioning/internal/logger"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"github.com/tikman/olt-provisioning/internal/worker"
	"go.uber.org/zap"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(fmt.Sprintf("Failed to load config: %v", err))
	}

	log, err := logger.New(cfg.LogLevel)
	if err != nil {
		panic(fmt.Sprintf("Failed to create logger: %v", err))
	}
	defer log.Sync()

	log.Info("Starting API server", zap.Int("port", cfg.APIPort))

	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatal("Failed to connect to database", zap.Error(err))
	}
	log.Info("Database connected successfully")

	if err := models.AutoMigrate(db); err != nil {
		log.Fatal("Failed to run migrations", zap.Error(err))
	}
	log.Info("Database migrations completed")

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

	// Initialize services for monitoring worker
	oltService := services.NewOLTService(db, cfg.EncryptionKey)
	ontService := services.NewONTService(db)
	metricsService := services.NewMetricsService(db)
	_ = metricsService // Will be used in Task 3 worker and Task 4 API handler

	// Start monitoring worker (30-second interval for real-time status)
	monitoringWorker := worker.NewMonitoringWorker(db, oltService, ontService, 30*time.Second)
	monitoringWorker.Start()
	log.Info("Monitoring worker started", zap.Duration("interval", 30*time.Second))

	router := api.Setup(cfg, db, sessionStore, log)

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
	monitoringWorker.Stop()
	log.Info("Server stopped gracefully")
}
