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
		// GORM's naming strategy maps the PPPoE field to pp_po_e (verified
		// against the schema AutoMigrate actually creates); "pppoe" is not a
		// column that exists.
		"pp_po_e": in.PPPoE, "serial_number": in.SerialNumber, "notes": in.Notes,
	}
	if err := s.db.Model(node).Updates(fields).Error; err != nil {
		return nil, fmt.Errorf("update node %s: %w", nodeID, err)
	}
	return s.GetNode(nodeID)
}

func (s *MappingService) DeleteNode(nodeID string) error {
	res := s.db.Where("node_id = ?", nodeID).Delete(&models.MappingNode{})
	if res.Error != nil {
		return fmt.Errorf("delete node %s: %w", nodeID, res.Error)
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("delete node %s: %w", nodeID, gorm.ErrRecordNotFound)
	}
	return nil
}
