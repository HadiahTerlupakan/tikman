package main

import (
	"time"

	"github.com/tikman/olt-provisioning/internal/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// trapRetention is how long received traps are kept.
//
// Long enough to correlate a trap against the status changes the poller
// recorded around it, which is how these notifications' meanings get
// established. Cariu alone sends about 200 a minute, so keeping them forever
// would trade a growing table for evidence nobody reads.
const trapRetention = 7 * 24 * time.Hour

// retentionInterval is how often expired traps are swept.
const retentionInterval = time.Hour

// trapStore writes received traps down so they outlive the container's logs.
type trapStore struct {
	db     *gorm.DB
	logger *zap.Logger
}

// record persists one trap.
//
// A failure is logged and nothing else. Losing a trap is bad; stopping the
// receiver over one is worse, because then every subsequent trap is lost too.
func (s *trapStore) record(trap Trap) {
	identity := identify(trap.Varbinds)

	event := models.ONTTrapEvent{
		OLTID:         trap.OLTID,
		ReceivedAt:    time.Now(),
		TrapOID:       trap.OID,
		SourceAddress: trap.Source,
		Varbinds:      trap.describe(),
	}
	if identity.SerialNumber != "" {
		event.SerialNumber = &identity.SerialNumber
	}
	if identity.Label != "" {
		event.ONULabel = &identity.Label
	}
	if identity.Name != "" {
		event.ONUName = &identity.Name
	}
	if identity.IfIndex != 0 {
		event.IfIndex = &identity.IfIndex
		event.ONUID = &identity.ONUID
	}

	if err := s.db.Create(&event).Error; err != nil {
		s.logger.Error("Failed to record a trap",
			zap.String("oid", trap.OID), zap.String("source", trap.Source), zap.Error(err))
	}
}

// sweepExpired deletes traps past the retention window, on its own timer.
func (s *trapStore) sweepExpired(stop <-chan struct{}) {
	ticker := time.NewTicker(retentionInterval)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			cutoff := time.Now().Add(-trapRetention)
			result := s.db.Where("received_at < ?", cutoff).Delete(&models.ONTTrapEvent{})
			if result.Error != nil {
				s.logger.Error("Failed to sweep expired traps", zap.Error(result.Error))
				continue
			}
			if result.RowsAffected > 0 {
				s.logger.Info("Swept expired traps", zap.Int64("removed", result.RowsAffected))
			}
		}
	}
}
