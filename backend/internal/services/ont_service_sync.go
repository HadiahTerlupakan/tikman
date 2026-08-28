package services

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// pruneGracePeriod is how long a freshly created ONT is left alone.
//
// An ONU registered moments ago is not in the OLT's tables yet — the walk that
// runs a minute later does not see it, and pruning then deletes the row the
// registration wrote, along with its metrics, its events, and the link its
// provisioning job holds. That is not a stale row; it is a row the OLT has not
// caught up with.
const pruneGracePeriod = 15 * time.Minute

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
	newestPrunable := time.Now().Add(-pruneGracePeriod)
	for _, ont := range localONTs {
		position := ontPosition{portID: ont.PortID, ontID: ont.ONTID}
		if _, found := discoveredPositions[position]; found {
			continue
		}
		if ont.CreatedAt.After(newestPrunable) {
			continue
		}
		staleIDs = append(staleIDs, ont.ID)
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
