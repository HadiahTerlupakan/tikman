package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// NodeType is what a point on the map is.
type NodeType string

const (
	NodeServer NodeType = "server"
	NodeODC    NodeType = "odc"
	NodeODP    NodeType = "odp"
	NodeONT    NodeType = "ont"
)

// FiberType is what a cable between two nodes carries. The cascade kinds exist
// because a distribution box is often fed from another one rather than from the
// cabinet, which the plant model before this could not express at all.
type FiberType string

const (
	FiberFeeder        FiberType = "feeder"
	FiberDistribution  FiberType = "distribution"
	FiberDrop          FiberType = "drop"
	FiberODPToODP      FiberType = "odp_to_odp"
	FiberODPToODPRatio FiberType = "odp_to_odp_ratio"
	FiberODCToODC      FiberType = "odc_to_odc"
	FiberODCToODCRatio FiberType = "odc_to_odc_ratio"
)

// MappingNode is one box, cabinet or customer unit on the map. Only a name and
// a position are required: a technician standing at a pole should be able to
// record it in two taps and fill the rest in later.
type MappingNode struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	NodeID    string    `gorm:"type:varchar(64);not null;uniqueIndex" json:"node_id"`
	Type      NodeType  `gorm:"type:varchar(16);not null" json:"type"`
	Name      string    `gorm:"type:varchar(120);not null" json:"name"`
	Latitude  float64   `gorm:"not null" json:"latitude"`
	Longitude float64   `gorm:"not null" json:"longitude"`
	Capacity  int       `json:"capacity"`
	Splitter  string    `gorm:"type:varchar(16)" json:"splitter"`
	// Explicit column name: GORM's naming strategy would otherwise derive
	// pp_po_e from this field, which nobody writing SQL against this table
	// later would guess.
	PPPoE        string    `gorm:"type:varchar(64);column:pppoe" json:"pppoe"`
	SerialNumber string    `gorm:"type:varchar(64)" json:"serial_number"`
	Notes        string    `gorm:"type:text" json:"notes"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (n *MappingNode) BeforeCreate(*gorm.DB) error {
	if n.ID == uuid.Nil {
		n.ID = uuid.New()
	}
	return nil
}

func (MappingNode) TableName() string { return "mapping_nodes" }

// MappingEdge is one cable. Waypoints hold the path it actually takes, so the
// length shown is the cable that was pulled rather than the straight line.
type MappingEdge struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	EdgeID    string         `gorm:"type:varchar(64);not null;uniqueIndex" json:"edge_id"`
	Source    string         `gorm:"type:varchar(64);not null" json:"source"`
	Target    string         `gorm:"type:varchar(64);not null" json:"target"`
	FiberType FiberType      `gorm:"type:varchar(24)" json:"fiber_type"`
	Distance  float64        `json:"distance"`
	Waypoints datatypes.JSON `gorm:"type:jsonb" json:"waypoints"`
	Notes     string         `gorm:"type:text" json:"notes"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

func (e *MappingEdge) BeforeCreate(*gorm.DB) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	return nil
}

func (MappingEdge) TableName() string { return "mapping_edges" }
