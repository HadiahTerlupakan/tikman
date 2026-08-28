package services

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

func seedONTAt(t *testing.T, db *gorm.DB, oltID uuid.UUID, serial string, ontID int, createdAt time.Time) uuid.UUID {
	t.Helper()

	ont := &models.ONT{
		ID: uuid.New(), OLTID: oltID, PortID: 1, ONTID: ontID,
		SerialNumber: serial, Status: models.ONTStatusUnknown,
	}
	require.NoError(t, db.Create(ont).Error)
	// Written after the fact: GORM stamps CreatedAt on insert.
	require.NoError(t, db.Model(&models.ONT{}).Where("id = ?", ont.ID).
		Update("created_at", createdAt).Error)

	return ont.ID
}

// The pruning transaction clears metrics for the rows it removes, and the
// SQLite test schema has no timeseries tables.
func createMetricsTable(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Exec(`
		CREATE TABLE IF NOT EXISTS ont_metrics (
			time DATETIME NOT NULL,
			ont_id TEXT NOT NULL,
			rx_power REAL
		)
	`).Error)
}

// A registration writes the ONT row, and the OLT does not report the ONU until
// its next walk. Pruning in that gap deleted the row, its metrics, its events,
// and left the provisioning job pointing at nothing.
func TestONTService_PruneMissingFromDiscoveryKeepsAFreshlyRegisteredONT(t *testing.T) {
	db := setupTestDB(t)
	oltID := seedDiscoveryOLT(t, db)
	fresh := seedONTAt(t, db, oltID, "HWTCB403E8A0", 15, time.Now())

	deleted, err := NewONTService(db).PruneMissingFromDiscovery(oltID, []connectivity.DiscoveredONT{
		{Slot: 3, PortID: 1, ONTID: 1, SerialNumber: "OTHER0000001"},
	})

	require.NoError(t, err)
	assert.Zero(t, deleted)

	var count int64
	require.NoError(t, db.Model(&models.ONT{}).Where("id = ?", fresh).Count(&count).Error)
	assert.Equal(t, int64(1), count, "an ONT registered moments ago is not a stale row")
}

// An ONU the OLT has not reported for longer than that is genuinely gone.
func TestONTService_PruneMissingFromDiscoveryStillRemovesAnOldMissingONT(t *testing.T) {
	db := setupTestDB(t)
	oltID := seedDiscoveryOLT(t, db)
	createMetricsTable(t, db)
	old := seedONTAt(t, db, oltID, "OLD000000001", 20, time.Now().Add(-pruneGracePeriod-time.Hour))

	deleted, err := NewONTService(db).PruneMissingFromDiscovery(oltID, []connectivity.DiscoveredONT{
		{Slot: 3, PortID: 1, ONTID: 1, SerialNumber: "OTHER0000001"},
	})

	require.NoError(t, err)
	assert.Equal(t, int64(1), deleted)

	var count int64
	require.NoError(t, db.Model(&models.ONT{}).Where("id = ?", old).Count(&count).Error)
	assert.Zero(t, count)
}
