package services

import (
	"errors"
	"fmt"
	"strings"

	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// ErrNodeExists marks a node id that is already on the map.
var ErrNodeExists = errors.New("node id already exists")

// ErrNodeInUse marks a node one or more ONTs still hold their drop on. See
// DeleteNode for why this is refused rather than left to dangle the way an
// edge is allowed to.
var ErrNodeInUse = errors.New("node masih dipakai")

// MappingService holds the network map: the boxes and the cables between them.
type MappingService struct {
	db *gorm.DB
}

func NewMappingService(db *gorm.DB) *MappingService {
	return &MappingService{db: db}
}

// CreateNode places one box on the map. Only a name and a position are
// required; the rest is filled in later, from a desk rather than a pole.
func (s *MappingService) CreateNode(in models.MappingNode) (*models.MappingNode, error) {
	if err := s.db.Create(&in).Error; err != nil {
		if strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "duplicate key") {
			return nil, fmt.Errorf("%w: %s", ErrNodeExists, in.NodeID)
		}
		return nil, fmt.Errorf("create node: %w", err)
	}
	return &in, nil
}

func (s *MappingService) ListNodes() ([]models.MappingNode, error) {
	var nodes []models.MappingNode
	if err := s.db.Order("type, name").Find(&nodes).Error; err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}
	return nodes, nil
}

func (s *MappingService) GetNode(nodeID string) (*models.MappingNode, error) {
	var node models.MappingNode
	if err := s.db.Where("node_id = ?", nodeID).First(&node).Error; err != nil {
		return nil, fmt.Errorf("get node %s: %w", nodeID, err)
	}
	return &node, nil
}

// UpdateNode changes what a box says about itself. Its node_id is its identity
// and the cables point at it, so it is never part of an update.
func (s *MappingService) UpdateNode(nodeID string, in models.MappingNode) (*models.MappingNode, error) {
	node, err := s.GetNode(nodeID)
	if err != nil {
		return nil, err
	}
	fields := map[string]any{
		"type": in.Type, "name": in.Name,
		"latitude": in.Latitude, "longitude": in.Longitude,
		"capacity": in.Capacity, "splitter": in.Splitter,
		"pppoe": in.PPPoE, "serial_number": in.SerialNumber, "notes": in.Notes,
	}
	if err := s.db.Model(node).Updates(fields).Error; err != nil {
		return nil, fmt.Errorf("update node %s: %w", nodeID, err)
	}
	return s.GetNode(nodeID)
}

// DeleteNode removes one box from the map. Refused while an ONT still names it
// as its ODP — deliberately asymmetric with edges, where a dangling pointer is
// tolerated by design (migrations/53_network_mapping.sql: a cable is drawn
// before its ends are named). An ONT is a real subscriber's service record,
// not a sketch; deleting the node under it would leave odp_id pointing at
// nothing while the assignment still held its slot in uq_onts_odp_port.
func (s *MappingService) DeleteNode(nodeID string) error {
	node, err := s.GetNode(nodeID)
	if err != nil {
		return err
	}
	var inUse int64
	if err := s.db.Model(&models.ONT{}).Where("odp_id = ?", node.ID).Count(&inUse).Error; err != nil {
		return fmt.Errorf("count ONTs on node %s: %w", nodeID, err)
	}
	if inUse > 0 {
		return fmt.Errorf("%w: %d ONT masih terhubung ke %s", ErrNodeInUse, inUse, nodeID)
	}

	res := s.db.Where("node_id = ?", nodeID).Delete(&models.MappingNode{})
	if res.Error != nil {
		return fmt.Errorf("delete node %s: %w", nodeID, res.Error)
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("delete node %s: %w", nodeID, gorm.ErrRecordNotFound)
	}
	return nil
}
