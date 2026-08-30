package main

import (
	"errors"

	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// oltReading is one cycle's SNMP read of a single OLT.
//
// It replaces four parallel maps keyed by OLT id that were threaded through
// these functions as pointers. There is one OLT in flight at a time now, so the
// keying bought nothing and the pointers hid that statuses could be present
// while the walk that filled them had failed.
type positionKey struct{ port, ontID int }

// locatedMetrics carries the card the reading came from, which an ONT whose own
// slot is still unknown adopts from it.
type locatedMetrics struct {
	metrics connectivity.ONTMetrics
	slot    int
}

type oltReading struct {
	statuses map[connectivity.ONTLocation]int
	// statusWalkOK separates "the OLT reported no ONUs" from "we could not ask".
	// Treating those alike is what once marked every subscriber offline at once.
	statusWalkOK bool
	metrics      map[connectivity.ONTLocation]connectivity.ONTMetrics
	rates        map[connectivity.ONTLocation]connectivity.ONUTrafficRates

	// Both indexes exist because the old code searched the whole map for every
	// ONT. On a chassis of 10,000 that is 100 million comparisons per table per
	// cycle, and it grows with the square of the subscriber count.
	statusPositions  map[positionKey]int
	metricsPositions map[positionKey]locatedMetrics
}

// index builds the position lookups once, after the walks have filled the maps.
func (r *oltReading) index() {
	r.statusPositions = make(map[positionKey]int, len(r.statuses))
	for loc, runState := range r.statuses {
		r.statusPositions[positionKey{loc.Port, loc.ONTID}] = runState
	}

	r.metricsPositions = make(map[positionKey]locatedMetrics, len(r.metrics))
	for loc, m := range r.metrics {
		key := positionKey{loc.Port, loc.ONTID}
		if _, taken := r.metricsPositions[key]; taken {
			// Several cards can carry the same port and ONU number. Only an ONT
			// whose own card is unknown reads this index, and for that ONT one
			// arbitrary match is all the old linear search offered too.
			continue
		}
		r.metricsPositions[key] = locatedMetrics{metrics: m, slot: loc.Slot}
	}
}

// reportsPosition says whether the OLT named this port and ONU in its status
// walk.
func (r *oltReading) reportsPosition(portID, ontID int) bool {
	_, found := r.statusPositions[positionKey{portID, ontID}]
	return found
}

// runStateFor returns the phase state the OLT reported for one ONT.
//
// An ONT that knows its card is answered from that card and no other: on a
// multi-card chassis the same port and ONU number exists several times over,
// and reading a neighbour's state would report the wrong subscriber up or down.
// An ONT whose card is still unknown falls back to the position index, which is
// what the old linear search gave it.
func (r *oltReading) runStateFor(ont models.ONT) (int, bool) {
	if ont.Slot != nil {
		runState, found := r.statuses[connectivity.ONTLocation{Slot: *ont.Slot, Port: ont.PortID, ONTID: ont.ONTID}]
		return runState, found
	}
	runState, found := r.statusPositions[positionKey{ont.PortID, ont.ONTID}]
	return runState, found
}

// readOLT walks statuses, metrics, and traffic rates for one OLT and records
// its online/offline connection status. Returns ok=false when the OLT has no
// usable driver or another reader holds it this cycle.
func readOLT(db *gorm.DB, olt models.OLT, logger *zap.Logger) (*oltReading, bool) {
	driver, err := connectivity.DriverFor(olt.Model)
	if err != nil {
		logger.Error("Cannot poll OLT", zap.String("olt", olt.Name), zap.Error(err))
		return nil, false
	}

	// The discovery poll runs alongside this and walks the same tables. Taking
	// turns rather than competing: it stores the same metrics, so a cycle that
	// stands down loses nothing.
	// Nothing is returned on the way out. A caller that received an empty but
	// present reading would treat it as "this OLT reported nothing", which read
	// as every subscriber having gone offline at once.
	release, free := services.TryLockOLTSNMP(olt.ID)
	if !free {
		logger.Info("Skipping OLT this cycle: discovery is reading it",
			zap.String("olt", olt.Name))
		return nil, false
	}
	defer release()

	reading := &oltReading{}

	// Statuses first: they name every ONU the OLT has, which is what lets the
	// metrics be fetched by instance instead of swept.
	walkStatusesForOLT(db, driver, olt, reading, logger)
	readMetricsForOLT(driver, olt, reading, logger)
	walkRatesForOLT(driver, olt, reading, logger)
	reading.index()

	return reading, true
}

func readMetricsForOLT(driver connectivity.Driver, olt models.OLT, reading *oltReading, logger *zap.Logger) {
	statuses := reading.statuses
	querier, direct := driver.(connectivity.MetricsQuerier)
	if direct && len(statuses) > 0 {
		locations := make([]connectivity.ONTLocation, 0, len(statuses))
		for loc := range statuses {
			locations = append(locations, loc)
		}
		metricsMap, err := querier.QueryMetricsFor(olt.IPAddress, olt.SNMPCommunity, olt.SNMPPort, locations)
		if err == nil {
			logger.Info("Read ONT metrics", zap.String("olt", olt.Name), zap.Int("count", len(metricsMap)))
			reading.metrics = metricsMap
			return
		}
		logger.Error("Failed to read ONT metrics", zap.String("olt", olt.Name), zap.Error(err))
	}

	metricsMap, err := driver.WalkMetrics(olt.IPAddress, olt.SNMPCommunity, olt.SNMPPort)
	if err != nil {
		logger.Error("Failed to walk ONT metrics", zap.String("olt", olt.Name), zap.Error(err))
		reading.metrics = make(map[connectivity.ONTLocation]connectivity.ONTMetrics)
		return
	}
	logger.Info("Walked ONT metrics", zap.String("olt", olt.Name), zap.Int("count", len(metricsMap)))
	reading.metrics = metricsMap
}

func walkStatusesForOLT(db *gorm.DB, driver connectivity.Driver, olt models.OLT, reading *oltReading, logger *zap.Logger) {
	statuses, err := driver.WalkStatuses(olt.IPAddress, olt.SNMPCommunity, olt.SNMPPort)
	if err != nil {
		logger.Error("Failed to walk ONT statuses", zap.String("olt", olt.Name), zap.Error(err))
		reading.statuses = make(map[connectivity.ONTLocation]int)
		if err := updateOLTConnectionStatus(db, olt.ID, models.OLTStatusOffline, logger); err != nil {
			logger.Error("Failed to update OLT status", zap.String("olt", olt.Name), zap.Error(err))
		}
		return
	}

	logger.Info("Walked ONT statuses", zap.String("olt", olt.Name), zap.Int("count", len(statuses)))
	reading.statuses = statuses
	reading.statusWalkOK = true
	if err := updateOLTConnectionStatus(db, olt.ID, models.OLTStatusOnline, logger); err != nil {
		logger.Error("Failed to update OLT status", zap.String("olt", olt.Name), zap.Error(err))
	}
}

func walkRatesForOLT(driver connectivity.Driver, olt models.OLT, reading *oltReading, logger *zap.Logger) {
	rates, err := driver.WalkTrafficRates(olt.IPAddress, olt.SNMPCommunity, olt.SNMPPort)
	switch {
	case errors.Is(err, connectivity.ErrUnsupported):
		// A model with no known rate OIDs is not a fault; rates stay unset.
		logger.Info("Traffic rate gauges unsupported", zap.String("olt", olt.Name), zap.String("model", string(olt.Model)))
		reading.rates = make(map[connectivity.ONTLocation]connectivity.ONUTrafficRates)
	case err != nil:
		logger.Error("Failed to walk ONT traffic rates", zap.String("olt", olt.Name), zap.Error(err))
		reading.rates = make(map[connectivity.ONTLocation]connectivity.ONUTrafficRates)
	default:
		logger.Info("Walked ONT traffic rates", zap.String("olt", olt.Name), zap.Int("count", len(rates)))
		reading.rates = rates
	}
}

// syncOntsWithDiscovery prunes ONTs the OLT no longer reports and registers
// newly discovered ones. Returns false when discovery or pruning failed, so
// the caller skips the ONT and retries the sync on the next ONT of this OLT.
