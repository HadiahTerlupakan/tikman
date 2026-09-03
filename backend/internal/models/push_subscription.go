package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PushSubscription is one browser/device's Firebase Installation ID, owned by
// the user who registered it. A user can hold several rows — one per browser or
// device.
type PushSubscription struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	UserID    uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`
	FID       string    `gorm:"column:fid;type:varchar(1024);uniqueIndex;not null" json:"fid"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (p *PushSubscription) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

func (PushSubscription) TableName() string { return "push_subscriptions" }
