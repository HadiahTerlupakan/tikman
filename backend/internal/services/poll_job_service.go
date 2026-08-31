package services

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Tier schedules. Measured on this installation, a full read of one chassis
// costs tens of seconds and does not scale with subscriber count, so these are
// how often each tier is worth paying that.
const (
	// StatusInterval is what decides how stale a subscriber's up/down state can
	// be. It reads one table.
	StatusInterval = time.Minute
	// MetricsInterval covers optical power and traffic counters, which do not
	// move minute to minute.
	MetricsInterval = 10 * time.Minute
	// DiscoveryInterval covers ONUs added and removed. It used to run every
	// cycle against every OLT and cost as much as the poll beside it.
	DiscoveryInterval = 6 * time.Hour
)

// Lease durations. A worker that dies holds its row until the lease expires, so
// each has to outlast the work comfortably while still releasing an abandoned
// job promptly.
//
// Sized from measurement rather than guessed. The longest jobs observed on this
// installation are 3.4s for status, 38s for metrics, and 96s for discovery, so
// each lease carries roughly an order of magnitude of headroom. The first
// guesses were three to thirty times longer, and a worker killed mid-discovery
// stranded that OLT for the better part of an hour.
const (
	statusLease    = 2 * time.Minute
	metricsLease   = 5 * time.Minute
	discoveryLease = 15 * time.Minute
)

// agentShare bounds how much of a chassis's SNMP agent this system takes: a
// tier waits at least as long as its own last run took, twice over, so the
// agent spends at most half its time answering us.
const agentShare = 2

// SpacingFor returns how long after a run of the given duration the tier is due
// again.
//
// The tier's interval is a floor, not a promise. A ZTE agent serves about 140
// values a second — measured, and the ceiling is its own CPU rather than the
// network — so a chassis of ten thousand ONUs needs roughly 72 seconds to answer
// one status walk. Holding such a chassis to the 60-second status tier would
// poll it with no gap at all, leaving its agent nothing for the metrics and
// discovery tiers queued behind. The interval therefore follows what each
// chassis actually managed, which is what "derived from the capacity of each
// chassis" has to mean in code.
func SpacingFor(kind models.PollKind, took time.Duration) time.Duration {
	tier := IntervalFor(kind)
	if stretched := took * agentShare; stretched > tier {
		return stretched
	}
	return tier
}

// maxBackoffShift caps the exponential backoff at 32 intervals, so an OLT that
// has been unreachable for a week is still retried within a reasonable window
// rather than drifting to never.
const maxBackoffShift = 5

// PollJobService hands out one OLT's work to one worker at a time.
type PollJobService struct {
	db *gorm.DB
}

// NewPollJobService creates a new poll job service.
func NewPollJobService(db *gorm.DB) *PollJobService {
	return &PollJobService{db: db}
}

// IntervalFor returns how long after a successful run the tier is due again.
func IntervalFor(kind models.PollKind) time.Duration {
	switch kind {
	case models.PollKindStatus:
		return StatusInterval
	case models.PollKindMetrics:
		return MetricsInterval
	default:
		return DiscoveryInterval
	}
}

// LeaseFor returns how long a claim on the tier stays valid without progress.
func LeaseFor(kind models.PollKind) time.Duration {
	switch kind {
	case models.PollKindStatus:
		return statusLease
	case models.PollKindMetrics:
		return metricsLease
	default:
		return discoveryLease
	}
}

// AllPollKinds is every tier, in the order a fresh OLT should first run them.
var AllPollKinds = []models.PollKind{
	models.PollKindDiscovery,
	models.PollKindStatus,
	models.PollKindMetrics,
}

// EnsureJobs gives every OLT a row per tier.
//
// Called at the start of each worker pass rather than only when an OLT is
// created: an OLT added while no worker was running would otherwise never be
// polled, and nothing would report it missing.
func (s *PollJobService) EnsureJobs() error {
	var oltIDs []uuid.UUID
	if err := s.db.Model(&models.OLT{}).Pluck("id", &oltIDs).Error; err != nil {
		return fmt.Errorf("list OLTs: %w", err)
	}

	for _, oltID := range oltIDs {
		for _, kind := range AllPollKinds {
			job := models.OLTPollJob{OLTID: oltID, Kind: kind, DueAt: time.Now()}
			if err := s.db.Where("olt_id = ? AND kind = ?", oltID, kind).
				FirstOrCreate(&job).Error; err != nil {
				return fmt.Errorf("ensure %s job for %s: %w", kind, oltID, err)
			}
		}
	}
	return nil
}

// Claim takes the most overdue job no other worker holds, or returns nil when
// nothing is due.
//
// The claim is the coordination: there is no scheduler process and no leader to
// elect. It is also what keeps two workers off one chassis at a time, which
// matters because a ZTE agent serves one reader far better than two.
func (s *PollJobService) Claim(workerID string) (*models.OLTPollJob, error) {
	var claimed models.OLTPollJob

	err := s.db.Transaction(func(tx *gorm.DB) error {
		var candidate models.OLTPollJob
		now := time.Now()
		condition, args := claimable(now)
		query := tx.Where("due_at <= ?", now).Where(condition, args...).Order("due_at")

		// SKIP LOCKED is what lets several workers read this table at once and
		// each come away with a different row instead of queueing behind one
		// another. SQLite has no such clause and no concurrent writers to need
		// it, so the test database takes the same query without it.
		if tx.Dialector.Name() == "postgres" {
			query = query.Clauses(lockingSkipLocked())
		}

		if err := query.First(&candidate).Error; err != nil {
			return err
		}

		candidate.LockedBy = &workerID
		candidate.LockedAt = &now
		if err := tx.Model(&models.OLTPollJob{}).
			Where("olt_id = ? AND kind = ?", candidate.OLTID, candidate.Kind).
			Updates(map[string]interface{}{"locked_by": workerID, "locked_at": now}).Error; err != nil {
			return err
		}

		claimed = candidate
		return nil
	})

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &claimed, nil
}

// claimable builds the condition for "no live lease holds this row".
//
// It has to be part of the query, not a check on what the query returned.
// Selecting the most overdue row and then rejecting it for being held made a
// worker give up while other jobs sat waiting: against a real Postgres, eight
// workers with eight jobs available came away with five.
//
// The thresholds are derived from LeaseFor rather than written into the SQL, so
// changing a lease cannot leave the query disagreeing with the code that
// enforces it.
func claimable(now time.Time) (string, []interface{}) {
	conditions := make([]string, 0, len(AllPollKinds))
	args := make([]interface{}, 0, len(AllPollKinds)*2)
	for _, kind := range AllPollKinds {
		conditions = append(conditions, "(kind = ? AND locked_at < ?)")
		args = append(args, kind, now.Add(-LeaseFor(kind)))
	}
	return "locked_at IS NULL OR " + strings.Join(conditions, " OR "), args
}

// ErrJobRunning reports that a tier is already being run for this OLT.
var ErrJobRunning = errors.New("poll job is already running")

// RunNow brings a tier's next run forward to immediately.
//
// It schedules rather than runs. Discovery against a populated chassis takes
// minutes — over six on the largest here — so an endpoint that performed it
// would hold an HTTP request open past every timeout between the browser and
// the worker. Making the job due instead hands it to the queue, which already
// has the lease that keeps two readers off one SNMP agent and already publishes
// the progress the OLT page draws.
//
// A tier a worker currently holds is left alone: the caller is asking for the
// pass that is already happening.
func (s *PollJobService) RunNow(oltID uuid.UUID, kind models.PollKind) error {
	var job models.OLTPollJob
	if err := s.db.Where("olt_id = ? AND kind = ?", oltID, kind).First(&job).Error; err != nil {
		return err
	}

	if job.LockedAt != nil && time.Since(*job.LockedAt) < LeaseFor(kind) {
		return ErrJobRunning
	}

	return s.db.Model(&models.OLTPollJob{}).
		Where("olt_id = ? AND kind = ?", oltID, kind).
		Update("due_at", time.Now()).Error
}

// Complete releases the job and schedules its next run.
func (s *PollJobService) Complete(job *models.OLTPollJob, took time.Duration) error {
	now := time.Now()
	ms := took.Milliseconds()
	return s.db.Model(&models.OLTPollJob{}).
		Where("olt_id = ? AND kind = ?", job.OLTID, job.Kind).
		Updates(map[string]interface{}{
			"due_at":               now.Add(SpacingFor(job.Kind, took)),
			"locked_by":            nil,
			"locked_at":            nil,
			"last_run_at":          now,
			"last_duration_ms":     ms,
			"last_error":           nil,
			"consecutive_failures": 0,
		}).Error
}

// Fail releases the job and backs its next run off, so an OLT that is down is
// not retried every minute for as long as it stays down.
func (s *PollJobService) Fail(job *models.OLTPollJob, cause error) error {
	failures := job.ConsecutiveFailures + 1
	now := time.Now()

	return s.db.Model(&models.OLTPollJob{}).
		Where("olt_id = ? AND kind = ?", job.OLTID, job.Kind).
		Updates(map[string]interface{}{
			"due_at":               now.Add(backoff(job.Kind, failures)),
			"locked_by":            nil,
			"locked_at":            nil,
			"last_run_at":          now,
			"last_error":           cause.Error(),
			"consecutive_failures": failures,
		}).Error
}

// backoff doubles the interval per consecutive failure, up to a cap.
func backoff(kind models.PollKind, failures int) time.Duration {
	shift := failures - 1
	if shift > maxBackoffShift {
		shift = maxBackoffShift
	}
	return IntervalFor(kind) * time.Duration(1<<shift)
}

// Defer pushes a job out by one interval without counting it as a failure.
// Used when the work could not be attempted at all — a tunnel that is down, or
// another reader holding the chassis — which is a reason to wait, not evidence
// that the OLT is broken.
func (s *PollJobService) Defer(job *models.OLTPollJob) error {
	return s.db.Model(&models.OLTPollJob{}).
		Where("olt_id = ? AND kind = ?", job.OLTID, job.Kind).
		Updates(map[string]interface{}{
			"due_at":    time.Now().Add(IntervalFor(job.Kind)),
			"locked_by": nil,
			"locked_at": nil,
		}).Error
}

// lockingSkipLocked is the FOR UPDATE SKIP LOCKED clause, kept in one place so
// the dialect check above reads as a decision rather than as SQL.
func lockingSkipLocked() clause.Locking {
	return clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}
}
