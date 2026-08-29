package main

import (
	"fmt"
	"time"

	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func processOnt(db *gorm.DB, oltSlot int, oltMetricsMap map[connectivity.ONTLocation]connectivity.ONTMetrics, oltStatusMap map[connectivity.ONTLocation]int, oltRatesMap map[connectivity.ONTLocation]connectivity.ONUTrafficRates, ont models.ONT, metricsService *services.MetricsService, eventService *services.EventService, logger *zap.Logger) {
	foundMetrics, discoveredSlot := matchMetricsForONT(ont, oltMetricsMap, logger)

	logFirstONTDebug(ont, oltSlot, oltMetricsMap, foundMetrics, logger)

	ontRates := lookupRatesForONT(ont, discoveredSlot, oltRatesMap)

	if !storeMetricsForONT(ont, foundMetrics, ontRates, metricsService, logger) {
		return
	}

	newStatus := determineOntStatus(ont, oltStatusMap, foundMetrics, logger)

	logger.Info("Status check", zap.String("serial", ont.SerialNumber), zap.String("newStatus", string(newStatus)), zap.String("currentStatus", string(ont.Status)))

	handleStatusChange(db, ont, newStatus, logger)

	logStatusChangeEvent(ont, newStatus, eventService, logger)

	updateOntFields(db, ont, foundMetrics, discoveredSlot, logger)

	var rxPower, txPower *float64
	if foundMetrics != nil {
		rxPower = foundMetrics.RxPower
		txPower = foundMetrics.TxPower
	}
	logger.Info("Collected metrics",
		zap.String("serial", ont.SerialNumber),
		zap.Float64p("rx_power", rxPower),
		zap.Float64p("tx_power", txPower))
}

func matchMetricsForONT(ont models.ONT, oltMetricsMap map[connectivity.ONTLocation]connectivity.ONTMetrics, logger *zap.Logger) (*connectivity.ONTMetrics, int) {
	var foundMetrics *connectivity.ONTMetrics
	var discoveredSlot int
	for loc, m := range oltMetricsMap {
		// Match by port and ontID, and slot if already known
		portMatch := loc.Port == ont.PortID
		ontIDMatch := loc.ONTID == ont.ONTID
		slotMatch := ont.Slot == nil || loc.Slot == *ont.Slot

		if portMatch && ontIDMatch && slotMatch {
			foundMetrics = &m
			discoveredSlot = loc.Slot
			break
		}
	}
	return foundMetrics, discoveredSlot
}

// logFirstONTDebug logs the first ONT for debugging.
func logFirstONTDebug(ont models.ONT, oltSlot int, oltMetricsMap map[connectivity.ONTLocation]connectivity.ONTMetrics, foundMetrics *connectivity.ONTMetrics, logger *zap.Logger) {
	if ont.PortID != 1 || ont.ONTID != 1 {
		return
	}
	firstLoc := connectivity.ONTLocation{}
	for loc := range oltMetricsMap {
		firstLoc = loc
		break
	}
	rxVal := "null"
	txVal := "null"
	if foundMetrics != nil {
		if foundMetrics.RxPower != nil {
			rxVal = fmt.Sprintf("%.2f", *foundMetrics.RxPower)
		}
		if foundMetrics.TxPower != nil {
			txVal = fmt.Sprintf("%.2f", *foundMetrics.TxPower)
		}
	}
	logger.Info("DEBUG: First ONT matching",
		zap.String("serial", ont.SerialNumber),
		zap.Int("ont_slot", oltSlot), zap.Int("snmp_slot_sample", firstLoc.Slot),
		zap.Int("ont_port", ont.PortID), zap.Int("snmp_port_sample", firstLoc.Port),
		zap.Int("ont_ontid", ont.ONTID), zap.Int("snmp_ontid_sample", firstLoc.ONTID),
		zap.Bool("matched", foundMetrics != nil),
		zap.String("rx", rxVal), zap.String("tx", txVal))
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
func storeMetricsForONT(ont models.ONT, foundMetrics *connectivity.ONTMetrics, ontRates *connectivity.ONUTrafficRates, metricsService *services.MetricsService, logger *zap.Logger) bool {
	if foundMetrics != nil {
		if err := metricsService.StoreMetrics(ont.ID, foundMetrics, ontRates); err != nil {
			logger.Error("Failed to save metrics", zap.String("serial", ont.SerialNumber), zap.Error(err))
			return false
		}
		return true
	}

	emptyMetrics := &connectivity.ONTMetrics{}
	if err := metricsService.StoreMetrics(ont.ID, emptyMetrics, ontRates); err != nil {
		logger.Error("Failed to save empty metrics", zap.String("serial", ont.SerialNumber), zap.Error(err))
	}
	return true
}

func determineOntStatus(ont models.ONT, oltStatusMap map[connectivity.ONTLocation]int, foundMetrics *connectivity.ONTMetrics, logger *zap.Logger) models.ONTStatus {
	var newStatus models.ONTStatus
	foundStatus := false
	for loc, runState := range oltStatusMap {
		portMatch := loc.Port == ont.PortID
		ontIDMatch := loc.ONTID == ont.ONTID
		slotMatch := ont.Slot == nil || loc.Slot == *ont.Slot

		if portMatch && ontIDMatch && slotMatch {
			newStatus = mapRunStateToStatus(runState)
			foundStatus = true
			logger.Info("Status from SNMP", zap.String("serial", ont.SerialNumber), zap.Int("runState", runState), zap.String("status", string(newStatus)))
			break
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

func handleStatusChange(db *gorm.DB, ont models.ONT, newStatus models.ONTStatus, logger *zap.Logger) {
	if newStatus == "" || newStatus == ont.Status {
		return
	}

	now := time.Now()
	statusUpdates := map[string]interface{}{
		"status":     string(newStatus),
		"updated_at": now,
	}

	if newStatus == models.ONTStatusOnline {
		statusUpdates["last_seen_at"] = now
		statusUpdates["last_online"] = now
	} else {
		statusUpdates["last_offline"] = now
		statusUpdates["last_offline_reason"] = string(newStatus)
	}

	result := db.Model(&models.ONT{}).Where("id = ?", ont.ID).Updates(statusUpdates)
	if result.Error != nil {
		logger.Error("Failed to update ONT status", zap.String("serial", ont.SerialNumber), zap.Error(result.Error))
	} else if result.RowsAffected > 0 {
		logger.Info("Updated ONT status", zap.String("serial", ont.SerialNumber), zap.String("status", string(newStatus)), zap.Int64("rows", result.RowsAffected))
	} else {
		logger.Warn("No rows updated", zap.String("serial", ont.SerialNumber), zap.String("ont_id", ont.ID.String()))
	}
}

// logStatusChangeEvent records the ONT's state on every cycle rather than only
// when the status differs. LogStatusChange is idempotent: it opens a baseline
// event for an ONT that has none, returns without writing when the state is
// unchanged, and closes the previous event's duration on a real transition.
//
// Guarding this on "status changed" would starve it, because an ONT registered
// with the status the OLT already reports never changes - which is every ONT
// on a newly added OLT. Availability needs that opening event to have any
// interval to measure.
func logStatusChangeEvent(ont models.ONT, newStatus models.ONTStatus, eventService *services.EventService, logger *zap.Logger) {
	if newStatus == "" {
		return
	}

	eventType := models.EventTypeOnline
	if newStatus != models.ONTStatusOnline {
		eventType = models.EventTypeOffline
	}
	if err := eventService.LogStatusChange(ont.ID, eventType, string(newStatus)); err != nil {
		logger.Error("Failed to log ONT status event",
			zap.String("serial", ont.SerialNumber), zap.Error(err))
	}
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
