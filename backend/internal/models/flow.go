package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// RouterFlowMinute is one minute of traffic for one (router, POP, upstream,
// direction, ASN), already scaled by the sampling ratio. A NULL POP is
// traffic of addresses in no POP's blocks. Like RouterInterfaceMetric it is a
// hypertable without a primary key, and its columns change in SQL migrations
// only: the continuous aggregates read them, and an ALTER COLUMN from
// AutoMigrate would be refused and abort startup.
type RouterFlowMinute struct {
	Time                time.Time  `gorm:"not null;index:idx_router_flow_minute_router_time,priority:2"`
	RouterID            uuid.UUID  `gorm:"type:uuid;not null;index:idx_router_flow_minute_router_time,priority:1"`
	POPInterfaceID      *uuid.UUID `gorm:"column:pop_interface_id;type:uuid"`
	UpstreamInterfaceID uuid.UUID  `gorm:"type:uuid;not null"`
	Direction           string     `gorm:"type:varchar(8);not null"`
	// bigint: four-byte AS numbers run past int4.
	ASN     int64 `gorm:"column:asn;not null"`
	Bytes   int64 `gorm:"not null"`
	Packets int64 `gorm:"not null"`
}

// TableName returns the table name for RouterFlowMinute.
func (RouterFlowMinute) TableName() string { return "router_flow_minute" }

// RouterFlowTopIP is one far-side address's bytes in a five-minute window,
// kept for the window's top addresses only.
type RouterFlowTopIP struct {
	Time           time.Time  `gorm:"not null;index:idx_router_flow_top_ips_router_time,priority:2"`
	RouterID       uuid.UUID  `gorm:"type:uuid;not null;index:idx_router_flow_top_ips_router_time,priority:1"`
	POPInterfaceID *uuid.UUID `gorm:"column:pop_interface_id;type:uuid"`
	Direction      string     `gorm:"type:varchar(8);not null"`
	IP             string     `gorm:"column:ip;type:inet;not null"`
	ASN            int64      `gorm:"column:asn;not null"`
	// Country is where the address was located (IPinfo), "" when unknown.
	Country string `gorm:"type:varchar(2);not null;default:''"`
	Bytes   int64  `gorm:"not null"`
	// Flows is the sampled flow records the window saw for the address.
	Flows int64 `gorm:"not null;default:0"`
	// Anonymity is tor, vpn or hosting from free lists, "" when none held it.
	Anonymity string `gorm:"type:varchar(8);not null;default:''"`
}

// TableName returns the table name for RouterFlowTopIP.
func (RouterFlowTopIP) TableName() string { return "router_flow_top_ips" }

// ASNName is an AS's name from the iptoasn dataset, kept so the API can label
// destinations without flowd. UpdatedAt is the dataset's download time.
type ASNName struct {
	ASN       int64     `gorm:"column:asn;primaryKey;autoIncrement:false" json:"asn"`
	Name      string    `gorm:"type:text;not null" json:"name"`
	Country   string    `gorm:"type:varchar(8);not null" json:"country"`
	UpdatedAt time.Time `gorm:"not null" json:"updated_at"`
}

// TableName returns the table name for ASNName.
func (ASNName) TableName() string { return "asn_names" }

// ASNType is a network's type as its operator states it in PeeringDB
// (Content, Cable/DSL/ISP, NSP, ...). UpdatedAt is when flowd fetched it.
type ASNType struct {
	ASN       int64     `gorm:"column:asn;primaryKey;autoIncrement:false" json:"asn"`
	Type      string    `gorm:"type:text;not null" json:"type"`
	UpdatedAt time.Time `gorm:"not null" json:"updated_at"`
}

// TableName returns the table name for ASNType.
func (ASNType) TableName() string { return "asn_types" }

// RouterRoutedPrefix is a block a static route hands to a device behind one
// of the router's interfaces, such as a POP router's /27. Discovery refills
// these; Prefix is always the masked network, as cidr requires.
type RouterRoutedPrefix struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	RouterID    uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_router_routed_prefixes_router_prefix"`
	Prefix      string    `gorm:"type:cidr;not null;uniqueIndex:idx_router_routed_prefixes_router_prefix"`
	InterfaceID uuid.UUID `gorm:"type:uuid;not null"`
	UpdatedAt   time.Time
}

func (p *RouterRoutedPrefix) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

// TableName returns the table name for RouterRoutedPrefix.
func (RouterRoutedPrefix) TableName() string { return "router_routed_prefixes" }
