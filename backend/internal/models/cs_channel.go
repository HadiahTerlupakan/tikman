package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ChannelRole is how much a paired number may do on a WhatsApp Channel. Only
// the two roles that may post are ever stored; whatsmeow also reports
// "subscriber" and "guest", which the sync drops.
type ChannelRole string

const (
	ChannelRoleOwner ChannelRole = "owner"
	ChannelRoleAdmin ChannelRole = "admin"
)

// WAChannel is one WhatsApp Channel a paired number administers.
//
// It is a mirror of what WhatsApp answered, never a source of truth: the wa
// process replaces every row for an account on each sync, which is what makes
// a revoked admin right disappear without any removal logic.
type WAChannel struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	WAAccountID uuid.UUID `gorm:"type:uuid;not null;index" json:"wa_account_id"`
	// The column tag is load-bearing for the same reason it is on WAAccount:
	// GORM's naming strategy renders JID as "j_id", which is not the column
	// the migration and every query name.
	JID             string      `gorm:"column:jid;type:varchar(128);not null" json:"jid"`
	Name            string      `gorm:"type:varchar(255)" json:"name"`
	Role            ChannelRole `gorm:"type:varchar(20);not null" json:"role"`
	SubscriberCount int         `json:"subscriber_count"`
	SyncedAt        time.Time   `gorm:"not null" json:"synced_at"`
}

func (c *WAChannel) BeforeCreate(tx *gorm.DB) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return nil
}

// TableName specifies the table name for GORM.
func (WAChannel) TableName() string { return "wa_channels" }
