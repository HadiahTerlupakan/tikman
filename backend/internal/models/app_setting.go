package models

import (
	"time"

	"github.com/google/uuid"
)

// AppSetting stores one credential for an external integration. Value is always
// AES-256-GCM ciphertext, and is tagged json:"-" so the model can never be
// serialised into a response by accident.
type AppSetting struct {
	Name      string     `gorm:"primaryKey;size:64" json:"name"`
	Value     string     `gorm:"type:text;not null" json:"-"`
	UpdatedAt time.Time  `json:"updated_at"`
	UpdatedBy *uuid.UUID `gorm:"type:uuid" json:"updated_by,omitempty"`
}

// TableName returns the table name for AppSetting.
func (AppSetting) TableName() string {
	return "app_settings"
}
