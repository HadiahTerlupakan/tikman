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

	fmt.Println("Starting Worker service")
	fmt.Printf("Connected to Redis at %s:%d\n", cfg.RedisHost, cfg.RedisPort)
}
