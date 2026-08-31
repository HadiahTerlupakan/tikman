package main

import (
	"errors"
	"strings"

	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// onuTrapFamily is the subtree ZTE sends ONU state notifications under.
//
// It is the guard, not the meaning. Different chassis in this network use
// different OIDs within it for the same two states — a C300 sends .1 and .2, a
// C320 sends .9, .10, .15 and .16 — so an OID list would have to be extended for
// every model met. What the subtree does establish is that the notification is
// about an ONU at all, which keeps this path off board and system alarms.
const onuTrapFamily = "1.3.6.1.4.1.3902.1082.500.10.3.1."

// eventLevelField is how ZTE labels the severity it packs into the community.
const eventLevelField = "eventLevel="

// trapStatus reports the status a trap carries, when it carries one.
//
// The severity comes from the device rather than from a table of ours: ZTE packs
// the event's own level into the v2c community, as
// "public@eventId=40366@eventLevel=minor@confirm@20260211174422". A cleared
// alarm is an ONU that came back; any severity is one that went away.
//
// This was cross-checked against the other evidence before it was trusted:
// correlating 23,158 recorded traps against the poller's own status transitions
// had already established .1 as down and .2 as up on the C300, and the levels
// those traps carry — major and cleared — say exactly the same thing.
//
// notification is deliberately not a state. Those traps name no subscriber and
// carry no ONU varbinds; treating them as alarms would write a status from a
// message that is not about a subscriber.
func trapStatus(oid, community string) (models.ONTStatus, bool) {
	if !strings.HasPrefix(normaliseOID(oid), onuTrapFamily) {
		return "", false
	}

	switch severityOf(community) {
	case "cleared":
		return models.ONTStatusOnline, true
	case "critical", "major", "minor", "warning":
		return models.ONTStatusOffline, true
	default:
		return "", false
	}
}

// severityOf reads the eventLevel field out of a trap's community string.
func severityOf(community string) string {
	for _, field := range strings.Split(community, "@") {
		if level, found := strings.CutPrefix(field, eventLevelField); found {
			return level
		}
	}
	return ""
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
	status, known := trapStatus(trap.OID, trap.Community)
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

	if !worthWriting(ont.Status, status) {
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

// worthWriting says whether a trap's verdict should replace the status a
// subscriber already holds.
//
// A trap reports a transition, not a diagnosis. The poller reads the OLT's phase
// state and can tell los from dying_gasp; a trap can only say down. So a down
// trap speaks only for an ONT the system still believes is up — otherwise it
// overwrites the specific reason with a vaguer one and the next poll writes it
// straight back, leaving the subscriber's status oscillating between two words
// for one outage.
func worthWriting(current, reported models.ONTStatus) bool {
	if reported == models.ONTStatusOnline {
		return current != models.ONTStatusOnline
	}
	return current == models.ONTStatusOnline
}
