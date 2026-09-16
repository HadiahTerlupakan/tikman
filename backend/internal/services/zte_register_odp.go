package services

import (
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
	return validateODPPortPlacement(db, *req.ODPID, *req.ODPPort)
}

// validateODPPortPlacement is the one rule for whether a port on a
// distribution box is a legal place to land an ONT's drop. Registration
// (validateRegisterODP, above) and AssignONTToODP (ont_odp_assign.go) both
// write ODPID/ODPPort onto an ONT, and both call this rather than each
// carrying their own copy of the rule, so the two paths cannot drift apart.
func validateODPPortPlacement(db *gorm.DB, odpID uuid.UUID, port int) error {
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
	return nil
}
