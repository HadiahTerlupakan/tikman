package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
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

type OLT struct {
	ID                uuid.UUID   `gorm:"type:uuid;primary_key"`
	SiteID            uuid.UUID   `gorm:"type:uuid;not null;index"`
	Name              string      `gorm:"type:varchar(255);not null"`
	IPAddress         string      `gorm:"type:varchar(45);not null"`
	SSHPort           int         `gorm:"default:22"`
	TelnetPort        int         `gorm:"default:23"`
	SNMPPort          int         `gorm:"default:161"`
	SNMPCommunity     string      `gorm:"type:varchar(100);default:'public'"`
	PreferredProtocol OLTProtocol `gorm:"type:varchar(20);default:'ssh'"`
	Username          string      `gorm:"type:varchar(100);not null"`
	Password          string      `gorm:"type:varchar(255);not null"` // encrypted
	Status            OLTStatus   `gorm:"type:varchar(20);default:'offline'"`
	LastSeen          *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time

	Site            Site             `gorm:"foreignKey:SiteID"`
	ServiceProfiles []ServiceProfile `gorm:"foreignKey:OLTID"`
	LineProfiles    []LineProfile    `gorm:"foreignKey:OLTID"`
	ONTs            []ONT            `gorm:"foreignKey:OLTID"`
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
