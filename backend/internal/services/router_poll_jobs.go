package services

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// Router tier schedules. Unlike an OLT chassis, a router answers a full IF-MIB
// read and a BGP listing in well under a second, so these are fixed rather than
// stretched to the device's capacity.
const (
	RouterInterfacesInterval = time.Minute
	RouterBGPInterval        = time.Minute
	RouterDiscoveryInterval  = 6 * time.Hour
)

// Leases: an order of magnitude over what each tier needs, short enough that a
// killed worker's router is picked up again within minutes.
const (
	routerPollLease      = 2 * time.Minute
	routerDiscoveryLease = 5 * time.Minute
)

// AllRouterPollKinds is every router tier, discovery first so a new router has
// interfaces before its first counter poll.
var AllRouterPollKinds = []models.RouterPollKind{
	models.RouterPollKindDiscovery,
	models.RouterPollKindInterfaces,
	models.RouterPollKindBGP,
}

// RouterPollJobService hands out one router's work to one worker at a time.
type RouterPollJobService struct {
	db *gorm.DB
}

// NewRouterPollJobService creates a router poll job service.
func NewRouterPollJobService(db *gorm.DB) *RouterPollJobService {
	return &RouterPollJobService{db: db}
}

// RouterIntervalFor returns how long after a successful run the tier is due
// again.
func RouterIntervalFor(kind models.RouterPollKind) time.Duration {
	switch kind {
	case models.RouterPollKindInterfaces:
		return RouterInterfacesInterval
	case models.RouterPollKindBGP:
		return RouterBGPInterval
	default:
		return RouterDiscoveryInterval
	}
}

// Claim takes the most overdue job no other worker holds, or nil when nothing
// is due.
func (s *RouterPollJobService) Claim(workerID string) (*models.RouterPollJob, error) {
	var claimed models.RouterPollJob
	err := s.db.Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		query := tx.Where("due_at <= ?", now).
			Where("locked_at IS NULL OR (kind = ? AND locked_at < ?) OR (kind <> ? AND locked_at < ?)",
				models.RouterPollKindDiscovery, now.Add(-routerDiscoveryLease),
				models.RouterPollKindDiscovery, now.Add(-routerPollLease)).
			Order("due_at")
		if tx.Dialector.Name() == "postgres" {
			query = query.Clauses(lockingSkipLocked())
		}

		var candidate models.RouterPollJob
		if err := query.First(&candidate).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.RouterPollJob{}).
			Where("router_id = ? AND kind = ?", candidate.RouterID, candidate.Kind).
			Updates(map[string]interface{}{"locked_by": workerID, "locked_at": now}).Error; err != nil {
			return err
		}
		candidate.LockedBy, candidate.LockedAt = &workerID, &now
		claimed = candidate
		return nil
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &claimed, nil
}

// Complete releases the job and schedules its next run.
func (s *RouterPollJobService) Complete(job *models.RouterPollJob, took time.Duration) error {
	now := time.Now()
	return s.release(job, map[string]interface{}{
		"due_at":               now.Add(RouterIntervalFor(job.Kind)),
		"last_run_at":          now,
		"last_duration_ms":     took.Milliseconds(),
		"last_error":           nil,
		"consecutive_failures": 0,
	})
}

// Fail releases the job and backs its next run off, so a router that is down
// is not retried every minute for as long as it stays down.
func (s *RouterPollJobService) Fail(job *models.RouterPollJob, cause error) error {
	failures := job.ConsecutiveFailures + 1
	now := time.Now()
	return s.release(job, map[string]interface{}{
		"due_at":               now.Add(routerBackoff(job.Kind, failures)),
		"last_run_at":          now,
		"last_error":           cause.Error(),
		"consecutive_failures": failures,
	})
}

func (s *RouterPollJobService) release(job *models.RouterPollJob, updates map[string]interface{}) error {
	updates["locked_by"] = nil
	updates["locked_at"] = nil
	return s.db.Model(&models.RouterPollJob{}).
		Where("router_id = ? AND kind = ?", job.RouterID, job.Kind).
		Updates(updates).Error
}

// maxRouterBackoffShift caps router backoff at 4 intervals, far below the
// OLT's 32: a failed router poll costs at most a 10s timeout, and an operator
// watching an upstream recover must see it within minutes, not half an hour.
const maxRouterBackoffShift = 2

func routerBackoff(kind models.RouterPollKind, failures int) time.Duration {
	shift := failures - 1
	if shift > maxRouterBackoffShift {
		shift = maxRouterBackoffShift
	}
	return RouterIntervalFor(kind) * time.Duration(1<<shift)
}

// RunNow makes the given tiers due immediately. It schedules rather than runs,
// so an HTTP request never waits on a router.
func (s *RouterPollJobService) RunNow(routerID uuid.UUID, kinds ...models.RouterPollKind) error {
	return s.db.Model(&models.RouterPollJob{}).
		Where("router_id = ? AND kind IN ?", routerID, kinds).
		Update("due_at", time.Now()).Error
}
