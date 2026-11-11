package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// The link statuses a monitored port is recorded in.
const (
	PortLinkUp   = "up"
	PortLinkDown = "down"
)

// PortLinkState is the last link status TikMan saw for a port a graph report
// names. LibreNMS's alert only says that something on a device changed; this
// row is what tells a new transition from an alert delivered twice.
type PortLinkState struct {
	ID       uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	DeviceID int       `gorm:"not null;uniqueIndex:idx_port_link_states_port" json:"device_id"`
	IfName   string    `gorm:"type:varchar(255);not null;uniqueIndex:idx_port_link_states_port" json:"if_name"`
	Status   string    `gorm:"type:varchar(16);not null" json:"status"`
	// ChangedAt is when Status last changed, which a recovery notice counts
	// the outage from.
	ChangedAt time.Time `gorm:"not null" json:"changed_at"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName pins the table name.
func (PortLinkState) TableName() string { return "port_link_states" }

// BeforeCreate assigns the ID.
func (p *PortLinkState) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

// PortAlertDestination is the NOC's own WhatsApp group and Telegram chat,
// which hear of every monitored port going down or up. There is at most one
// row; migration 77 enforces it.
type PortAlertDestination struct {
	ID             uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	WAGroupID      *uuid.UUID `gorm:"type:uuid" json:"wa_group_id"`
	TelegramChatID *uuid.UUID `gorm:"type:uuid" json:"telegram_chat_id"`
	// TelegramThreadID is the forum topic; nil is General, or the chat itself
	// when it is no forum.
	TelegramThreadID *int64    `json:"telegram_thread_id"`
	Enabled          bool      `gorm:"not null;default:true" json:"enabled"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// TableName pins the table name; singular because there is one.
func (PortAlertDestination) TableName() string { return "port_alert_destination" }

// BeforeCreate assigns the ID.
func (d *PortAlertDestination) BeforeCreate(tx *gorm.DB) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	return nil
}
