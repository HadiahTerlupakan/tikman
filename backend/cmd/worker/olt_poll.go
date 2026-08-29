package main

import (
	"errors"

	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// getOrInitOLT loads the ONT's OLT and, on first sight of that OLT, walks its
// SNMP data into the caches. Returns ok=false when the OLT cannot be loaded or
// has no usable driver, so the caller skips this ONT.
func getOrInitOLT(db *gorm.DB, ont models.ONT, oltMetricsCache *map[string]map[connectivity.ONTLocation]connectivity.ONTMetrics, oltStatusCache *map[string]map[connectivity.ONTLocation]int, oltStatusWalkOK *map[string]bool, oltRatesCache *map[string]map[connectivity.ONTLocation]connectivity.ONUTrafficRates, logger *zap.Logger) (string, *models.OLT, bool) {
	var olt models.OLT
	if err := db.First(&olt, "id = ?", ont.OLTID).Error; err != nil {
		logger.Error("Failed to get OLT", zap.String("ont_id", ont.ID.String()), zap.Error(err))
		return "", nil, false
	}

	oltKey := olt.ID.String()

	if _, exists := (*oltMetricsCache)[oltKey]; !exists {
		if !initOLTCaches(db, olt, oltKey, oltMetricsCache, oltStatusCache, oltStatusWalkOK, oltRatesCache, logger) {
			return "", nil, false
		}
	}

	return oltKey, &olt, true
}

// initOLTCaches walks metrics, statuses, and traffic rates for one OLT into the
// per-OLT caches and records its online/offline connection status. Returns
// false when no driver exists for the OLT model.
func initOLTCaches(db *gorm.DB, olt models.OLT, oltKey string, oltMetricsCache *map[string]map[connectivity.ONTLocation]connectivity.ONTMetrics, oltStatusCache *map[string]map[connectivity.ONTLocation]int, oltStatusWalkOK *map[string]bool, oltRatesCache *map[string]map[connectivity.ONTLocation]connectivity.ONUTrafficRates, logger *zap.Logger) bool {
	driver, err := connectivity.DriverFor(olt.Model)
	if err != nil {
		logger.Error("Cannot poll OLT", zap.String("olt", olt.Name), zap.Error(err))
		return false
	}

	// The discovery poll runs alongside this and walks the same tables. Taking
	// turns rather than competing: it stores the same metrics, so a cycle that
	// stands down loses nothing.
	// Nothing is cached on the way out. getOrInitOLT treats a populated cache
	// as "this OLT was read this cycle", so seeding it with empty maps here
	// skipped only the first ONT and then handed the other 199 an empty status
	// map, which read as every subscriber having gone offline at once.
	release, free := services.TryLockOLTSNMP(olt.ID)
	if !free {
		logger.Info("Skipping OLT this cycle: discovery is reading it",
			zap.String("olt", olt.Name))
		return false
	}
	defer release()

	// Statuses first: they name every ONU the OLT has, which is what lets the
	// metrics be fetched by instance instead of swept.
	walkStatusesForOLT(db, driver, olt, oltKey, oltStatusCache, oltStatusWalkOK, logger)

	readMetricsForOLT(driver, olt, oltKey, (*oltStatusCache)[oltKey], oltMetricsCache, logger)

	walkRatesForOLT(driver, olt, oltKey, oltRatesCache, logger)

	return true
}

func readMetricsForOLT(driver connectivity.Driver, olt models.OLT, oltKey string, statuses map[connectivity.ONTLocation]int, oltMetricsCache *map[string]map[connectivity.ONTLocation]connectivity.ONTMetrics, logger *zap.Logger) {
	querier, direct := driver.(connectivity.MetricsQuerier)
	if direct && len(statuses) > 0 {
		locations := make([]connectivity.ONTLocation, 0, len(statuses))
		for loc := range statuses {
			locations = append(locations, loc)
		}
		metricsMap, err := querier.QueryMetricsFor(olt.IPAddress, olt.SNMPCommunity, olt.SNMPPort, locations)
		if err == nil {
			logger.Info("Read ONT metrics", zap.String("olt", olt.Name), zap.Int("count", len(metricsMap)))
			(*oltMetricsCache)[oltKey] = metricsMap
			return
		}
		logger.Error("Failed to read ONT metrics", zap.String("olt", olt.Name), zap.Error(err))
	}

	metricsMap, err := driver.WalkMetrics(olt.IPAddress, olt.SNMPCommunity, olt.SNMPPort)
	if err != nil {
		logger.Error("Failed to walk ONT metrics", zap.String("olt", olt.Name), zap.Error(err))
		(*oltMetricsCache)[oltKey] = make(map[connectivity.ONTLocation]connectivity.ONTMetrics)
		return
	}
	logger.Info("Walked ONT metrics", zap.String("olt", olt.Name), zap.Int("count", len(metricsMap)))
	(*oltMetricsCache)[oltKey] = metricsMap
}

func walkStatusesForOLT(db *gorm.DB, driver connectivity.Driver, olt models.OLT, oltKey string, oltStatusCache *map[string]map[connectivity.ONTLocation]int, oltStatusWalkOK *map[string]bool, logger *zap.Logger) {
	statuses, err := driver.WalkStatuses(olt.IPAddress, olt.SNMPCommunity, olt.SNMPPort)
	if err != nil {
		logger.Error("Failed to walk ONT statuses", zap.String("olt", olt.Name), zap.Error(err))
		(*oltStatusCache)[oltKey] = make(map[connectivity.ONTLocation]int)
		if err := updateOLTConnectionStatus(db, olt.ID, models.OLTStatusOffline, logger); err != nil {
			logger.Error("Failed to update OLT status", zap.String("olt", olt.Name), zap.Error(err))
		}
		return
	}

	logger.Info("Walked ONT statuses", zap.String("olt", olt.Name), zap.Int("count", len(statuses)))
	(*oltStatusCache)[oltKey] = statuses
	(*oltStatusWalkOK)[oltKey] = true
	if err := updateOLTConnectionStatus(db, olt.ID, models.OLTStatusOnline, logger); err != nil {
		logger.Error("Failed to update OLT status", zap.String("olt", olt.Name), zap.Error(err))
	}
}

func walkRatesForOLT(driver connectivity.Driver, olt models.OLT, oltKey string, oltRatesCache *map[string]map[connectivity.ONTLocation]connectivity.ONUTrafficRates, logger *zap.Logger) {
	rates, err := driver.WalkTrafficRates(olt.IPAddress, olt.SNMPCommunity, olt.SNMPPort)
	switch {
	case errors.Is(err, connectivity.ErrUnsupported):
		// A model with no known rate OIDs is not a fault; rates stay unset.
		logger.Info("Traffic rate gauges unsupported", zap.String("olt", olt.Name), zap.String("model", string(olt.Model)))
		(*oltRatesCache)[oltKey] = make(map[connectivity.ONTLocation]connectivity.ONUTrafficRates)
	case err != nil:
		logger.Error("Failed to walk ONT traffic rates", zap.String("olt", olt.Name), zap.Error(err))
		(*oltRatesCache)[oltKey] = make(map[connectivity.ONTLocation]connectivity.ONUTrafficRates)
	default:
		logger.Info("Walked ONT traffic rates", zap.String("olt", olt.Name), zap.Int("count", len(rates)))
		(*oltRatesCache)[oltKey] = rates
	}
}

// syncOntsWithDiscovery prunes ONTs the OLT no longer reports and registers
// newly discovered ones. Returns false when discovery or pruning failed, so
// the caller skips the ONT and retries the sync on the next ONT of this OLT.
