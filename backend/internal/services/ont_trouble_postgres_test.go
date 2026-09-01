package services

import (
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// setupTroublePostgres connects to the Postgres this query is written for. It
// sums an interval out of ont_events with EXTRACT(EPOCH …), which SQLite has no
// equivalent for, so the ranking cannot be exercised anywhere else.
func setupTroublePostgres(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		if os.Getenv("CI") != "" {
			t.Fatal("TEST_POSTGRES_DSN is unset under CI; the troubled-ONT ranking is then never tested")
		}
		t.Skip("set TEST_POSTGRES_DSN to run the troubled-ONT ranking against Postgres")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	for _, table := range []string{"ont_trap_events", "ont_events", "onts", "olts", "sites"} {
		require.NoError(t, db.Exec("DROP TABLE IF EXISTS "+table+" CASCADE").Error)
	}
	require.NoError(t, db.AutoMigrate(&models.Site{}, &models.OLT{}, &models.ONT{},
		&models.ONTEvent{}, &models.ONTTrapEvent{}))
	return db
}

type troubleFixture struct {
	db    *gorm.DB
	oltID uuid.UUID
}

func newTroubleFixture(t *testing.T, db *gorm.DB) troubleFixture {
	t.Helper()
	site := models.Site{ID: uuid.New(), Name: "Site", Location: "L"}
	require.NoError(t, db.Create(&site).Error)
	olt := models.OLT{
		ID: uuid.New(), SiteID: site.ID, Name: "Cariu", IPAddress: "172.30.30.3",
		SNMPCommunity: "public", Username: "a", Password: "b",
		SSHPort: 22, TelnetPort: 23, SNMPPort: 161,
		PreferredProtocol: models.OLTProtocolTelnet, Status: models.OLTStatusOnline,
	}
	require.NoError(t, db.Create(&olt).Error)
	return troubleFixture{db: db, oltID: olt.ID}
}

func (f troubleFixture) ont(t *testing.T, serial string, port, number int) models.ONT {
	t.Helper()
	ont := models.ONT{
		ID: uuid.New(), OLTID: f.oltID, PortID: port, ONTID: number,
		SerialNumber: serial, Name: "Pelanggan " + serial, Status: models.ONTStatusOnline,
	}
	require.NoError(t, f.db.Create(&ont).Error)
	return ont
}

func (f troubleFixture) traps(t *testing.T, serial string, count int, age time.Duration) {
	t.Helper()
	for i := 0; i < count; i++ {
		require.NoError(t, f.db.Create(&models.ONTTrapEvent{
			OLTID: f.oltID, ReceivedAt: time.Now().Add(-age),
			TrapOID: ".1.3.6.1.4.1.3902.1082.500.10.3.1.1", SourceAddress: "172.30.30.3",
			SerialNumber: &serial, Varbinds: "x",
		}).Error)
	}
}

func (f troubleFixture) outage(t *testing.T, ontID uuid.UUID, seconds int64, age time.Duration) {
	t.Helper()
	require.NoError(t, f.db.Create(&models.ONTEvent{
		ONTID: ontID, EventType: models.EventTypeOffline,
		EventTime: time.Now().Add(-age), DurationSeconds: &seconds,
	}).Error)
}

func TestTroubledONTsRanksTheNoisiestFirst(t *testing.T) {
	db := setupTroublePostgres(t)
	f := newTroubleFixture(t, db)
	quiet := f.ont(t, "SN-QUIET", 1, 1)
	noisy := f.ont(t, "SN-NOISY", 1, 2)
	f.traps(t, quiet.SerialNumber, 3, time.Hour)
	f.traps(t, noisy.SerialNumber, 40, time.Hour)

	troubled, _, err := NewONTService(db).TroubledONTs(TroubledFilter{Window: 24 * time.Hour, Limit: 10, OLTID: nil})
	require.NoError(t, err)
	require.Len(t, troubled, 2)

	assert.Equal(t, "SN-NOISY", troubled[0].SerialNumber)
	assert.EqualValues(t, 40, troubled[0].TrapCount)
	assert.Equal(t, "Cariu", troubled[0].OLTName)
	assert.Equal(t, 1, troubled[0].PortID)
	assert.Equal(t, 2, troubled[0].ONTNumber)
}

func TestTroubledONTsCountsOutageMinutes(t *testing.T) {
	db := setupTroublePostgres(t)
	f := newTroubleFixture(t, db)
	ont := f.ont(t, "SN-DOWN", 1, 1)
	f.traps(t, ont.SerialNumber, 1, time.Hour)
	f.outage(t, ont.ID, 600, time.Hour)
	f.outage(t, ont.ID, 300, 2*time.Hour)

	troubled, _, err := NewONTService(db).TroubledONTs(TroubledFilter{Window: 24 * time.Hour, Limit: 10, OLTID: nil})
	require.NoError(t, err)
	require.Len(t, troubled, 1)

	assert.EqualValues(t, 15, troubled[0].DownMinutes)
}

func TestTroubledONTsCountsAnOutageStillRunning(t *testing.T) {
	db := setupTroublePostgres(t)
	f := newTroubleFixture(t, db)
	ont := f.ont(t, "SN-STILL-DOWN", 1, 1)
	f.traps(t, ont.SerialNumber, 1, time.Hour)

	// No duration yet: the subscriber is still down, and an outage that has not
	// ended is the one most worth showing rather than the one to count as zero.
	require.NoError(t, db.Create(&models.ONTEvent{
		ONTID: ont.ID, EventType: models.EventTypeOffline,
		EventTime: time.Now().Add(-30 * time.Minute),
	}).Error)

	troubled, _, err := NewONTService(db).TroubledONTs(TroubledFilter{Window: 24 * time.Hour, Limit: 10, OLTID: nil})
	require.NoError(t, err)
	require.Len(t, troubled, 1)
	assert.InDelta(t, 30, troubled[0].DownMinutes, 1)
}

func TestTroubledONTsHonoursTheWindow(t *testing.T) {
	db := setupTroublePostgres(t)
	f := newTroubleFixture(t, db)
	recent := f.ont(t, "SN-RECENT", 1, 1)
	old := f.ont(t, "SN-OLD", 1, 2)
	f.traps(t, recent.SerialNumber, 5, 2*time.Hour)
	f.traps(t, old.SerialNumber, 50, 5*24*time.Hour)

	troubled, _, err := NewONTService(db).TroubledONTs(TroubledFilter{Window: 24 * time.Hour, Limit: 10, OLTID: nil})
	require.NoError(t, err)

	// The five-day-old storm is over; showing it above a live fault would send a
	// technician to the wrong address.
	require.Len(t, troubled, 1)
	assert.Equal(t, "SN-RECENT", troubled[0].SerialNumber)
}

func TestTroubledONTsLeavesOutTheUntroubled(t *testing.T) {
	db := setupTroublePostgres(t)
	f := newTroubleFixture(t, db)
	f.ont(t, "SN-FINE", 1, 1)

	troubled, _, err := NewONTService(db).TroubledONTs(TroubledFilter{Window: 24 * time.Hour, Limit: 10, OLTID: nil})
	require.NoError(t, err)
	assert.Empty(t, troubled)
}

func TestTroubledONTsRespectsTheLimit(t *testing.T) {
	db := setupTroublePostgres(t)
	f := newTroubleFixture(t, db)
	for i := 0; i < 5; i++ {
		ont := f.ont(t, "SN-"+uuid.NewString()[:8], 1, i+1)
		f.traps(t, ont.SerialNumber, i+1, time.Hour)
	}

	troubled, _, err := NewONTService(db).TroubledONTs(TroubledFilter{Window: 24 * time.Hour, Limit: 3, OLTID: nil})
	require.NoError(t, err)
	assert.Len(t, troubled, 3)
}

func (f troubleFixture) secondOLT(t *testing.T) uuid.UUID {
	t.Helper()
	var site models.Site
	require.NoError(t, f.db.First(&site).Error)
	olt := models.OLT{
		ID: uuid.New(), SiteID: site.ID, Name: "Bekasi", IPAddress: "172.30.30.2",
		SNMPCommunity: "public", Username: "a", Password: "b",
		SSHPort: 22, TelnetPort: 23, SNMPPort: 161,
		PreferredProtocol: models.OLTProtocolTelnet, Status: models.OLTStatusOnline,
	}
	require.NoError(t, f.db.Create(&olt).Error)
	return olt.ID
}

func TestTroubledONTsFiltersToOneOLT(t *testing.T) {
	db := setupTroublePostgres(t)
	f := newTroubleFixture(t, db)
	here := f.ont(t, "SN-HERE", 1, 1)
	f.traps(t, here.SerialNumber, 5, time.Hour)

	otherOLT := f.secondOLT(t)
	elsewhere := models.ONT{
		ID: uuid.New(), OLTID: otherOLT, PortID: 1, ONTID: 1,
		SerialNumber: "SN-ELSEWHERE", Name: "Lain", Status: models.ONTStatusOnline,
	}
	require.NoError(t, db.Create(&elsewhere).Error)
	require.NoError(t, db.Create(&models.ONTTrapEvent{
		OLTID: otherOLT, ReceivedAt: time.Now().Add(-time.Hour),
		TrapOID: ".1", SourceAddress: "172.30.30.2",
		SerialNumber: &elsewhere.SerialNumber, Varbinds: "x",
	}).Error)

	troubled, _, err := NewONTService(db).TroubledONTs(TroubledFilter{Window: 24 * time.Hour, Limit: 10, OLTID: &f.oltID})
	require.NoError(t, err)

	require.Len(t, troubled, 1)
	assert.Equal(t, "SN-HERE", troubled[0].SerialNumber)
}

func TestTroubledSummaryCountsBeyondThePageShown(t *testing.T) {
	db := setupTroublePostgres(t)
	f := newTroubleFixture(t, db)
	for i := 0; i < 6; i++ {
		ont := f.ont(t, "SN-"+uuid.NewString()[:8], 1, i+1)
		f.traps(t, ont.SerialNumber, i+1, time.Hour)
		f.outage(t, ont.ID, 60, time.Hour)
	}

	troubled, summary, err := NewONTService(db).TroubledONTs(TroubledFilter{Window: 24 * time.Hour, Limit: 2, OLTID: nil})
	require.NoError(t, err)

	// Two rows are shown, but a summary drawn from those two would tell an
	// operator a third of the truth.
	require.Len(t, troubled, 2)
	assert.EqualValues(t, 6, summary.ONTCount)
	assert.EqualValues(t, 6, summary.TotalDownMinutes)
}

func TestTroubledSummaryIsEmptyWhenNothingIsWrong(t *testing.T) {
	db := setupTroublePostgres(t)
	f := newTroubleFixture(t, db)
	f.ont(t, "SN-FINE-2", 1, 1)

	troubled, summary, err := NewONTService(db).TroubledONTs(TroubledFilter{Window: 24 * time.Hour, Limit: 10, OLTID: nil})
	require.NoError(t, err)

	assert.Empty(t, troubled)
	assert.EqualValues(t, 0, summary.ONTCount)
}

func TestTroubledONTsFiltersByStatus(t *testing.T) {
	db := setupTroublePostgres(t)
	f := newTroubleFixture(t, db)

	up := f.ont(t, "SN-UP", 1, 1)
	f.traps(t, up.SerialNumber, 10, time.Hour)

	down := f.ont(t, "SN-DOWN-NOW", 1, 2)
	f.traps(t, down.SerialNumber, 10, time.Hour)
	require.NoError(t, db.Model(&models.ONT{}).Where("id = ?", down.ID).
		Update("status", models.ONTStatusLOS).Error)

	los := models.ONTStatusLOS
	troubled, _, err := NewONTService(db).TroubledONTs(TroubledFilter{
		Window: 24 * time.Hour, Limit: 10, Status: &los,
	})
	require.NoError(t, err)

	require.Len(t, troubled, 1)
	assert.Equal(t, "SN-DOWN-NOW", troubled[0].SerialNumber)
}

func TestTroubledSummaryFollowsTheStatusFilter(t *testing.T) {
	db := setupTroublePostgres(t)
	f := newTroubleFixture(t, db)

	for i := 0; i < 3; i++ {
		ont := f.ont(t, "SN-ON-"+uuid.NewString()[:6], 1, i+1)
		f.traps(t, ont.SerialNumber, 5, time.Hour)
	}
	down := f.ont(t, "SN-OFF", 2, 1)
	f.traps(t, down.SerialNumber, 5, time.Hour)
	require.NoError(t, db.Model(&models.ONT{}).Where("id = ?", down.ID).
		Update("status", models.ONTStatusLOS).Error)

	los := models.ONTStatusLOS
	_, summary, err := NewONTService(db).TroubledONTs(TroubledFilter{
		Window: 24 * time.Hour, Limit: 10, Status: &los,
	})
	require.NoError(t, err)

	// A summary that ignored the filter would report four while the table
	// showed one, and the operator would trust the larger number.
	assert.EqualValues(t, 1, summary.ONTCount)
}

func TestTroubledONTsCarriesTheCardAsWellAsThePort(t *testing.T) {
	db := setupTroublePostgres(t)
	f := newTroubleFixture(t, db)
	slot := 9
	carded := models.ONT{
		ID: uuid.New(), OLTID: f.oltID, Slot: &slot, PortID: 8, ONTID: 3,
		SerialNumber: "SN-CARD9", Name: "Pelanggan", Status: models.ONTStatusOnline,
	}
	require.NoError(t, db.Create(&carded).Error)
	f.traps(t, carded.SerialNumber, 40, time.Hour)
	// Discovery has not always filled the slot in. Such a row is carried as card
	// zero rather than as "any card", so narrowing to a PON cannot pick it up by
	// accident.
	uncarded := f.ont(t, "SN-NOSLOT", 8, 3)
	f.traps(t, uncarded.SerialNumber, 10, time.Hour)

	troubled, _, err := NewONTService(db).TroubledONTs(TroubledFilter{Window: 24 * time.Hour, Limit: 10})
	require.NoError(t, err)
	require.Len(t, troubled, 2)

	assert.Equal(t, 9, troubled[0].Slot)
	assert.Equal(t, 0, troubled[1].Slot)
}
