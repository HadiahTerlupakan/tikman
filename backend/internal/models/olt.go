package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type OLTStatus string
type OLTProtocol string

// OLTModel selects which SNMP dialect the connectivity layer speaks to a
// device. C300 and C320 are distinct models even though their OIDs currently
// match, so a firmware divergence later needs no migration.
type OLTModel string

const (
	OLTStatusOnline  OLTStatus = "online"
	OLTStatusOffline OLTStatus = "offline"
	OLTStatusError   OLTStatus = "error"

	OLTProtocolSSH    OLTProtocol = "ssh"
	OLTProtocolTelnet OLTProtocol = "telnet"

	OLTModelZTEC300 OLTModel = "zte_c300"
	OLTModelZTEC320 OLTModel = "zte_c320"
	OLTModelHSGQ    OLTModel = "hsgq"
)

type OLT struct {
	ID                  uuid.UUID   `gorm:"type:uuid;primaryKey"`
	SiteID              uuid.UUID   `gorm:"type:uuid;not null;index"`
	Name                string      `gorm:"type:varchar(255);not null"`
	IPAddress           string      `gorm:"type:varchar(45);not null"`
	SSHPort             int         `gorm:"default:22"`
	TelnetPort          int         `gorm:"default:23"`
	SNMPPort            int         `gorm:"default:161"`
	SNMPCommunity       string      `gorm:"type:varchar(100);default:public"`
	PreferredProtocol   OLTProtocol `gorm:"type:varchar(20);default:ssh"`
	Model               OLTModel    `gorm:"type:varchar(30);not null;default:zte_c300"`
	Username            string      `gorm:"type:varchar(100);not null"`
	Password            string      `gorm:"type:varchar(255);not null"` // encrypted
	Status              OLTStatus   `gorm:"type:varchar(20);default:offline"`
	Rack                int         `gorm:"default:0"`
	Shelf               int         `gorm:"default:0"`
	Slot                int         `gorm:"default:0"`
	DiscoveryPhase      string      `gorm:"type:varchar(20);default:idle"`
	DiscoveryTotal      int         `gorm:"default:0"`
	DiscoveryRegistered int         `gorm:"default:0"`
	DiscoveryPolled     int         `gorm:"default:0"`
	DiscoveryError      string      `gorm:"type:text"`
	DiscoveryStartedAt  *time.Time  `json:"discovery_started_at,omitempty"`
	DiscoveryLastPollAt *time.Time  `json:"discovery_last_poll_at,omitempty"`
	LastSeen            *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

func (o *OLT) BeforeCreate(tx *gorm.DB) error {
	if o.ID == uuid.Nil {
		o.ID = uuid.New()
	}
	return nil
}

func (o *OLT) TableName() string {
	return "olts"
}
