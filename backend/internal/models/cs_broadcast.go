package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// BroadcastDestination is where one announcement goes.
type BroadcastDestination string

const (
	DestinationChannel BroadcastDestination = "channel"
	DestinationStatus  BroadcastDestination = "status"
)

// BroadcastPostStatus is how far an announcement has travelled. Three, not the
// five a chat message has: neither a channel nor a status sends receipts.
type BroadcastPostStatus string

const (
	BroadcastQueued BroadcastPostStatus = "queued"
	BroadcastSent   BroadcastPostStatus = "sent"
	BroadcastFailed BroadcastPostStatus = "failed"
)

// WABroadcastPost is one announcement on its way out, and afterwards the
// record that it went.
//
// A row with Status BroadcastQueued is the outbox — there is no second table,
// the same decision CSMessage records. An announcement written while the wa
// process was down is still sitting here when it comes back.
//
// One announcement sent to two destinations is two rows, so a partial failure
// reads as what it is rather than forcing one row to carry two outcomes.
type WABroadcastPost struct {
	ID          uuid.UUID            `gorm:"type:uuid;primaryKey" json:"id"`
	WAAccountID uuid.UUID            `gorm:"type:uuid;not null;index" json:"wa_account_id"`
	Destination BroadcastDestination `gorm:"type:varchar(20);not null;index" json:"destination"`
	// DestinationJID is the channel this went to, and nil for a status: a
	// status has no target beyond WAAccountID. It is copied rather than
	// pointing at wa_channels, because the channel sync deletes and rebuilds
	// those rows and history must not follow them.
	DestinationJID *string             `gorm:"column:destination_jid;type:varchar(128);index" json:"destination_jid,omitempty"`
	SenderUserID   uuid.UUID           `gorm:"type:uuid;not null;index" json:"sender_user_id"`
	Kind           MessageKind         `gorm:"type:varchar(20);not null" json:"kind"`
	Body           string              `gorm:"type:text" json:"body"`
	MediaPath      string              `gorm:"type:text" json:"-"`
	MediaMime      string              `gorm:"type:varchar(100)" json:"media_mime,omitempty"`
	MediaSize      int64               `json:"media_size,omitempty"`
	MediaFilename  string              `gorm:"type:varchar(255)" json:"media_filename,omitempty"`
	Status         BroadcastPostStatus `gorm:"type:varchar(20);not null;index" json:"status"`
	FailReason     string              `gorm:"type:text" json:"fail_reason,omitempty"`
	WAMessageID    *string             `gorm:"type:varchar(128)" json:"wa_message_id,omitempty"`
	CreatedAt      time.Time           `json:"created_at"`
	SentAt         *time.Time          `json:"sent_at,omitempty"`
}

func (p *WABroadcastPost) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

// TableName specifies the table name for GORM.
func (WABroadcastPost) TableName() string { return "wa_broadcast_posts" }
