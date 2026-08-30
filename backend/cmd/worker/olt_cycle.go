package main

import (
	"time"

	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ontPageSize is how many ONTs one page of the poll carries. The cycle holds a
// page at a time rather than the whole chassis, so memory does not grow with
// the subscriber count.
const ontPageSize = 1000

// pollOLT reads one OLT and applies that reading to every ONT it owns.
//
// The cycle used to fetch one page of at most 1000 ONTs across all OLTs and
// walk that, loading each ONT's OLT from the database as it went. That capped
// monitoring at 1000 subscribers without saying so, and cost one query per ONT
// to learn something already known once per OLT.
func pollOLT(
	db *gorm.DB,
	olt models.OLT,
	ontService *services.ONTService,
	metricsService *services.MetricsService,
	eventService *services.EventService,
	logger *zap.Logger,
) {
	started := time.Now()

	reading, ok := readOLT(db, olt, logger)
	if !ok {
		return
	}
	readDone := time.Now()

	// Pruning and registration run once per OLT, against the discovery walk. A
	// failure here leaves the stored ONTs alone rather than acting on a partial
	// view of the chassis.
	if reading.statusWalkOK && !syncOntsWithDiscovery(olt, ontService, logger) {
		return
	}
	syncDone := time.Now()

	processed, skipped := 0, 0
	err := ontService.EachONTOfOLT(olt.ID, ontPageSize, func(rows []models.ONT) error {
		samples := make([]services.MetricSample, 0, len(rows))
		for _, ont := range rows {
			// An ONT the status walk did not name is one the OLT no longer
			// reports at that position. Only trust that when the walk succeeded:
			// a failed walk names nothing, and acting on it would mark every
			// subscriber offline at once.
			if reading.statusWalkOK && !reading.reportsPosition(ont.PortID, ont.ONTID) {
				skipped++
				continue
			}
			samples = append(samples, processOnt(db, reading, ont, eventService, logger))
			processed++
		}

		// A write failure loses this page's readings, and the operator has to
		// hear that. It does not abandon the OLT: the status changes and events
		// for these ONTs are already recorded, and the next page's readings are
		// still worth taking.
		if err := metricsService.StoreMetricsBatch(samples); err != nil {
			logger.Error("Failed to store metrics page",
				zap.String("olt", olt.Name), zap.Int("samples", len(samples)), zap.Error(err))
		}
		return nil
	})
	if err != nil {
		logger.Error("Failed to read ONTs for OLT", zap.String("olt", olt.Name), zap.Error(err))
		return
	}

	// One line per OLT, not per ONT. The old logging wrote three lines for every
	// subscriber, which at any real size buries everything else in the log.
	// Split three ways because they scale differently and the whole point of
	// this work is knowing which one to attack next: the SNMP read is bounded by
	// the OLT's agent, the sync by a second walk of the same chassis, and the
	// ONT pass by database round trips per subscriber.
	logger.Info("Polled OLT",
		zap.String("olt", olt.Name),
		zap.Int("onts", processed),
		zap.Int("not_reported", skipped),
		zap.Duration("snmp", readDone.Sub(started)),
		zap.Duration("sync", syncDone.Sub(readDone)),
		zap.Duration("onts_pass", time.Since(syncDone)),
		zap.Duration("took", time.Since(started)))
}
