package main

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// idleWait is how long a worker sleeps when nothing is due. Short enough that a
// job coming due is picked up promptly, long enough that an idle worker is not
// polling the database in a tight loop.
const idleWait = 5 * time.Second

// errorWait is how long a worker sleeps after the queue itself failed, so a
// database that is down is not hammered by every worker at once.
const errorWait = 30 * time.Second

// workerRuntime is everything a job needs to run. It is assembled once at
// startup and passed to each job rather than threaded through as eight
// arguments.
type workerRuntime struct {
	db      *gorm.DB
	id      string
	jobs    *services.PollJobService
	onts    *services.ONTService
	olts    *services.OLTService
	metrics *services.MetricsService
	events  *services.EventService
	logger  *zap.Logger
}

// newWorkerID names this process in the queue. The hostname alone is not
// enough: several workers run on one host, and two of them sharing a name would
// make one's lease look like the other's own.
func newWorkerID() string {
	host, err := os.Hostname()
	if err != nil {
		host = "worker"
	}
	return fmt.Sprintf("%s-%d-%s", host, os.Getpid(), uuid.New().String()[:8])
}

// runNextJob claims one job and runs it, reporting whether there was work.
//
// This replaced a ticker that swept every OLT on one schedule, doing a full
// discovery walk of each chassis every minute. Measured here, that second walk
// cost as much as the poll beside it — Depok spent 38 seconds reading and
// another 36 rediscovering, every minute, to learn what changes a few times a
// day.
func runNextJob(rt *workerRuntime) bool {
	job, err := rt.jobs.Claim(rt.id)
	if err != nil {
		rt.logger.Error("Failed to claim a poll job", zap.Error(err))
		time.Sleep(errorWait)
		return true
	}
	if job == nil {
		return false
	}

	var olt models.OLT
	if err := rt.db.First(&olt, "id = ?", job.OLTID).Error; err != nil {
		// The OLT is gone. Its jobs go with it by foreign key, so this is a race
		// with a deletion rather than a fault worth backing off over.
		rt.logger.Info("Skipping job for an OLT that no longer exists", zap.String("olt_id", job.OLTID.String()))
		_ = rt.jobs.Defer(job)
		return true
	}

	started := time.Now()
	if err := runJob(rt, job, olt); err != nil {
		handleJobError(rt, job, olt, err)
		return true
	}

	if err := rt.jobs.Complete(job, time.Since(started)); err != nil {
		rt.logger.Error("Failed to record a job completion", zap.Error(err))
	}
	return true
}

// handleJobError decides whether a job failed or merely could not be attempted.
//
// Work that could not be attempted is not evidence that the OLT is broken.
// Counting it would back a healthy chassis off for hours over a reader that
// happened to be holding it, or a tunnel that was down for a minute.
func handleJobError(rt *workerRuntime, job *models.OLTPollJob, olt models.OLT, err error) {
	var deferred deferredError
	if errors.As(err, &deferred) {
		rt.logger.Info("Deferring job",
			zap.String("olt", olt.Name), zap.String("kind", string(job.Kind)),
			zap.String("reason", deferred.reason))
		if deferErr := rt.jobs.Defer(job); deferErr != nil {
			rt.logger.Error("Failed to defer a job", zap.Error(deferErr))
		}
		return
	}

	rt.logger.Warn("Poll job failed",
		zap.String("olt", olt.Name), zap.String("kind", string(job.Kind)), zap.Error(err))
	if failErr := rt.jobs.Fail(job, err); failErr != nil {
		rt.logger.Error("Failed to record a job failure", zap.Error(failErr))
	}
}

// deferredError marks work that could not be attempted, as opposed to work that
// was attempted and failed. A tunnel that is down is a reason to wait, not
// evidence that the OLT is broken, and counting it would back the OLT off for
// hours over a transient.
type deferredError struct{ reason string }

func (d deferredError) Error() string { return d.reason }

func runJob(rt *workerRuntime, job *models.OLTPollJob, olt models.OLT) error {
	if blockedOLTs := oltsBehindDownTunnel(rt.db, time.Now(), rt.logger); blockedOLTs[olt.ID] {
		return deferredError{reason: "site tunnel is down"}
	}

	switch job.Kind {
	case models.PollKindStatus:
		return runStatusJob(rt, olt)
	case models.PollKindMetrics:
		return runMetricsJob(rt, olt)
	case models.PollKindDiscovery:
		return runDiscoveryJob(rt, olt)
	default:
		return fmt.Errorf("unknown poll kind %q", job.Kind)
	}
}
