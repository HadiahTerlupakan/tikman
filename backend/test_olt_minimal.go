package main

import (
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/config"
	"github.com/tikman/olt-provisioning/internal/database"
)

type OLTStatus string
type OLTProtocol string

const (
	OLTStatusOnline  OLTStatus = "online"
	OLTStatusOffline OLTStatus = "offline"
	OLTStatusError   OLTStatus = "error"

	OLTProtocolSSH    OLTProtocol = "ssh"
	OLTProtocolTelnet OLTProtocol = "telnet"
)

// Minimal OLT without foreign keys
type OLTMinimal struct {
	ID                uuid.UUID   `gorm:"type:uuid;primary_key"`
	SiteID            uuid.UUID   `gorm:"type:uuid;not null;index"`
	Name              string      `gorm:"type:varchar(255);not null"`
	IPAddress         string      `gorm:"type:varchar(45);not null"`
	SSHPort           int         `gorm:"default:22"`
	TelnetPort        int         `gorm:"default:23"`
	SNMPPort          int         `gorm:"default:161"`
	SNMPCommunity     string      `gorm:"type:varchar(100);default:public"`
	PreferredProtocol OLTProtocol `gorm:"type:varchar(20);default:ssh"`
	Username          string      `gorm:"type:varchar(100);not null"`
	Password          string      `gorm:"type:varchar(255);not null"`
	Status            OLTStatus   `gorm:"type:varchar(20);default:offline"`
	LastSeen          *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (OLTMinimal) TableName() string {
	return "olts_test"
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("Failed to load config:", err)
	}

	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatal("Failed to connect:", err)
	}

	fmt.Println("Testing OLT migration without foreign keys...")
	if err := db.AutoMigrate(&OLTMinimal{}); err != nil {
		fmt.Printf("FAILED: %v\n", err)
	} else {
		fmt.Println("OK!")
	}
}
