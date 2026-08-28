package services

import (
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

func seedReservedONT(t *testing.T, db *gorm.DB, slot *int) models.ONT {
	t.Helper()
	ont := models.ONT{
		ID: uuid.New(), OLTID: uuid.New(), PortID: 1, ONTID: 15, Slot: slot,
		SerialNumber: "HWTCB403E8A0", Status: models.ONTStatusUnknown,
	}
	require.NoError(t, db.Create(&ont).Error)
	return ont
}

// Registering against a busy OLT can take minutes. An operator who reloads and
// submits again used to get a raw Postgres constraint violation, which reads
// as a fault in TikMan rather than as the first attempt still running.
func TestReserveONUErrorNamesARunningRegistration(t *testing.T) {
	db := setupTestDB(t)
	slot := 3
	ont := seedReservedONT(t, db, &slot)
	require.NoError(t, db.Create(&models.ProvisioningJob{
		ID: uuid.New(), ONTID: ont.ID, Status: "running",
	}).Error)

	err := reserveONUError(db, "HWTCB403E8A0",
		fmt.Errorf(`ERROR: duplicate key value violates unique constraint "idx_onts_serial_number"`))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "already running")
}

// With no job in flight the serial is simply taken, and the message says where
// so the operator can go and look at it.
func TestReserveONUErrorNamesThePositionAlreadyHoldingTheSerial(t *testing.T) {
	db := setupTestDB(t)
	slot := 3
	seedReservedONT(t, db, &slot)

	err := reserveONUError(db, "HWTCB403E8A0",
		fmt.Errorf(`ERROR: duplicate key value violates unique constraint "idx_onts_serial_number"`))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "1/3/1:15")
}

// Any other insert failure keeps its own cause rather than being reported as a
// duplicate serial.
func TestReserveONUErrorPassesThroughAnUnrelatedFailure(t *testing.T) {
	db := setupTestDB(t)

	err := reserveONUError(db, "HWTCB403E8A0", fmt.Errorf("disk is full"))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "disk is full")
	assert.NotContains(t, err.Error(), "already")
}
