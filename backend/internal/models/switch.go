package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// SwitchVendor is the make of a managed switch. The column accepts all three;
// the service only lets Huawei CE be created so far.
type SwitchVendor string

// The switch vendors.
const (
	SwitchVendorCRS       SwitchVendor = "crs"
	SwitchVendorHuaweiCE  SwitchVendor = "huawei_ce"
	SwitchVendorFiberHome SwitchVendor = "fiberhome"
)

// SwitchProtocol is how a switch's CLI is reached.
type SwitchProtocol string

// The switch access protocols.
const (
	SwitchProtocolTelnet SwitchProtocol = "telnet"
	SwitchProtocolSSH    SwitchProtocol = "ssh"
	SwitchProtocolREST   SwitchProtocol = "rest"
)

// SwitchStatus is what SNMP last said about a switch.
type SwitchStatus string

// The switch statuses.
const (
	SwitchStatusUnknown SwitchStatus = "unknown"
	SwitchStatusUp      SwitchStatus = "up"
	SwitchStatusDown    SwitchStatus = "down"
)

// SwitchPollKind is one tier of polling work for a switch.
type SwitchPollKind string

// The switch poll kinds.
const (
	SwitchPollKindInterfaces SwitchPollKind = "interfaces"
	SwitchPollKindInventory  SwitchPollKind = "inventory"
	SwitchPollKindBackup     SwitchPollKind = "backup"
)

// Switch is a managed switch read over SNMP (traffic) and its CLI (inventory,
// config backup). Nothing in phase 1 writes to it.
//
// The tags must match migrations/84_switch_module.sql: AutoMigrate runs first,
// so a disagreeing tag wins and the migration silently does nothing.
type Switch struct {
	ID     uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	SiteID *uuid.UUID `gorm:"type:uuid" json:"site_id"`
	Name   string     `gorm:"type:varchar(255);not null" json:"name"`
	Host   string     `gorm:"type:varchar(255);not null" json:"host"`

	Vendor  SwitchVendor `gorm:"type:varchar(16);not null" json:"vendor"`
	Model   *string      `gorm:"type:varchar(64)" json:"model"`
	Version *string      `gorm:"type:varchar(64)" json:"version"`

	SNMPPort      int            `gorm:"not null" json:"snmp_port"`
	SNMPCommunity string         `gorm:"type:text;not null" json:"-"`
	Protocol      SwitchProtocol `gorm:"type:varchar(8);not null" json:"protocol"`
	Port          int            `gorm:"not null" json:"port"`
	Username      string         `gorm:"type:varchar(64);not null" json:"username"`
	Password      string         `gorm:"type:text;not null" json:"-"`

	// The SSH host key pin. Telnet has no host key, so these stay empty.
	HostKeyFingerprint   string  `gorm:"type:varchar(64);not null;default:''" json:"host_key_fingerprint"`
	PresentedFingerprint *string `gorm:"type:varchar(64)" json:"presented_fingerprint"`
	HostKeyChanged       bool    `gorm:"not null;default:false" json:"host_key_changed"`
	TLSFingerprint       string  `gorm:"type:varchar(64);not null;default:''" json:"tls_fingerprint"`

	// No default on Enabled: GORM replaces a false zero value with the column
	// default on insert, which would make "add disabled" impossible.
	Enabled    bool         `gorm:"not null" json:"enabled"`
	Status     SwitchStatus `gorm:"type:varchar(16);not null" json:"status"`
	LastSeenAt *time.Time   `json:"last_seen_at"`

	// WritesEnabled is always false in phase 1; the column exists so a later
	// phase has something to gate on without a migration.
	WritesEnabled bool `gorm:"not null;default:false" json:"writes_enabled"`
	// MgmtPorts are the ports a later write must never touch, as a JSON array
	// of names. Empty until then. jsonb rather than text[] so the SQLite test
	// database can hold it.
	MgmtPorts    datatypes.JSON `gorm:"type:jsonb" json:"mgmt_ports"`
	RackPosition *int           `json:"rack_position"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// BeforeCreate assigns a UUID when none is set.
func (s *Switch) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}

// TableName returns the table name for Switch.
func (Switch) TableName() string { return "switches" }

// SwitchInterface is one port of a switch with its last reading.
type SwitchInterface struct {
	ID       uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	SwitchID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_switch_interfaces_switch_name" json:"switch_id"`
	Name     string    `gorm:"type:varchar(255);not null;uniqueIndex:idx_switch_interfaces_switch_name" json:"name"`
	Alias    string    `gorm:"type:text;not null;default:''" json:"alias"`
	// Kind is rj, sfp, qsfp or virtual.
	Kind       string `gorm:"type:varchar(8);not null" json:"kind"`
	IfIndex    *int   `json:"if_index"`
	AdminUp    bool   `gorm:"not null" json:"admin_up"`
	OperStatus string `gorm:"type:varchar(8);not null" json:"oper_status"`
	SpeedMbps  *int64 `json:"speed_mbps"`

	// Mode is access, trunk, hybrid, or empty when the port has none. The
	// column is wider than those words so a vendor's own term cannot fail an
	// insert.
	Mode         string `gorm:"type:varchar(16);not null;default:''" json:"mode"`
	PVID         *int   `gorm:"column:pvid" json:"pvid"`
	AllowedVLANs string `gorm:"column:allowed_vlans;type:text;not null;default:''" json:"allowed_vlans"`
	STPEdge      *bool  `gorm:"column:stp_edge" json:"stp_edge"`
	// STPDisabled is nil for a port that does not switch.
	STPDisabled *bool `gorm:"column:stp_disabled" json:"stp_disabled"`

	LastInBps    *float64   `json:"last_in_bps"`
	LastOutBps   *float64   `json:"last_out_bps"`
	InErrors     *int64     `json:"in_errors"`
	OutErrors    *int64     `json:"out_errors"`
	InDiscards   *int64     `json:"in_discards"`
	LastPolledAt *time.Time `json:"last_polled_at"`

	// Counter64 values stored as int64; the conversion keeps every bit.
	PrevInOctets  *int64     `json:"-"`
	PrevOutOctets *int64     `json:"-"`
	PrevSampleAt  *time.Time `json:"-"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// BeforeCreate assigns a UUID when none is set.
func (i *SwitchInterface) BeforeCreate(tx *gorm.DB) error {
	if i.ID == uuid.Nil {
		i.ID = uuid.New()
	}
	return nil
}

// TableName returns the table name for SwitchInterface.
func (SwitchInterface) TableName() string { return "switch_interfaces" }

// SwitchInterfaceMetric is one per-minute traffic sample. The table is a
// TimescaleDB hypertable with no primary key, like router_interface_metrics.
type SwitchInterfaceMetric struct {
	Time        time.Time `gorm:"not null;index:idx_switch_interface_metrics_if_time,priority:2" json:"time"`
	InterfaceID uuid.UUID `gorm:"type:uuid;not null;index:idx_switch_interface_metrics_if_time,priority:1" json:"interface_id"`
	InBps       float64   `gorm:"not null" json:"in_bps"`
	OutBps      float64   `gorm:"not null" json:"out_bps"`
}

// TableName returns the table name for SwitchInterfaceMetric.
func (SwitchInterfaceMetric) TableName() string { return "switch_interface_metrics" }

// SwitchVLAN is a VLAN defined on a switch.
type SwitchVLAN struct {
	SwitchID uuid.UUID `gorm:"type:uuid;primaryKey" json:"switch_id"`
	VID      int       `gorm:"column:vid;primaryKey;autoIncrement:false" json:"vid"`
	Name     string    `gorm:"type:varchar(255);not null;default:''" json:"name"`
}

// TableName returns the table name for SwitchVLAN.
func (SwitchVLAN) TableName() string { return "switch_vlans" }

// SwitchNeighbor is something seen behind a port: an LLDP neighbour or a MAC
// address.
type SwitchNeighbor struct {
	ID       uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	SwitchID uuid.UUID `gorm:"type:uuid;not null;index:idx_switch_neighbors_switch" json:"switch_id"`
	Port     string    `gorm:"type:varchar(255);not null" json:"port"`
	// Kind is lldp or mac.
	Kind           string `gorm:"type:varchar(8);not null" json:"kind"`
	MAC            string `gorm:"column:mac;type:varchar(17);not null;default:''" json:"mac"`
	VID            *int   `gorm:"column:vid" json:"vid"`
	RemoteName     string `gorm:"type:text;not null;default:''" json:"remote_name"`
	RemotePort     string `gorm:"type:text;not null;default:''" json:"remote_port"`
	RemotePlatform string `gorm:"type:text;not null;default:''" json:"remote_platform"`
}

// BeforeCreate assigns a UUID when none is set.
func (n *SwitchNeighbor) BeforeCreate(tx *gorm.DB) error {
	if n.ID == uuid.Nil {
		n.ID = uuid.New()
	}
	return nil
}

// TableName returns the table name for SwitchNeighbor.
func (SwitchNeighbor) TableName() string { return "switch_neighbors" }

// SwitchConfigBackup is a config snapshot. Masked is the text with secrets
// removed; RawHMAC lets a change be seen without keeping the raw text. It is
// keyed with ENCRYPTION_KEY and never served: a plain hash beside the masked
// text would let anyone who can read it test guesses at a masked secret.
type SwitchConfigBackup struct {
	ID       uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	SwitchID uuid.UUID `gorm:"type:uuid;not null;index:idx_switch_config_backups_switch_taken,priority:1" json:"switch_id"`
	TakenAt  time.Time `gorm:"not null;index:idx_switch_config_backups_switch_taken,priority:2" json:"taken_at"`
	// Reason is daily or manual; before_change and after_change come with
	// the write paths.
	Reason  string `gorm:"type:varchar(16);not null" json:"reason"`
	RawHMAC string `gorm:"column:raw_hmac;type:varchar(64);not null" json:"-"`
	Masked  string `gorm:"type:text;not null" json:"masked"`
	Size    int    `gorm:"not null" json:"size"`
}

// BeforeCreate assigns a UUID when none is set.
func (b *SwitchConfigBackup) BeforeCreate(tx *gorm.DB) error {
	if b.ID == uuid.Nil {
		b.ID = uuid.New()
	}
	return nil
}

// TableName returns the table name for SwitchConfigBackup.
func (SwitchConfigBackup) TableName() string { return "switch_config_backups" }

// SwitchPollJob is the queue row for one tier of one switch, claimed by the
// worker the way router_poll_jobs is.
type SwitchPollJob struct {
	SwitchID uuid.UUID      `gorm:"type:uuid;not null;primaryKey" json:"switch_id"`
	Kind     SwitchPollKind `gorm:"type:varchar(16);not null;primaryKey" json:"kind"`

	DueAt    time.Time  `gorm:"not null;index" json:"due_at"`
	LockedBy *string    `gorm:"type:varchar(64)" json:"locked_by"`
	LockedAt *time.Time `json:"locked_at"`

	LastRunAt           *time.Time `json:"last_run_at"`
	LastDurationMs      *int64     `json:"last_duration_ms"`
	LastError           *string    `json:"last_error"`
	ConsecutiveFailures int        `gorm:"not null;default:0" json:"consecutive_failures"`
	// ManualRequested marks a backup an operator asked for, so it is stored
	// as manual; the next release clears it.
	ManualRequested bool `gorm:"not null;default:false" json:"manual_requested"`
}

// TableName returns the table name for SwitchPollJob.
func (SwitchPollJob) TableName() string { return "switch_poll_jobs" }
