package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type OLTStatus string
type OLTProtocol string

// OLTModel selects which SNMP dialect the connectivity layer speaks to a
// device. C300 and C320 are distinct models even though their OIDs currently
// match, so a firmware divergence later needs no migration.
type OLTModel string

const (
	OLTStatusOnline  OLTStatus = "online"
	OLTStatusOffline OLTStatus = "offline"
	OLTStatusError   OLTStatus = "error"

	OLTProtocolSSH    OLTProtocol = "ssh"
	OLTProtocolTelnet OLTProtocol = "telnet"

	OLTModelZTEC300 OLTModel = "zte_c300"
	OLTModelZTEC320 OLTModel = "zte_c320"
	OLTModelHSGQ    OLTModel = "hsgq"
)

type OLT struct {
	ID                uuid.UUID   `gorm:"type:uuid;primaryKey"`
	SiteID            uuid.UUID   `gorm:"type:uuid;not null;index"`
	Name              string      `gorm:"type:varchar(255);not null"`
	IPAddress         string      `gorm:"type:varchar(45);not null"`
	SSHPort           int         `gorm:"default:22"`
	TelnetPort        int         `gorm:"default:23"`
	SNMPPort          int         `gorm:"default:161"`
	SNMPCommunity     string      `gorm:"type:varchar(100);default:public"`
	PreferredProtocol OLTProtocol `gorm:"type:varchar(20);default:ssh"`
	Model             OLTModel    `gorm:"type:varchar(30);not null;default:zte_c300"`
	Username          string      `gorm:"type:varchar(100);not null"`
	Password          string      `gorm:"type:varchar(255);not null"` // encrypted
	Status            OLTStatus   `gorm:"type:varchar(20);default:offline"`
	// Where the device physically sits, so the map can pin it. Nullable:
	// an OLT nobody has located yet is still a valid OLT.
	Latitude            *float64 `gorm:"type:double precision"`
	Longitude           *float64 `gorm:"type:double precision"`
	Rack                int      `gorm:"default:0"`
	Shelf               int      `gorm:"default:0"`
	Slot                int      `gorm:"default:0"`
	DiscoveryPhase      string   `gorm:"type:varchar(20);default:idle"`
	DiscoveryTotal      int      `gorm:"default:0"`
	DiscoveryRegistered int      `gorm:"default:0"`
	DiscoveryPolled     int      `gorm:"default:0"`
	DiscoveryError      string   `gorm:"type:text"`
	// VLANs caches the OLT's VLAN table from the last successful discovery poll,
	// so the provisioning form can offer a list without its own SNMP walk.
	// The column names are explicit: GORM derives "vla_ns" from VLANs, because
	// the trailing lowercase s defeats its acronym handling, and reads would
	// then silently miss the column the migration actually created.
	VLANs          datatypes.JSON `gorm:"column:vlans;type:jsonb" json:"vlans,omitempty"`
	VLANsUpdatedAt *time.Time     `gorm:"column:vlans_updated_at" json:"vlans_updated_at,omitempty"`
	// TCONTProfiles caches the CLI's T-CONT profile names, which the ZTE
	// provisioning command references as "tcont ... profile-name X".
	TCONTProfiles datatypes.JSON `gorm:"column:tcont_profiles;type:jsonb" json:"tcont_profiles,omitempty"`
	// VLANProfiles is recovered from the ONU configs, since the CLI has no
	// listing command for it. Read in the same session as the T-CONT profiles,
	// so TCONTProfilesUpdatedAt times both.
	VLANProfiles datatypes.JSON `gorm:"column:vlan_profiles;type:jsonb" json:"vlan_profiles,omitempty"`
	// ONUTypes are the names the registration command accepts, which are not the
	// model strings ONUs report over OMCI.
	ONUTypes datatypes.JSON `gorm:"column:onu_types;type:jsonb" json:"onu_types,omitempty"`
	// Cards are the line cards fitted to the OLT. A card carrying no ONU cannot
	// be inferred from ONT rows, so the inventory is kept.
	Cards datatypes.JSON `gorm:"column:cards;type:jsonb" json:"cards,omitempty"`
	// SystemInfo and Ports are the chassis summary and interface inventory read
	// over standard SNMP MIBs, so the configuration page costs no live walk.
	SystemInfo datatypes.JSON `gorm:"column:system_info;type:jsonb" json:"system_info,omitempty"`
	// CardHealth is per-slot temperature, CPU and memory, read from the ZTE
	// enterprise tables because this C300 leaves entPhySensorTable empty.
	CardHealth datatypes.JSON `gorm:"column:card_health;type:jsonb" json:"card_health,omitempty"`
	// TCONTProfileDetails carries the bandwidths behind the profile names, which
	// are only labels and can disagree with what the profile grants.
	TCONTProfileDetails datatypes.JSON `gorm:"column:tcont_profile_details;type:jsonb" json:"tcont_profile_details,omitempty"`
	// ONUTypeDetails is what the running config says about each ONU type, which
	// the plain name list cannot carry.
	ONUTypeDetails         datatypes.JSON `gorm:"column:onu_type_details;type:jsonb" json:"onu_type_details,omitempty"`
	Ports                  datatypes.JSON `gorm:"column:ports;type:jsonb" json:"ports,omitempty"`
	SystemUpdatedAt        *time.Time     `gorm:"column:system_updated_at" json:"system_updated_at,omitempty"`
	TCONTProfilesUpdatedAt *time.Time     `gorm:"column:tcont_profiles_updated_at" json:"tcont_profiles_updated_at,omitempty"`
	DiscoveryStartedAt     *time.Time     `json:"discovery_started_at,omitempty"`
	DiscoveryLastPollAt    *time.Time     `json:"discovery_last_poll_at,omitempty"`
	// DiscoveryHeartbeatAt is restamped on every progress publish, so a claim
	// held by a run that has died can be told apart from one still working.
	DiscoveryHeartbeatAt *time.Time `gorm:"column:discovery_heartbeat_at" json:"discovery_heartbeat_at,omitempty"`
	LastSeen             *time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

func (o *OLT) BeforeCreate(tx *gorm.DB) error {
	if o.ID == uuid.Nil {
		o.ID = uuid.New()
	}
	return nil
}

func (o *OLT) TableName() string {
	return "olts"
}
