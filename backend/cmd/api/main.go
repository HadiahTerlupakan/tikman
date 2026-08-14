package main

import (
	"fmt"

	"github.com/tikman/olt-provisioning/internal/config"
	"github.com/tikman/olt-provisioning/internal/database"
	"github.com/tikman/olt-provisioning/internal/logger"
	"github.com/tikman/olt-provisioning/internal/models"
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

	// Run migrations
	if err := models.AutoMigrate(db); err != nil {
		log.Fatal("Failed to run migrations", zap.Error(err))
	}
	log.Info("Database migrations completed")

	// TODO: Start server
	_ = db // Will be used when implementing server
	select {}
}
