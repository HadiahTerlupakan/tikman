package main

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"go.uber.org/zap"
)

func settledDiscoveryJob(t *testing.T, runErr error) models.RouterPollJob {
	t.Helper()
	db := setupWorkerOLTStatusTestDB(t)
	job := models.RouterPollJob{RouterID: uuid.New(), Kind: models.RouterPollKindDiscovery, DueAt: time.Now()}
	require.NoError(t, db.Create(&job).Error)
	rt := &workerRuntime{routerJobs: services.NewRouterPollJobService(db), logger: zap.NewNop()}

	settleRouterJob(rt, &job, &models.MikrotikRouter{ID: job.RouterID, Name: "border-a"}, runErr, time.Second)

	var stored models.RouterPollJob
	require.NoError(t, db.First(&stored, "router_id = ?", job.RouterID).Error)
	return stored
}

func TestUnreadRoutesCompleteDiscoveryAtItsNormalInterval(t *testing.T) {
	stored := settledDiscoveryJob(t, fmt.Errorf("%w: timeout", services.ErrStaticRoutesUnread))

	assert.Equal(t, 0, stored.ConsecutiveFailures)
	assert.Nil(t, stored.LastError)
	assert.WithinDuration(t, time.Now().Add(services.RouterDiscoveryInterval), stored.DueAt, time.Minute)
}

func TestOtherDiscoveryErrorsStillBackOff(t *testing.T) {
	stored := settledDiscoveryJob(t, errors.New("router unreachable"))

	assert.Equal(t, 1, stored.ConsecutiveFailures)
	require.NotNil(t, stored.LastError)
	assert.Equal(t, "router unreachable", *stored.LastError)
}

func TestBRASJobRunsThePollBRAS(t *testing.T) {
	db := setupWorkerOLTStatusTestDB(t)
	rt := &workerRuntime{routerPoll: services.NewRouterPollService(db, services.NewRouterService(db, ""))}
	router := &models.MikrotikRouter{ID: uuid.New(), Name: "bras-1", Role: models.RouterRoleBRAS}
	job := &models.RouterPollJob{RouterID: router.ID, Kind: models.RouterPollKindBRAS}

	// The router's secrets cannot be decrypted, so the poll stops at its first
	// step; what matters is that the kind reached it.
	err := runRouterJob(rt, job, router)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "unknown router poll kind")
	assert.Contains(t, err.Error(), "decrypt")
}
