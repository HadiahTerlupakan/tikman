package api

import (
	"gorm.io/datatypes"

	"github.com/tikman/olt-provisioning/internal/models"
)

// nodeRequest is what the map sends when a box is placed or edited. Only the
// name and the position are required — that is the whole point of this map.
type nodeRequest struct {
	NodeID       string          `json:"node_id" binding:"required"`
	Type         models.NodeType `json:"type" binding:"required,oneof=server odc odp ont"`
	Name         string          `json:"name" binding:"required"`
	Latitude     *float64        `json:"latitude" binding:"required"`
	Longitude    *float64        `json:"longitude" binding:"required"`
	Capacity     int             `json:"capacity"`
	Splitter     string          `json:"splitter"`
	PPPoE        string          `json:"pppoe"`
	SerialNumber string          `json:"serial_number"`
	Notes        string          `json:"notes"`
}

func (r nodeRequest) toModel() models.MappingNode {
	return models.MappingNode{
		NodeID: r.NodeID, Type: r.Type, Name: r.Name,
		Latitude: *r.Latitude, Longitude: *r.Longitude,
		Capacity: r.Capacity, Splitter: r.Splitter,
		PPPoE: r.PPPoE, SerialNumber: r.SerialNumber, Notes: r.Notes,
	}
}

// edgeRequest is one cable. Waypoints are the path it actually takes.
type edgeRequest struct {
	EdgeID    string           `json:"edge_id" binding:"required"`
	Source    string           `json:"source" binding:"required"`
	Target    string           `json:"target" binding:"required"`
	FiberType models.FiberType `json:"fiber_type" binding:"omitempty,oneof=feeder distribution drop odp_to_odp odp_to_odp_ratio odc_to_odc odc_to_odc_ratio"`
	Distance  float64          `json:"distance"`
	Waypoints datatypes.JSON   `json:"waypoints"`
	Notes     string           `json:"notes"`
}

func (r edgeRequest) toModel() models.MappingEdge {
	return models.MappingEdge{
		EdgeID: r.EdgeID, Source: r.Source, Target: r.Target,
		FiberType: r.FiberType, Distance: r.Distance,
		Waypoints: r.Waypoints, Notes: r.Notes,
	}
}
