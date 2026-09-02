package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ODC is an optical distribution cabinet: the first splitting stage between an
// OLT and the subscribers. Its feeds live in ODCFeed rather than here, because
// a cabinet can be fed by more than one PON port.
type ODC struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	SiteID    uuid.UUID `gorm:"type:uuid;not null;index" json:"site_id"`
	Name      string    `gorm:"type:varchar(255);not null" json:"name"`
	Code      string    `gorm:"type:varchar(64)" json:"code"`
	Latitude  *float64  `gorm:"type:double precision" json:"latitude,omitempty"`
	Longitude *float64  `gorm:"type:double precision" json:"longitude,omitempty"`
	Address   string    `gorm:"type:text" json:"address"`
	Notes     string    `gorm:"type:text" json:"notes"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (o *ODC) BeforeCreate(tx *gorm.DB) error {
	if o.ID == uuid.Nil {
		o.ID = uuid.New()
	}
	return nil
}

func (ODC) TableName() string {
	return "odcs"
}

// ODCFeed is one PON port supplying one cabinet, with the splitter it lands on.
//
// A PON port can supply at most one cabinet, which the unique index states: the
// light from a port is split once at this stage, and a second cabinet claiming
// the same port would describe a network that cannot exist.
type ODCFeed struct {
	ID     uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	ODCID  uuid.UUID `gorm:"type:uuid;not null;index" json:"odc_id"`
	OLTID  uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:uq_odc_feeds_pon,priority:1" json:"olt_id"`
	Slot   int       `gorm:"not null;uniqueIndex:uq_odc_feeds_pon,priority:2" json:"slot"`
	PortID int       `gorm:"not null;uniqueIndex:uq_odc_feeds_pon,priority:3" json:"port_id"`
	// SplitterOutputs is the N of a 1:N splitter on this feed.
	SplitterOutputs int       `gorm:"not null" json:"splitter_outputs"`
	Notes           string    `gorm:"type:text" json:"notes"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (f *ODCFeed) BeforeCreate(tx *gorm.DB) error {
	if f.ID == uuid.Nil {
		f.ID = uuid.New()
	}
	return nil
}

func (ODCFeed) TableName() string {
	return "odc_feeds"
}

// ODP is an optical distribution point: the box a subscriber's drop cable
// terminates in. It holds exactly one splitter, so PortCount is both the
// splitter's output count and the number of subscribers it can carry — the
// ratio an operator says out loud, 1:8, is 1:PortCount.
//
// Its parent is either a cabinet or a PON port directly, never both and never
// neither. Networks grow both ways, and a model that allowed only one of them
// would be wrong for half of this one.
type ODP struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	Name      string    `gorm:"type:varchar(255);not null" json:"name"`
	Code      string    `gorm:"type:varchar(64)" json:"code"`
	PortCount int       `gorm:"not null" json:"port_count"`
	Latitude  *float64  `gorm:"type:double precision" json:"latitude,omitempty"`
	Longitude *float64  `gorm:"type:double precision" json:"longitude,omitempty"`
	Address   string    `gorm:"type:text" json:"address"`
	Notes     string    `gorm:"type:text" json:"notes"`

	// Exactly one parent: ODCID, or the OLTID/Slot/PortID triple.
	ODCID  *uuid.UUID `gorm:"type:uuid;index" json:"odc_id,omitempty"`
	OLTID  *uuid.UUID `gorm:"type:uuid;index" json:"olt_id,omitempty"`
	Slot   *int       `json:"slot,omitempty"`
	PortID *int       `json:"port_id,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (o *ODP) BeforeCreate(tx *gorm.DB) error {
	if o.ID == uuid.Nil {
		o.ID = uuid.New()
	}
	return nil
}

func (ODP) TableName() string {
	return "odps"
}

// HasODCParent reports whether this ODP hangs off a cabinet.
func (o *ODP) HasODCParent() bool {
	return o.ODCID != nil
}

// HasPONParent reports whether this ODP hangs off a PON port directly, which
// needs all three coordinates of that port.
func (o *ODP) HasPONParent() bool {
	return o.OLTID != nil && o.Slot != nil && o.PortID != nil
}
