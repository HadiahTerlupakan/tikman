package services

import (
	"fmt"
	"time"

	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// A freshly registered ONU has to range before the OLT reports a phase state
// for it, which takes a few seconds. Waiting briefly here is what saves the
// operator the wait for the next discovery cycle, which is gated at thirty
// minutes: an ONU registered a minute after one sat at "unknown" for the rest
// of the window even though the OLT already had it online.
// Variables, not constants, only so the tests covering the wait do not have to
// spend it.
var (
	onuSettleAttempts = 5
	onuSettleInterval = 1500 * time.Millisecond
)

// resolveONUStatusAfterProvision reads one ONU's phase state straight off the
// OLT and stores it. An ONU that has not ranged by the time the attempts run
// out keeps the status it has, for the poll to correct later.
func resolveONUStatusAfterProvision(db *gorm.DB, olt models.OLT, ont models.ONT) error {
	driver, err := connectivity.DriverFor(olt.Model)
	if err != nil {
		return err
	}
	querier, ok := driver.(connectivity.StatusQuerier)
	if !ok {
		return connectivity.ErrUnsupported
	}
	return storeSettledONUStatus(db, querier, olt, ont)
}

// storeSettledONUStatus polls one ONU until the OLT reports a state it can map,
// then writes it. Split from the driver lookup so the wait can be exercised
// without a reachable OLT.
func storeSettledONUStatus(db *gorm.DB, querier connectivity.StatusQuerier, olt models.OLT, ont models.ONT) error {
	if ont.Slot == nil {
		return fmt.Errorf("ONT has no slot")
	}

	location := connectivity.ONTLocation{Slot: *ont.Slot, Port: ont.PortID, ONTID: ont.ONTID}
	locations := []connectivity.ONTLocation{location}

	for attempt := 0; attempt < onuSettleAttempts; attempt++ {
		if attempt > 0 {
			time.Sleep(onuSettleInterval)
		}
		statuses, err := querier.QueryStatusFor(olt.IPAddress, olt.SNMPCommunity, olt.SNMPPort, locations)
		if err != nil {
			return err
		}
		state, found := statuses[location]
		if !found || state == connectivity.PhaseStateUnknown {
			continue
		}
		return NewONTService(db).UpdateStatus(ont.ID, models.ONTStatus(connectivity.PhaseStateName(state)))
	}

	return nil
}
