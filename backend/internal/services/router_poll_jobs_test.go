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

func seedRouterJob(t *testing.T, service *RouterPollJobService, kind models.RouterPollKind, due time.Time) uuid.UUID {
	t.Helper()
	id := uuid.New()
	require.NoError(t, service.db.Create(&models.RouterPollJob{RouterID: id, Kind: kind, DueAt: due}).Error)
	return id
}

func TestRouterClaimTakesTheMostOverdueJob(t *testing.T) {
	service := NewRouterPollJobService(setupTestDB(t))
	seedRouterJob(t, service, models.RouterPollKindBGP, time.Now().Add(-time.Minute))
	oldest := seedRouterJob(t, service, models.RouterPollKindInterfaces, time.Now().Add(-time.Hour))
	seedRouterJob(t, service, models.RouterPollKindBGP, time.Now().Add(time.Hour))

	job, err := service.Claim("worker-a")
	require.NoError(t, err)
	require.NotNil(t, job)
	assert.Equal(t, oldest, job.RouterID)
	require.NotNil(t, job.LockedBy)
	assert.Equal(t, "worker-a", *job.LockedBy)
}

func TestRouterClaimSkipsAJobWhoseLeaseIsLive(t *testing.T) {
	service := NewRouterPollJobService(setupTestDB(t))
	seedRouterJob(t, service, models.RouterPollKindBGP, time.Now().Add(-time.Minute))

	first, err := service.Claim("worker-a")
	require.NoError(t, err)
	require.NotNil(t, first)

	second, err := service.Claim("worker-b")
	require.NoError(t, err)
	assert.Nil(t, second, "a held job is not handed out twice")
}

func TestRouterClaimRetakesAnAbandonedJob(t *testing.T) {
	service := NewRouterPollJobService(setupTestDB(t))
	id := seedRouterJob(t, service, models.RouterPollKindBGP, time.Now().Add(-time.Hour))
	stale := time.Now().Add(-routerPollLease - time.Minute)
	holder := "dead-worker"
	require.NoError(t, service.db.Model(&models.RouterPollJob{}).Where("router_id = ?", id).
		Updates(map[string]interface{}{"locked_by": holder, "locked_at": stale}).Error)

	job, err := service.Claim("worker-b")
	require.NoError(t, err)
	require.NotNil(t, job)
	assert.Equal(t, id, job.RouterID)
}

func TestRouterCompleteSchedulesTheNextRunAndReleases(t *testing.T) {
	service := NewRouterPollJobService(setupTestDB(t))
	seedRouterJob(t, service, models.RouterPollKindDiscovery, time.Now().Add(-time.Minute))
	job, err := service.Claim("worker-a")
	require.NoError(t, err)

	require.NoError(t, service.Complete(job, 300*time.Millisecond))

	var stored models.RouterPollJob
	require.NoError(t, service.db.First(&stored, "router_id = ?", job.RouterID).Error)
	assert.Nil(t, stored.LockedBy)
	assert.WithinDuration(t, time.Now().Add(RouterDiscoveryInterval), stored.DueAt, 5*time.Second)
	assert.Zero(t, stored.ConsecutiveFailures)
}

func TestRouterFailBacksOffAndCaps(t *testing.T) {
	service := NewRouterPollJobService(setupTestDB(t))
	seedRouterJob(t, service, models.RouterPollKindBGP, time.Now().Add(-time.Minute))
	job, err := service.Claim("worker-a")
	require.NoError(t, err)

	job.ConsecutiveFailures = 1
	require.NoError(t, service.Fail(job, errors.New("timeout")))

	var stored models.RouterPollJob
	require.NoError(t, service.db.First(&stored, "router_id = ?", job.RouterID).Error)
	assert.Equal(t, 2, stored.ConsecutiveFailures)
	assert.WithinDuration(t, time.Now().Add(2*RouterBGPInterval), stored.DueAt, 5*time.Second)
	require.NotNil(t, stored.LastError)
	assert.Equal(t, "timeout", *stored.LastError)

	assert.Equal(t, RouterBGPInterval*4, routerBackoff(models.RouterPollKindBGP, 40))
}

func TestRouterRunNowMakesTheGivenTiersDue(t *testing.T) {
	service := NewRouterPollJobService(setupTestDB(t))
	later := time.Now().Add(time.Hour)
	id := uuid.New()
	for _, kind := range AllRouterPollKinds {
		require.NoError(t, service.db.Create(&models.RouterPollJob{RouterID: id, Kind: kind, DueAt: later}).Error)
	}

	require.NoError(t, service.RunNow(id, models.RouterPollKindDiscovery))

	var discovery, bgp models.RouterPollJob
	require.NoError(t, service.db.First(&discovery, "router_id = ? AND kind = ?", id, models.RouterPollKindDiscovery).Error)
	require.NoError(t, service.db.First(&bgp, "router_id = ? AND kind = ?", id, models.RouterPollKindBGP).Error)
	assert.True(t, discovery.DueAt.Before(time.Now().Add(time.Second)))
	assert.True(t, bgp.DueAt.After(time.Now().Add(30*time.Minute)), "other tiers keep their schedule")
}
