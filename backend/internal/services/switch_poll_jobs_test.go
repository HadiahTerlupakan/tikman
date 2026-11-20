package services

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

func seedSwitchJob(t *testing.T, service *SwitchPollJobService, kind models.SwitchPollKind, due time.Time) uuid.UUID {
	t.Helper()
	id := uuid.New()
	require.NoError(t, service.db.Create(&models.SwitchPollJob{SwitchID: id, Kind: kind, DueAt: due}).Error)
	return id
}

func TestSwitchClaimTakesTheMostOverdueJobOnce(t *testing.T) {
	service := NewSwitchPollJobService(setupTestDB(t))
	seedSwitchJob(t, service, models.SwitchPollKindInterfaces, time.Now().Add(-time.Minute))
	oldest := seedSwitchJob(t, service, models.SwitchPollKindBackup, time.Now().Add(-time.Hour))
	seedSwitchJob(t, service, models.SwitchPollKindInventory, time.Now().Add(time.Hour))

	job, err := service.Claim("worker-a")
	require.NoError(t, err)
	require.NotNil(t, job)
	assert.Equal(t, oldest, job.SwitchID)

	next, err := service.Claim("worker-b")
	require.NoError(t, err)
	require.NotNil(t, next)
	assert.NotEqual(t, oldest, next.SwitchID, "a held job is not handed out twice")
}

// An inventory reads the whole config over a CLI and can take minutes: a
// holder past the short lease is still working.
func TestSwitchInventoryKeepsTheLongLease(t *testing.T) {
	service := NewSwitchPollJobService(setupTestDB(t))
	id := seedSwitchJob(t, service, models.SwitchPollKindInventory, time.Now().Add(-time.Hour))
	working := time.Now().Add(-switchPollLease - time.Minute)
	require.NoError(t, service.db.Model(&models.SwitchPollJob{}).Where("switch_id = ?", id).
		Updates(map[string]interface{}{"locked_by": "worker-a", "locked_at": working}).Error)

	job, err := service.Claim("worker-b")
	require.NoError(t, err)
	assert.Nil(t, job)

	abandoned := time.Now().Add(-switchLongLease - time.Minute)
	require.NoError(t, service.db.Model(&models.SwitchPollJob{}).Where("switch_id = ?", id).
		Update("locked_at", abandoned).Error)
	job, err = service.Claim("worker-b")
	require.NoError(t, err)
	require.NotNil(t, job)
}

func TestSwitchJobSchedules(t *testing.T) {
	assert.Equal(t, time.Minute, SwitchIntervalFor(models.SwitchPollKindInterfaces))
	assert.Equal(t, 30*time.Minute, SwitchIntervalFor(models.SwitchPollKindInventory))
	assert.Equal(t, 24*time.Hour, SwitchIntervalFor(models.SwitchPollKindBackup))
}

func TestSwitchCompleteAndFailReschedule(t *testing.T) {
	service := NewSwitchPollJobService(setupTestDB(t))
	id := seedSwitchJob(t, service, models.SwitchPollKindInventory, time.Now().Add(-time.Minute))
	job, err := service.Claim("worker-a")
	require.NoError(t, err)

	require.NoError(t, service.Fail(job, errors.New("telnet refused")))
	var stored models.SwitchPollJob
	require.NoError(t, service.db.First(&stored, "switch_id = ?", id).Error)
	assert.Equal(t, 1, stored.ConsecutiveFailures)
	require.NotNil(t, stored.LastError)
	assert.Equal(t, "telnet refused", *stored.LastError)
	assert.Nil(t, stored.LockedBy)

	require.NoError(t, service.Complete(&stored, time.Second))
	require.NoError(t, service.db.First(&stored, "switch_id = ?", id).Error)
	assert.Equal(t, 0, stored.ConsecutiveFailures)
	assert.Nil(t, stored.LastError)
	assert.WithinDuration(t, time.Now().Add(SwitchInventoryInterval), stored.DueAt, time.Minute)
}

func TestSwitchRunNowMakesTheTierDue(t *testing.T) {
	service := NewSwitchPollJobService(setupTestDB(t))
	id := seedSwitchJob(t, service, models.SwitchPollKindBackup, time.Now().Add(time.Hour))

	require.NoError(t, service.RunNow(id, models.SwitchPollKindBackup))

	job, err := service.Claim("worker-a")
	require.NoError(t, err)
	require.NotNil(t, job)
	assert.Equal(t, id, job.SwitchID)
}

// The manual mark belongs to one run: once that run is released, the next
// daily backup must not be labelled manual too.
func TestSwitchRequestBackupMarksOneRunManual(t *testing.T) {
	service := NewSwitchPollJobService(setupTestDB(t))
	id := seedSwitchJob(t, service, models.SwitchPollKindBackup, time.Now().Add(time.Hour))

	require.NoError(t, service.RequestBackup(id))
	job, err := service.Claim("worker-a")
	require.NoError(t, err)
	require.NotNil(t, job)
	assert.True(t, job.ManualRequested)

	require.NoError(t, service.Complete(job, time.Second))
	var released models.SwitchPollJob
	require.NoError(t, service.db.First(&released, "switch_id = ? AND kind = ?", id, models.SwitchPollKindBackup).Error)
	assert.False(t, released.ManualRequested)
}
