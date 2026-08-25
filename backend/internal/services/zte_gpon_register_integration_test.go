package services

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
	"go.uber.org/zap"
)

type zteIntegrationCommander struct {
	commands []string
	failOn   map[string]error
}

func (c *zteIntegrationCommander) ExecuteCommand(_ context.Context, command string) (*connectivity.CommandResult, error) {
	c.commands = append(c.commands, command)
	if err := c.failOn[command]; err != nil {
		return &connectivity.CommandResult{Success: false, Error: err.Error()}, nil
	}
	return &connectivity.CommandResult{Success: true, Output: "ok"}, nil
}

func (c *zteIntegrationCommander) BatchExecute(ctx context.Context, commands []string) ([]*connectivity.CommandResult, error) {
	results := make([]*connectivity.CommandResult, 0, len(commands))
	for _, command := range commands {
		result, err := c.ExecuteCommand(ctx, command)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, nil
}

type zteIntegrationSnapshotFake struct {
	after *ConfigSnapshot
}

func (f *zteIntegrationSnapshotFake) CaptureBeforeSnapshot(models.ONT) (*ConfigSnapshot, error) {
	return &ConfigSnapshot{ZTE: &ZTESnapshot{ServiceMode: "gpon-register"}}, nil
}

func (f *zteIntegrationSnapshotFake) CaptureAfterSnapshot(models.ONT) (*ConfigSnapshot, error) {
	return f.after, nil
}

type zteIntegrationRollbackFake struct {
	called bool
	snap   *ConfigSnapshot
}

func (f *zteIntegrationRollbackFake) RollbackToSnapshot(_ context.Context, _ models.ONT, snap *ConfigSnapshot) error {
	f.called = true
	f.snap = snap
	return nil
}

func newZTEGPONIntegrationService(t *testing.T, commander connectivity.OLTCommander, snapshot ZTESnapshotter, rollback ZTERollbacker) (*ZTEGPONRegisterService, *models.OLT) {
	t.Helper()
	db := setupJobTestDB(t)
	olt := &models.OLT{
		SiteID: uuid.New(), Name: "integration-c300", IPAddress: "192.0.2.10",
		Model: models.OLTModelZTEC300, Username: "admin", Password: "encrypted", TelnetPort: 23,
	}
	require.NoError(t, db.Create(olt).Error)
	return NewZTEGPONRegisterService(db, NewJobService(db, nil), snapshot, &fakeCommanderFactory{commander: commander}, rollback, zap.NewNop()), olt
}

func TestZTEGPONRegisterIntegration_Success(t *testing.T) {
	commander := &zteIntegrationCommander{}
	reqSnapshot := &zteIntegrationSnapshotFake{after: &ConfigSnapshot{ZTE: &ZTESnapshot{SerialNumber: "HWTCB403E8A0"}}}
	svc, olt := newZTEGPONIntegrationService(t, commander, reqSnapshot, &zteIntegrationRollbackFake{})

	job, err := svc.RegisterAndConfigure(context.Background(), validZTERequest(olt.ID), uuid.New())

	require.NoError(t, err)
	assert.Equal(t, models.ProvisioningStatusSuccess, job.Status)
	want := []string{
		"configure terminal",
		"interface gpon-olt_1/1/3",
		"onu 7 type HG8245H5 sn HWTCB403E8A0",
		"exit",
		"interface gpon-onu_1/1/3:7",
		"name customer-42",
		"tcont 1 name internet profile-name 100M",
		"gemport 1 name internet tcont 1",
		"service-port 1 vport 1 user-vlan 100 vlan 100",
		"wan-ip 1 mode pppoe username example-user password secret-password vlan-profile INTERNET",
		"exit",
		"commit",
	}
	assertOrderedCommands(t, commander.commands, want)
}

func assertOrderedCommands(t *testing.T, got, want []string) {
	t.Helper()
	assert.Equal(t, want, got)
}

func TestZTEGPONRegisterIntegration_CustomConflict(t *testing.T) {
	commander := &zteIntegrationCommander{}
	svc, olt := newZTEGPONIntegrationService(t, commander, &zteIntegrationSnapshotFake{}, &zteIntegrationRollbackFake{})
	req := validZTERequest(olt.ID)
	require.NoError(t, svc.db.Create(&models.ONT{OLTID: olt.ID, Slot: intPtr(req.Card), PortID: req.PON, ONTID: req.ONUID, SerialNumber: "ZXCV12345678"}).Error)

	_, err := svc.RegisterAndConfigure(context.Background(), req, uuid.New())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "already used")
	assert.Empty(t, commander.commands, "conflicts must be rejected before the commander is created")
}

func TestZTEGPONRegisterIntegration_AutoAllocation(t *testing.T) {
	commander := &zteIntegrationCommander{}
	svc, olt := newZTEGPONIntegrationService(t, commander, &zteIntegrationSnapshotFake{after: &ConfigSnapshot{ZTE: &ZTESnapshot{SerialNumber: "HWTCB403E8A0"}}}, &zteIntegrationRollbackFake{})
	req := validZTERequest(olt.ID)
	req.ONUIDMode = models.ZTEONUIDAuto
	req.ONUID = 0
	require.NoError(t, svc.db.Create(&models.ONT{OLTID: olt.ID, Slot: intPtr(req.Card), PortID: req.PON, ONTID: 1, SerialNumber: "ZXCV12345678"}).Error)

	job, err := svc.RegisterAndConfigure(context.Background(), req, uuid.New())

	require.NoError(t, err)
	assert.Equal(t, models.ProvisioningStatusSuccess, job.Status)
	assert.Contains(t, commander.commands, "onu 2 type HG8245H5 sn HWTCB403E8A0")
	var ont models.ONT
	require.NoError(t, svc.db.Where("olt_id = ? AND port_id = ? AND ont_id = ?", olt.ID, req.PON, 2).First(&ont).Error)
	assert.Equal(t, 2, ont.ONTID)
}

func TestZTEGPONRegisterIntegration_CommandFailureRollsBack(t *testing.T) {
	commander := &zteIntegrationCommander{failOn: map[string]error{"commit": errors.New("OLT rejected commit")}}
	rollback := &zteIntegrationRollbackFake{}
	svc, olt := newZTEGPONIntegrationService(t, commander, &zteIntegrationSnapshotFake{}, rollback)

	_, err := svc.RegisterAndConfigure(context.Background(), validZTERequest(olt.ID), uuid.New())

	require.Error(t, err)
	assert.True(t, rollback.called)
	assert.Equal(t, "gpon-register", rollback.snap.ZTE.ServiceMode)
	var jobs []models.ProvisioningJob
	require.NoError(t, svc.db.Find(&jobs).Error)
	require.Len(t, jobs, 1)
	assert.Equal(t, models.ProvisioningStatusRolledBack, jobs[0].Status)
}

func TestZTEGPONRegisterIntegration_ReadbackMismatchRollsBack(t *testing.T) {
	commander := &zteIntegrationCommander{}
	rollback := &zteIntegrationRollbackFake{}
	svc, olt := newZTEGPONIntegrationService(t, commander, &zteIntegrationSnapshotFake{after: &ConfigSnapshot{ZTE: &ZTESnapshot{SerialNumber: "WRONGSERIAL1"}}}, rollback)

	_, err := svc.RegisterAndConfigure(context.Background(), validZTERequest(olt.ID), uuid.New())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "verification failed")
	assert.True(t, rollback.called)
	assert.NotContains(t, strings.Join(commander.commands, " "), "<redacted>")
}

func TestZTEGPONRegisterIntegration_DuplicateCustomSubmissionHasOneActiveJob(t *testing.T) {
	commander := &zteIntegrationCommander{}
	svc, olt := newZTEGPONIntegrationService(t, commander, &zteIntegrationSnapshotFake{after: &ConfigSnapshot{ZTE: &ZTESnapshot{SerialNumber: "HWTCB403E8A0"}}}, &zteIntegrationRollbackFake{})
	req := validZTERequest(olt.ID)

	_, err := svc.RegisterAndConfigure(context.Background(), req, uuid.New())
	require.NoError(t, err)
	before := len(commander.commands)
	_, err = svc.RegisterAndConfigure(context.Background(), req, uuid.New())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "already used")
	assert.Equal(t, before, len(commander.commands))
	var jobs []models.ProvisioningJob
	require.NoError(t, svc.db.Find(&jobs).Error)
	assert.Len(t, jobs, 1)
}
