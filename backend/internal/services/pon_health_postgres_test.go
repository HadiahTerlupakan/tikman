package services

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// ponFixture builds ONTs on a given card and port, with traps and outages, so a
// test can state a PON's shape in one line.
func ponOnPort(t *testing.T, f troubleFixture, slot, port, count int, trapsEach int, downSeconds int64) {
	t.Helper()
	for i := 0; i < count; i++ {
		serial := "SN-" + uuid.NewString()[:8]
		ont := models.ONT{
			ID: uuid.New(), OLTID: f.oltID, Slot: &slot, PortID: port, ONTID: i + 1,
			SerialNumber: serial, Name: "Pelanggan", Status: models.ONTStatusOnline,
		}
		require.NoError(t, f.db.Create(&ont).Error)
		if trapsEach > 0 {
			f.traps(t, serial, trapsEach, time.Hour)
		}
		if downSeconds > 0 {
			f.outage(t, ont.ID, downSeconds, time.Hour)
		}
	}
}

func healthFor(t *testing.T, db *gorm.DB, oltID uuid.UUID) PonHealth {
	t.Helper()
	health, err := NewONTService(db).PonHealthFor(oltID, 24*time.Hour)
	require.NoError(t, err)
	return health
}

func TestPonHealthShowsAPortLosingService(t *testing.T) {
	db := setupTroublePostgres(t)
	f := newTroubleFixture(t, db)
	// Quiet ports set the median low; the last one loses a tenth of the day on
	// almost no traps, which is the Depok 3/2 shape that one criterion misses.
	ponOnPort(t, f, 1, 1, 10, 1, 0)
	ponOnPort(t, f, 1, 2, 10, 1, 0)
	ponOnPort(t, f, 2, 7, 6, 1, 9000)

	health := healthFor(t, db, f.oltID)

	require.Len(t, health.Cards, 1)
	assert.Equal(t, 2, health.Cards[0].Slot)
	require.Len(t, health.Cards[0].Pons, 1)
	assert.Equal(t, 7, health.Cards[0].Pons[0].Port)
}

func TestPonHealthShowsAPortThatChurnsWithoutOutage(t *testing.T) {
	db := setupTroublePostgres(t)
	f := newTroubleFixture(t, db)
	ponOnPort(t, f, 1, 1, 10, 10, 0)
	ponOnPort(t, f, 1, 2, 10, 10, 0)
	ponOnPort(t, f, 9, 8, 10, 900, 0)

	health := healthFor(t, db, f.oltID)

	// 900 per ONT is far past five times the median of ten and past the floor
	// of a hundred, with no outage at all: the Cariu 9/8 shape.
	require.Len(t, health.Cards, 1)
	assert.Equal(t, 9, health.Cards[0].Slot)
	assert.EqualValues(t, 900, health.Cards[0].Pons[0].TrapPerONT)
}

func TestPonHealthLeavesOutAnOutlierInAQuietNetwork(t *testing.T) {
	db := setupTroublePostgres(t)
	f := newTroubleFixture(t, db)
	ponOnPort(t, f, 1, 1, 10, 1, 0)
	ponOnPort(t, f, 1, 2, 10, 1, 0)
	ponOnPort(t, f, 2, 2, 10, 13, 0)

	health := healthFor(t, db, f.oltID)

	// Thirteen is thirteen times this network's median and still not a fault.
	// The floor is what keeps a quiet OLT from reporting one.
	assert.Empty(t, health.Cards)
}

func TestPonHealthLeavesOutAPortWithTooFewONTs(t *testing.T) {
	db := setupTroublePostgres(t)
	f := newTroubleFixture(t, db)
	ponOnPort(t, f, 1, 1, 10, 1, 0)
	ponOnPort(t, f, 1, 2, 10, 1, 0)
	ponOnPort(t, f, 3, 3, 4, 5000, 0)

	// One bad ONT on a port serving four would top any per-ONT ranking and say
	// nothing about the port.
	health := healthFor(t, db, f.oltID)
	assert.Empty(t, health.Cards)
}

func TestPonHealthNamesTheWorstSubscribersOnAPort(t *testing.T) {
	db := setupTroublePostgres(t)
	f := newTroubleFixture(t, db)
	ponOnPort(t, f, 1, 1, 10, 1, 0)
	ponOnPort(t, f, 1, 2, 10, 1, 0)
	ponOnPort(t, f, 4, 4, 8, 900, 0)

	health := healthFor(t, db, f.oltID)

	require.Len(t, health.Cards, 1)
	// Eight subscribers on the port, five named: a port can hold seventy, and
	// drawing them all restores the problem this view exists to remove.
	assert.Len(t, health.Cards[0].Pons[0].Worst, 5)
}

func TestPonHealthReportsTheThresholdsItApplied(t *testing.T) {
	db := setupTroublePostgres(t)
	f := newTroubleFixture(t, db)
	ponOnPort(t, f, 1, 1, 10, 20, 0)
	ponOnPort(t, f, 1, 2, 10, 20, 0)

	health := healthFor(t, db, f.oltID)

	// Shown on screen rather than hidden, so the operator can judge the rule
	// instead of trusting it.
	assert.EqualValues(t, 20, health.MedianTrapPerONT)
	assert.EqualValues(t, 100, health.TrapThreshold)
}

func TestPonHealthKeepsCardsApartOnTheSamePortNumber(t *testing.T) {
	db := setupTroublePostgres(t)
	f := newTroubleFixture(t, db)
	ponOnPort(t, f, 1, 1, 10, 1, 0)
	ponOnPort(t, f, 1, 2, 10, 1, 0)
	// The same port number on two cards, only one of them at fault: the Cariu
	// 8/12 and 9/8 shape, where a port-only match sends a technician to the
	// wrong chassis slot.
	ponOnPort(t, f, 8, 8, 6, 1, 0)
	ponOnPort(t, f, 9, 8, 6, 900, 0)

	health := healthFor(t, db, f.oltID)

	require.Len(t, health.Cards, 1)
	assert.Equal(t, 9, health.Cards[0].Slot)
	require.Len(t, health.Cards[0].Pons, 1)
	require.NotEmpty(t, health.Cards[0].Pons[0].Worst)
	for _, worst := range health.Cards[0].Pons[0].Worst {
		// The label has to name the card, or two subscribers on port 8 of
		// different cards read as the same line on screen.
		assert.Contains(t, worst.Label, "ONU-9/8:")
		assert.EqualValues(t, 900, worst.TrapCount)
	}
}

func TestPonHealthClampsTheWindowToRetention(t *testing.T) {
	db := setupTroublePostgres(t)
	f := newTroubleFixture(t, db)
	ponOnPort(t, f, 1, 1, 10, 1, 0)
	// Roughly a tenth of a week lost per subscriber. Over the seven days the
	// trap table actually keeps that is a fault; spread over a month asked for
	// it disappears, so the service has to clamp the way the ranking does.
	ponOnPort(t, f, 2, 7, 10, 1, 60000)

	health, err := NewONTService(db).PonHealthFor(f.oltID, 30*24*time.Hour)
	require.NoError(t, err)

	require.Len(t, health.Cards, 1)
	assert.Equal(t, 2, health.Cards[0].Slot)
}

func TestPonHealthSendsAnEmptyCardListNotNull(t *testing.T) {
	db := setupTroublePostgres(t)
	f := newTroubleFixture(t, db)
	ponOnPort(t, f, 1, 1, 10, 1, 0)
	ponOnPort(t, f, 1, 2, 10, 1, 0)

	health := healthFor(t, db, f.oltID)
	body, err := json.Marshal(health)
	require.NoError(t, err)

	// A nil slice marshals to null, and the topology reads cards.length before
	// it can decide there is nothing to draw. A healthy OLT crashed the page.
	assert.Contains(t, string(body), `"cards":[]`)
}
