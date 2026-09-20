package services

import (
	"errors"

	"github.com/tikman/olt-provisioning/internal/models"
)

// ErrImportInvalid marks a KMZ import commit that cannot be written as asked:
// a row still collides after the person confirmed the preview, a required
// field is still blank, or a server node was included. It is always the
// commit refusing everything in one piece - see CommitImport - never a
// partial write, since a half-imported map is worse than one rejected
// outright: nobody could tell what landed, and retrying would collide with
// the half that did.
var ErrImportInvalid = errors.New("data impor tidak valid")

// ImportedNode is one Point placemark read back from an uploaded KMZ, already
// resolved as far as it safely can be without a person's say-so. Nothing this
// type describes is written to mapping_nodes until CommitImport is called
// with Include set - see PreviewImport.
type ImportedNode struct {
	// Row is the placemark's position in the file (1-based), for a preview
	// table key and for naming which row an error is about.
	Row          int             `json:"row"`
	NodeID       string          `json:"node_id"`
	Type         models.NodeType `json:"type"`
	Name         string          `json:"name"`
	Latitude     float64         `json:"latitude"`
	Longitude    float64         `json:"longitude"`
	Capacity     int             `json:"capacity"`
	Splitter     string          `json:"splitter"`
	PPPoE        string          `json:"pppoe"`
	SerialNumber string          `json:"serial_number"`
	Notes        string          `json:"notes"`

	// Reason is the Indonesian sentence saying where Type (and NodeID, for a
	// guess) came from - ExtendedData, a folder name or the placemark's own
	// name - so the preview never presents a guess as a fact.
	Reason string `json:"reason"`

	// Conflict marks a NodeID that already names a row on the map, or a
	// second row in this same file - resolvable by editing NodeID before
	// commit, unlike Blocked below.
	Conflict       bool   `json:"conflict"`
	ConflictReason string `json:"conflict_reason"`

	// Blocked marks a node no edit can fix: a "server" node mirrors a real
	// OLT row (migrations/55_olt_map_node.sql) and is only ever created by
	// locating that OLT, never by import. Include is meaningless on a
	// Blocked row and CommitImport refuses it outright regardless of what a
	// client sends for Include - see nodesToCreate.
	Blocked       bool   `json:"blocked"`
	BlockedReason string `json:"blocked_reason"`

	// Include is the person's decision, defaulted by PreviewImport and
	// editable before commit: false wherever Blocked, Conflict or an unknown
	// Type would otherwise write something nobody confirmed.
	Include bool `json:"include"`
}

// ImportedEdge is one LineString placemark read back from an uploaded KMZ.
// Waypoints holds only the corners traced between its ends - edgePath's own
// source-then-corners-then-target order (mapping_kml_edge.go) has already
// had its two endpoints stripped back off, the same shape mapping_edges.go
// stores.
type ImportedEdge struct {
	Row       int              `json:"row"`
	EdgeID    string           `json:"edge_id"`
	Source    string           `json:"source"`
	Target    string           `json:"target"`
	FiberType models.FiberType `json:"fiber_type"`
	Distance  float64          `json:"distance"`
	Waypoints []kmlWaypoint    `json:"waypoints"`
	Notes     string           `json:"notes"`

	Reason string `json:"reason"`

	Conflict       bool   `json:"conflict"`
	ConflictReason string `json:"conflict_reason"`

	// Unresolved marks a required field (EdgeID, Source or Target) that
	// could not be filled in at all - migrations/53_network_mapping.sql
	// tolerates a cable naming a node that does not exist, but mapping_edges'
	// NOT NULL columns still refuse one naming nothing.
	Unresolved bool `json:"unresolved"`

	Include bool `json:"include"`
}

// ImportIssue is a placemark that was neither a usable node nor a usable
// cable - an unsupported shape, or one whose own coordinates could not be
// read. Reported rather than silently dropped, so a count of what the file
// held always reconciles with what the preview shows.
type ImportIssue struct {
	Row    int    `json:"row"`
	Name   string `json:"name"`
	Folder string `json:"folder"`
	Reason string `json:"reason"`
}

// ImportPreview is what an uploaded KMZ resolves to before anyone has
// confirmed anything - see PreviewImport. Nothing behind it has touched the
// database.
type ImportPreview struct {
	Nodes           []ImportedNode `json:"nodes"`
	Edges           []ImportedEdge `json:"edges"`
	Issues          []ImportIssue  `json:"issues"`
	TotalPlacemarks int            `json:"total_placemarks"`
}

// ImportResult is what actually got written once CommitImport ran.
type ImportResult struct {
	NodesCreated int `json:"nodes_created"`
	EdgesCreated int `json:"edges_created"`
}
