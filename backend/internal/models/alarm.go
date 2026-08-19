package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AlarmSeverity represents the severity level of an alarm
type AlarmSeverity string

const (
	AlarmSeverityCritical AlarmSeverity = "critical"
	AlarmSeverityWarning  AlarmSeverity = "warning"
	AlarmSeverityInfo     AlarmSeverity = "info"
)

// AlarmStatus represents the status of an alarm
type AlarmStatus string

const (
	AlarmStatusActive       AlarmStatus = "active"
	AlarmStatusAcknowledged AlarmStatus = "acknowledged"
	AlarmStatusCleared      AlarmStatus = "cleared"
)

// AlarmType represents the type of alarm
type AlarmType string

const (
	AlarmTypeONTOffline       AlarmType = "ont_offline"
	AlarmTypeSignalLow        AlarmType = "signal_low"
	AlarmTypeHighTemperature  AlarmType = "high_temperature"
	AlarmTypeHighErrorRate    AlarmType = "high_error_rate"
	AlarmTypeSignalDegradation AlarmType = "signal_degradation"
)

// Alarm represents a monitoring alarm
type Alarm struct {
	ID              uuid.UUID      `gorm:"type:uuid;primary_key" json:"id"`
	OLTID           *uuid.UUID     `gorm:"type:uuid;index" json:"olt_id"`
	ONTID           *uuid.UUID     `gorm:"type:uuid;index" json:"ont_id"`
	Severity        AlarmSeverity  `gorm:"type:varchar(20);not null;index" json:"severity"`
	Type            AlarmType      `gorm:"type:varchar(50);not null" json:"type"`
	Message         string         `gorm:"type:text;not null" json:"message"`
	MetricValue     *float64       `gorm:"type:decimal(10,2)" json:"metric_value"`
	ThresholdValue  *float64       `gorm:"type:decimal(10,2)" json:"threshold_value"`
	Status          AlarmStatus    `gorm:"type:varchar(20);default:active;index" json:"status"`
	OccurredAt      time.Time      `gorm:"type:timestamp;not null;index" json:"occurred_at"`
	AcknowledgedAt  *time.Time     `json:"acknowledged_at"`
	AcknowledgedBy  *uuid.UUID     `gorm:"type:uuid" json:"acknowledged_by"`
	ClearedAt       *time.Time     `json:"cleared_at"`
	CreatedAt       time.Time      `json:"created_at"`
}

// TableName specifies the table name for GORM
func (Alarm) TableName() string {
	return "alarms"
}

// BeforeCreate hook to generate UUID
func (a *Alarm) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}
