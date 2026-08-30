package services

import (
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

func dashboardOLT(t *testing.T, db *gorm.DB, name, ip string, status models.OLTStatus) uuid.UUID {
	t.Helper()
	olt := &models.OLT{
		ID:        uuid.New(),
		SiteID:    uuid.New(),
		Name:      name,
		IPAddress: ip,
		Model:     models.OLTModelZTEC320,
		Username:  "admin",
		Password:  "pass",
		Status:    status,
	}
	require.NoError(t, db.Create(olt).Error)
	return olt.ID
}

func dashboardONT(t *testing.T, db *gorm.DB, oltID uuid.UUID, serial string, status models.ONTStatus, rx *float64) {
	t.Helper()
	require.NoError(t, db.Create(&models.ONT{
		ID:           uuid.New(),
		OLTID:        oltID,
		PortID:       1,
		ONTID:        int(uuid.New().ID() % 100000),
		SerialNumber: serial,
		Status:       status,
		RxPower:      rx,
	}).Error)
}

func rx(v float64) *float64 { return &v }

func TestDashboardCountsEveryONTNotJustTheFirstPage(t *testing.T) {
	// The browser used to count a page of at most 500 rows, so a network of 930
	// reported 500. The count has to come from the database or it is a fiction
	// that scales with how much the page happened to fetch.
	db := setupTestDB(t)
	oltID := dashboardOLT(t, db, "Cariu", "172.30.30.3", models.OLTStatusOnline)

	for i := 0; i < 651; i++ {
		dashboardONT(t, db, oltID, fmt.Sprintf("ZTEG%08d", i), models.ONTStatusOnline, nil)
	}

	stats, err := NewDashboardService(db).Stats()
	require.NoError(t, err)
	require.Equal(t, int64(651), stats.ONTs.Total)
	require.Equal(t, int64(651), stats.ONTs.Online)
	require.Len(t, stats.OLTs, 1)
	require.Equal(t, int64(651), stats.OLTs[0].ONTTotal)
}

func TestDashboardSeparatesEveryONTState(t *testing.T) {
	db := setupTestDB(t)
	oltID := dashboardOLT(t, db, "Cariu", "172.30.30.3", models.OLTStatusOnline)

	dashboardONT(t, db, oltID, "SN0001", models.ONTStatusOnline, nil)
	dashboardONT(t, db, oltID, "SN0002", models.ONTStatusOffline, nil)
	dashboardONT(t, db, oltID, "SN0003", models.ONTStatusLOS, nil)
	dashboardONT(t, db, oltID, "SN0004", models.ONTStatusDyingGas, nil)
	dashboardONT(t, db, oltID, "SN0005", models.ONTStatusUnknown, nil)

	stats, err := NewDashboardService(db).Stats()
	require.NoError(t, err)
	require.Equal(t, ONTStatusCounts{Total: 5, Online: 1, Offline: 1, LOS: 1, DyingGasp: 1, Unknown: 1}, stats.ONTs)
	require.Equal(t, int64(4), stats.OLTs[0].Impaired, "everything that is not online needs attention")
}

func TestDashboardKeepsAnOLTThatHasNoONTs(t *testing.T) {
	// An OLT that has gone quiet is what the table exists to surface. Dropping
	// it would make a total outage look like a clean board.
	db := setupTestDB(t)
	dashboardOLT(t, db, "Depok", "192.168.220.22", models.OLTStatusOffline)

	stats, err := NewDashboardService(db).Stats()
	require.NoError(t, err)
	require.Len(t, stats.OLTs, 1)
	require.Equal(t, "Depok", stats.OLTs[0].OLTName)
	require.Equal(t, int64(0), stats.OLTs[0].ONTTotal)
	require.Equal(t, string(models.OLTStatusOffline), stats.OLTs[0].OLTStatus)
}

func TestDashboardCountsEachOLTSeparately(t *testing.T) {
	db := setupTestDB(t)
	cariu := dashboardOLT(t, db, "Cariu", "172.30.30.3", models.OLTStatusOnline)
	bekasi := dashboardOLT(t, db, "Bekasi", "172.30.30.2", models.OLTStatusOnline)

	dashboardONT(t, db, cariu, "SN0001", models.ONTStatusOnline, nil)
	dashboardONT(t, db, cariu, "SN0002", models.ONTStatusOffline, nil)
	dashboardONT(t, db, bekasi, "SN0003", models.ONTStatusOnline, nil)

	stats, err := NewDashboardService(db).Stats()
	require.NoError(t, err)

	byName := map[string]OLTBreakdown{}
	for _, row := range stats.OLTs {
		byName[row.OLTName] = row
	}
	require.Equal(t, int64(2), byName["Cariu"].ONTTotal)
	require.Equal(t, int64(1), byName["Cariu"].Online)
	require.Equal(t, int64(1), byName["Bekasi"].ONTTotal)
}

func TestDashboardWeakestSignalsIgnoreONTsThatAreDown(t *testing.T) {
	// An offline ONT keeps the reading it took before it went dark, and those
	// are the most negative in the table. Letting them in fills the card with
	// links nobody can act on and hides the ones still degrading.
	db := setupTestDB(t)
	oltID := dashboardOLT(t, db, "Cariu", "172.30.30.3", models.OLTStatusOnline)

	dashboardONT(t, db, oltID, "SNDARK001", models.ONTStatusOffline, rx(-40))
	dashboardONT(t, db, oltID, "SNLIVE001", models.ONTStatusOnline, rx(-31.5))
	dashboardONT(t, db, oltID, "SNLIVE002", models.ONTStatusOnline, rx(-18))
	dashboardONT(t, db, oltID, "SNNORX001", models.ONTStatusOnline, nil)

	stats, err := NewDashboardService(db).Stats()
	require.NoError(t, err)
	require.Len(t, stats.WeakestSignals, 2)
	require.Equal(t, "SNLIVE001", stats.WeakestSignals[0].SerialNumber, "worst reading first")
	require.Equal(t, -31.5, stats.WeakestSignals[0].RxPower)
	require.Equal(t, "Cariu", stats.WeakestSignals[0].OLTName)
	require.Equal(t, "SNLIVE002", stats.WeakestSignals[1].SerialNumber)
}

func TestDashboardWeakestSignalsStopAtFive(t *testing.T) {
	db := setupTestDB(t)
	oltID := dashboardOLT(t, db, "Cariu", "172.30.30.3", models.OLTStatusOnline)

	for i := 0; i < 9; i++ {
		dashboardONT(t, db, oltID, fmt.Sprintf("SN%06d", i), models.ONTStatusOnline, rx(-30+float64(i)))
	}

	stats, err := NewDashboardService(db).Stats()
	require.NoError(t, err)
	require.Len(t, stats.WeakestSignals, weakSignalCount)
	require.Equal(t, -30.0, stats.WeakestSignals[0].RxPower)
}

func TestDashboardOnAnEmptyInstallation(t *testing.T) {
	db := setupTestDB(t)

	stats, err := NewDashboardService(db).Stats()
	require.NoError(t, err)
	require.Equal(t, ONTStatusCounts{}, stats.ONTs)
	require.Empty(t, stats.OLTs)
	require.Empty(t, stats.WeakestSignals)
}
