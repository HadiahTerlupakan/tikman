package services

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// serverNodeIDMaxLen mirrors mapping_nodes.node_id's varchar(64).
// serverNodeIDSuffixLen reserves room for the collision suffix ('-' plus the
// first 8 characters of the OLT's own id, the same shape migration
// 55_olt_map_node.sql's backfill uses), so truncating the candidate can never
// leave the suffix itself to be cut off instead of the name.
const (
	serverNodeIDMaxLen    = 64
	serverNodeIDSuffixLen = 9
)

// syncOLTMapNode keeps mapping_nodes in step with one OLT's current
// coordinates, wherever they were just written from - the OLT menu (via
// Create/Update below) or the map itself (MappingService.UpdateNode writes
// the OLT row directly, so this function is never on that path, but the end
// state - both rows agreeing - is the same either way). It must run in the
// same transaction as the OLT write it follows, so a failure here rolls that
// write back rather than leaving the two out of step.
func syncOLTMapNode(tx *gorm.DB, olt *models.OLT) error {
	if olt.Latitude == nil || olt.Longitude == nil {
		return removeOLTMapNode(tx, olt.ID)
	}

	var existing models.MappingNode
	err := tx.Where("olt_id = ?", olt.ID).First(&existing).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return createOLTMapNode(tx, olt)
	case err != nil:
		return fmt.Errorf("find map node for OLT %s: %w", olt.ID, err)
	default:
		if err := tx.Model(&existing).Updates(map[string]any{
			"latitude": *olt.Latitude, "longitude": *olt.Longitude,
		}).Error; err != nil {
			return fmt.Errorf("move map node for OLT %s: %w", olt.ID, err)
		}
		return nil
	}
}

// createOLTMapNode places the first pin for an OLT that just got coordinates,
// either at creation or on a later edit.
func createOLTMapNode(tx *gorm.DB, olt *models.OLT) error {
	nodeID, err := serverNodeIDForOLT(tx, olt)
	if err != nil {
		return err
	}
	node := models.MappingNode{
		NodeID: nodeID, Type: models.NodeServer, Name: olt.Name,
		Latitude: *olt.Latitude, Longitude: *olt.Longitude, OLTID: &olt.ID,
	}
	if err := tx.Create(&node).Error; err != nil {
		return fmt.Errorf("create map node for OLT %s: %w", olt.ID, err)
	}
	return nil
}

// removeOLTMapNode drops the mirror once an OLT's coordinates are cleared.
// Its cables are left pointing at a node_id that no longer resolves to
// anything - migrations/53_network_mapping.sql keeps no foreign key from
// edge to node for exactly this reason (a cable is drawn before its ends are
// named), so a feeder drawn from this OLT is not silently deleted along with
// the pin; it just stops being drawn until the OLT is relocated. Silently
// dropping those cables too, here, would read as a bug the next time
// someone traced a missing feeder back to an OLT edit nobody connected to it.
func removeOLTMapNode(tx *gorm.DB, oltID uuid.UUID) error {
	if err := tx.Where("olt_id = ?", oltID).Delete(&models.MappingNode{}).Error; err != nil {
		return fmt.Errorf("remove map node for OLT %s: %w", oltID, err)
	}
	return nil
}

// serverNodeIDForOLT derives a node_id the same way migration
// 55_olt_map_node.sql's backfill does, so an OLT located through the API
// today and one already in production before this feature existed end up
// with ids of the same shape. Collisions are checked one at a time rather
// than with the migration's window function, since this runs per OLT rather
// than over a whole table at once.
func serverNodeIDForOLT(tx *gorm.DB, olt *models.OLT) (string, error) {
	candidate := truncateToRunes("SERVER-"+olt.Name, serverNodeIDMaxLen-serverNodeIDSuffixLen)

	var count int64
	if err := tx.Model(&models.MappingNode{}).Where("node_id = ?", candidate).Count(&count).Error; err != nil {
		return "", fmt.Errorf("check node id collision for OLT %s: %w", olt.ID, err)
	}
	if count == 0 {
		return candidate, nil
	}
	// olt.ID is already unique, so its own first 8 characters double as the
	// collision suffix - the same value migration 55_olt_map_node.sql's
	// left(o.id::text, 8) takes from the canonical hyphenated form.
	return candidate + "-" + olt.ID.String()[:8], nil
}

// truncateToRunes bounds s to max characters rather than bytes: Postgres's
// varchar(n) counts characters, and slicing a Go string by byte index can
// split a multi-byte UTF-8 rune in half.
func truncateToRunes(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}
