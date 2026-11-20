package main

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
)

func TestSwitchJobsReachTheirPoll(t *testing.T) {
	db := setupWorkerOLTStatusTestDB(t)
	rt := &workerRuntime{switchPoll: services.NewSwitchPollService(db, services.NewSwitchService(db, ""))}
	sw := &models.Switch{ID: uuid.New(), Name: "SW-TEST", Vendor: models.SwitchVendorHuaweiCE}

	for _, kind := range []models.SwitchPollKind{
		models.SwitchPollKindInterfaces, models.SwitchPollKindInventory, models.SwitchPollKindBackup,
	} {
		// The switch's secrets cannot be decrypted, so each poll stops at its
		// first step; what matters is that the kind reached it.
		err := runSwitchJob(rt, &models.SwitchPollJob{SwitchID: sw.ID, Kind: kind}, sw)
		require.Error(t, err, kind)
		assert.NotContains(t, err.Error(), "unknown switch poll kind", kind)
		assert.Contains(t, err.Error(), "decrypt", kind)
	}

	err := runSwitchJob(rt, &models.SwitchPollJob{SwitchID: sw.ID, Kind: "reboot"}, sw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown switch poll kind")
}
