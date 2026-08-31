package main

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const (
	oidONUOffline = ".1.3.6.1.4.1.3902.1082.500.10.3.1.1"
	oidONUOnline  = ".1.3.6.1.4.1.3902.1082.500.10.3.1.2"
	// The pair a C320 sends for the same states, which no OID list held.
	oidONUOfflineC320 = ".1.3.6.1.4.1.3902.1082.500.10.3.1.9"
	oidONUOnlineC320  = ".1.3.6.1.4.1.3902.1082.500.10.3.1.16"
	// A board notification: outside the ONU family and never about a subscriber.
	oidBoardNotice = ".1.3.6.1.4.1.3902.1082.20.10.3.1"

	commRaised  = "public@eventId=40366@eventLevel=minor@confirm@20260211174422"
	commMajor   = "public@eventId=5401608@eventLevel=major@confirm@20260830224728"
	commCleared = "public@eventId=135432@eventLevel=cleared@confirm@20260809003658"
	commNotice  = "public@eventId=135435@eventLevel=notification@20260809003658"
)

func setupApplyTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, models.AutoMigrate(db))
	return db
}

func newApplier(t *testing.T, db *gorm.DB) *statusApplier {
	t.Helper()
	return newStatusApplier(db, zap.NewNop())
}

func seedONT(t *testing.T, db *gorm.DB, oltID uuid.UUID, serial string, status models.ONTStatus) models.ONT {
	t.Helper()
	ont := models.ONT{
		ID:           uuid.New(),
		OLTID:        oltID,
		PortID:       1,
		ONTID:        39,
		SerialNumber: serial,
		Status:       status,
	}
	require.NoError(t, db.Create(&ont).Error)
	return ont
}

func statusOf(t *testing.T, db *gorm.DB, id uuid.UUID) models.ONTStatus {
	t.Helper()
	var ont models.ONT
	require.NoError(t, db.First(&ont, "id = ?", id).Error)
	return ont.Status
}

func countEvents(t *testing.T, db *gorm.DB, ontID uuid.UUID) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Model(&models.ONTEvent{}).Where("ont_id = ?", ontID).Count(&n).Error)
	return n
}

func TestTrapStatusReadsTheSeverityTheDeviceStates(t *testing.T) {
	cases := []struct {
		name      string
		oid       string
		community string
		want      models.ONTStatus
	}{
		{"C300 alarm raised", oidONUOffline, commMajor, models.ONTStatusOffline},
		{"C300 alarm cleared", oidONUOnline, commCleared, models.ONTStatusOnline},
		// The C320 pairs carry different OIDs for the same two states, which is
		// why the severity rather than the OID decides.
		{"C320 alarm raised", oidONUOfflineC320, commRaised, models.ONTStatusOffline},
		{"C320 alarm cleared", oidONUOnlineC320, commCleared, models.ONTStatusOnline},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, known := trapStatus(tc.oid, tc.community)
			assert.True(t, known)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestTrapStatusRefusesWhatIsNotAnONUAlarm(t *testing.T) {
	cases := []struct {
		name      string
		oid       string
		community string
	}{
		// Informational, not an alarm: it names no subscriber state.
		{"notification level", oidONUOffline, commNotice},
		// A severity on a board notification is not a subscriber's status, and
		// the family prefix is what keeps this path off every other subtree.
		{"outside the ONU family", oidBoardNotice, commRaised},
		{"community carries no level", oidONUOffline, "public"},
		{"empty community", oidONUOffline, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, known := trapStatus(tc.oid, tc.community)
			assert.False(t, known)
		})
	}
}

func TestApplyWritesTheStatusATrapReports(t *testing.T) {
	db := setupApplyTestDB(t)
	oltID := uuid.New()
	ont := seedONT(t, db, oltID, "ZTEGCAFF2C7F", models.ONTStatusOnline)

	newApplier(t, db).apply(Trap{OLTID: oltID, OID: oidONUOffline, Community: commMajor},
		onuIdentity{SerialNumber: "ZTEGCAFF2C7F"})

	assert.Equal(t, models.ONTStatusOffline, statusOf(t, db, ont.ID))
	assert.Equal(t, int64(1), countEvents(t, db, ont.ID),
		"a status the trap changed needs an event, or availability cannot measure it")
}

func TestApplyWritesNothingWhenTheONTAlreadyHoldsThatStatus(t *testing.T) {
	db := setupApplyTestDB(t)
	oltID := uuid.New()
	ont := seedONT(t, db, oltID, "ZTEGCAFF2C7F", models.ONTStatusOnline)

	newApplier(t, db).apply(Trap{OLTID: oltID, OID: oidONUOnline, Community: commCleared},
		onuIdentity{SerialNumber: "ZTEGCAFF2C7F"})

	// A flapping ONU repeats its state; the poller does not open an event for an
	// unchanged status and neither may this.
	assert.Equal(t, int64(0), countEvents(t, db, ont.ID))
}

func TestApplyIgnoresATrapThatIsNotAnONUAlarm(t *testing.T) {
	db := setupApplyTestDB(t)
	oltID := uuid.New()
	ont := seedONT(t, db, oltID, "ZTEGCAFF2C7F", models.ONTStatusOnline)

	newApplier(t, db).apply(Trap{OLTID: oltID, OID: oidBoardNotice, Community: commRaised},
		onuIdentity{SerialNumber: "ZTEGCAFF2C7F"})

	assert.Equal(t, models.ONTStatusOnline, statusOf(t, db, ont.ID))
	assert.Equal(t, int64(0), countEvents(t, db, ont.ID))
}

func TestApplyIgnoresATrapNamingAnONTTheOLTHasNoRowFor(t *testing.T) {
	db := setupApplyTestDB(t)
	oltID := uuid.New()
	ont := seedONT(t, db, oltID, "ZTEGCAFF2C7F", models.ONTStatusOnline)

	newApplier(t, db).apply(Trap{OLTID: oltID, OID: oidONUOffline, Community: commMajor},
		onuIdentity{SerialNumber: "GGCLA6B8DC90"})

	// An unregistered ONU is evidence worth keeping, which the trap record
	// already holds. It must not touch a subscriber we do have.
	assert.Equal(t, models.ONTStatusOnline, statusOf(t, db, ont.ID))
}

func TestApplyRefusesAnONTBelongingToAnotherOLT(t *testing.T) {
	db := setupApplyTestDB(t)
	ont := seedONT(t, db, uuid.New(), "ZTEGCAFF2C7F", models.ONTStatusOnline)

	// Same serial, different chassis. Serial numbers are not unique across
	// OLTs, so matching on serial alone would let one site write another's
	// subscriber offline.
	newApplier(t, db).apply(Trap{OLTID: uuid.New(), OID: oidONUOffline, Community: commMajor},
		onuIdentity{SerialNumber: "ZTEGCAFF2C7F"})

	assert.Equal(t, models.ONTStatusOnline, statusOf(t, db, ont.ID))
}

func TestApplyIgnoresATrapThatNamesNoSerial(t *testing.T) {
	db := setupApplyTestDB(t)
	oltID := uuid.New()
	ont := seedONT(t, db, oltID, "ZTEGCAFF2C7F", models.ONTStatusOnline)

	newApplier(t, db).apply(Trap{OLTID: oltID, OID: oidONUOffline, Community: commMajor}, onuIdentity{})

	assert.Equal(t, models.ONTStatusOnline, statusOf(t, db, ont.ID))
}

func TestApplyLeavesAMoreSpecificDownReasonAlone(t *testing.T) {
	db := setupApplyTestDB(t)
	oltID := uuid.New()
	ont := seedONT(t, db, oltID, "ZTEGCAFF2C7F", models.ONTStatusLOS)

	newApplier(t, db).apply(Trap{OLTID: oltID, OID: oidONUOffline, Community: commMajor},
		onuIdentity{SerialNumber: "ZTEGCAFF2C7F"})

	// The poller reads a phase state and can say los or dying_gasp; the trap
	// only says down. Overwriting los with the vaguer offline loses the
	// diagnosis, and the next poll writes it straight back — leaving the
	// subscriber's status oscillating between two words for the same outage.
	assert.Equal(t, models.ONTStatusLOS, statusOf(t, db, ont.ID))
}

func TestApplyStillReportsAnONTComingBackUp(t *testing.T) {
	db := setupApplyTestDB(t)
	oltID := uuid.New()
	ont := seedONT(t, db, oltID, "ZTEGCAFF2C7F", models.ONTStatusLOS)

	newApplier(t, db).apply(Trap{OLTID: oltID, OID: oidONUOnline, Community: commCleared},
		onuIdentity{SerialNumber: "ZTEGCAFF2C7F"})

	assert.Equal(t, models.ONTStatusOnline, statusOf(t, db, ont.ID))
	assert.Equal(t, int64(1), countEvents(t, db, ont.ID))
}
