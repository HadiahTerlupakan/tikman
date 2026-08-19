package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// WebhookType represents the type of webhook integration
type WebhookType string

const (
	WebhookTypeGeneric WebhookType = "generic"
	WebhookTypeDiscord WebhookType = "discord"
	WebhookTypeSlack   WebhookType = "slack"
)

// NotificationSettings represents user notification preferences
type NotificationSettings struct {
	ID             uuid.UUID   `gorm:"type:uuid;primary_key" json:"id"`
	UserID         uuid.UUID   `gorm:"type:uuid;uniqueIndex;not null" json:"user_id"`
	EmailEnabled   bool        `gorm:"default:false" json:"email_enabled"`
	EmailAddress   string      `gorm:"type:varchar(255)" json:"email_address"`
	WebhookEnabled bool        `gorm:"default:false" json:"webhook_enabled"`
	WebhookURL     string      `gorm:"type:varchar(500)" json:"webhook_url"`
	WebhookType    WebhookType `gorm:"type:varchar(20);default:generic" json:"webhook_type"`
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
}

// TableName specifies the table name for GORM
func (NotificationSettings) TableName() string {
	return "notification_settings"
}

// BeforeCreate hook to generate UUID
func (ns *NotificationSettings) BeforeCreate(tx *gorm.DB) error {
	if ns.ID == uuid.Nil {
		ns.ID = uuid.New()
	}
	return nil
}
