package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type AuditLog struct {
	ID           uuid.UUID      `gorm:"type:uuid;primary_key"`
	UserID       *uuid.UUID     `gorm:"type:uuid;index:idx_user_created"`
	Action       string         `gorm:"type:varchar(100);not null"`
	ResourceType string         `gorm:"type:varchar(50);not null;index:idx_resource"`
	ResourceID   *uuid.UUID     `gorm:"type:uuid;index:idx_resource"`
	OldValue     datatypes.JSON `gorm:"type:jsonb"`
	NewValue     datatypes.JSON `gorm:"type:jsonb"`
	IPAddress    string         `gorm:"type:varchar(45)"`
	UserAgent    string         `gorm:"type:text"`
	CreatedAt    time.Time      `gorm:"index:idx_user_created"`

	User *User `gorm:"foreignKey:UserID"`
}

func (al *AuditLog) BeforeCreate(tx *gorm.DB) error {
	if al.ID == uuid.Nil {
		al.ID = uuid.New()
	}
	return nil
}

func (al *AuditLog) TableName() string {
	return "audit_logs"
}
