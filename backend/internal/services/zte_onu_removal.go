package services

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// BuildZTEONURemovalCommands returns the commands that take an ONU off the OLT.
//
// The registration lives under the PON interface as "onu N type X sn Y", so the
// removal is issued in that context. Removing the ONU takes its per-ONU
// interface and management config with it, which is why neither is addressed
// separately; a leftover would surface as the serial still appearing in the
// running config, which the caller checks.
func BuildZTEONURemovalCommands(card, pon, onuID int) []string {
	return []string{
		fmt.Sprintf("interface gpon-olt_1/%d/%d", card, pon),
		fmt.Sprintf("no onu %d", onuID),
		"exit",
	}
}

// ZTEONURemovalService takes an ONU off the OLT before TikMan forgets it.
type ZTEONURemovalService struct {
	db         *gorm.DB
	ontService *ONTService
	factory    CommanderFactory
}

func NewZTEONURemovalService(db *gorm.DB, ontService *ONTService, factory CommanderFactory) *ZTEONURemovalService {
	return &ZTEONURemovalService{db: db, ontService: ontService, factory: factory}
}

// PreviewRemoval returns the commands a removal would send, without sending
// them. The operator sees exactly what will reach the device first.
func (s *ZTEONURemovalService) PreviewRemoval(ontID uuid.UUID) ([]string, error) {
	ont, err := s.ontService.GetByID(ontID)
	if err != nil {
		return nil, err
	}
	card, err := ontCard(ont)
	if err != nil {
		return nil, err
	}
	return BuildZTEONURemovalCommands(card, ont.PortID, ont.ONTID), nil
}

// RemoveFromOLT deletes the ONU's registration from the OLT. It does not touch
// TikMan's own records: the caller deletes those only once this has succeeded,
// so a failed removal never leaves an ONU on the OLT that TikMan has forgotten.
func (s *ZTEONURemovalService) RemoveFromOLT(ctx context.Context, ontID uuid.UUID) error {
	ont, err := s.ontService.GetByID(ontID)
	if err != nil {
		return err
	}
	card, err := ontCard(ont)
	if err != nil {
		return err
	}

	var olt models.OLT
	if err := s.db.First(&olt, "id = ?", ont.OLTID).Error; err != nil {
		return fmt.Errorf("cannot load the OLT: %w", err)
	}
	if olt.Model != models.OLTModelZTEC300 && olt.Model != models.OLTModelZTEC320 {
		return fmt.Errorf("removing an ONU from a %s is not supported", olt.Model)
	}

	commander, err := createCommanderForOLT(s.factory, olt)
	if err != nil {
		return fmt.Errorf("cannot open a session to the OLT: %w", err)
	}
	defer closeCommander(commander)

	commands := BuildZTEONURemovalCommands(card, ont.PortID, ont.ONTID)
	results, err := commander.BatchExecute(ctx, commands)
	if err != nil {
		return fmt.Errorf("removal failed: %w", err)
	}
	for i, result := range results {
		if result == nil || !result.Success {
			return failedZTECommand(commands, i, result)
		}
	}
	if len(results) != len(commands) {
		return failedZTECommand(commands, len(results), nil)
	}

	return nil
}

// ontCard reports the card an ONT sits on. A row the discovery poll has not
// placed cannot be addressed on the OLT, and guessing a card would delete a
// different subscriber's ONU.
func ontCard(ont *models.ONT) (int, error) {
	if ont.Slot == nil {
		return 0, fmt.Errorf("ONT %s has no card recorded yet, so it cannot be addressed on the OLT", ont.SerialNumber)
	}
	return *ont.Slot, nil
}
