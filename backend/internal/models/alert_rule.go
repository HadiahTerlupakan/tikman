package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AlertMetricType represents the type of metric being monitored
type AlertMetricType string

const (
	AlertMetricRXPower      AlertMetricType = "rx_power"
	AlertMetricTXPower      AlertMetricType = "tx_power"
	AlertMetricTemperature  AlertMetricType = "temperature"
	AlertMetricErrorRate    AlertMetricType = "error_rate"
	AlertMetricONTStatus    AlertMetricType = "ont_status"
)

// AlertCondition represents the condition for triggering an alert
type AlertCondition string

const (
	AlertConditionBelow  AlertCondition = "below"
	AlertConditionAbove  AlertCondition = "above"
	AlertConditionEquals AlertCondition = "equals"
)

// AlertRule represents a configurable alert rule
type AlertRule struct {
	ID            uuid.UUID       `gorm:"type:uuid;primary_key" json:"id"`
	Name          string          `gorm:"type:varchar(100);not null" json:"name"`
	MetricType    AlertMetricType `gorm:"type:varchar(50);not null" json:"metric_type"`
	Condition     AlertCondition  `gorm:"type:varchar(20);not null" json:"condition"`
	Threshold     float64         `gorm:"type:decimal(10,2);not null" json:"threshold"`
	Severity      AlarmSeverity   `gorm:"type:varchar(20);not null" json:"severity"`
	Enabled       bool            `gorm:"default:true" json:"enabled"`
	NotifyEmail   bool            `gorm:"default:false" json:"notify_email"`
	NotifyWebhook bool            `gorm:"default:false" json:"notify_webhook"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

// TableName specifies the table name for GORM
func (AlertRule) TableName() string {
	return "alert_rules"
}

// BeforeCreate hook to generate UUID
func (ar *AlertRule) BeforeCreate(tx *gorm.DB) error {
	if ar.ID == uuid.Nil {
		ar.ID = uuid.New()
	}
	return nil
}

// Evaluate checks if a metric value triggers this rule
func (ar *AlertRule) Evaluate(value float64) bool {
	if !ar.Enabled {
		return false
	}

	switch ar.Condition {
	case AlertConditionBelow:
		return value < ar.Threshold
	case AlertConditionAbove:
		return value > ar.Threshold
	case AlertConditionEquals:
		return value == ar.Threshold
	default:
		return false
	}
}
