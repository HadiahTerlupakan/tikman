package services

import (
	"errors"
	"fmt"
	"strings"

	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

var (
	// ErrEdgeExists marks a cable id that is already drawn.
	ErrEdgeExists = errors.New("edge id already exists")
	// ErrSlotsFull marks a box asked to hold more than it has ports for. Its
	// text is Indonesian, unlike validateODPPortPlacement's — this one is not
	// shared with the ZTE registration path and reaches the operator directly
	// through the network map's conflict alert.
	ErrSlotsFull = errors.New("slot penuh")
)

// CreateEdge draws one cable. A cabinet or box with a stated capacity refuses
// the connection that would overfill it, counted per kind of thing hanging off
// it — a cascade to another box does not eat a customer's slot.
func (s *MappingService) CreateEdge(in models.MappingEdge) (*models.MappingEdge, error) {
	if err := s.checkSlots(in, "", false); err != nil {
		return nil, err
	}
	if err := s.db.Create(&in).Error; err != nil {
		if strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "duplicate key") {
			return nil, fmt.Errorf("%w: %s", ErrEdgeExists, in.EdgeID)
		}
		return nil, fmt.Errorf("create edge: %w", err)
	}
	return &in, nil
}

// checkSlots counts what already hangs off the source, of the same kind as what
// is being added. A capacity of zero means nobody has counted the ports yet.
// excludeID leaves one stored edge out of the count: on an update, that row is
// the edge being edited, which already occupies one of the slots it would
// otherwise be checked against — without this, a notes-only edit of a cable
// on an already-full box would refuse itself forever.
//
// tolerateMissing treats a source or target that resolves to no node at all
// as nothing to check, rather than an error. CreateEdge/UpdateEdge pass
// false: the drawing UI only ever cables two boxes already on the map, so an
// unresolvable endpoint there is a real bug. CommitImport passes true — an
// import may legitimately bring a cable without its ends
// (migrations/53_network_mapping.sql; see also removeOLTMapNode), and
// refusing the whole commit over a dangling reference this schema already
// tolerates would be import inventing a stricter rule than the map itself
// has.
func (s *MappingService) checkSlots(in models.MappingEdge, excludeID string, tolerateMissing bool) error {
	source, err := s.GetNode(in.Source)
	if err != nil {
		if tolerateMissing && errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return fmt.Errorf("source %s: %w", in.Source, err)
	}
	if source.Capacity <= 0 {
		// Skips the target lookup below too, so a cable off an unlimited box is
		// never checked for a dangling target. Tolerated by design, not an
		// oversight: migrations/53_network_mapping.sql keeps no foreign key
		// from edge to node, because a cable is drawn before its ends are
		// named, and that same migration backfills every legacy ODC with
		// capacity 0 — so in production this lenient path is the normal one.
		return nil
	}
	target, err := s.GetNode(in.Target)
	if err != nil {
		if tolerateMissing && errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return fmt.Errorf("target %s: %w", in.Target, err)
	}
	kind, counts := slotKind(source.Type, target.Type, in.FiberType)
	if !counts {
		return nil
	}

	var used int64
	q := s.db.Model(&models.MappingEdge{}).
		Where("source = ?", in.Source).
		Where("target IN (SELECT node_id FROM mapping_nodes WHERE type = ?)", kind.target)
	if kind.fiber != "" {
		q = q.Where("fiber_type = ?", kind.fiber)
	}
	if excludeID != "" {
		q = q.Where("edge_id <> ?", excludeID)
	}
	if err := q.Count(&used).Error; err != nil {
		return fmt.Errorf("count slots on %s: %w", in.Source, err)
	}
	if used >= int64(source.Capacity) {
		return fmt.Errorf("%w: %q sudah penuh (%d/%d)", ErrSlotsFull, in.Source, used, source.Capacity)
	}
	return nil
}

type slotCount struct {
	target models.NodeType
	fiber  models.FiberType
}

// slotKind says which slots this cable consumes, or that it consumes none.
func slotKind(source, target models.NodeType, fiber models.FiberType) (slotCount, bool) {
	switch {
	case source == models.NodeODC && target == models.NodeODP:
		return slotCount{target: models.NodeODP}, true
	case source == models.NodeODC && target == models.NodeODC && fiber == models.FiberODCToODC:
		return slotCount{target: models.NodeODC, fiber: models.FiberODCToODC}, true
	case source == models.NodeODP && target == models.NodeONT:
		return slotCount{target: models.NodeONT}, true
	case source == models.NodeODP && target == models.NodeODP && fiber == models.FiberODPToODP:
		return slotCount{target: models.NodeODP, fiber: models.FiberODPToODP}, true
	}
	return slotCount{}, false
}

func (s *MappingService) ListEdges() ([]models.MappingEdge, error) {
	var edges []models.MappingEdge
	if err := s.db.Order("edge_id").Find(&edges).Error; err != nil {
		return nil, fmt.Errorf("list edges: %w", err)
	}
	return edges, nil
}

// UpdateEdge can repoint a cable at a different box, so it must pass through
// the same capacity check as CreateEdge before it writes anything — built from
// the incoming source, target and fiber, since it is what the edge would
// become that has to fit, not what it already is. The edge's own stored row
// is excluded from that count so a notes-only edit of a cable that already
// hangs off a full box is not refused by its own slot.
func (s *MappingService) UpdateEdge(edgeID string, in models.MappingEdge) (*models.MappingEdge, error) {
	var edge models.MappingEdge
	if err := s.db.Where("edge_id = ?", edgeID).First(&edge).Error; err != nil {
		return nil, fmt.Errorf("get edge %s: %w", edgeID, err)
	}
	candidate := models.MappingEdge{Source: in.Source, Target: in.Target, FiberType: in.FiberType}
	if err := s.checkSlots(candidate, edgeID, false); err != nil {
		return nil, err
	}
	fields := map[string]any{
		"source": in.Source, "target": in.Target, "fiber_type": in.FiberType,
		"distance": in.Distance, "waypoints": in.Waypoints, "notes": in.Notes,
	}
	if err := s.db.Model(&edge).Updates(fields).Error; err != nil {
		return nil, fmt.Errorf("update edge %s: %w", edgeID, err)
	}
	var updated models.MappingEdge
	if err := s.db.Where("edge_id = ?", edgeID).First(&updated).Error; err != nil {
		return nil, fmt.Errorf("get edge %s: %w", edgeID, err)
	}
	return &updated, nil
}

func (s *MappingService) DeleteEdge(edgeID string) error {
	res := s.db.Where("edge_id = ?", edgeID).Delete(&models.MappingEdge{})
	if res.Error != nil {
		return fmt.Errorf("delete edge %s: %w", edgeID, res.Error)
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("delete edge %s: %w", edgeID, gorm.ErrRecordNotFound)
	}
	return nil
}
