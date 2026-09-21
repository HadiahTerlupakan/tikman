package services

import (
	"encoding/json"
	"fmt"

	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// CommitImport writes exactly the rows marked Include, in one transaction:
// every row lands, or none does. A half-imported file is worse than one
// rejected outright — nobody could tell what landed, and re-importing would
// then collide with the half that did.
//
// Every check here is re-derived from the database as it stands right now,
// never trusted from the caller's own Conflict/Blocked fields: those were
// true at the moment PreviewImport ran, which may have been minutes ago, and
// a client is untrusted input regardless. Nodes are created before edges in
// the same transaction so a cable importing alongside the node it names
// still passes the capacity check below.
func (s *MappingService) CommitImport(nodes []ImportedNode, edges []ImportedEdge) (*ImportResult, error) {
	existingNodeIDs, existingEdgeIDs, err := s.existingMappingIDs()
	if err != nil {
		return nil, err
	}
	toCreateNodes, err := nodesToCreate(nodes, existingNodeIDs)
	if err != nil {
		return nil, err
	}
	toCreateEdges, err := edgesToCreate(edges, existingEdgeIDs)
	if err != nil {
		return nil, err
	}

	result := &ImportResult{}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		for i := range toCreateNodes {
			if err := tx.Create(&toCreateNodes[i]).Error; err != nil {
				return fmt.Errorf("simpan node %s: %w", toCreateNodes[i].NodeID, err)
			}
			result.NodesCreated++
		}
		txMapping := &MappingService{db: tx}
		for i := range toCreateEdges {
			if err := txMapping.checkSlots(toCreateEdges[i], "", true); err != nil {
				return err
			}
			if err := tx.Create(&toCreateEdges[i]).Error; err != nil {
				return fmt.Errorf("simpan kabel %s: %w", toCreateEdges[i].EdgeID, err)
			}
			result.EdgesCreated++
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// nodesToCreate is the commit's final say on every node row: Include decides
// whether it is considered at all, and everything after that is a rule the
// caller cannot override by setting a flag, however the row arrived.
func nodesToCreate(nodes []ImportedNode, existing map[string]bool) ([]models.MappingNode, error) {
	var out []models.MappingNode
	seen := make(map[string]bool)
	for _, n := range nodes {
		if !n.Include {
			continue
		}
		// Checked on Type itself, not the caller's Blocked flag: a tampered
		// or out-of-date request must not be able to sneak a fabricated
		// server node in by clearing a flag it does not otherwise control.
		if n.Type == models.NodeServer {
			return nil, fmt.Errorf("%w: node %q adalah node server dan tidak bisa dibuat lewat impor", ErrImportInvalid, n.NodeID)
		}
		if !isValidNodeType(n.Type) {
			return nil, fmt.Errorf("%w: node %q tidak punya jenis yang valid", ErrImportInvalid, n.NodeID)
		}
		if n.NodeID == "" {
			return nil, fmt.Errorf("%w: ada node tanpa kode", ErrImportInvalid)
		}
		// Every other field here is re-derived or re-checked against the
		// database regardless of what the caller claims (Type, NodeID,
		// collisions, Blocked above) - Latitude/Longitude must not be the one
		// exception. validateCoordinates (site_service.go) is the same check
		// ImportNodesTable's own Lintang/Bujur columns exist so a person can
		// see and fix before commit; nothing stops a client from sending
		// something the preview never showed.
		if err := validateCoordinates(&n.Latitude, &n.Longitude); err != nil {
			return nil, fmt.Errorf("%w: node %q: %v", ErrImportInvalid, n.NodeID, err)
		}
		if existing[n.NodeID] || seen[n.NodeID] {
			return nil, fmt.Errorf("%w: kode node %q masih bentrok", ErrImportInvalid, n.NodeID)
		}
		seen[n.NodeID] = true
		out = append(out, models.MappingNode{
			NodeID: n.NodeID, Type: n.Type, Name: firstNonEmpty(n.Name, n.NodeID),
			Latitude: n.Latitude, Longitude: n.Longitude, Capacity: n.Capacity,
			Splitter: n.Splitter, PPPoE: n.PPPoE, SerialNumber: n.SerialNumber, Notes: n.Notes,
		})
	}
	return out, nil
}

func edgesToCreate(edges []ImportedEdge, existing map[string]bool) ([]models.MappingEdge, error) {
	var out []models.MappingEdge
	seen := make(map[string]bool)
	for _, e := range edges {
		if !e.Include {
			continue
		}
		if e.EdgeID == "" || e.Source == "" || e.Target == "" {
			return nil, fmt.Errorf("%w: ada kabel tanpa kode, sumber, atau tujuan", ErrImportInvalid)
		}
		if !isValidFiberType(e.FiberType) {
			return nil, fmt.Errorf("%w: kabel %q punya jenis serat yang tidak valid", ErrImportInvalid, e.EdgeID)
		}
		// maxLineStringPoints (mapping_kml_import_fields.go) only ever
		// bounded parseLineString - the KMZ parse path. CommitImport's own
		// JSON body carries Waypoints directly, never routed back through
		// that parser, so a request built by hand (or a tampered client)
		// could otherwise put an unbounded mapping_edges.waypoints value
		// straight into the row ListEdges serves to every signed-in user.
		if len(e.Waypoints) > maxLineStringPoints {
			return nil, fmt.Errorf("%w: kabel %q punya terlalu banyak titik jalur (lebih dari %d)", ErrImportInvalid, e.EdgeID, maxLineStringPoints)
		}
		if existing[e.EdgeID] || seen[e.EdgeID] {
			return nil, fmt.Errorf("%w: kode kabel %q masih bentrok", ErrImportInvalid, e.EdgeID)
		}
		seen[e.EdgeID] = true
		waypoints, err := waypointsJSON(e.Waypoints)
		if err != nil {
			return nil, err
		}
		out = append(out, models.MappingEdge{
			EdgeID: e.EdgeID, Source: e.Source, Target: e.Target, FiberType: e.FiberType,
			Distance: e.Distance, Waypoints: waypoints, Notes: e.Notes,
		})
	}
	return out, nil
}

func isValidFiberType(f models.FiberType) bool {
	switch f {
	case "", models.FiberFeeder, models.FiberDistribution, models.FiberDrop,
		models.FiberODPToODP, models.FiberODPToODPRatio, models.FiberODCToODC, models.FiberODCToODCRatio:
		return true
	}
	return false
}

func waypointsJSON(points []kmlWaypoint) (datatypes.JSON, error) {
	if len(points) == 0 {
		return nil, nil
	}
	b, err := json.Marshal(points)
	if err != nil {
		return nil, fmt.Errorf("encode waypoints: %w", err)
	}
	return datatypes.JSON(b), nil
}
