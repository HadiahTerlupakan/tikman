package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Reserving an ONU id on a PON port, and the failure that means someone else
// took it first.

func (s *ZTEGPONRegisterService) loadOLT(id uuid.UUID) (*models.OLT, error) {
	var olt models.OLT
	if err := s.db.First(&olt, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("load OLT: %w", err)
	}
	return &olt, nil
}

func resolveZTEONUIDLocked(ctx context.Context, db *gorm.DB, oltID uuid.UUID, slotID, portID, requested int) (int, error) {
	if db.Name() == "postgres" {
		if err := db.WithContext(ctx).Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", fmt.Sprintf("zte-onu:%s:%d:%d", oltID, slotID, portID)).Error; err != nil {
			return 0, fmt.Errorf("lock ONU position: %w", err)
		}
	}
	var onts []models.ONT
	if err := db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("olt_id = ? AND slot = ? AND port_id = ?", oltID, slotID, portID).Find(&onts).Error; err != nil {
		return 0, fmt.Errorf("resolve ONU ID: %w", err)
	}
	used := make(map[int]struct{}, len(onts))
	for _, ont := range onts {
		used[ont.ONTID] = struct{}{}
	}
	if requested > 0 {
		if _, ok := used[requested]; ok {
			return 0, fmt.Errorf("ONU ID %d is already used on this port", requested)
		}
		return requested, nil
	}
	for id := minZTEONUID; id <= maxZTEONUID; id++ {
		if _, ok := used[id]; !ok {
			return id, nil
		}
	}
	return 0, fmt.Errorf("no free ONU IDs remain on this port")
}

// reserveONUError turns the serial index's rejection into something an operator
// can act on. A registration against a busy OLT can take minutes, so an
// operator who reloads and submits again lands here; the raw constraint
// violation read as a fault in TikMan rather than as the first attempt still
// running.
func reserveONUError(tx *gorm.DB, serial string, cause error) error {
	if !errors.Is(cause, gorm.ErrDuplicatedKey) && !strings.Contains(cause.Error(), "idx_onts_serial_number") {
		return fmt.Errorf("reserve ONU position: %w", cause)
	}

	var existing models.ONT
	if err := tx.Where("serial_number = ?", serial).First(&existing).Error; err != nil {
		return fmt.Errorf("serial %s is already registered", serial)
	}

	var running int64
	tx.Model(&models.ProvisioningJob{}).
		Where("ont_id = ? AND status = ?", existing.ID, "running").
		Count(&running)
	if running > 0 {
		return fmt.Errorf("a registration for %s is already running; wait for it to finish rather than starting another", serial)
	}

	position := "an unknown position"
	if existing.Slot != nil {
		position = fmt.Sprintf("1/%d/%d:%d", *existing.Slot, existing.PortID, existing.ONTID)
	}
	return fmt.Errorf("serial %s is already registered at %s", serial, position)
}

// zteRegistrationIndex reports where the "onu N type X sn Y" line sits in a
// command list, or -1 when the list has none.
func zteRegistrationIndex(commands []string) int {
	for i, command := range commands {
		if zteRegistrationLine.MatchString(strings.TrimSpace(command)) {
			return i
		}
	}
	return -1
}
