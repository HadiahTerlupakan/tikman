package models

import (
	"time"

	"github.com/google/uuid"
)

// ONTEvent represents a status change event for an ONT (online/offline)
type ONTEvent struct {
	ID              int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ONTID           uuid.UUID `gorm:"type:uuid;not null;index:idx_ont_events_ont_id;index:idx_ont_events_ont_time" json:"ont_id"`
	EventType       string    `gorm:"type:varchar(20);not null;check:event_type IN ('online','offline');index:idx_ont_events_type" json:"event_type"`
	EventTime       time.Time `gorm:"not null;index:idx_ont_events_event_time;index:idx_ont_events_ont_time" json:"event_time"`
	Reason          string    `gorm:"type:text" json:"reason,omitempty"`
	DurationSeconds *int64    `gorm:"column:duration_seconds" json:"duration_seconds,omitempty"`
	CreatedAt       time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"created_at"`

	// Relations
	ONT ONT `gorm:"foreignKey:ONTID;constraint:OnDelete:CASCADE" json:"ont,omitempty"`
}

// TableName specifies the table name for GORM
func (ONTEvent) TableName() string {
	return "ont_events"
}

// EventType constants
const (
	EventTypeOnline  = "online"
	EventTypeOffline = "offline"
)

// Common offline reasons from ZTE OLT
const (
	ReasonLOS        = "LOS"         // Loss of Signal
	ReasonDyingGasp  = "Dying-Gasp"  // ONT shutdown signal
	ReasonPowerOff   = "Power-Off"   // Manual power off
	ReasonUnknown    = "Unknown"     // Reason not available
	ReasonManual     = "Manual"      // Manual deactivation
	ReasonAuthFailed = "Auth-Failed" // Authentication failure
)
