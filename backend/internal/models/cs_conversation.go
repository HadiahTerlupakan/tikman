package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// WAAccountStatus is where a connected WhatsApp number stands.
type WAAccountStatus string

const (
	WAAccountDisconnected WAAccountStatus = "disconnected"
	WAAccountPairing      WAAccountStatus = "pairing"
	WAAccountConnected    WAAccountStatus = "connected"
	WAAccountBanned       WAAccountStatus = "banned"
)

// WAAccount is one WhatsApp number the team answers from. More than one row is
// allowed from the start, so adding a second number later to spread the load —
// or to survive one number being blocked — costs a row rather than a migration.
type WAAccount struct {
	ID              uuid.UUID       `gorm:"type:uuid;primaryKey" json:"id"`
	Label           string          `gorm:"type:varchar(100);not null" json:"label"`
	JID             string          `gorm:"type:varchar(64)" json:"jid"`
	Status          WAAccountStatus `gorm:"type:varchar(20);not null" json:"status"`
	LastConnectedAt *time.Time      `json:"last_connected_at,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

func (a *WAAccount) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}

// TableName specifies the table name for GORM.
func (WAAccount) TableName() string { return "wa_accounts" }

// ConversationStatus is how far a customer's thread has got.
type ConversationStatus string

const (
	ConversationUnassigned ConversationStatus = "unassigned"
	ConversationOpen       ConversationStatus = "open"
	ConversationClosed     ConversationStatus = "closed"
)

// CSConversation is one customer's thread on one WhatsApp number.
type CSConversation struct {
	ID             uuid.UUID          `gorm:"type:uuid;primaryKey" json:"id"`
	WAAccountID    uuid.UUID          `gorm:"type:uuid;not null;uniqueIndex:uq_cs_conversations_peer,priority:1" json:"wa_account_id"`
	CustomerJID    string             `gorm:"type:varchar(64);not null;uniqueIndex:uq_cs_conversations_peer,priority:2" json:"customer_jid"`
	CustomerPhone  string             `gorm:"type:varchar(20);not null;index" json:"customer_phone"`
	CustomerName   string             `gorm:"type:varchar(255)" json:"customer_name"`
	AssignedUserID *uuid.UUID         `gorm:"type:uuid;index" json:"assigned_user_id,omitempty"`
	Status         ConversationStatus `gorm:"type:varchar(20);not null;index" json:"status"`
	ONTID          *uuid.UUID         `gorm:"type:uuid;index" json:"ont_id,omitempty"`
	LastMessageAt  time.Time          `gorm:"index" json:"last_message_at"`
	UnreadCount    int                `gorm:"not null;default:0" json:"unread_count"`
	CreatedAt      time.Time          `json:"created_at"`
	UpdatedAt      time.Time          `json:"updated_at"`
}

func (c *CSConversation) BeforeCreate(tx *gorm.DB) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return nil
}

// TableName specifies the table name for GORM.
func (CSConversation) TableName() string { return "cs_conversations" }
