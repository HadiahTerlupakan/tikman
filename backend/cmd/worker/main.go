package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/config"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/database"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	zapLogger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer func() {
		_ = zapLogger.Sync()
	}()

	db, err := database.Connect(cfg)
	if err != nil {
		zapLogger.Fatal(fmt.Sprintf("Failed to connect to database: %v", err))
	}

	ontService := services.NewONTService(db)
	metricsService := services.NewMetricsService(db)
	// Status history was never recorded by the running worker: the only writer of
	// ont_events was the /admin/seed-events endpoint, so an ONT's Events tab and
	// its availability figure were empty unless someone had seeded demo data.
	eventService := services.NewEventService(db)

	zapLogger.Info("Starting Worker service")
	zapLogger.Info("Metrics collection interval: 5 minutes")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	collectMetrics(db, ontService, metricsService, eventService, zapLogger)

	for {
		select {
		case <-ticker.C:
			collectMetrics(db, ontService, metricsService, eventService, zapLogger)
		case <-sigCh:
			zapLogger.Info("Received shutdown signal")
			return
		case <-ctx.Done():
			return
		}
	}
}

func collectMetrics(db *gorm.DB, ontService *services.ONTService, metricsService *services.MetricsService, eventService *services.EventService, logger *zap.Logger) {
	logger.Info("Starting metrics collection cycle")

	onts, _, err := ontService.List(nil, nil, 1000, 0)
	if err != nil {
		logger.Error("Failed to list ONTs", zap.Error(err))
		return
	}

	logger.Info(fmt.Sprintf("Found %d ONTs to collect metrics", len(onts)))

	oltMetricsCache := make(map[string]map[connectivity.ONTLocation]connectivity.ONTMetrics)
	oltStatusCache := make(map[string]map[connectivity.ONTLocation]int)
	oltStatusWalkOK := make(map[string]bool)
	oltPruned := make(map[string]bool)
	oltRatesCache := make(map[string]map[connectivity.ONTLocation]connectivity.ONUTrafficRates)

	for _, ont := range onts {
		var olt models.OLT
		if err := db.First(&olt, "id = ?", ont.OLTID).Error; err != nil {
			logger.Error("Failed to get OLT", zap.String("ont_id", ont.ID.String()), zap.Error(err))
			continue
		}

		oltKey := olt.ID.String()

		if _, exists := oltMetricsCache[oltKey]; !exists {
			driver, err := connectivity.DriverFor(olt.Model)
			if err != nil {
				logger.Error("Cannot poll OLT", zap.String("olt", olt.Name), zap.Error(err))
				oltMetricsCache[oltKey] = make(map[connectivity.ONTLocation]connectivity.ONTMetrics)
				oltStatusCache[oltKey] = make(map[connectivity.ONTLocation]int)
				oltRatesCache[oltKey] = make(map[connectivity.ONTLocation]connectivity.ONUTrafficRates)
				continue
			}

			metricsMap, err := driver.WalkMetrics(olt.IPAddress, olt.SNMPCommunity, olt.SNMPPort)
			if err != nil {
				logger.Error("Failed to walk ONT metrics", zap.String("olt", olt.Name), zap.Error(err))
				oltMetricsCache[oltKey] = make(map[connectivity.ONTLocation]connectivity.ONTMetrics)
			} else {
				logger.Info("Walked ONT metrics", zap.String("olt", olt.Name), zap.Int("count", len(metricsMap)))
				oltMetricsCache[oltKey] = metricsMap
			}

			statuses, err := driver.WalkStatuses(olt.IPAddress, olt.SNMPCommunity, olt.SNMPPort)
			if err != nil {
				logger.Error("Failed to walk ONT statuses", zap.String("olt", olt.Name), zap.Error(err))
				oltStatusCache[oltKey] = make(map[connectivity.ONTLocation]int)
				if err := updateOLTConnectionStatus(db, olt.ID, models.OLTStatusOffline, logger); err != nil {
					logger.Error("Failed to update OLT status", zap.String("olt", olt.Name), zap.Error(err))
				}
			} else {
				logger.Info("Walked ONT statuses", zap.String("olt", olt.Name), zap.Int("count", len(statuses)))
				oltStatusCache[oltKey] = statuses
				oltStatusWalkOK[oltKey] = true
				if err := updateOLTConnectionStatus(db, olt.ID, models.OLTStatusOnline, logger); err != nil {
					logger.Error("Failed to update OLT status", zap.String("olt", olt.Name), zap.Error(err))
				}
			}

			rates, err := driver.WalkTrafficRates(olt.IPAddress, olt.SNMPCommunity, olt.SNMPPort)
			switch {
			case errors.Is(err, connectivity.ErrUnsupported):
				// A model with no known rate OIDs is not a fault; rates stay unset.
				logger.Info("Traffic rate gauges unsupported", zap.String("olt", olt.Name), zap.String("model", string(olt.Model)))
				oltRatesCache[oltKey] = make(map[connectivity.ONTLocation]connectivity.ONUTrafficRates)
			case err != nil:
				logger.Error("Failed to walk ONT traffic rates", zap.String("olt", olt.Name), zap.Error(err))
				oltRatesCache[oltKey] = make(map[connectivity.ONTLocation]connectivity.ONUTrafficRates)
			default:
				logger.Info("Walked ONT traffic rates", zap.String("olt", olt.Name), zap.Int("count", len(rates)))
				oltRatesCache[oltKey] = rates
			}
		}

		if oltStatusWalkOK[oltKey] && !oltPruned[oltKey] {
			discovered, err := discoverONTsForSync(olt)
			if err != nil {
				logger.Error("Failed to discover ONTs for sync", zap.String("olt", olt.Name), zap.Error(err))
				continue
			}

			deleted, err := ontService.PruneMissingFromDiscovery(olt.ID, discovered)
			if err != nil {
				logger.Error("Failed to prune stale ONTs", zap.String("olt", olt.Name), zap.Error(err))
				continue
			}
			if deleted > 0 {
				logger.Info("Pruned stale ONTs", zap.String("olt", olt.Name), zap.Int64("deleted", deleted))
			}

			regResult := ontService.BulkRegisterFromDiscovery(olt.ID, discovered)
			if regResult.Registered > 0 {
				logger.Info("Synced ONTs from OLT", zap.String("olt", olt.Name), zap.Int("changed", regResult.Registered))
			}
			for _, e := range regResult.Errors {
				logger.Warn("ONT sync registration error", zap.String("olt", olt.Name), zap.String("error", e))
			}

			oltPruned[oltKey] = true
		}

		if oltStatusWalkOK[oltKey] {
			if _, found := lookupByPortAndONT(oltStatusCache[oltKey], ont.PortID, ont.ONTID); !found {
				continue
			}
		}

		var foundMetrics *connectivity.ONTMetrics
		var discoveredSlot int
		matchCount := 0
		for loc, m := range oltMetricsCache[oltKey] {
			// Match by port and ontID, and slot if already known
			portMatch := loc.Port == ont.PortID
			ontIDMatch := loc.ONTID == ont.ONTID
			slotMatch := ont.Slot == nil || loc.Slot == *ont.Slot

			if portMatch && ontIDMatch && slotMatch {
				foundMetrics = &m
				discoveredSlot = loc.Slot
				matchCount++
				break
			}
		}

		// Log first ONT for debugging
		if ont.PortID == 1 && ont.ONTID == 1 {
			firstLoc := connectivity.ONTLocation{}
			for loc := range oltMetricsCache[oltKey] {
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
				zap.Int("ont_slot", olt.Slot), zap.Int("snmp_slot_sample", firstLoc.Slot),
				zap.Int("ont_port", ont.PortID), zap.Int("snmp_port_sample", firstLoc.Port),
				zap.Int("ont_ontid", ont.ONTID), zap.Int("snmp_ontid_sample", firstLoc.ONTID),
				zap.Bool("matched", foundMetrics != nil),
				zap.String("rx", rxVal), zap.String("tx", txVal))
		}

		// Rate lookup key: prefer the ONT's stored slot, fall back to the slot
		// just discovered from the metrics walk (first cycles after discovery).
		rateSlot := discoveredSlot
		if ont.Slot != nil {
			rateSlot = *ont.Slot
		}
		var ontRates *connectivity.ONUTrafficRates
		if r, ok := oltRatesCache[oltKey][connectivity.ONTLocation{Slot: rateSlot, Port: ont.PortID, ONTID: ont.ONTID}]; ok {
			ontRates = &r
		}

		if foundMetrics != nil {
			if err := metricsService.StoreMetrics(ont.ID, foundMetrics, ontRates); err != nil {
				logger.Error("Failed to save metrics", zap.String("serial", ont.SerialNumber), zap.Error(err))
				continue
			}
		} else {
			emptyMetrics := &connectivity.ONTMetrics{}
			if err := metricsService.StoreMetrics(ont.ID, emptyMetrics, ontRates); err != nil {
				logger.Error("Failed to save empty metrics", zap.String("serial", ont.SerialNumber), zap.Error(err))
			}
		}

		var newStatus models.ONTStatus
		foundStatus := false
		for loc, runState := range oltStatusCache[oltKey] {
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
				newStatus = models.ONTStatusOffline
				logger.Info("Status defaulted to offline", zap.String("serial", ont.SerialNumber), zap.String("status", string(newStatus)))
			}
		}

		logger.Info("Status check", zap.String("serial", ont.SerialNumber), zap.String("newStatus", string(newStatus)), zap.String("currentStatus", string(ont.Status)))

		if newStatus != "" && newStatus != ont.Status {
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

		// Recorded on every cycle rather than only when the status differs.
		// LogStatusChange is idempotent: it opens a baseline event for an ONT that
		// has none, returns without writing when the state is unchanged, and closes
		// the previous event's duration on a real transition.
		//
		// Guarding this on "status changed" would starve it, because an ONT
		// registered with the status the OLT already reports never changes - which
		// is every ONT on a newly added OLT. Availability needs that opening event
		// to have any interval to measure.
		if newStatus != "" {
			eventType := models.EventTypeOffline
			if newStatus == models.ONTStatusOnline {
				eventType = models.EventTypeOnline
			}
			if err := eventService.LogStatusChange(ont.ID, eventType, string(newStatus)); err != nil {
				logger.Error("Failed to log ONT status event",
					zap.String("serial", ont.SerialNumber), zap.Error(err))
			}
		}

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

		logger.Info("Collected metrics",
			zap.String("serial", ont.SerialNumber),
			zap.Float64p("rx_power", rxPower),
			zap.Float64p("tx_power", txPower))
	}

	logger.Info("Metrics collection cycle completed")
}

func updateOLTConnectionStatus(db *gorm.DB, oltID uuid.UUID, status models.OLTStatus, logger *zap.Logger) error {
	updates := map[string]interface{}{"status": status}
	if status == models.OLTStatusOnline {
		updates["last_seen"] = time.Now()
	}

	result := db.Model(&models.OLT{}).Where("id = ?", oltID).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		logger.Info("Updated OLT connection status", zap.String("olt_id", oltID.String()), zap.String("status", string(status)))
	}
	return nil
}

func lookupByPortAndONT[T any](entries map[connectivity.ONTLocation]T, portID, ontID int) (T, bool) {
	for loc, value := range entries {
		if loc.Port == portID && loc.ONTID == ontID {
			return value, true
		}
	}
	var zero T
	return zero, false
}

func discoverONTsForSync(olt models.OLT) ([]connectivity.DiscoveredONT, error) {
	driver, err := connectivity.DriverFor(olt.Model)
	if err != nil {
		return nil, err
	}

	topology, err := connectivity.DiscoverOLTTopology(driver, olt.IPAddress, olt.SNMPCommunity, olt.SNMPPort)
	if err != nil {
		return nil, err
	}

	discovered := make([]connectivity.DiscoveredONT, 0)
	for _, slot := range topology {
		for _, port := range slot.Ports {
			discovered = append(discovered, port.ONTs...)
		}
	}
	return discovered, nil
}

func mapRunStateToStatus(runState int) models.ONTStatus {
	switch runState {
	case 3:
		return models.ONTStatusOnline
	case 6:
		return models.ONTStatusOffline
	case 1:
		return models.ONTStatusLOS
	case 4:
		return models.ONTStatusDyingGas
	default:
		return models.ONTStatusUnknown
	}
}
