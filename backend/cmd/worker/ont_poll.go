package main

import (
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// processOnt applies one OLT reading to one ONT and returns the sample and the
// status change to be written for it.
//
// The sample is returned rather than written: the caller collects a page of
// them and writes them in one statement, where this used to cost one INSERT and
// three log lines per subscriber per cycle.
func processOnt(rt *workerRuntime, reading *oltReading, ont models.ONT) (services.MetricSample, services.StatusChange) {
	foundMetrics, discoveredSlot := matchMetricsForONT(ont, reading)

	ontRates := lookupRatesForONT(ont, discoveredSlot, reading.rates)

	newStatus := determineOntStatus(ont, reading, foundMetrics, rt.logger)

	handleStatusChange(rt.onts, ont, newStatus, rt.logger)

	updateOntFields(rt.db, ont, foundMetrics, discoveredSlot, rt.logger)

	// Every ONT gets a row, including one the walk returned nothing for. A gap
	// in the series would be indistinguishable from a cycle that never ran.
	return services.MetricSample{ONTID: ont.ID, Metrics: foundMetrics, Rates: ontRates}, statusChangeFor(ont, newStatus)
}

func matchMetricsForONT(ont models.ONT, reading *oltReading) (*connectivity.ONTMetrics, int) {
	// An ONT that knows its card is addressed directly. This used to search the
	// whole reading for every ONT, which costs the square of the subscriber
	// count and was the second largest expense in a cycle.
	if ont.Slot != nil {
		loc := connectivity.ONTLocation{Slot: *ont.Slot, Port: ont.PortID, ONTID: ont.ONTID}
		if m, ok := reading.metrics[loc]; ok {
			return &m, loc.Slot
		}
		return nil, 0
	}

	// An ONT registered before its OLT reported card numbers has none to match
	// on, so it takes the reading at its port and ONU number and adopts the card
	// that reading came from.
	if found, ok := reading.metricsPositions[positionKey{ont.PortID, ont.ONTID}]; ok {
		return &found.metrics, found.slot
	}
	return nil, 0
}

func lookupRatesForONT(ont models.ONT, discoveredSlot int, oltRatesMap map[connectivity.ONTLocation]connectivity.ONUTrafficRates) *connectivity.ONUTrafficRates {
	// Rate lookup key: prefer the ONT's stored slot, fall back to the slot
	// just discovered from the metrics walk (first cycles after discovery).
	rateSlot := discoveredSlot
	if ont.Slot != nil {
		rateSlot = *ont.Slot
	}
	if r, ok := oltRatesMap[connectivity.ONTLocation{Slot: rateSlot, Port: ont.PortID, ONTID: ont.ONTID}]; ok {
		return &r
	}
	return nil
}

// storeMetricsForONT stores the matched metrics, or empty metrics when the OLT
// reported none. Returns false only when storing real metrics failed, so the
// caller skips status handling for this ONT.
func determineOntStatus(ont models.ONT, reading *oltReading, foundMetrics *connectivity.ONTMetrics, logger *zap.Logger) models.ONTStatus {
	var newStatus models.ONTStatus
	foundStatus := false
	if runState, reported := reading.runStateFor(ont); reported {
		mapped := mapRunStateToStatus(runState)
		// A phase the ONU passes through while it comes up is not a verdict.
		// Writing it as "unknown" threw away the status the row already had and
		// stamped a last_offline the ONU never had, for a device that was online
		// a minute later.
		if mapped == models.ONTStatusUnknown {
			logger.Info("Transitional phase state; leaving the row",
				zap.String("serial", ont.SerialNumber), zap.Int("runState", runState))
		} else {
			newStatus = mapped
			foundStatus = true
		}
	}

	if !foundStatus {
		if foundMetrics != nil && foundMetrics.RxPower != nil {
			newStatus = models.ONTStatusOnline
			logger.Info("Status from RX Power", zap.String("serial", ont.SerialNumber), zap.String("status", string(newStatus)))
		} else {
			// No entry and no reading is no evidence, not evidence of being
			// down: an OLT the cycle failed to read reported nothing for every
			// ONT, and defaulting marked all two hundred offline at once. The
			// OLT lists an ONU it holds whether it is up or not, so a genuine
			// outage arrives as a phase state, not as silence. An empty status
			// leaves the row for the next cycle.
			logger.Info("No status evidence this cycle", zap.String("serial", ont.SerialNumber))
		}
	}

	return newStatus
}

// handleStatusChange writes a changed status through ONTService, which is also
// what the trap receiver writes through. A subscriber's status therefore has one
// way of being set rather than two implementations that can drift apart.
func handleStatusChange(onts *services.ONTService, ont models.ONT, newStatus models.ONTStatus, logger *zap.Logger) {
	if newStatus == "" || newStatus == ont.Status {
		return
	}

	if err := onts.UpdateStatus(ont.ID, newStatus); err != nil {
		logger.Error("Failed to update ONT status",
			zap.String("serial", ont.SerialNumber), zap.Error(err))
		return
	}
	logger.Info("Updated ONT status",
		zap.String("serial", ont.SerialNumber), zap.String("status", string(newStatus)))
}

// statusChangeFor states the ONT's status for the page's event write, on every
// cycle rather than only when the status differs. LogStatusChanges is
// idempotent: it opens a baseline event for an ONT that has none, writes
// nothing when the state is unchanged, and closes the previous event's duration
// on a real transition.
//
// Guarding this on "status changed" would starve it, because an ONT registered
// with the status the OLT already reports never changes - which is every ONT
// on a newly added OLT. Availability needs that opening event to have any
// interval to measure.
//
// A zero StatusChange means the cycle found no evidence for this ONT, and the
// caller leaves it out of the page.
func statusChangeFor(ont models.ONT, newStatus models.ONTStatus) services.StatusChange {
	if newStatus == "" {
		return services.StatusChange{}
	}

	eventType := models.EventTypeOnline
	if newStatus != models.ONTStatusOnline {
		eventType = models.EventTypeOffline
	}
	return services.StatusChange{ONTID: ont.ID, EventType: eventType, Reason: string(newStatus)}
}

func updateOntFields(db *gorm.DB, ont models.ONT, foundMetrics *connectivity.ONTMetrics, discoveredSlot int, logger *zap.Logger) {
	var rxPower, txPower *float64
	var distance int
	if foundMetrics != nil {
		rxPower = foundMetrics.RxPower
		txPower = foundMetrics.TxPower
		distance = foundMetrics.Distance

		updates := make(map[string]interface{})
		if rxPower != nil {
			updates["rx_power"] = *rxPower
		}
		if txPower != nil {
			updates["tx_power"] = *txPower
		}
		if distance > 0 {
			updates["distance"] = distance
		}
		if foundMetrics.SoftwareVersion != "" {
			updates["software_version"] = foundMetrics.SoftwareVersion
		}
		// Written whenever the OLT matched this ONT, including slot 0. The guard
		// used to be "discoveredSlot > 0", which conflated a real slot 0 with
		// "not discovered yet": a chassis-less OLT such as the HSGQ XE08ID has
		// no card slots and correctly reports 0, so its ONTs kept a NULL slot
		// forever. GetRealtimeMetrics refuses a NULL slot, which is why the
		// Traffic Statistics tab reported nothing for those ONTs.
		//
		// Reaching here means foundMetrics != nil, so the slot came from a
		// location the OLT actually reported rather than from a zero value.
		updates["slot"] = discoveredSlot

		if len(updates) > 0 {
			if err := db.Table("onts").Where("id = ?", ont.ID).Updates(updates).Error; err != nil {
				logger.Error("Failed to update ONT fields", zap.String("serial", ont.SerialNumber), zap.Error(err))
			}
		}
	}
}
