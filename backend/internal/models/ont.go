package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// ONTStatus represents the status of an ONT
type ONTStatus string

const (
	ONTStatusOnline   ONTStatus = "online"     // working - ONU registered and passing traffic
	ONTStatusOffline  ONTStatus = "offline"    // powered off or cable disconnected
	ONTStatusLOS      ONTStatus = "los"        // Loss of Signal
	ONTStatusDyingGas ONTStatus = "dying_gasp" // dying gasp state - just lost power
	ONTStatusUnknown  ONTStatus = "unknown"    // unrecognized state value
)

// ONT represents an Optical Network Terminal for monitoring
type ONT struct {
	ID     uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	OLTID  uuid.UUID `gorm:"type:uuid;not null;index" json:"olt_id"`
	PortID int       `gorm:"not null;uniqueIndex:uq_onts_olt_slot_port_ont,priority:3" json:"port_id"`
	ONTID  int       `gorm:"not null;uniqueIndex:uq_onts_olt_slot_port_ont,priority:4" json:"ont_id"`
	Slot   *int      `gorm:"uniqueIndex:uq_onts_olt_slot_port_ont,priority:2" json:"slot,omitempty"`
	// ServiceConfig is the ONU's provisioned service read back from the OLT, so
	// the configure form opens showing what is actually running. It carries no
	// PPPoE password: the operator retypes that.
	ServiceConfig   datatypes.JSON `gorm:"column:service_config;type:jsonb" json:"service_config,omitempty"`
	ServiceConfigAt *time.Time     `gorm:"column:service_config_at" json:"service_config_at,omitempty"`
	// Encrypted at rest and never serialised with the ONT: it reaches a browser
	// only through the service-config endpoint, for the operator about to
	// reconfigure this subscriber.
	PPPoEPassword        string     `gorm:"column:pppoe_password;type:text" json:"-"`
	SerialNumber         string     `gorm:"type:varchar(20);not null;uniqueIndex" json:"serial_number"`
	Name                 string     `gorm:"type:varchar(255)" json:"name"`
	Description          string     `gorm:"type:varchar(255)" json:"description"`
	Status               ONTStatus  `gorm:"type:varchar(20);index" json:"status"`
	DeviceType           string     `gorm:"type:varchar(100)" json:"device_type,omitempty"`
	HardwareVersion      string     `gorm:"type:varchar(50)" json:"hardware_version,omitempty"`
	SoftwareVersion      string     `gorm:"type:varchar(50)" json:"software_version,omitempty"`
	IPAddress            string     `gorm:"type:varchar(45)" json:"ip_address,omitempty"`
	MACAddress           string     `gorm:"type:varchar(17)" json:"mac_address,omitempty"`
	RxPower              *float64   `gorm:"type:decimal(10,2)" json:"rx_power"`                     // Receiving optical power in dBm (nullable: null means no signal)
	TxPower              *float64   `gorm:"type:decimal(10,2)" json:"tx_power"`                     // Transmitting optical power in dBm (nullable: null means no signal)
	Distance             int        `gorm:"default:0" json:"distance"`                              // GPON optical distance in meters
	LastOnline           *time.Time `json:"last_online,omitempty"`                                  // Timestamp of last online status
	LastOffline          *time.Time `json:"last_offline,omitempty"`                                 // Timestamp of last offline status
	LastOfflineReason    string     `gorm:"type:varchar(255)" json:"last_offline_reason,omitempty"` // Reason for last offline event
	Uptime               int64      `gorm:"default:0" json:"uptime"`                                // Uptime in seconds
	LastDownTimeDuration int64      `gorm:"default:0" json:"last_down_time_duration"`               // Last downtime duration in seconds
	LastSeenAt           *time.Time `json:"last_seen_at"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

// TableName specifies the table name for GORM
func (ONT) TableName() string {
	return "onts"
}

// BeforeCreate hook to generate UUID
func (o *ONT) BeforeCreate(tx *gorm.DB) error {
	if o.ID == uuid.Nil {
		o.ID = uuid.New()
	}
	return nil
}
