package services

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// validateRegisterODP checks the plant position a registration asks for.
//
// The database refuses one half of a pairing, so the request must carry both or
// neither; saying so here names which half is missing.
func validateRegisterODP(db *gorm.DB, req models.ZTEGPONRegisterRequest) error {
	if req.ODPID == nil && req.ODPPort == nil {
		return nil
	}
	if req.ODPID == nil {
		return fmt.Errorf("%w: a port was given without an ODP", ErrValidation)
	}
	if req.ODPPort == nil {
		return fmt.Errorf("%w: an ODP was given without a port", ErrValidation)
	}
	// A fresh registration has no existing ONT of its own to exclude from the
	// occupancy check below.
	return validateODPPortPlacement(db, *req.ODPID, *req.ODPPort, uuid.Nil)
}

// validateODPPortPlacement is the one rule for whether a port on a
// distribution box is a legal place to land an ONT's drop. Registration
// (validateRegisterODP, above) and AssignONTToODP (ont_odp_assign.go) both
// write ODPID/ODPPort onto an ONT, and both call this rather than each
// carrying their own copy of the rule, so the two paths cannot drift apart.
//
// excluding is the ONT allowed to already hold the port — the subscriber being
// re-patched onto the port it already has, not a stranger. Registration has no
// such ONT yet and passes uuid.Nil, which no real row matches.
func validateODPPortPlacement(db *gorm.DB, odpID uuid.UUID, port int, excluding uuid.UUID) error {
	// The plant model moved: a drop lands in a mapping node of type odp, not in
	// the old odps table. Naming anything else is a mistake worth refusing here
	// rather than discovering at the pole.
	var node models.MappingNode
	if err := db.Where("id = ? AND type = ?", odpID, models.NodeODP).
		First(&node).Error; err != nil {
		return fmt.Errorf("%w: no distribution box with that id", ErrValidation)
	}
	if port < 1 {
		return fmt.Errorf("%w: port %d is not a valid port number", ErrValidation, port)
	}
	if node.Capacity > 0 && port > node.Capacity {
		return fmt.Errorf("%w: port %d is past the %d this box has",
			ErrValidation, port, node.Capacity)
	}

	// Named here so the operator gets a clean 400 instead of a registration or
	// assignment reaching the database and bouncing off uq_onts_odp_port as a
	// raw constraint violation. A racing pair can still both pass this check;
	// that index remains the actual arbiter under concurrency.
	var holder models.ONT
	err := db.Where("odp_id = ? AND odp_port = ? AND id <> ?", odpID, port, excluding).
		First(&holder).Error
	if err == nil {
		return fmt.Errorf("%w: port %d is taken by %s", ErrValidation, port, holder.SerialNumber)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("check port occupancy: %w", err)
	}
	return nil
}
