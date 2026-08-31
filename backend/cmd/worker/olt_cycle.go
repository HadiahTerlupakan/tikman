package main

import (
	"fmt"
	"time"

	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"go.uber.org/zap"
)

// ontPageSize is how many ONTs one page of the pass carries. The pass holds a
// page at a time rather than the whole chassis, so memory does not grow with
// the subscriber count.
const ontPageSize = 1000

// runStatusJob reads the one table that says whether a subscriber is up.
//
// This is the tier that has to be fresh, and it is the cheapest: one walk, and
// a write only where a status actually changed. Nothing goes to ont_metrics — a
// row per ONT per minute would be hundreds of thousands of rows an hour
// recording that optical power had not moved.
func runStatusJob(rt *workerRuntime, olt models.OLT) error {
	reading, err := readStatuses(rt.db, olt, rt.logger)
	if err != nil {
		return err
	}
	return applyReading(rt, olt, reading, models.PollKindStatus)
}

// runMetricsJob reads optical power and traffic counters.
//
// It reads statuses too, because the metrics tables are addressed by the
// locations the status walk names. Having both, it settles status the way the
// old single cycle did, including the fall back to optical power for an ONT the
// status table does not list.
func runMetricsJob(rt *workerRuntime, olt models.OLT) error {
	reading, err := readFull(rt.db, olt, rt.logger)
	if err != nil {
		return err
	}
	return applyReading(rt, olt, reading, models.PollKindMetrics)
}

// runDiscoveryJob finds ONUs added to or removed from the chassis.
//
// It used to run on every cycle for every OLT, at the cost of a second full
// walk each time. ONUs are added and removed a few times a day, so it now has
// its own hourly schedule.
func runDiscoveryJob(rt *workerRuntime, olt models.OLT) error {
	started := time.Now()

	if !syncOntsWithDiscovery(olt, rt.onts, rt.logger) {
		return fmt.Errorf("discovery sync failed")
	}

	// The richer walk caches VLANs, chassis entities, service configs and ONU
	// types, and publishes the progress the OLT page shows. It used to be
	// launched on every cycle for every OLT and left to run unattended; here it
	// runs inside the job's lease, so the next tier waits for it rather than
	// competing with it for the agent.
	rt.olts.AutoDiscoverONTMetrics(&olt)

	rt.logger.Info("Discovered OLT",
		zap.String("olt", olt.Name),
		zap.Duration("took", time.Since(started)))
	return nil
}

// applyReading walks every ONT of the OLT and applies what was read.
//
// What gets written follows from what was read rather than from a flag: a
// reading with no metrics writes no metric rows, which is what makes the status
// tier cheap enough to run every minute.
func applyReading(rt *workerRuntime, olt models.OLT, reading *oltReading, kind models.PollKind) error {
	started := time.Now()
	processed, skipped := 0, 0

	err := rt.onts.EachONTOfOLT(olt.ID, ontPageSize, func(rows []models.ONT) error {
		samples := make([]services.MetricSample, 0, len(rows))
		changes := make([]services.StatusChange, 0, len(rows))

		for _, ont := range rows {
			// An ONT the status walk did not name is one the OLT no longer
			// reports at that position. Only trust that when the walk succeeded:
			// a failed walk names nothing, and acting on it would mark every
			// subscriber offline at once.
			if reading.statusWalkOK && !reading.reportsPosition(ont.PortID, ont.ONTID) {
				skipped++
				continue
			}
			sample, change := processOnt(rt, reading, ont)
			if reading.metrics != nil {
				samples = append(samples, sample)
			}
			if change.EventType != "" {
				changes = append(changes, change)
			}
			processed++
		}

		// One read of the event history for the whole page. A failure is logged
		// rather than returned for the same reason the metrics write is: the
		// statuses are already on the ONT rows, and the next page is still worth
		// taking.
		if err := rt.events.LogStatusChanges(changes); err != nil {
			rt.logger.Error("Failed to record ONT status events",
				zap.String("olt", olt.Name), zap.Int("changes", len(changes)), zap.Error(err))
		}

		// A write failure loses this page's readings, and the operator has to
		// hear that. It does not abandon the OLT: the status changes and events
		// for these ONTs are already recorded, and the next page is still worth
		// taking.
		if err := rt.metrics.StoreMetricsBatch(samples); err != nil {
			rt.logger.Error("Failed to store metrics page",
				zap.String("olt", olt.Name), zap.Int("samples", len(samples)), zap.Error(err))
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("read ONTs: %w", err)
	}

	rt.logger.Info("Polled OLT",
		zap.String("olt", olt.Name),
		zap.String("kind", string(kind)),
		zap.Int("onts", processed),
		zap.Int("not_reported", skipped),
		zap.Duration("took", time.Since(started)))
	return nil
}
