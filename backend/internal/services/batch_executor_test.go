package services

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
	"go.uber.org/zap"
)

func setupBatchExecutor(t *testing.T, commander connectivity.OLTCommander, driver connectivity.Driver) (*BatchExecutor, *testFixtures, *JobService) {
	t.Helper()
	db := setupSnapshotTestDB(t)
	// In-memory SQLite gives each pooled connection its own empty database,
	// so concurrent provisioning goroutines must share a single connection.
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	olt, ont := seedOLTAndONT(t, db, models.OLTModelZTEC300)

	snapshotSvc := newSnapshotService(db, driver)
	auditSvc := NewAuditService(db, zap.NewNop())
	jobService := NewJobService(db, auditSvc)
	provisioner := NewOntProvisioningService(db, jobService, snapshotSvc, commander, auditSvc, zap.NewNop())
	executor := NewBatchExecutor(db, provisioner, jobService, snapshotSvc, zap.NewNop())

	return executor, &testFixtures{db: db, olt: olt, ont: ont, jobService: jobService}, jobService
}

func seedSecondONT(t *testing.T, fixtures *testFixtures) models.ONT {
	t.Helper()
	ont := models.ONT{
		OLTID:        fixtures.olt.ID,
		PortID:       fixtures.ont.PortID,
		ONTID:        fixtures.ont.ONTID + 1,
		Slot:         fixtures.ont.Slot,
		SerialNumber: "ZTEGC0A1B2C4",
		Status:       models.ONTStatusOnline,
	}
	require.NoError(t, fixtures.db.Create(&ont).Error)
	return ont
}

func TestBatchExecutor_Execute_AllSuccess(t *testing.T) {
	cmdr := &fakeCommander{}
	driver := &fakeDriver{
		model: models.OLTModelZTEC300,
		inventory: connectivity.ONTInventory{
			SerialNumber:    "ZTEGC0A1B2C3",
			Name:            "customer-42",
			DeviceType:      "F660",
			HardwareVersion: "V3",
		},
		metrics: &connectivity.ONTMetrics{SoftwareVersion: "V5.2.10"},
	}

	executor, fixtures, jobService := setupBatchExecutor(t, cmdr, driver)
	secondOnt := seedSecondONT(t, fixtures)

	templateID := uuid.New()
	result, err := executor.Execute(context.Background(), BatchConfig{
		TemplateID: &templateID,
		UserID:     uuid.New(),
		ONTIDs:     []uuid.UUID{fixtures.ont.ID, secondOnt.ID},
	})
	require.NoError(t, err)
	assert.Equal(t, models.BatchStatusSuccess, result.Job.Status)
	assert.Len(t, result.Succeeded, 2)
	assert.Empty(t, result.Failed)
	assert.Empty(t, result.RolledBack)

	// Verify batch job persisted as success
	batch, err := jobService.GetBatchJob(result.Job.ID)
	require.NoError(t, err)
	assert.Equal(t, models.BatchStatusSuccess, batch.Status)
}

func TestBatchExecutor_Execute_RequiresTemplate(t *testing.T) {
	cmdr := &fakeCommander{}
	driver := &fakeDriver{
		model:     models.OLTModelZTEC300,
		inventory: connectivity.ONTInventory{SerialNumber: "ZTEGC0A1B2C3"},
	}

	executor, fixtures, _ := setupBatchExecutor(t, cmdr, driver)

	_, err := executor.Execute(context.Background(), BatchConfig{
		UserID: uuid.New(),
		ONTIDs: []uuid.UUID{fixtures.ont.ID},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "template id")
}

func TestBatchExecutor_Execute_RequiresAtLeastOneONT(t *testing.T) {
	cmdr := &fakeCommander{}
	driver := &fakeDriver{
		model:     models.OLTModelZTEC300,
		inventory: connectivity.ONTInventory{SerialNumber: "ZTEGC0A1B2C3"},
	}

	executor, _, _ := setupBatchExecutor(t, cmdr, driver)
	templateID := uuid.New()

	_, err := executor.Execute(context.Background(), BatchConfig{
		TemplateID: &templateID,
		UserID:     uuid.New(),
		ONTIDs:     []uuid.UUID{},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one ONT")
}

func TestBatchExecutor_Execute_FailureTriggersRollback(t *testing.T) {
	cmdr := &fakeCommander{
		failOn: map[string]error{
			"commit": errors.New("commit rejected by OLT"),
		},
	}
	driver := &fakeDriver{
		model: models.OLTModelZTEC300,
		inventory: connectivity.ONTInventory{
			SerialNumber: "ZTEGC0A1B2C3",
		},
	}

	executor, fixtures, jobService := setupBatchExecutor(t, cmdr, driver)

	templateID := uuid.New()
	result, err := executor.Execute(context.Background(), BatchConfig{
		TemplateID: &templateID,
		UserID:     uuid.New(),
		ONTIDs:     []uuid.UUID{fixtures.ont.ID},
	})
	// Batch fails but Execute returns structured result, not error
	require.NoError(t, err)
	assert.NotEqual(t, models.BatchStatusSuccess, result.Job.Status)
	assert.Len(t, result.Failed, 1)

	batch, err := jobService.GetBatchJob(result.Job.ID)
	require.NoError(t, err)
	assert.Equal(t, models.BatchStatusFailed, batch.Status)
}

func TestBatchExecutor_GetBatchResult(t *testing.T) {
	cmdr := &fakeCommander{}
	driver := &fakeDriver{
		model:     models.OLTModelZTEC300,
		inventory: connectivity.ONTInventory{SerialNumber: "ZTEGC0A1B2C3"},
	}

	executor, fixtures, jobService := setupBatchExecutor(t, cmdr, driver)
	templateID := uuid.New()

	batch, err := jobService.CreateBatchJob(templateID, []uuid.UUID{fixtures.ont.ID}, uuid.New())
	require.NoError(t, err)

	fetched, err := executor.GetBatchResult(batch.ID)
	require.NoError(t, err)
	assert.Equal(t, batch.ID, fetched.ID)
}
