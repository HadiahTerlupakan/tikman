package database

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const (
	silenceBackfillSchema  = "silence_backfill_check"
	silenceThresholdSchema = "silence_threshold_check"
)

func insertMetric(t *testing.T, db *gorm.DB, interfaceID uuid.UUID, at time.Time, inBps float64) {
	t.Helper()
	require.NoError(t, db.Exec(`INSERT INTO router_interface_metrics (time, interface_id, in_bps, out_bps)
		VALUES (?, ?, ?, 1700)`, at, interfaceID, inBps).Error)
}

// A POP that went silent before silence tracking existed must still be
// caught: migration 59 dates each interface's last traffic from the samples
// already stored. An interface with no traffic in them stays unmarked.
func TestMigration59DatesLastTrafficFromStoredSamples(t *testing.T) {
	db := freshPostgresBefore(t, silenceBackfillSchema, "59")
	routerID := insertTestRouter(t, db)
	pop := insertTestInterface(t, db, routerID)
	spare := uuid.New()
	require.NoError(t, db.Exec(`INSERT INTO router_interfaces
		(id, router_id, name, comment, type, disabled, oper_status, addresses, created_at, updated_at)
		VALUES (?, ?, 'vlan-spare', '', 'vlan', false, 'up', '', now(), now())`, spare, routerID).Error)
	lastTraffic := time.Now().UTC().Truncate(time.Second).Add(-2 * time.Hour)
	insertMetric(t, db, pop, lastTraffic.Add(-time.Minute), 9_000_000)
	insertMetric(t, db, pop, lastTraffic, 10_000_000)
	for i := 1; i <= 4; i++ {
		insertMetric(t, db, pop, lastTraffic.Add(time.Duration(i)*time.Minute), 0)
	}
	insertMetric(t, db, spare, lastTraffic, 0)

	require.NoError(t, RunSQLMigrations(db, "../../migrations"))

	var got struct {
		Pop         *time.Time
		Spare       *time.Time
		SilentSince *time.Time
		ZeroInPolls int
	}
	require.NoError(t, db.Raw(`SELECT
		(SELECT last_traffic_at FROM router_interfaces WHERE id = ?) AS pop,
		(SELECT last_traffic_at FROM router_interfaces WHERE id = ?) AS spare,
		(SELECT silent_since FROM router_interfaces WHERE id = ?) AS silent_since,
		(SELECT zero_in_polls FROM router_interfaces WHERE id = ?) AS zero_in_polls`,
		pop, spare, pop, pop).Scan(&got).Error)
	require.NotNil(t, got.Pop)
	assert.True(t, lastTraffic.Equal(*got.Pop), "got %v", got.Pop)
	assert.Nil(t, got.Spare)
	// The silence already under way is dated from its first stored sample,
	// not from the first poll after the deploy.
	require.NotNil(t, got.SilentSince)
	assert.True(t, lastTraffic.Add(time.Minute).Equal(*got.SilentSince), "got %v", got.SilentSince)
	assert.Equal(t, 4, got.ZeroInPolls)
}

func insertSilentInterface(t *testing.T, db *gorm.DB, routerID uuid.UUID, name string, lastTraffic time.Time) uuid.UUID {
	t.Helper()
	id := uuid.New()
	require.NoError(t, db.Exec(`INSERT INTO router_interfaces
		(id, router_id, name, comment, type, disabled, oper_status, addresses, created_at, updated_at,
		 last_traffic_at, silent_since, zero_in_since, zero_in_polls)
		VALUES (?, ?, ?, '', 'vlan', false, 'up', '', now(), now(), ?, ?, ?, 5)`,
		id, routerID, name, lastTraffic, lastTraffic.Add(time.Minute), lastTraffic.Add(time.Minute)).Error)
	return id
}

// Migration 59 counted any inbound packet as traffic, so a management VLAN
// that only ever trickles was marked silent. Migration 60 keeps only real
// traffic (64 kbit/s and up) and clears the marks that rested on a trickle.
func TestMigration60ClearsSilenceThatRestedOnATrickle(t *testing.T) {
	db := freshPostgresBefore(t, silenceThresholdSchema, "60")
	routerID := insertTestRouter(t, db)
	now := time.Now().UTC().Truncate(time.Second)
	busy := insertSilentInterface(t, db, routerID, "vlan-pop-a", now.Add(-2*time.Hour))
	insertMetric(t, db, busy, now.Add(-2*time.Hour), 10_000_000)
	insertMetric(t, db, busy, now.Add(-2*time.Hour+time.Minute), 0)
	trickle := insertSilentInterface(t, db, routerID, "vlan-mgmt", now.Add(-30*time.Minute))
	insertMetric(t, db, trickle, now.Add(-time.Hour), 48_000)
	insertMetric(t, db, trickle, now.Add(-30*time.Minute), 800)

	require.NoError(t, RunSQLMigrations(db, "../../migrations"))

	type state struct {
		LastTrafficAt *time.Time
		SilentSince   *time.Time
		ZeroInPolls   int
	}
	read := func(id uuid.UUID) state {
		var s state
		require.NoError(t, db.Raw(`SELECT last_traffic_at, silent_since, zero_in_polls
			FROM router_interfaces WHERE id = ?`, id).Scan(&s).Error)
		return s
	}
	got := read(busy)
	require.NotNil(t, got.LastTrafficAt)
	assert.True(t, now.Add(-2*time.Hour).Equal(*got.LastTrafficAt), "got %v", got.LastTrafficAt)
	assert.NotNil(t, got.SilentSince, "a POP that carried real traffic stays marked")

	got = read(trickle)
	assert.Nil(t, got.LastTrafficAt)
	assert.Nil(t, got.SilentSince)
	assert.Zero(t, got.ZeroInPolls)
}
