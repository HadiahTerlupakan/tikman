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

// fakeCommander satisfies connectivity.OLTCommander for provisioning tests
type fakeCommander struct {
	commands []string
	failOn   map[string]error
}

func (c *fakeCommander) ExecuteCommand(ctx context.Context, cmd string) (*connectivity.CommandResult, error) {
	c.commands = append(c.commands, cmd)
	if err, ok := c.failOn[cmd]; ok {
		return &connectivity.CommandResult{Success: false, Error: err.Error()}, nil
	}
	return &connectivity.CommandResult{Success: true, Output: "ok"}, nil
}

func (c *fakeCommander) BatchExecute(ctx context.Context, cmds []string) ([]*connectivity.CommandResult, error) {
	c.commands = append(c.commands, cmds...)
	results := make([]*connectivity.CommandResult, len(cmds))
	for i, cmd := range cmds {
		r, err := c.ExecuteCommand(ctx, cmd)
		if err != nil {
			return nil, err
		}
		results[i] = r
	}
	return results, nil
}

type testFixtures struct {
	db         interface{}
	olt        models.OLT
	ont        models.ONT
	jobService *JobService
}

func newProvisioningService(t *testing.T, model models.OLTModel, commander connectivity.OLTCommander, driver connectivity.Driver) (*OntProvisioningService, *testFixtures) {
	t.Helper()
	db := setupSnapshotTestDB(t)
	olt, ont := seedOLTAndONT(t, db, model)

	snapshotSvc := newSnapshotService(db, driver)
	auditSvc := NewAuditService(db, zap.NewNop())
	jobService := NewJobService(db, auditSvc)
	svc := NewOntProvisioningService(
		db,
		jobService,
		snapshotSvc,
		commander,
		auditSvc,
		zap.NewNop(),
	)
	return svc, &testFixtures{db: db, olt: olt, ont: ont, jobService: jobService}
}

func TestOntProvisioningService_ProvisionOnt_Success(t *testing.T) {
	cmdr := &fakeCommander{}
	driver := &fakeDriver{
		model: models.OLTModelZTEC300,
		inventory: connectivity.ONTInventory{
			SerialNumber:    "ZTEGC0A1B2C3",
			Name:            "customer-42",
			DeviceType:      "F660",
			HardwareVersion: "V3",
			IPAddress:       "10.0.1.5",
		},
		metrics: &connectivity.ONTMetrics{SoftwareVersion: "V5.2.10"},
	}

	svc, fixtures := newProvisioningService(t, models.OLTModelZTEC300, cmdr, driver)

	result, err := svc.ProvisionOnt(context.Background(), fixtures.ont.ID, uuid.New(), ProvisionConfig{
		ManualConfig: map[string]interface{}{
			"bandwidth": "100M",
			"vlan":      "100",
		},
	})
	require.NoError(t, err)
	assert.NotNil(t, result.Job)

	// Verify job reached terminal success state in the database
	jobs, total, err := svc.ListProvisioningJobsByONT(fixtures.ont.ID, 10, 0)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	assert.Equal(t, models.ProvisioningStatusSuccess, jobs[0].Status)
	assert.NotEmpty(t, cmdr.commands, "commands should have been sent to the commander")
}

func TestOntProvisioningService_ProvisionOnt_CommandFailure(t *testing.T) {
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

	svc, fixtures := newProvisioningService(t, models.OLTModelZTEC300, cmdr, driver)

	_, err := svc.ProvisionOnt(context.Background(), fixtures.ont.ID, uuid.New(), ProvisionConfig{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "provision execution failed")

	// Verify job was created and marked as failed
	jobs, total, err := svc.ListProvisioningJobsByONT(fixtures.ont.ID, 10, 0)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	assert.Equal(t, models.ProvisioningStatusFailed, jobs[0].Status)
}

func TestOntProvisioningService_ProvisionOnt_SnapshotFailure(t *testing.T) {
	cmdr := &fakeCommander{}
	driver := &fakeDriver{
		model:  models.OLTModelZTEC300,
		invErr: errors.New("SNMP timeout"),
	}

	svc, fixtures := newProvisioningService(t, models.OLTModelZTEC300, cmdr, driver)

	_, err := svc.ProvisionOnt(context.Background(), fixtures.ont.ID, uuid.New(), ProvisionConfig{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to capture before snapshot")
}

func TestOntProvisioningService_ProvisionOnt_ConcurrentJobBlocked(t *testing.T) {
	cmdr := &fakeCommander{}
	driver := &fakeDriver{
		model:     models.OLTModelZTEC300,
		inventory: connectivity.ONTInventory{SerialNumber: "ZTEGC0A1B2C3"},
	}

	svc, fixtures := newProvisioningService(t, models.OLTModelZTEC300, cmdr, driver)

	// Create a running job directly to block second attempt
	fixtureJob, err := fixtures.jobService.CreateProvisioningJob(
		fixtures.ont.ID, nil, nil, nil, uuid.New(),
	)
	require.NoError(t, err)
	// Transition to running to trigger ensureNoRunningJob check
	err = fixtures.jobService.UpdateStatusProvisioning(fixtureJob.ID, models.ProvisioningStatusRunning, nil)
	require.NoError(t, err)

	_, err = svc.ProvisionOnt(context.Background(), fixtures.ont.ID, uuid.New(), ProvisionConfig{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "another provisioning job is already running")
}
