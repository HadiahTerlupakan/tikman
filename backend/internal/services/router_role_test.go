package services

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

func TestRouterPollKindsFor(t *testing.T) {
	cases := map[models.RouterRole][]models.RouterPollKind{
		models.RouterRoleBorder: {models.RouterPollKindDiscovery, models.RouterPollKindInterfaces, models.RouterPollKindBGP, models.RouterPollKindRoutes},
		models.RouterRoleOther:  {models.RouterPollKindDiscovery, models.RouterPollKindInterfaces, models.RouterPollKindBGP},
		models.RouterRoleBRAS:   {models.RouterPollKindDiscovery, models.RouterPollKindInterfaces, models.RouterPollKindBRAS},
	}
	for role, want := range cases {
		assert.Equal(t, want, RouterPollKindsFor(role), string(role))
	}
}

func jobKinds(t *testing.T, service *RouterService, id uuid.UUID) map[models.RouterPollKind]models.RouterPollJob {
	t.Helper()
	var jobs []models.RouterPollJob
	require.NoError(t, service.db.Where("router_id = ?", id).Find(&jobs).Error)
	out := map[models.RouterPollKind]models.RouterPollJob{}
	for _, job := range jobs {
		out[job.Kind] = job
	}
	return out
}

func TestSetRouterRoleSyncsJobs(t *testing.T) {
	service, db := newRouterServiceForTest(t)
	router, err := service.Create(validRouterInput())
	require.NoError(t, err)
	for _, name := range []string{"upstream-a", "upstream-b"} {
		require.NoError(t, db.Create(&models.RouterBGPSession{RouterID: router.ID, Name: name,
			RemoteAddress: "192.0.2.9", State: models.BGPStateEstablished}).Error)
	}
	later := time.Now().Add(time.Hour)
	require.NoError(t, db.Model(&models.RouterPollJob{}).
		Where("router_id = ? AND kind = ?", router.ID, models.RouterPollKindInterfaces).
		Update("due_at", later).Error)

	require.NoError(t, SetRouterRole(db, router.ID, models.RouterRoleBRAS, true))

	jobs := jobKinds(t, service, router.ID)
	require.Len(t, jobs, 3)
	assert.Contains(t, jobs, models.RouterPollKindBRAS)
	assert.Contains(t, jobs, models.RouterPollKindDiscovery)
	assert.False(t, jobs[models.RouterPollKindBRAS].DueAt.After(time.Now()))
	assert.WithinDuration(t, later, jobs[models.RouterPollKindInterfaces].DueAt, time.Second, "kept rows keep their schedule")
	var sessions int64
	require.NoError(t, db.Model(&models.RouterBGPSession{}).Where("router_id = ?", router.ID).Count(&sessions).Error)
	assert.Zero(t, sessions)
	stored, err := service.Get(router.ID)
	require.NoError(t, err)
	assert.Equal(t, models.RouterRoleBRAS, stored.Role)
	assert.True(t, stored.RoleSetByAdmin)
}

func TestSetRouterRoleBackToBorderRestoresJobs(t *testing.T) {
	service, db := newRouterServiceForTest(t)
	router, err := service.Create(validRouterInput())
	require.NoError(t, err)
	require.NoError(t, SetRouterRole(db, router.ID, models.RouterRoleBRAS, true))

	require.NoError(t, SetRouterRole(db, router.ID, models.RouterRoleBorder, true))

	assert.Len(t, jobKinds(t, service, router.ID), 4)
}

func TestCompleteOnDeletedJobIsNoop(t *testing.T) {
	service, db := newRouterServiceForTest(t)
	router, err := service.Create(validRouterInput())
	require.NoError(t, err)
	jobs := NewRouterPollJobService(db)
	require.NoError(t, jobs.RunNow(router.ID, models.RouterPollKindRoutes))
	claimed := claimKind(t, jobs, models.RouterPollKindRoutes)
	require.NoError(t, SetRouterRole(db, router.ID, models.RouterRoleBRAS, true))

	require.NoError(t, jobs.Complete(claimed, time.Second))

	assert.NotContains(t, jobKinds(t, service, router.ID), models.RouterPollKindRoutes)
}

func claimKind(t *testing.T, jobs *RouterPollJobService, kind models.RouterPollKind) *models.RouterPollJob {
	t.Helper()
	for {
		job, err := jobs.Claim("w")
		require.NoError(t, err)
		require.NotNil(t, job, "no %s job was due", kind)
		if job.Kind == kind {
			return job
		}
	}
}
