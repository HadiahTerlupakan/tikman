package services

import (
	"context"
	"errors"
	"strings"
	"testing"

	"encoding/json"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
	"go.uber.org/zap"
)

type zteSnapshotFake struct{ before, after *ConfigSnapshot }

func (f *zteSnapshotFake) CaptureBeforeSnapshot(models.ONT) (*ConfigSnapshot, error) {
	return f.before, nil
}
func (f *zteSnapshotFake) CaptureAfterSnapshot(models.ONT) (*ConfigSnapshot, error) {
	return f.after, nil
}

type zteRollbackFake struct {
	called bool
	snap   *ConfigSnapshot
	err    error
}

func (f *zteRollbackFake) RollbackToSnapshot(_ context.Context, _ models.ONT, snap *ConfigSnapshot) error {
	f.called, f.snap = true, snap
	return f.err
}

func intPtr(value int) *int { return &value }

func validZTERequest(oltID uuid.UUID) models.ZTEGPONRegisterRequest {
	return models.ZTEGPONRegisterRequest{OLTID: oltID, Card: 1, PON: 3, ONUIDMode: models.ZTEONUIDCustom, ONUID: 7, SerialNumber: "HWTCB403E8A0", ONUType: "HG8245H5", Name: "customer-42", ServiceEnabled: true, VLANMode: "tag", ServiceType: "internet", VLANID: 100, DownloadProfile: "100M", UploadProfile: "100M", WANMode: "wan_ip", WANIPMode: "pppoe", VLANProfile: "INTERNET", PPPoEUsername: "example-user", PPPoEPassword: "secret-password"}
}

func newZTEServiceTest(t *testing.T, commander connectivity.OLTCommander, snapshot ZTESnapshotter, rollback ZTERollbacker) (*ZTEGPONRegisterService, *models.OLT) {
	db := setupJobTestDB(t)
	olt := &models.OLT{SiteID: uuid.New(), Name: "c300", IPAddress: "192.0.2.1", Model: models.OLTModelZTEC300, Username: "admin", Password: "encrypted", TelnetPort: 23}
	require.NoError(t, db.Create(olt).Error)
	jobs := NewJobService(db, nil)
	return NewZTEGPONRegisterService(db, jobs, snapshot, &fakeCommanderFactory{commander: commander}, rollback, zap.NewNop()), olt
}

func TestZTEGPONRegisterService_SuccessPersistsRedactedJobAndONT(t *testing.T) {
	commander := &fakeCommander{}
	svc, olt := newZTEServiceTest(t, commander, &zteSnapshotFake{after: &ConfigSnapshot{ZTE: &ZTESnapshot{SerialNumber: "HWTCB403E8A0"}}}, &zteRollbackFake{})
	job, err := svc.RegisterAndConfigure(context.Background(), validZTERequest(olt.ID), uuid.New())
	require.NoError(t, err)
	assert.Equal(t, models.ProvisioningStatusSuccess, job.Status)
	assert.NotContains(t, string(job.ConfigSnapshot), "secret-password")
	var persisted map[string]interface{}
	require.NoError(t, json.Unmarshal(job.ConfigSnapshot, &persisted))
	assert.Equal(t, "<redacted>", persisted["pppoe_password"])
	assert.Contains(t, commander.commands, "onu 7 type HG8245H5 sn HWTCB403E8A0")
	assert.True(t, containsCommandWith(commander.commands, "password secret-password"))
	assert.False(t, containsCommandWith(RedactZTECommands(commander.commands), "secret-password"))
	assert.NotContains(t, strings.Join(commander.commands, " "), "<redacted>")
	var ont models.ONT
	require.NoError(t, svc.db.Where("olt_id = ?", olt.ID).First(&ont).Error)
	assert.Equal(t, 7, ont.ONTID)
}

func containsCommandWith(commands []string, value string) bool {
	for _, command := range commands {
		if strings.Contains(command, value) {
			return true
		}
	}
	return false
}

func TestZTEGPONRegisterService_CommandFailureRollsBack(t *testing.T) {
	commander := &fakeCommander{failOn: map[string]error{"commit": errors.New("rejected")}}
	rollback := &zteRollbackFake{}
	svc, olt := newZTEServiceTest(t, commander, &zteSnapshotFake{}, rollback)
	_, err := svc.RegisterAndConfigure(context.Background(), validZTERequest(olt.ID), uuid.New())
	require.Error(t, err)
	assert.True(t, rollback.called)
}

func TestZTEGPONRegisterService_DuplicateCustomIDAvoidsCommander(t *testing.T) {
	commander := &fakeCommander{}
	svc, olt := newZTEServiceTest(t, commander, &zteSnapshotFake{}, &zteRollbackFake{})
	require.NoError(t, svc.db.Create(&models.ONT{OLTID: olt.ID, Slot: intPtr(1), PortID: 3, ONTID: 7, SerialNumber: "ZXCV12345678"}).Error)
	_, err := svc.RegisterAndConfigure(context.Background(), validZTERequest(olt.ID), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already used")
	assert.Empty(t, commander.commands)
}

func TestZTEGPONRegisterService_ConfigureExistingRejectsHSGQ(t *testing.T) {
	commander := &fakeCommander{}
	svc, olt := newZTEServiceTest(t, commander, &zteSnapshotFake{}, &zteRollbackFake{})
	olt.Model = models.OLTModelHSGQ
	require.NoError(t, svc.db.Save(olt).Error)
	ont := &models.ONT{OLTID: olt.ID, Slot: intPtr(1), PortID: 3, ONTID: 7, SerialNumber: "ZXCV12345678"}
	require.NoError(t, svc.db.Create(ont).Error)
	_, err := svc.ConfigureExisting(context.Background(), ont.ID, validZTERequest(olt.ID), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "C300 or C320")
	assert.Empty(t, commander.commands)
}

func TestZTEGPONRegisterService_ConfigureExistingSendsServiceOnly(t *testing.T) {
	commander := &fakeCommander{}
	svc, olt := newZTEServiceTest(t, commander, &zteSnapshotFake{
		after: &ConfigSnapshot{ZTE: &ZTESnapshot{SerialNumber: "HWTCB403E8A0"}},
	}, &zteRollbackFake{})
	ont := &models.ONT{
		OLTID: olt.ID, Slot: intPtr(1), PortID: 3, ONTID: 7,
		SerialNumber: "HWTCB403E8A0", DeviceType: "HG8245H5",
	}
	require.NoError(t, svc.db.Create(ont).Error)

	job, err := svc.ConfigureExisting(context.Background(), ont.ID, validZTERequest(olt.ID), uuid.New())

	require.NoError(t, err)
	assert.Equal(t, models.ProvisioningStatusSuccess, job.Status)
	assert.Contains(t, commander.commands, "interface gpon-onu_1/1/3:7")
	assert.NotContains(t, commander.commands, "onu 7 type HG8245H5 sn HWTCB403E8A0")
	assert.NotContains(t, strings.Join(commander.commands, " "), "interface gpon-olt_1/1/3")
}

func TestZTEGPONRegisterService_AutoIDIsPersisted(t *testing.T) {
	commander := &fakeCommander{}
	svc, olt := newZTEServiceTest(t, commander, &zteSnapshotFake{
		after: &ConfigSnapshot{ZTE: &ZTESnapshot{SerialNumber: "HWTCB403E8A0"}},
	}, &zteRollbackFake{})
	req := validZTERequest(olt.ID)
	req.ONUIDMode = models.ZTEONUIDAuto
	req.ONUID = 0

	job, err := svc.RegisterAndConfigure(context.Background(), req, uuid.New())
	require.NoError(t, err)
	assert.Equal(t, models.ProvisioningStatusSuccess, job.Status)

	var ont models.ONT
	require.NoError(t, svc.db.Where("olt_id = ?", olt.ID).First(&ont).Error)
	assert.Equal(t, 1, ont.ONTID)
	assert.Contains(t, commander.commands, "onu 1 type HG8245H5 sn HWTCB403E8A0")
}

func TestZTEGPONRegisterService_FailedNewRegistrationRemovesReservedONT(t *testing.T) {
	commander := &fakeCommander{failOn: map[string]error{"commit": errors.New("rejected")}}
	svc, olt := newZTEServiceTest(t, commander, &zteSnapshotFake{}, &zteRollbackFake{})

	_, err := svc.RegisterAndConfigure(context.Background(), validZTERequest(olt.ID), uuid.New())

	require.Error(t, err)
	var onts []models.ONT
	require.NoError(t, svc.db.Where("olt_id = ?", olt.ID).Find(&onts).Error)
	assert.Empty(t, onts)
}

func TestZTEGPONRegisterService_AutoIDIsReturnedAndRequestSerialIsCanonical(t *testing.T) {
	commander := &fakeCommander{}
	svc, olt := newZTEServiceTest(t, commander, &zteSnapshotFake{
		after: &ConfigSnapshot{ZTE: &ZTESnapshot{SerialNumber: "HWTCB403E8A0"}},
	}, &zteRollbackFake{})
	req := validZTERequest(olt.ID)
	req.ONUIDMode = models.ZTEONUIDAuto
	req.ONUID = 0
	req.SerialNumber = " hwtcb403e8a0 "

	job, err := svc.RegisterAndConfigure(context.Background(), req, uuid.New())

	require.NoError(t, err)
	var ont models.ONT
	require.NoError(t, svc.db.Where("olt_id = ?", olt.ID).First(&ont).Error)
	assert.Equal(t, 1, ont.ONTID)
	var persisted map[string]interface{}
	require.NoError(t, json.Unmarshal(job.ConfigSnapshot, &persisted))
	assert.Equal(t, "HWTCB403E8A0", persisted["serial_number"])
}

func TestZTEGPONRegisterService_ReadbackPositionMismatchRollsBack(t *testing.T) {
	commander := &fakeCommander{}
	rollback := &zteRollbackFake{}
	svc, olt := newZTEServiceTest(t, commander, &zteSnapshotFake{
		after: &ConfigSnapshot{
			ZTE:         &ZTESnapshot{SerialNumber: "HWTCB403E8A0"},
			RawReadings: map[string]interface{}{"slot": 1, "port": 3, "ont_id": 8},
		},
	}, rollback)

	ont := &models.ONT{OLTID: olt.ID, Slot: intPtr(1), PortID: 3, ONTID: 7, SerialNumber: "HWTCB403E8A0"}
	require.NoError(t, svc.db.Create(ont).Error)

	_, err := svc.ConfigureExisting(context.Background(), ont.ID, validZTERequest(olt.ID), uuid.New())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "verification failed")
	assert.True(t, rollback.called)
}

func TestZTEGPONRegisterService_ReadbackFailureRollsBack(t *testing.T) {
	commander := &fakeCommander{}
	rollback := &zteRollbackFake{}
	svc, olt := newZTEServiceTest(t, commander, &zteSnapshotFake{
		after: &ConfigSnapshot{ZTE: &ZTESnapshot{SerialNumber: "WRONGSERIAL1"}},
	}, rollback)

	_, err := svc.RegisterAndConfigure(context.Background(), validZTERequest(olt.ID), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "verification failed")
	assert.True(t, rollback.called, "readback failure must invoke rollback")

	var jobs []models.ProvisioningJob
	require.NoError(t, svc.db.Find(&jobs).Error)
	require.Len(t, jobs, 1)
	assert.Equal(t, models.ProvisioningStatusRolledBack, jobs[0].Status)
}
