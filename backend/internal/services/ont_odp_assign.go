package services

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// AssignONTToODP records which distribution box and port an ONT's drop cable
// lands in. It shares validateODPPortPlacement with fresh registration
// (zte_register_odp.go), so the same box/port rule applies whether the
// pairing is set at register time or afterward from the ODP tab.
//
// A racing pair of assignments can both pass validateODPPortPlacement; the
// write below leans on the uq_onts_odp_port unique index to be the actual
// arbiter, rather than adding a read-then-write check that would only narrow
// the race window without closing it.
func (s *ONTService) AssignONTToODP(ontID, odpNodeID uuid.UUID, port int) error {
	if err := validateODPPortPlacement(s.db, odpNodeID, port); err != nil {
		return err
	}

	updates := map[string]interface{}{
		"odp_id": odpNodeID, "odp_port": port, "updated_at": time.Now(),
	}
	result := s.db.Model(&models.ONT{}).Where("id = ?", ontID).Updates(updates)
	if result.Error != nil {
		if strings.Contains(result.Error.Error(), "UNIQUE") || strings.Contains(result.Error.Error(), "duplicate key") {
			return fmt.Errorf("%w: port %d sudah dipakai ONT lain", ErrValidation, port)
		}
		return fmt.Errorf("assign ONT to ODP: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("assign ONT to ODP: %w", gorm.ErrRecordNotFound)
	}
	return nil
}

// UnassignONTFromODP clears where an ONT's drop lands, freeing its port for
// another subscriber.
func (s *ONTService) UnassignONTFromODP(ontID uuid.UUID) error {
	updates := map[string]interface{}{
		"odp_id": nil, "odp_port": nil, "updated_at": time.Now(),
	}
	result := s.db.Model(&models.ONT{}).Where("id = ?", ontID).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("unassign ONT from ODP: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("unassign ONT from ODP: %w", gorm.ErrRecordNotFound)
	}
	return nil
}
