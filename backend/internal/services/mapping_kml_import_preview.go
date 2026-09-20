package services

import (
	"fmt"

	"github.com/tikman/olt-provisioning/internal/models"
)

// PreviewImport parses an uploaded KMZ and reports what it found - nothing is
// written to the database. Node/edge type, capacity and every other field
// are resolved as far as classifyPlacemarks safely can; this layer's own job
// is only to add what classifyPlacemarks cannot know without a database:
// whether a row's id already names something on the live map.
func (s *MappingService) PreviewImport(data []byte) (*ImportPreview, error) {
	raw, err := parseKMZForImport(data)
	if err != nil {
		return nil, err
	}
	nodes, edges, issues := classifyPlacemarks(raw)

	existingNodeIDs, existingEdgeIDs, err := s.existingMappingIDs()
	if err != nil {
		return nil, err
	}
	flagNodeConflicts(nodes, existingNodeIDs)
	flagEdgeConflicts(edges, existingEdgeIDs)

	return &ImportPreview{Nodes: nodes, Edges: edges, Issues: issues, TotalPlacemarks: len(raw)}, nil
}

// existingMappingIDs reads every node_id/edge_id already on the map, once,
// so checking a whole file's worth of rows for a collision is one query each
// rather than one query per row.
func (s *MappingService) existingMappingIDs() (map[string]bool, map[string]bool, error) {
	var nodeIDs []string
	if err := s.db.Model(&models.MappingNode{}).Pluck("node_id", &nodeIDs).Error; err != nil {
		return nil, nil, fmt.Errorf("baca node_id yang sudah ada: %w", err)
	}
	var edgeIDs []string
	if err := s.db.Model(&models.MappingEdge{}).Pluck("edge_id", &edgeIDs).Error; err != nil {
		return nil, nil, fmt.Errorf("baca edge_id yang sudah ada: %w", err)
	}
	return toSet(nodeIDs), toSet(edgeIDs), nil
}

func toSet(values []string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, v := range values {
		set[v] = true
	}
	return set
}

// flagNodeConflicts marks a NodeID that already names a row on the map, or a
// second row in this same file - either way, defaulted out of Include, since
// writing it as asked would either overwrite something nobody confirmed
// changing or collide with a row already claimed earlier in the same file.
func flagNodeConflicts(nodes []ImportedNode, existing map[string]bool) {
	seen := make(map[string]bool)
	for i := range nodes {
		n := &nodes[i]
		switch {
		case existing[n.NodeID]:
			n.Conflict = true
			n.ConflictReason = fmt.Sprintf("Kode %q sudah dipakai node lain di peta", n.NodeID)
		case seen[n.NodeID]:
			n.Conflict = true
			n.ConflictReason = fmt.Sprintf("Kode %q dipakai lebih dari sekali di berkas ini", n.NodeID)
		}
		seen[n.NodeID] = true
		if n.Conflict {
			n.Include = false
		}
	}
}

func flagEdgeConflicts(edges []ImportedEdge, existing map[string]bool) {
	seen := make(map[string]bool)
	for i := range edges {
		e := &edges[i]
		switch {
		case existing[e.EdgeID]:
			e.Conflict = true
			e.ConflictReason = fmt.Sprintf("Kode %q sudah dipakai kabel lain di peta", e.EdgeID)
		case seen[e.EdgeID]:
			e.Conflict = true
			e.ConflictReason = fmt.Sprintf("Kode %q dipakai lebih dari sekali di berkas ini", e.EdgeID)
		}
		seen[e.EdgeID] = true
		if e.Conflict {
			e.Include = false
		}
	}
}
