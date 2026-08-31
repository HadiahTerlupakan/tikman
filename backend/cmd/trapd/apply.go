package main

import (
	"errors"

	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// trapStatuses is what a notification OID says about an ONU.
//
// Established by correlating 23,158 recorded traps against the status
// transitions the poller wrote independently: for each transition, the last
// trap for that ONT within the two minutes before it. `.2` preceded an online
// status 265 times across 229 subscribers, `.24` 29 times across 22, `.23`
// preceded offline 19 times across 15 and never online, and `.1` preceded
// offline 40 times. A flapping ONU alternating `.2` and `.1` every few seconds
// settled the pair independently: a `.1` at 05:07:54 was followed by the poller
// recording `los` two seconds later.
//
// The other pairs this chassis sends — `.9`/`.10` and `.15`/`.16` — are
// deliberately absent. They behave like the ones above but were seen too rarely
// before a transition to say which way round they run, and a status written
// from a guess is the thing this whole path exists to avoid. They keep being
// recorded, so the evidence for them accumulates.
var trapStatuses = map[string]models.ONTStatus{
	"1.3.6.1.4.1.3902.1082.500.10.3.1.1":  models.ONTStatusOffline,
	"1.3.6.1.4.1.3902.1082.500.10.3.1.23": models.ONTStatusOffline,
	"1.3.6.1.4.1.3902.1082.500.10.3.1.2":  models.ONTStatusOnline,
	"1.3.6.1.4.1.3902.1082.500.10.3.1.24": models.ONTStatusOnline,
}

// trapStatus reports the status a notification OID carries, if it is one whose
// meaning is established.
func trapStatus(oid string) (models.ONTStatus, bool) {
	status, known := trapStatuses[normaliseOID(oid)]
	return status, known
}

// statusApplier writes what a trap reports onto the subscriber it names.
type statusApplier struct {
	db     *gorm.DB
	onts   *services.ONTService
	events *services.EventService
	logger *zap.Logger
}

func newStatusApplier(db *gorm.DB, logger *zap.Logger) *statusApplier {
	return &statusApplier{
		db:     db,
		onts:   services.NewONTService(db),
		events: services.NewEventService(db),
		logger: logger,
	}
}

// apply writes the trap's status through the same service the poller writes
// through, so a subscriber's status has one way of being set rather than two.
//
// It only ever reports a change. A trap naming an ONU we hold no row for is
// left alone — the recorded trap is the evidence of it — and nothing here
// deletes a row or concludes absence. The one-minute status poll remains the
// truth and reconciles whatever a lost trap left stale.
func (a *statusApplier) apply(trap Trap, identity onuIdentity) {
	status, known := trapStatus(trap.OID)
	if !known || identity.SerialNumber == "" {
		return
	}

	// Matched on the OLT as well as the serial: serial numbers are not unique
	// across chassis, and matching on serial alone would let one site write
	// another site's subscriber offline.
	var ont models.ONT
	err := a.db.Where("olt_id = ? AND serial_number = ?", trap.OLTID, identity.SerialNumber).
		First(&ont).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return
	}
	if err != nil {
		a.logger.Error("Failed to look up the ONT a trap names",
			zap.String("serial", identity.SerialNumber), zap.Error(err))
		return
	}

	if ont.Status == status {
		return
	}

	if err := a.onts.UpdateStatus(ont.ID, status); err != nil {
		a.logger.Error("Failed to apply a trap's status",
			zap.String("serial", ont.SerialNumber), zap.Error(err))
		return
	}

	eventType := models.EventTypeOnline
	if status != models.ONTStatusOnline {
		eventType = models.EventTypeOffline
	}
	if err := a.events.LogStatusChanges([]services.StatusChange{
		{ONTID: ont.ID, EventType: eventType, Reason: string(status)},
	}); err != nil {
		a.logger.Error("Failed to record a trap's status event",
			zap.String("serial", ont.SerialNumber), zap.Error(err))
	}

	a.logger.Info("Status from trap",
		zap.String("serial", ont.SerialNumber),
		zap.String("onu", identity.Label),
		zap.String("status", string(status)),
		zap.String("oid", trap.OID))
}
