package database

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const silenceBackfillSchema = "silence_backfill_check"

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
	insertMetric(t, db, pop, lastTraffic.Add(time.Minute), 0)
	insertMetric(t, db, spare, lastTraffic, 0)

	require.NoError(t, RunSQLMigrations(db, "../../migrations"))

	var got struct {
		Pop   *time.Time
		Spare *time.Time
	}
	require.NoError(t, db.Raw(`SELECT
		(SELECT last_traffic_at FROM router_interfaces WHERE id = ?) AS pop,
		(SELECT last_traffic_at FROM router_interfaces WHERE id = ?) AS spare`, pop, spare).Scan(&got).Error)
	require.NotNil(t, got.Pop)
	assert.True(t, lastTraffic.Equal(*got.Pop), "got %v", got.Pop)
	assert.Nil(t, got.Spare)
}
