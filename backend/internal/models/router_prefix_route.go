package models

import (
	"time"

	"github.com/google/uuid"
)

// RouterPrefixRoute is one destination prefix's BGP candidates as the router
// last reported them. Routes is the candidates as JSON, kept as text so the
// SQLite test database can hold it too.
type RouterPrefixRoute struct {
	RouterID  uuid.UUID `gorm:"type:uuid;not null;primaryKey" json:"router_id"`
	Prefix    string    `gorm:"type:varchar(64);not null;primaryKey" json:"prefix"`
	Routes    string    `gorm:"type:text;not null" json:"-"`
	CheckedAt time.Time `gorm:"not null" json:"checked_at"`
}

// TableName returns the table name for RouterPrefixRoute.
func (RouterPrefixRoute) TableName() string { return "router_prefix_routes" }
