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

// RouterAnnouncedPrefix is one prefix the router announces to its BGP
// peers, from the address list their connections name as output.network.
type RouterAnnouncedPrefix struct {
	RouterID uuid.UUID `gorm:"type:uuid;not null;primaryKey" json:"router_id"`
	Prefix   string    `gorm:"type:varchar(64);not null;primaryKey" json:"prefix"`
}

// TableName returns the table name for RouterAnnouncedPrefix.
func (RouterAnnouncedPrefix) TableName() string { return "router_announced_prefixes" }

// RouterFilterRule is one routing filter rule as the router last listed it;
// Position is its place in evaluation order.
type RouterFilterRule struct {
	RouterID uuid.UUID `gorm:"type:uuid;not null;primaryKey" json:"router_id"`
	Position int       `gorm:"not null;primaryKey;autoIncrement:false" json:"position"`
	Chain    string    `gorm:"type:varchar(255);not null" json:"chain"`
	Rule     string    `gorm:"type:text;not null" json:"rule"`
	Disabled bool      `gorm:"not null" json:"disabled"`
	Comment  string    `gorm:"type:text;not null" json:"comment"`
}

// TableName returns the table name for RouterFilterRule.
func (RouterFilterRule) TableName() string { return "router_filter_rules" }
