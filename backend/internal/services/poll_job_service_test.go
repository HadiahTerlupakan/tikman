package services

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

func jobFor(t *testing.T, db *gorm.DB, oltID uuid.UUID, kind models.PollKind) models.OLTPollJob {
	t.Helper()
	var job models.OLTPollJob
	require.NoError(t, db.Where("olt_id = ? AND kind = ?", oltID, kind).First(&job).Error)
	return job
}

func setDue(t *testing.T, db *gorm.DB, oltID uuid.UUID, kind models.PollKind, at time.Time) {
	t.Helper()
	require.NoError(t, db.Model(&models.OLTPollJob{}).
		Where("olt_id = ? AND kind = ?", oltID, kind).
		Update("due_at", at).Error)
}

func TestEnsureJobsGivesEveryOLTOneRowPerTier(t *testing.T) {
	db := setupTestDB(t)
	service := NewPollJobService(db)
	oltForPositions(t, db, "Cariu", "172.30.30.3")
	oltForPositions(t, db, "Bekasi", "172.30.30.2")

	require.NoError(t, service.EnsureJobs())

	var count int64
	require.NoError(t, db.Model(&models.OLTPollJob{}).Count(&count).Error)
	require.Equal(t, int64(6), count)
}

func TestEnsureJobsRunsAgainWithoutDuplicating(t *testing.T) {
	// It runs on every worker pass so an OLT added while nothing was running
	// still gets picked up. That only works if repeating it is free.
	db := setupTestDB(t)
	service := NewPollJobService(db)
	oltID := oltForPositions(t, db, "Cariu", "172.30.30.3")

	require.NoError(t, service.EnsureJobs())
	setDue(t, db, oltID, models.PollKindStatus, time.Now().Add(time.Hour))
	require.NoError(t, service.EnsureJobs())

	var count int64
	require.NoError(t, db.Model(&models.OLTPollJob{}).Count(&count).Error)
	require.Equal(t, int64(3), count)

	job := jobFor(t, db, oltID, models.PollKindStatus)
	require.True(t, job.DueAt.After(time.Now().Add(30*time.Minute)),
		"a second pass reset a schedule it should have left alone")
}

func TestClaimTakesTheMostOverdueJob(t *testing.T) {
	db := setupTestDB(t)
	service := NewPollJobService(db)
	oltID := oltForPositions(t, db, "Cariu", "172.30.30.3")
	require.NoError(t, service.EnsureJobs())

	setDue(t, db, oltID, models.PollKindStatus, time.Now().Add(-time.Minute))
	setDue(t, db, oltID, models.PollKindMetrics, time.Now().Add(-time.Hour))
	setDue(t, db, oltID, models.PollKindDiscovery, time.Now().Add(time.Hour))

	claimed, err := service.Claim("worker-1")
	require.NoError(t, err)
	require.NotNil(t, claimed)
	require.Equal(t, models.PollKindMetrics, claimed.Kind)
}

func TestClaimReturnsNothingWhenNoJobIsDue(t *testing.T) {
	db := setupTestDB(t)
	service := NewPollJobService(db)
	oltID := oltForPositions(t, db, "Cariu", "172.30.30.3")
	require.NoError(t, service.EnsureJobs())

	for _, kind := range AllPollKinds {
		setDue(t, db, oltID, kind, time.Now().Add(time.Hour))
	}

	claimed, err := service.Claim("worker-1")
	require.NoError(t, err)
	require.Nil(t, claimed, "a job not yet due was handed out")
}

func TestClaimLeavesAJobAnotherWorkerHolds(t *testing.T) {
	// Two workers on one chassis is what the claim exists to prevent: a ZTE
	// agent serves one reader far better than two.
	db := setupTestDB(t)
	service := NewPollJobService(db)
	oltID := oltForPositions(t, db, "Cariu", "172.30.30.3")
	require.NoError(t, service.EnsureJobs())
	for _, kind := range AllPollKinds {
		setDue(t, db, oltID, kind, time.Now().Add(time.Hour))
	}
	setDue(t, db, oltID, models.PollKindStatus, time.Now().Add(-time.Minute))

	first, err := service.Claim("worker-1")
	require.NoError(t, err)
	require.NotNil(t, first)

	second, err := service.Claim("worker-2")
	require.NoError(t, err)
	require.Nil(t, second, "the same job was handed to two workers")
}

func TestClaimRecoversAJobWhoseHolderDisappeared(t *testing.T) {
	// A worker that dies mid-job leaves its row locked. Nothing else releases
	// it, so the lease expiring is what stops one crash stranding an OLT.
	db := setupTestDB(t)
	service := NewPollJobService(db)
	oltID := oltForPositions(t, db, "Cariu", "172.30.30.3")
	require.NoError(t, service.EnsureJobs())
	for _, kind := range AllPollKinds {
		setDue(t, db, oltID, kind, time.Now().Add(time.Hour))
	}

	dead := "worker-that-died"
	longGone := time.Now().Add(-LeaseFor(models.PollKindStatus) - time.Minute)
	require.NoError(t, db.Model(&models.OLTPollJob{}).
		Where("olt_id = ? AND kind = ?", oltID, models.PollKindStatus).
		Updates(map[string]interface{}{
			"due_at": time.Now().Add(-time.Minute), "locked_by": dead, "locked_at": longGone,
		}).Error)

	claimed, err := service.Claim("worker-2")
	require.NoError(t, err)
	require.NotNil(t, claimed, "an expired lease still blocked the job")
	require.Equal(t, models.PollKindStatus, claimed.Kind)
}

func TestCompleteSchedulesTheTiersOwnInterval(t *testing.T) {
	db := setupTestDB(t)
	service := NewPollJobService(db)
	oltID := oltForPositions(t, db, "Cariu", "172.30.30.3")
	require.NoError(t, service.EnsureJobs())

	job := jobFor(t, db, oltID, models.PollKindDiscovery)
	require.NoError(t, service.Complete(&job, 12*time.Second))

	stored := jobFor(t, db, oltID, models.PollKindDiscovery)
	require.Nil(t, stored.LockedBy)
	require.Nil(t, stored.LockedAt)
	require.Zero(t, stored.ConsecutiveFailures)
	require.NotNil(t, stored.LastDurationMs)
	require.Equal(t, int64(12000), *stored.LastDurationMs)
	require.WithinDuration(t, time.Now().Add(DiscoveryInterval), stored.DueAt, time.Minute)
}

func TestFailBacksOffFurtherEachTime(t *testing.T) {
	// An OLT that is down should not be retried every minute for as long as it
	// stays down: that is a wasted read per minute per chassis, and it crowds
	// out the OLTs that are answering.
	db := setupTestDB(t)
	service := NewPollJobService(db)
	oltID := oltForPositions(t, db, "Cariu", "172.30.30.3")
	require.NoError(t, service.EnsureJobs())

	job := jobFor(t, db, oltID, models.PollKindStatus)
	require.NoError(t, service.Fail(&job, errors.New("unreachable")))

	first := jobFor(t, db, oltID, models.PollKindStatus)
	require.Equal(t, 1, first.ConsecutiveFailures)
	require.WithinDuration(t, time.Now().Add(StatusInterval), first.DueAt, 30*time.Second)

	require.NoError(t, service.Fail(&first, errors.New("unreachable")))
	second := jobFor(t, db, oltID, models.PollKindStatus)
	require.Equal(t, 2, second.ConsecutiveFailures)
	require.WithinDuration(t, time.Now().Add(2*StatusInterval), second.DueAt, 30*time.Second)

	require.NotNil(t, second.LastError)
	require.Equal(t, "unreachable", *second.LastError)
}

func TestBackoffStopsGrowingAtTheCap(t *testing.T) {
	// Without a cap an OLT down for a week drifts to a retry interval measured
	// in days, and never comes back on its own once the link is fixed.
	require.Equal(t, StatusInterval*32, backoff(models.PollKindStatus, 6))
	require.Equal(t, StatusInterval*32, backoff(models.PollKindStatus, 40))
}

func TestDeferDoesNotCountAsAFailure(t *testing.T) {
	// A tunnel that is down, or another reader holding the chassis, is a reason
	// to wait — not evidence that the OLT is broken. Counting it would back the
	// OLT off for hours over a transient.
	db := setupTestDB(t)
	service := NewPollJobService(db)
	oltID := oltForPositions(t, db, "Cariu", "172.30.30.3")
	require.NoError(t, service.EnsureJobs())

	job := jobFor(t, db, oltID, models.PollKindStatus)
	require.NoError(t, service.Fail(&job, errors.New("real failure")))
	failed := jobFor(t, db, oltID, models.PollKindStatus)

	require.NoError(t, service.Defer(&failed))

	stored := jobFor(t, db, oltID, models.PollKindStatus)
	require.Equal(t, 1, stored.ConsecutiveFailures, "deferring reset or advanced the failure count")
	require.Nil(t, stored.LockedBy)
	require.WithinDuration(t, time.Now().Add(StatusInterval), stored.DueAt, 30*time.Second)
}
