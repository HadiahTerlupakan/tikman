package services

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// PruneMissingFromDiscovery deletes local ONTs that are no longer reported by the OLT.
//
// An empty discovery prunes nothing. A walk that returns no rows without
// returning an error is indistinguishable from an OLT that genuinely has no
// ONTs, and on a busy C300 the phase-state table does come back empty: a
// contended walk once deleted a whole 198-ONT inventory and its event history.
// Leaving stale rows until a discovery reports something is the cheaper
// mistake of the two.
func (s *ONTService) PruneMissingFromDiscovery(oltID uuid.UUID, discovered []connectivity.DiscoveredONT) (int64, error) {
	if len(discovered) == 0 {
		return 0, nil
	}

	discoveredPositions := make(map[ontPosition]struct{}, len(discovered))
	for _, ont := range discovered {
		discoveredPositions[ontPosition{portID: ont.PortID, ontID: ont.ONTID}] = struct{}{}
	}

	var localONTs []models.ONT
	if err := s.db.Where("olt_id = ?", oltID).Find(&localONTs).Error; err != nil {
		return 0, fmt.Errorf("failed to list local ONTs: %w", err)
	}

	staleIDs := make([]uuid.UUID, 0)
	for _, ont := range localONTs {
		position := ontPosition{portID: ont.PortID, ontID: ont.ONTID}
		if _, found := discoveredPositions[position]; !found {
			staleIDs = append(staleIDs, ont.ID)
		}
	}

	if len(staleIDs) == 0 {
		return 0, nil
	}

	return int64(len(staleIDs)), s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM ont_metrics WHERE ont_id IN ?", staleIDs).Error; err != nil {
			return fmt.Errorf("failed to delete stale ONT metrics: %w", err)
		}
		if err := tx.Delete(&models.ONTEvent{}, "ont_id IN ?", staleIDs).Error; err != nil {
			return fmt.Errorf("failed to delete stale ONT events: %w", err)
		}
		if err := tx.Delete(&models.ONT{}, "id IN ?", staleIDs).Error; err != nil {
			return fmt.Errorf("failed to delete stale ONTs: %w", err)
		}
		return nil
	})
}

type ontPosition struct {
	portID int
	ontID  int
}
