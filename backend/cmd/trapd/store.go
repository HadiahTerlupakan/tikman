package main

import (
	"time"

	"github.com/tikman/olt-provisioning/internal/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

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
