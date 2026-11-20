package services

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// Switch tier schedules. A port's traffic moves by the minute; its VLANs and
// neighbours change when someone works on the switch; its config is kept daily.
const (
	SwitchInterfacesInterval = time.Minute
	SwitchInventoryInterval  = 30 * time.Minute
	SwitchBackupInterval     = 24 * time.Hour
)

// Leases: the SNMP walk takes seconds; an inventory or backup reads the whole
// config over a CLI and can take minutes on a slow login.
const (
	switchPollLease = 2 * time.Minute
	switchLongLease = 5 * time.Minute
)

var switchLongLeaseKinds = []models.SwitchPollKind{models.SwitchPollKindInventory, models.SwitchPollKindBackup}

// SwitchPollJobService hands out one switch's work to one worker at a time.
type SwitchPollJobService struct {
	db *gorm.DB
}

// NewSwitchPollJobService creates a switch poll job service.
func NewSwitchPollJobService(db *gorm.DB) *SwitchPollJobService {
	return &SwitchPollJobService{db: db}
}

// SwitchIntervalFor returns how long after a successful run the tier is due
// again.
func SwitchIntervalFor(kind models.SwitchPollKind) time.Duration {
	switch kind {
	case models.SwitchPollKindInterfaces:
		return SwitchInterfacesInterval
	case models.SwitchPollKindInventory:
		return SwitchInventoryInterval
	default:
		return SwitchBackupInterval
	}
}

// Claim takes the most overdue job no other worker holds, or nil when nothing
// is due.
func (s *SwitchPollJobService) Claim(workerID string) (*models.SwitchPollJob, error) {
	var claimed models.SwitchPollJob
	err := s.db.Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		query := tx.Where("due_at <= ?", now).
			Where("locked_at IS NULL OR (kind IN ? AND locked_at < ?) OR (kind NOT IN ? AND locked_at < ?)",
				switchLongLeaseKinds, now.Add(-switchLongLease),
				switchLongLeaseKinds, now.Add(-switchPollLease)).
			Order("due_at")
		if tx.Dialector.Name() == "postgres" {
			query = query.Clauses(lockingSkipLocked())
		}

		var candidate models.SwitchPollJob
		if err := query.First(&candidate).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.SwitchPollJob{}).
			Where("switch_id = ? AND kind = ?", candidate.SwitchID, candidate.Kind).
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
func (s *SwitchPollJobService) Complete(job *models.SwitchPollJob, took time.Duration) error {
	now := time.Now()
	return s.release(job, map[string]interface{}{
		"due_at":               now.Add(SwitchIntervalFor(job.Kind)),
		"last_run_at":          now,
		"last_duration_ms":     took.Milliseconds(),
		"last_error":           nil,
		"consecutive_failures": 0,
	})
}

// Fail releases the job and backs its next run off, so a switch that is down
// is not retried every minute for as long as it stays down.
func (s *SwitchPollJobService) Fail(job *models.SwitchPollJob, cause error) error {
	failures := job.ConsecutiveFailures + 1
	now := time.Now()
	return s.release(job, map[string]interface{}{
		"due_at":               now.Add(switchBackoff(job.Kind, failures)),
		"last_run_at":          now,
		"last_error":           cause.Error(),
		"consecutive_failures": failures,
	})
}

func (s *SwitchPollJobService) release(job *models.SwitchPollJob, updates map[string]interface{}) error {
	updates["locked_by"] = nil
	updates["locked_at"] = nil
	updates["manual_requested"] = false
	return s.db.Model(&models.SwitchPollJob{}).
		Where("switch_id = ? AND kind = ?", job.SwitchID, job.Kind).
		Updates(updates).Error
}

// switchBackoff doubles the interval per failure up to four intervals, as
// routers do, for the same reason: an operator watching a switch come back
// must see it within minutes.
func switchBackoff(kind models.SwitchPollKind, failures int) time.Duration {
	shift := failures - 1
	if shift > maxRouterBackoffShift {
		shift = maxRouterBackoffShift
	}
	return SwitchIntervalFor(kind) * time.Duration(1<<shift)
}

// RequestBackup makes the backup job due now and marks the backup it takes
// as manual.
func (s *SwitchPollJobService) RequestBackup(switchID uuid.UUID) error {
	return s.db.Model(&models.SwitchPollJob{}).
		Where("switch_id = ? AND kind = ?", switchID, models.SwitchPollKindBackup).
		Updates(map[string]interface{}{"due_at": time.Now(), "manual_requested": true}).Error
}

// RunNow makes the given tiers due immediately. It schedules rather than runs,
// so an HTTP request never waits on a switch.
func (s *SwitchPollJobService) RunNow(switchID uuid.UUID, kinds ...models.SwitchPollKind) error {
	return s.db.Model(&models.SwitchPollJob{}).
		Where("switch_id = ? AND kind IN ?", switchID, kinds).
		Update("due_at", time.Now()).Error
}
