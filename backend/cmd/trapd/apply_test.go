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
	oidUnproven   = ".1.3.6.1.4.1.3902.1082.500.10.3.1.16"
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

func TestTrapStatusReadsTheOIDsTheEvidenceEstablished(t *testing.T) {
	cases := map[string]models.ONTStatus{
		oidONUOffline:                          models.ONTStatusOffline,
		oidONUOnline:                           models.ONTStatusOnline,
		".1.3.6.1.4.1.3902.1082.500.10.3.1.23": models.ONTStatusOffline,
		".1.3.6.1.4.1.3902.1082.500.10.3.1.24": models.ONTStatusOnline,
		// Agents are inconsistent about the leading dot, and a status must not
		// hinge on punctuation.
		"1.3.6.1.4.1.3902.1082.500.10.3.1.2": models.ONTStatusOnline,
	}

	for oid, want := range cases {
		got, known := trapStatus(oid)
		assert.True(t, known, oid)
		assert.Equal(t, want, got, oid)
	}
}

func TestTrapStatusRefusesAnOIDTheEvidenceDoesNotEstablish(t *testing.T) {
	// .9, .10, .15 and .16 pair up like the others but were seen too rarely
	// before a status transition to say which way round they run. Acting on
	// them would be the guess this whole path exists to avoid.
	for _, oid := range []string{
		".1.3.6.1.4.1.3902.1082.500.10.3.1.9",
		".1.3.6.1.4.1.3902.1082.500.10.3.1.10",
		".1.3.6.1.4.1.3902.1082.500.10.3.1.15",
		oidUnproven,
		".1.3.6.1.4.1.3902.1082.500.20.3.1.129",
	} {
		_, known := trapStatus(oid)
		assert.False(t, known, oid)
	}
}

func TestApplyWritesTheStatusATrapReports(t *testing.T) {
	db := setupApplyTestDB(t)
	oltID := uuid.New()
	ont := seedONT(t, db, oltID, "ZTEGCAFF2C7F", models.ONTStatusOnline)

	newApplier(t, db).apply(Trap{OLTID: oltID, OID: oidONUOffline},
		onuIdentity{SerialNumber: "ZTEGCAFF2C7F"})

	assert.Equal(t, models.ONTStatusOffline, statusOf(t, db, ont.ID))
	assert.Equal(t, int64(1), countEvents(t, db, ont.ID),
		"a status the trap changed needs an event, or availability cannot measure it")
}

func TestApplyWritesNothingWhenTheONTAlreadyHoldsThatStatus(t *testing.T) {
	db := setupApplyTestDB(t)
	oltID := uuid.New()
	ont := seedONT(t, db, oltID, "ZTEGCAFF2C7F", models.ONTStatusOnline)

	newApplier(t, db).apply(Trap{OLTID: oltID, OID: oidONUOnline},
		onuIdentity{SerialNumber: "ZTEGCAFF2C7F"})

	// A flapping ONU repeats its state; the poller does not open an event for an
	// unchanged status and neither may this.
	assert.Equal(t, int64(0), countEvents(t, db, ont.ID))
}

func TestApplyIgnoresATrapWhoseOIDMeaningIsNotEstablished(t *testing.T) {
	db := setupApplyTestDB(t)
	oltID := uuid.New()
	ont := seedONT(t, db, oltID, "ZTEGCAFF2C7F", models.ONTStatusOnline)

	newApplier(t, db).apply(Trap{OLTID: oltID, OID: oidUnproven},
		onuIdentity{SerialNumber: "ZTEGCAFF2C7F"})

	assert.Equal(t, models.ONTStatusOnline, statusOf(t, db, ont.ID))
	assert.Equal(t, int64(0), countEvents(t, db, ont.ID))
}

func TestApplyIgnoresATrapNamingAnONTTheOLTHasNoRowFor(t *testing.T) {
	db := setupApplyTestDB(t)
	oltID := uuid.New()
	ont := seedONT(t, db, oltID, "ZTEGCAFF2C7F", models.ONTStatusOnline)

	newApplier(t, db).apply(Trap{OLTID: oltID, OID: oidONUOffline},
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
	newApplier(t, db).apply(Trap{OLTID: uuid.New(), OID: oidONUOffline},
		onuIdentity{SerialNumber: "ZTEGCAFF2C7F"})

	assert.Equal(t, models.ONTStatusOnline, statusOf(t, db, ont.ID))
}

func TestApplyIgnoresATrapThatNamesNoSerial(t *testing.T) {
	db := setupApplyTestDB(t)
	oltID := uuid.New()
	ont := seedONT(t, db, oltID, "ZTEGCAFF2C7F", models.ONTStatusOnline)

	newApplier(t, db).apply(Trap{OLTID: oltID, OID: oidONUOffline}, onuIdentity{})

	assert.Equal(t, models.ONTStatusOnline, statusOf(t, db, ont.ID))
}

func TestApplyLeavesAMoreSpecificDownReasonAlone(t *testing.T) {
	db := setupApplyTestDB(t)
	oltID := uuid.New()
	ont := seedONT(t, db, oltID, "ZTEGCAFF2C7F", models.ONTStatusLOS)

	newApplier(t, db).apply(Trap{OLTID: oltID, OID: oidONUOffline},
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

	newApplier(t, db).apply(Trap{OLTID: oltID, OID: oidONUOnline},
		onuIdentity{SerialNumber: "ZTEGCAFF2C7F"})

	assert.Equal(t, models.ONTStatusOnline, statusOf(t, db, ont.ID))
	assert.Equal(t, int64(1), countEvents(t, db, ont.ID))
}
