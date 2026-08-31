package main

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"go.uber.org/zap"
)

func onlineONT() models.ONT {
	slot := 3
	return models.ONT{
		ID: uuid.New(), OLTID: uuid.New(), Slot: &slot, PortID: 1, ONTID: 1,
		SerialNumber: "RTEGC609E381", Status: models.ONTStatusOnline,
	}
}

// A cycle that read nothing off the OLT - because discovery held it, or the
// walk failed - reported no status for any ONT, and every subscriber was
// written offline in one go. The OLT lists an ONU it holds whether that ONU is
// up or down, so silence carries no verdict.
func TestDetermineOntStatusHoldsWhenTheCycleReadNothing(t *testing.T) {
	status := determineOntStatus(onlineONT(), readingWith(nil, map[connectivity.ONTLocation]int{}), nil, zap.NewNop())

	assert.Equal(t, models.ONTStatus(""), status)
}

// An empty status is what handleStatusChange treats as "leave the row alone".
func TestHandleStatusChangeLeavesRowOnEmptyStatus(t *testing.T) {
	db := setupWorkerOLTStatusTestDB(t)
	ont := onlineONT()
	require.NoError(t, db.Create(&ont).Error)

	handleStatusChange(services.NewONTService(db), ont, "", zap.NewNop())

	var stored models.ONT
	require.NoError(t, db.First(&stored, "id = ?", ont.ID).Error)
	assert.Equal(t, models.ONTStatusOnline, stored.Status)
}

// An OLT that does report the ONU still decides: down is down.
func TestDetermineOntStatusTakesTheOLTVerdict(t *testing.T) {
	ont := onlineONT()
	statuses := map[connectivity.ONTLocation]int{
		{Slot: 3, Port: 1, ONTID: 1}: connectivity.PhaseStateOffline,
	}

	status := determineOntStatus(ont, readingWith(nil, statuses), nil, zap.NewNop())

	assert.Equal(t, models.ONTStatusOffline, status)
}

// Falling back to the optical reading stays: an ONT missing from the status
// table but showing RX power is up.
func TestDetermineOntStatusFallsBackToRxPower(t *testing.T) {
	rx := -25.85
	status := determineOntStatus(onlineONT(), readingWith(nil, nil),
		&connectivity.ONTMetrics{RxPower: &rx}, zap.NewNop())

	assert.Equal(t, models.ONTStatusOnline, status)
}

// A ZTE OLT reports phases an ONU only passes through while it comes up, and
// the run state map names four. Writing the rest as "unknown" threw away the
// status the row already had and stamped a last_offline the ONU never had: a
// freshly registered ONU read as one of these a minute in, and was online a
// minute after that.
func TestDetermineOntStatusIgnoresATransitionalPhase(t *testing.T) {
	const ranging = 2
	statuses := map[connectivity.ONTLocation]int{
		{Slot: 3, Port: 1, ONTID: 1}: ranging,
	}

	status := determineOntStatus(onlineONT(), readingWith(nil, statuses), nil, zap.NewNop())

	assert.Equal(t, models.ONTStatus(""), status)
}
