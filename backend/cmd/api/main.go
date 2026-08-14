package main

import (
	"fmt"
	"log"

	"github.com/tikman/olt-provisioning/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	fmt.Printf("Starting API server on port %d\n", cfg.APIPort)
	fmt.Println("Config loaded successfully")
}
