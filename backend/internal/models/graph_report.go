package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// GraphReport sends a customer's LibreNMS port graphs once a day into a
// WhatsApp group, a Telegram chat, or both. Each carries at most one report,
// so the group is the report's identity as far as the admins are concerned;
// in a Telegram forum that holds per topic, General counting as one.
type GraphReport struct {
	ID        uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	WAGroupID *uuid.UUID `gorm:"type:uuid;uniqueIndex" json:"wa_group_id"`
	// TelegramChatID has no index tag: a report is unique per topic, through
	// an expression index migration 76 builds, and AutoMigrate, which runs
	// before the SQL files, would rebuild the per-chat one it drops.
	TelegramChatID *uuid.UUID `gorm:"type:uuid" json:"telegram_chat_id"`
	// TelegramThreadID is the forum topic the report goes to; nil is General,
	// or the chat itself when it is no forum.
	TelegramThreadID *int64 `json:"telegram_thread_id"`
	CustomerName     string `gorm:"type:varchar(255);not null" json:"customer_name"`
	Enabled          bool   `gorm:"not null;default:true" json:"enabled"`
	// Message is the caption the image is sent with; empty means the default
	// greeting (services.DefaultGraphReportMessage).
	Message string `gorm:"type:text;not null;default:''" json:"message"`
	// MentionAll tags every member but TikMan's own numbers without writing their
	// addresses into the caption, so even those who muted the group are told.
	MentionAll bool `gorm:"not null;default:false" json:"mention_all"`
	// NotifyPortStatus also posts a notice into the report's destinations when
	// one of its ports goes down or comes back. Off by default: a customer
	// group that only asked for the morning graphs is not told of outages
	// unless an admin decides it should be.
	NotifyPortStatus bool `gorm:"not null;default:false" json:"notify_port_status"`
	// SendTime is when the daily send starts, "HH:MM" in WIB.
	SendTime string `gorm:"type:varchar(5);not null;default:'06:00'" json:"send_time"`
	// LastSentOn is the WIB calendar date of the last daily send, stored as
	// that date at UTC midnight: a WIB midnight written to a date column is
	// cast through the session zone and lands on the day before.
	// It is set once every destination has that day's report.
	LastSentOn *time.Time `gorm:"type:date" json:"-"`
	// WASentOn and TelegramSentOn are the same per destination, so a retry
	// after one of them failed does not post into the other twice.
	WASentOn       *time.Time `gorm:"type:date" json:"-"`
	TelegramSentOn *time.Time `gorm:"type:date" json:"-"`
	LastAttemptAt  *time.Time `json:"-"`
	LastError      string     `gorm:"type:text;not null;default:''" json:"-"`
	CreatedBy      *uuid.UUID `gorm:"type:uuid" json:"-"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// TableName pins the table name.
func (GraphReport) TableName() string { return "graph_reports" }

// BeforeCreate assigns the ID.
func (r *GraphReport) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return nil
}

// GraphReportPort is one graph in a report, sent in Position order.
type GraphReportPort struct {
	ID       uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	ReportID uuid.UUID `gorm:"type:uuid;not null;index" json:"-"`
	// DeviceID is LibreNMS's device_id; DeviceName only labels it on the page.
	DeviceID   int    `gorm:"not null" json:"device_id"`
	DeviceName string `gorm:"type:varchar(255);not null" json:"device_name"`
	IfName     string `gorm:"type:varchar(255);not null" json:"if_name"`
	Label      string `gorm:"type:varchar(255);not null" json:"label"`
	Position   int    `gorm:"not null" json:"position"`
}

// TableName pins the table name.
func (GraphReportPort) TableName() string { return "graph_report_ports" }

// BeforeCreate assigns the ID.
func (p *GraphReportPort) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}
