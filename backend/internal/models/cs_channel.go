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

// ChannelPostStatus is how far an update has travelled. There are three, not
// the five a chat message has: a channel sends no receipts, so "delivered"
// and "read" would never arrive and must not be promised on screen.
type ChannelPostStatus string

const (
	ChannelPostQueued ChannelPostStatus = "queued"
	ChannelPostSent   ChannelPostStatus = "sent"
	ChannelPostFailed ChannelPostStatus = "failed"
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

// WAChannelPost is one update on its way to a channel, and afterwards the
// record that it went.
//
// A row with Status ChannelPostQueued is the outbox — there is no second
// table, the same decision CSMessage records. An update written while the wa
// process was down is still sitting here when it comes back.
//
// ChannelJID is copied rather than pointing at wa_channels, deliberately: the
// sync deletes and rebuilds those rows, and history must not follow them.
type WAChannelPost struct {
	ID            uuid.UUID         `gorm:"type:uuid;primaryKey" json:"id"`
	WAAccountID   uuid.UUID         `gorm:"type:uuid;not null;index" json:"wa_account_id"`
	ChannelJID    string            `gorm:"column:channel_jid;type:varchar(128);not null;index" json:"channel_jid"`
	SenderUserID  uuid.UUID         `gorm:"type:uuid;not null;index" json:"sender_user_id"`
	Kind          MessageKind       `gorm:"type:varchar(20);not null" json:"kind"`
	Body          string            `gorm:"type:text" json:"body"`
	MediaPath     string            `gorm:"type:text" json:"-"`
	MediaMime     string            `gorm:"type:varchar(100)" json:"media_mime,omitempty"`
	MediaSize     int64             `json:"media_size,omitempty"`
	MediaFilename string            `gorm:"type:varchar(255)" json:"media_filename,omitempty"`
	Status        ChannelPostStatus `gorm:"type:varchar(20);not null;index" json:"status"`
	FailReason    string            `gorm:"type:text" json:"fail_reason,omitempty"`
	WAMessageID   *string           `gorm:"type:varchar(128)" json:"wa_message_id,omitempty"`
	CreatedAt     time.Time         `json:"created_at"`
	SentAt        *time.Time        `json:"sent_at,omitempty"`
}

func (p *WAChannelPost) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

// TableName specifies the table name for GORM.
func (WAChannelPost) TableName() string { return "wa_channel_posts" }
