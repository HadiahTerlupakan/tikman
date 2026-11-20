package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/connectivity/shell"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/switchdriver"
	"gorm.io/gorm"
)

func newSwitchQueryForTest(t *testing.T) (*SwitchQueryService, *models.Switch, *gorm.DB) {
	t.Helper()
	switches, db := newSwitchServiceForTest(t)
	sw, err := switches.Create(validSwitchInput())
	require.NoError(t, err)
	return NewSwitchQueryService(db), sw, db
}

func TestSwitchQueryListsPortsVLANsAndNeighborsInOrder(t *testing.T) {
	query, sw, db := newSwitchQueryForTest(t)
	require.NoError(t, db.Create(&[]models.SwitchInterface{
		{SwitchID: sw.ID, Name: "10GE1/0/2", Kind: "sfp", OperStatus: "up"},
		{SwitchID: sw.ID, Name: "10GE1/0/1", Kind: "sfp", OperStatus: "down"},
	}).Error)
	require.NoError(t, db.Create(&[]models.SwitchVLAN{{SwitchID: sw.ID, VID: 20}, {SwitchID: sw.ID, VID: 10}}).Error)
	require.NoError(t, db.Create(&[]models.SwitchNeighbor{
		{SwitchID: sw.ID, Port: "10GE1/0/2", Kind: "lldp"}, {SwitchID: sw.ID, Port: "10GE1/0/1", Kind: "mac"},
	}).Error)

	ports, err := query.Interfaces(sw.ID)
	require.NoError(t, err)
	require.Len(t, ports, 2)
	assert.Equal(t, "10GE1/0/1", ports[0].Name)

	vlans, err := query.VLANs(sw.ID)
	require.NoError(t, err)
	assert.Equal(t, 10, vlans[0].VID)

	neighbors, err := query.Neighbors(sw.ID)
	require.NoError(t, err)
	assert.Equal(t, "10GE1/0/1", neighbors[0].Port)
}

func seedMaskedBackup(t *testing.T, db *gorm.DB, sw *models.Switch, at time.Time, masked string) uuid.UUID {
	t.Helper()
	b := models.SwitchConfigBackup{SwitchID: sw.ID, TakenAt: at, Reason: backupReasonDaily,
		RawSHA256: uuid.NewString(), Masked: masked, Size: len(masked)}
	require.NoError(t, db.Create(&b).Error)
	return b.ID
}

func TestSwitchQueryBackupListLeavesOutTheTextNewestFirst(t *testing.T) {
	query, sw, db := newSwitchQueryForTest(t)
	now := time.Now()
	seedMaskedBackup(t, db, sw, now.Add(-time.Hour), "old config")
	newest := seedMaskedBackup(t, db, sw, now, "new config")

	list, err := query.Backups(sw.ID)

	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, newest, list[0].ID)
	assert.Equal(t, len("new config"), list[0].Size)
}

func TestSwitchQueryBackupIsScopedToItsSwitch(t *testing.T) {
	query, sw, db := newSwitchQueryForTest(t)
	id := seedMaskedBackup(t, db, sw, time.Now(), "config")

	got, err := query.Backup(sw.ID, id)
	require.NoError(t, err)
	assert.Equal(t, "config", got.Masked)

	_, err = query.Backup(uuid.New(), id)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestSwitchQueryDiffComparesTheMaskedTexts(t *testing.T) {
	query, sw, db := newSwitchQueryForTest(t)
	from := seedMaskedBackup(t, db, sw, time.Now().Add(-time.Hour), diffBefore)
	to := seedMaskedBackup(t, db, sw, time.Now(), diffAfter)

	got, err := query.Diff(sw.ID, from, to)

	require.NoError(t, err)
	assert.Contains(t, got, "-vlan batch 10 20\n+vlan batch 10 20 30\n")

	_, err = query.Diff(sw.ID, from, uuid.New())
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestSwitchTrustHostKeyPinsThePresentedFingerprintAndClearsTheFlag(t *testing.T) {
	switches, db := newSwitchServiceForTest(t)
	in := validSwitchInput()
	in.Protocol = models.SwitchProtocolSSH
	sw, err := switches.Create(in)
	require.NoError(t, err)
	presented := "SHA256:bmV3LWtleS1mb3ItdGVzdHMtb25seS0xMjM0NTY3ODk"
	require.NoError(t, db.Model(sw).Updates(map[string]interface{}{
		"presented_fingerprint": presented, "host_key_changed": true}).Error)

	trusted, err := switches.TrustHostKey(sw.ID)

	require.NoError(t, err)
	assert.Equal(t, presented, trusted.HostKeyFingerprint)
	assert.False(t, trusted.HostKeyChanged)
}

func TestSwitchTrustHostKeyRefusesWhenThereIsNothingToTrust(t *testing.T) {
	switches, db := newSwitchServiceForTest(t)
	telnet, err := switches.Create(validSwitchInput())
	require.NoError(t, err)
	require.NoError(t, db.Model(telnet).Update("presented_fingerprint", "SHA256:x").Error)
	_, err = switches.TrustHostKey(telnet.ID)
	assert.ErrorIs(t, err, ErrValidation, "telnet has no host key")

	in := validSwitchInput()
	in.Protocol = models.SwitchProtocolSSH
	ssh, err := switches.Create(in)
	require.NoError(t, err)
	_, err = switches.TrustHostKey(ssh.ID)
	assert.ErrorIs(t, err, ErrValidation, "no fingerprint was presented yet")
}

func TestSwitchProbeReturnsTheIdentityAndClosesTheSession(t *testing.T) {
	reader := &fakeSwitchReader{identity: switchdriver.Identity{Model: "CE6850", Version: "V100R006C00"}}
	var opened *models.Switch
	probe := &SwitchProber{open: func(_ context.Context, sw *models.Switch, community, password string) (switchdriver.Reader, error) {
		opened = sw
		assert.Equal(t, "community-secret", community)
		assert.Equal(t, "password-secret", password)
		return reader, nil
	}}

	id, err := probe.Probe(context.Background(), validSwitchInput())

	require.NoError(t, err)
	assert.Equal(t, "CE6850", id.Model)
	assert.Equal(t, 1, reader.closed)
	assert.Equal(t, 23, opened.Port)
	assert.Empty(t, opened.HostKeyFingerprint)
}

func TestSwitchProbeSurfacesTheUnpinnedFingerprint(t *testing.T) {
	cause := errors.Join(errors.New("open"), shell.ErrHostKeyUnpinned{Presented: "SHA256:abc"})
	probe := &SwitchProber{open: func(context.Context, *models.Switch, string, string) (switchdriver.Reader, error) {
		return nil, cause
	}}
	in := validSwitchInput()
	in.Protocol = models.SwitchProtocolSSH

	_, err := probe.Probe(context.Background(), in)

	var unpinned shell.ErrHostKeyUnpinned
	require.ErrorAs(t, err, &unpinned)
	assert.Equal(t, "SHA256:abc", unpinned.Presented)
}

func TestSwitchProbeRefusesAVendorWithoutADriver(t *testing.T) {
	probe := &SwitchProber{open: func(context.Context, *models.Switch, string, string) (switchdriver.Reader, error) {
		t.Fatal("must not open")
		return nil, nil
	}}
	in := validSwitchInput()
	in.Vendor = models.SwitchVendorCRS

	_, err := probe.Probe(context.Background(), in)

	assert.ErrorIs(t, err, ErrValidation)
}
