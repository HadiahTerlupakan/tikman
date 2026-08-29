package services

import (
	"fmt"
	"time"

	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// A freshly registered ONU has to range before the OLT reports a phase state
// for it, and a busy OLT is slow to answer while it does: three and seven
// seconds for a GET that costs seven milliseconds once it is idle. Reading it
// here is what saves the operator the wait for the discovery poll, which is
// gated at thirty minutes.
//
// Variables, not constants, only so the tests covering the wait do not have to
// spend it.
var (
	onuSettleInterval = 1500 * time.Millisecond
	// How long the registration request itself waits. Short: it is the
	// operator's dialog that is being held open.
	onuSettleWindow = 8 * time.Second
	// How long the read carries on for after the request has answered. The ONU
	// is on the OLT either way, so this only decides whether the row says so
	// within the minute or at the next poll.
	onuSettleBackstop = 90 * time.Second
)

// resolveONUStatusAfterProvision reads one ONU's phase state straight off the
// OLT and stores it, giving up after window. It reports whether the OLT named
// a state; an ONU still ranging keeps the status it has.
func resolveONUStatusAfterProvision(db *gorm.DB, olt models.OLT, ont models.ONT, window time.Duration) (bool, error) {
	driver, err := connectivity.DriverFor(olt.Model)
	if err != nil {
		return false, err
	}
	querier, ok := driver.(connectivity.StatusQuerier)
	if !ok {
		return false, connectivity.ErrUnsupported
	}
	return storeSettledONUStatus(db, querier, olt, ont, window)
}

// storeSettledONUStatus polls one ONU until the OLT reports a state it can map,
// then writes it. Split from the driver lookup so the wait can be exercised
// without a reachable OLT.
func storeSettledONUStatus(db *gorm.DB, querier connectivity.StatusQuerier, olt models.OLT, ont models.ONT, window time.Duration) (bool, error) {
	if ont.Slot == nil {
		return false, fmt.Errorf("ONT has no slot")
	}

	location := connectivity.ONTLocation{Slot: *ont.Slot, Port: ont.PortID, ONTID: ont.ONTID}
	locations := []connectivity.ONTLocation{location}
	deadline := time.Now().Add(window)

	for attempt := 0; ; attempt++ {
		if attempt > 0 {
			if time.Now().After(deadline) {
				return false, nil
			}
			time.Sleep(onuSettleInterval)
		}
		statuses, err := querier.QueryStatusFor(olt.IPAddress, olt.SNMPCommunity, olt.SNMPPort, locations)
		if err != nil {
			return false, err
		}
		state, found := statuses[location]
		if !found || state == connectivity.PhaseStateUnknown {
			continue
		}
		return true, NewONTService(db).UpdateStatus(ont.ID, models.ONTStatus(connectivity.PhaseStateName(state)))
	}
}
