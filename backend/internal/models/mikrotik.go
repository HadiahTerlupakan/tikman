package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// RouterStatus is what SNMP last said about a router.
type RouterStatus string

const (
	RouterStatusUnknown RouterStatus = "unknown"
	RouterStatusUp      RouterStatus = "up"
	RouterStatusDown    RouterStatus = "down"
)

// RouterAPIStatus is what the REST API last said, kept apart from RouterStatus
// because either can work while the other does not.
type RouterAPIStatus string

const (
	RouterAPIStatusUnknown RouterAPIStatus = "unknown"
	RouterAPIStatusOK      RouterAPIStatus = "ok"
	RouterAPIStatusError   RouterAPIStatus = "error"
	// RouterAPIStatusCertificateChanged stops REST polling until an admin
	// accepts the new certificate: trusting it silently would defeat the pin.
	RouterAPIStatusCertificateChanged RouterAPIStatus = "certificate_changed"
)

// BGPState is a configured BGP connection's state as TikMan reports it.
type BGPState string

const (
	BGPStateEstablished BGPState = "established"
	BGPStateDown        BGPState = "down"
)

// RouterPollKind is one tier of polling work for a router.
type RouterPollKind string

const (
	RouterPollKindInterfaces RouterPollKind = "interfaces"
	RouterPollKindBGP        RouterPollKind = "bgp"
	RouterPollKindDiscovery  RouterPollKind = "discovery"
)

// MikrotikRouter is a RouterOS v7 router read over SNMP (traffic) and REST
// (BGP, inventory, resources).
//
// The tags must match migrations/56_mikrotik_monitoring.sql: AutoMigrate runs
// first, so a disagreeing tag wins and the migration silently does nothing.
type MikrotikRouter struct {
	ID     uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	Name   string     `gorm:"type:varchar(255);not null" json:"name"`
	Host   string     `gorm:"type:varchar(255);not null" json:"host"`
	SiteID *uuid.UUID `gorm:"type:uuid" json:"site_id"`

	SNMPCommunity string `gorm:"type:text;not null" json:"-"`
	SNMPPort      int    `gorm:"not null" json:"snmp_port"`
	APIUsername   string `gorm:"type:varchar(64);not null" json:"api_username"`
	APIPassword   string `gorm:"type:text;not null" json:"-"`
	APIPort       int    `gorm:"not null" json:"api_port"`
	// No default on Enabled: GORM replaces a false zero value with the column
	// default on insert, which would make "add disabled" impossible.
	Enabled bool `gorm:"not null" json:"enabled"`

	TLSFingerprint       string  `gorm:"type:varchar(64);not null" json:"tls_fingerprint"`
	PresentedFingerprint *string `gorm:"type:varchar(64)" json:"presented_fingerprint"`

	Status    RouterStatus    `gorm:"type:varchar(16);not null" json:"status"`
	APIStatus RouterAPIStatus `gorm:"type:varchar(32);not null" json:"api_status"`
	APIError  *string         `gorm:"type:text" json:"api_error"`

	CPULoad       *int64     `json:"cpu_load"`
	MemoryTotal   *int64     `json:"memory_total"`
	MemoryFree    *int64     `json:"memory_free"`
	UptimeSeconds *int64     `json:"uptime_seconds"`
	ROSVersion    *string    `gorm:"type:varchar(64)" json:"ros_version"`
	BoardName     *string    `gorm:"type:varchar(64)" json:"board_name"`
	LastSeenAt    *time.Time `json:"last_seen_at"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (r *MikrotikRouter) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return nil
}

// TableName returns the table name for MikrotikRouter.
func (MikrotikRouter) TableName() string { return "mikrotik_routers" }

// RouterInterface is one interface of a router, VLANs included.
//
// Its identity is the name, not ifIndex: RouterOS renumbers ifIndex when
// interfaces are added or removed, and keying on it would swap two VLANs'
// graphs without anything noticing.
type RouterInterface struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	RouterID   uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_router_interfaces_router_name" json:"router_id"`
	Name       string    `gorm:"type:varchar(255);not null;uniqueIndex:idx_router_interfaces_router_name" json:"name"`
	Comment    string    `gorm:"type:text;not null" json:"comment"`
	Type       string    `gorm:"type:varchar(32);not null" json:"type"`
	VLANID     *int      `json:"vlan_id"`
	ParentName *string   `gorm:"type:varchar(255)" json:"parent_name"`
	IfIndex    *int      `json:"if_index"`
	SpeedMbps  *int64    `json:"speed_mbps"`
	Disabled   bool      `gorm:"not null" json:"disabled"`
	OperStatus string    `gorm:"type:varchar(8);not null" json:"oper_status"`
	// Comma-separated CIDRs from /ip/address. Text rather than cidr[] so the
	// SQLite test database can hold it too.
	Addresses string `gorm:"type:text;not null" json:"addresses"`

	LastInBps    *float64   `json:"last_in_bps"`
	LastOutBps   *float64   `json:"last_out_bps"`
	LastPolledAt *time.Time `json:"last_polled_at"`

	// Counter64 values stored as int64. The conversion keeps every bit, so
	// uint64(stored) gives back the exact counter even past 2^63.
	PrevInOctets  *int64     `json:"-"`
	PrevOutOctets *int64     `json:"-"`
	PrevSampleAt  *time.Time `json:"-"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (i *RouterInterface) BeforeCreate(tx *gorm.DB) error {
	if i.ID == uuid.Nil {
		i.ID = uuid.New()
	}
	return nil
}

// TableName returns the table name for RouterInterface.
func (RouterInterface) TableName() string { return "router_interfaces" }

// RouterInterfaceMetric is one per-minute traffic sample. The table is a
// TimescaleDB hypertable; it has no primary key because a hypertable's unique
// keys must include the time column and nothing reads a sample by key.
type RouterInterfaceMetric struct {
	Time        time.Time `gorm:"not null;index:idx_router_interface_metrics_if_time,priority:2" json:"time"`
	InterfaceID uuid.UUID `gorm:"type:uuid;not null;index:idx_router_interface_metrics_if_time,priority:1" json:"interface_id"`
	InBps       float64   `gorm:"not null" json:"in_bps"`
	OutBps      float64   `gorm:"not null" json:"out_bps"`
}

// TableName returns the table name for RouterInterfaceMetric.
func (RouterInterfaceMetric) TableName() string { return "router_interface_metrics" }

// RouterBGPSession is one configured BGP connection and the state of its
// running session.
type RouterBGPSession struct {
	ID            uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	RouterID      uuid.UUID  `gorm:"type:uuid;not null;uniqueIndex:idx_router_bgp_sessions_router_name" json:"router_id"`
	Name          string     `gorm:"type:varchar(255);not null;uniqueIndex:idx_router_bgp_sessions_router_name" json:"name"`
	RemoteAddress string     `gorm:"type:varchar(64);not null" json:"remote_address"`
	RemoteAS      int64      `gorm:"not null" json:"remote_as"`
	LocalAddress  string     `gorm:"type:varchar(64);not null" json:"local_address"`
	State         BGPState   `gorm:"type:varchar(16);not null" json:"state"`
	UptimeSeconds int64      `gorm:"not null" json:"uptime_seconds"`
	PrefixCount   int64      `gorm:"not null" json:"prefix_count"`
	LastStartedAt *time.Time `json:"last_started_at"`
	LastStoppedAt *time.Time `json:"last_stopped_at"`
	InterfaceID   *uuid.UUID `gorm:"type:uuid" json:"interface_id"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

func (s *RouterBGPSession) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}

// TableName returns the table name for RouterBGPSession.
func (RouterBGPSession) TableName() string { return "router_bgp_sessions" }

// RouterBGPEvent records a session changing state.
type RouterBGPEvent struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	RouterID    uuid.UUID `gorm:"type:uuid;not null;index:idx_router_bgp_events_router_time,priority:1" json:"router_id"`
	SessionName string    `gorm:"type:varchar(255);not null" json:"session_name"`
	FromState   BGPState  `gorm:"type:varchar(16);not null" json:"from_state"`
	ToState     BGPState  `gorm:"type:varchar(16);not null" json:"to_state"`
	OccurredAt  time.Time `gorm:"not null;index:idx_router_bgp_events_router_time,priority:2" json:"occurred_at"`
}

func (e *RouterBGPEvent) BeforeCreate(tx *gorm.DB) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	return nil
}

// TableName returns the table name for RouterBGPEvent.
func (RouterBGPEvent) TableName() string { return "router_bgp_events" }

// RouterPollJob is one router's schedule for one tier, and the row a worker
// claims to take that work. Same shape as OLTPollJob, keyed on the router.
type RouterPollJob struct {
	RouterID uuid.UUID      `gorm:"type:uuid;not null;primaryKey" json:"router_id"`
	Kind     RouterPollKind `gorm:"type:varchar(16);not null;primaryKey" json:"kind"`

	DueAt    time.Time  `gorm:"not null;index" json:"due_at"`
	LockedBy *string    `gorm:"type:varchar(64)" json:"locked_by"`
	LockedAt *time.Time `json:"locked_at"`

	LastRunAt           *time.Time `json:"last_run_at"`
	LastDurationMs      *int64     `json:"last_duration_ms"`
	LastError           *string    `json:"last_error"`
	ConsecutiveFailures int        `gorm:"not null;default:0" json:"consecutive_failures"`
}

// TableName returns the table name for RouterPollJob.
func (RouterPollJob) TableName() string { return "router_poll_jobs" }
