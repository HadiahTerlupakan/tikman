package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// GraphReport sends a customer's LibreNMS port graphs into one WhatsApp group
// once a day. A group carries at most one report, so the group is the
// report's identity as far as the admins are concerned.
type GraphReport struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	WAGroupID    uuid.UUID `gorm:"type:uuid;not null;uniqueIndex" json:"wa_group_id"`
	CustomerName string    `gorm:"type:varchar(255);not null" json:"customer_name"`
	Enabled      bool      `gorm:"not null;default:true" json:"enabled"`
	// Message is the caption the image is sent with; empty means the default
	// greeting (services.DefaultGraphReportMessage).
	Message string `gorm:"type:text;not null;default:''" json:"message"`
	// MentionAll tags every member but our own number without writing their
	// addresses into the caption, so even those who muted the group are told.
	MentionAll bool `gorm:"not null;default:false" json:"mention_all"`
	// SendTime is when the daily send starts, "HH:MM" in WIB.
	SendTime string `gorm:"type:varchar(5);not null;default:'06:00'" json:"send_time"`
	// LastSentOn is the WIB calendar date of the last daily send, stored as
	// that date at UTC midnight: a WIB midnight written to a date column is
	// cast through the session zone and lands on the day before.
	LastSentOn    *time.Time `gorm:"type:date" json:"-"`
	LastAttemptAt *time.Time `json:"-"`
	LastError     string     `gorm:"type:text;not null;default:''" json:"-"`
	CreatedBy     *uuid.UUID `gorm:"type:uuid" json:"-"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
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
