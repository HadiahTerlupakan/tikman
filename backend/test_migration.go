package main

import (
	"fmt"
	"log"

	"github.com/tikman/olt-provisioning/internal/config"
	"github.com/tikman/olt-provisioning/internal/database"
	"github.com/tikman/olt-provisioning/internal/models"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("Failed to load config:", err)
	}

	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatal("Failed to connect:", err)
	}

	// Test each model individually
	models := []interface{}{
		&models.User{},
		&models.Site{},
		&models.OLT{},
		&models.ServiceProfile{},
		&models.LineProfile{},
		&models.ONT{},
		&models.AuditLog{},
	}

	for _, model := range models {
		fmt.Printf("Migrating %T... ", model)
		if err := db.AutoMigrate(model); err != nil {
			fmt.Printf("FAILED: %v\n", err)
		} else {
			fmt.Println("OK")
		}
	}
}
