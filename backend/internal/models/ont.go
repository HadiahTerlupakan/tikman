package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ONTStatus string

const (
	ONTStatusOnline      ONTStatus = "online"
	ONTStatusOffline     ONTStatus = "offline"
	ONTStatusLOS         ONTStatus = "los"
	ONTStatusDyingGasp   ONTStatus = "dying_gasp"
	ONTStatusUnconfirmed ONTStatus = "unconfirmed"
	ONTStatusPending     ONTStatus = "pending"
)

type ONT struct {
	ID               uuid.UUID  `gorm:"type:uuid;primary_key"`
	OLTID            uuid.UUID  `gorm:"type:uuid;not null;uniqueIndex:idx_olt_pon_ont"`
	SerialNumber     string     `gorm:"type:varchar(100);not null;index"`
	PONPort          string     `gorm:"type:varchar(50);not null;uniqueIndex:idx_olt_pon_ont"`
	ONTID            int        `gorm:"not null;uniqueIndex:idx_olt_pon_ont"`
	ServiceProfileID *uuid.UUID `gorm:"type:uuid"`
	LineProfileID    *uuid.UUID `gorm:"type:uuid"`
	CustomerName     string     `gorm:"type:varchar(255)"`
	Description      string     `gorm:"type:text"`
	Status           ONTStatus  `gorm:"type:varchar(20);default:pending;index:idx_olt_status"`
	SignalRX         *float64   `gorm:"comment:'Signal in dBm'"`
	SignalTX         *float64   `gorm:"comment:'Signal in dBm'"`
	Distance         *int       `gorm:"comment:'Distance in meter'"`
	LastOnline       *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (o *ONT) BeforeCreate(tx *gorm.DB) error {
	if o.ID == uuid.Nil {
		o.ID = uuid.New()
	}
	return nil
}

func (o *ONT) TableName() string {
	return "onts"
}
