package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// ODC is an optical distribution cabinet: the first splitting stage between an
// OLT and the subscribers. Its feeds live in ODCFeed rather than here, because
// a cabinet can be fed by more than one PON port.
type ODC struct {
	ID     uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	SiteID uuid.UUID `gorm:"type:uuid;not null;index" json:"site_id"`
	// Code is the cabinet's identity. There is no separate name: operators call
	// these by their code, and a second column holding the same words would
	// only drift from it.
	Code      string    `gorm:"type:varchar(64);not null" json:"code"`
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
	SplitterOutputs int    `gorm:"not null" json:"splitter_outputs"`
	Notes           string `gorm:"type:text" json:"notes"`
	// Route is the path the feeder cable takes, as vertices someone traced on
	// the map. Empty means nobody has traced it, and the map draws the straight
	// line between the ends instead — which follows the plant if it ever moves,
	// where a stored pair of endpoints would freeze where it used to be.
	Route datatypes.JSON `gorm:"type:jsonb" json:"route,omitempty"`
	// RouteMeters is that path's length, kept beside it so a list can add
	// lengths up without parsing every path.
	RouteMeters float64 `gorm:"not null;default:0" json:"route_meters"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
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
	ID uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	// Code is the box's identity, for the same reason a cabinet's is.
	Code      string   `gorm:"type:varchar(64);not null" json:"code"`
	PortCount int      `gorm:"not null" json:"port_count"`
	Latitude  *float64 `gorm:"type:double precision" json:"latitude,omitempty"`
	Longitude *float64 `gorm:"type:double precision" json:"longitude,omitempty"`
	Address   string   `gorm:"type:text" json:"address"`
	Notes     string   `gorm:"type:text" json:"notes"`

	// Route is the path the feeder cable takes, as vertices someone traced on
	// the map. Empty means nobody has traced it, and the map draws the straight
	// line between the ends instead — which follows the plant if it ever moves,
	// where a stored pair of endpoints would freeze where it used to be.
	Route datatypes.JSON `gorm:"type:jsonb" json:"route,omitempty"`
	// RouteMeters is that path's length, kept beside it so a list can add
	// lengths up without parsing every path.
	RouteMeters float64 `gorm:"not null;default:0" json:"route_meters"`

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

// RoutePoint is one vertex of a cable's path, as it is stored and drawn.
type RoutePoint struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

// RoutePath decodes a traced cable path. An unset column reads as no path,
// which is what a cable nobody has traced yet has.
func routePath(raw datatypes.JSON) ([]RoutePoint, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	points := []RoutePoint{}
	if err := json.Unmarshal(raw, &points); err != nil {
		return nil, err
	}
	return points, nil
}

// RoutePath is the feeder cable's traced path.
func (f *ODCFeed) RoutePath() ([]RoutePoint, error) {
	return routePath(f.Route)
}

// RoutePath is the distribution cable's traced path.
func (o *ODP) RoutePath() ([]RoutePoint, error) {
	return routePath(o.Route)
}
