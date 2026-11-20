package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/connectivity/shell"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/switchdriver"
	"gorm.io/gorm"
)

// fakeSwitchReader answers like a switch; neighborsErr fails the last
// inventory read, the way a switch that stops answering midway would.
type fakeSwitchReader struct {
	identity     switchdriver.Identity
	ports        []switchdriver.Port
	vlans        []switchdriver.VLAN
	neighbors    []switchdriver.Neighbor
	neighborsErr error
	config       string
	closed       int
}

func (f *fakeSwitchReader) Identify(context.Context) (switchdriver.Identity, error) {
	return f.identity, nil
}
func (f *fakeSwitchReader) ReadPorts(context.Context) ([]switchdriver.Port, error) {
	return f.ports, nil
}
func (f *fakeSwitchReader) ReadVLANs(context.Context) ([]switchdriver.VLAN, error) {
	return f.vlans, nil
}
func (f *fakeSwitchReader) ReadNeighbors(context.Context) ([]switchdriver.Neighbor, error) {
	return f.neighbors, f.neighborsErr
}
func (f *fakeSwitchReader) Backup(context.Context) (string, error) { return f.config, nil }
func (f *fakeSwitchReader) Close() error                           { f.closed++; return nil }

func newSwitchPollForTest(t *testing.T, reader *fakeSwitchReader, openErr error) (*SwitchPollService, *models.Switch, *gorm.DB) {
	t.Helper()
	switches, db := newSwitchServiceForTest(t)
	sw, err := switches.Create(validSwitchInput())
	require.NoError(t, err)
	poll := NewSwitchPollService(db, switches)
	poll.openDriver = func(context.Context, *models.Switch, string, string) (switchdriver.Reader, error) {
		if openErr != nil {
			return nil, openErr
		}
		return reader, nil
	}
	return poll, sw, db
}

func reloadSwitch(t *testing.T, db *gorm.DB, sw *models.Switch) models.Switch {
	t.Helper()
	var stored models.Switch
	require.NoError(t, db.First(&stored, "id = ?", sw.ID).Error)
	return stored
}

func TestSwitchPollInterfacesRecordsRatesAndPortState(t *testing.T) {
	poll, sw, db := newSwitchPollForTest(t, nil, nil)
	before := time.Now().Add(-time.Minute)
	prev := int64(1000)
	iface := models.SwitchInterface{SwitchID: sw.ID, Name: "10GE1/0/1", Kind: "sfp", OperStatus: "unknown",
		PrevInOctets: &prev, PrevOutOctets: &prev, PrevSampleAt: &before}
	require.NoError(t, db.Create(&iface).Error)
	poll.readCounters = func(context.Context, string, int, string) ([]connectivity.IfCounters, error) {
		return []connectivity.IfCounters{{IfIndex: 7, Name: "10GE1/0/1", InOctets: 1000 + 7500, OutOctets: 1000 + 750,
			SpeedMbps: 10000, OperUp: true}}, nil
	}
	poll.readIfState = func(context.Context, string, int, string) ([]connectivity.IfState, error) {
		return []connectivity.IfState{{IfIndex: 7, Alias: "uplink", AdminUp: true, InErrors: 3, OutErrors: 4, InDiscards: 5}}, nil
	}
	now := before.Add(time.Minute)

	require.NoError(t, poll.PollInterfaces(sw, now))

	var stored models.SwitchInterface
	require.NoError(t, db.First(&stored, "id = ?", iface.ID).Error)
	require.NotNil(t, stored.LastInBps)
	assert.InDelta(t, 1000, *stored.LastInBps, 0.01)
	assert.InDelta(t, 100, *stored.LastOutBps, 0.01)
	assert.Equal(t, "up", stored.OperStatus)
	assert.Equal(t, "uplink", stored.Alias)
	assert.True(t, stored.AdminUp)
	assert.Equal(t, int64(3), *stored.InErrors)
	assert.Equal(t, int64(4), *stored.OutErrors)
	assert.Equal(t, int64(5), *stored.InDiscards)
	assert.Equal(t, 7, *stored.IfIndex)
	var samples int64
	db.Model(&models.SwitchInterfaceMetric{}).Where("interface_id = ?", iface.ID).Count(&samples)
	assert.Equal(t, int64(1), samples)
	reloaded := reloadSwitch(t, db, sw)
	assert.Equal(t, models.SwitchStatusUp, reloaded.Status)
	require.NotNil(t, reloaded.LastSeenAt)
}

func TestSwitchPollInterfacesMarksDownWhenTheWalkFails(t *testing.T) {
	poll, sw, db := newSwitchPollForTest(t, nil, nil)
	poll.readCounters = func(context.Context, string, int, string) ([]connectivity.IfCounters, error) {
		return nil, errors.New("request timeout")
	}

	require.Error(t, poll.PollInterfaces(sw, time.Now()))
	assert.Equal(t, models.SwitchStatusDown, reloadSwitch(t, db, sw).Status)
}

func inventoryReader() *fakeSwitchReader {
	edge := true
	return &fakeSwitchReader{
		identity: switchdriver.Identity{Model: "CE6850-48S4Q-EI", Version: "V100R006C00SPC600"},
		ports: []switchdriver.Port{
			{Name: "10GE1/0/1", Kind: "sfp", Mode: "trunk", AdminUp: true, PVID: 1, AllowedVLANs: "10,20"},
			{Name: "10GE1/0/2", Kind: "sfp", Mode: "access", AdminUp: false, PVID: 30, STPEdge: &edge},
		},
		vlans:     []switchdriver.VLAN{{VID: 10, Name: "mgmt"}, {VID: 20}},
		neighbors: []switchdriver.Neighbor{{Port: "10GE1/0/1", Kind: "lldp", RemoteName: "SW-PEER", RemotePort: "10GE1/0/48"}},
	}
}

func TestSwitchInventoryMergesPortsAndReplacesVLANsAndNeighbors(t *testing.T) {
	reader := inventoryReader()
	poll, sw, db := newSwitchPollForTest(t, reader, nil)
	index, bps := 7, 1234.0
	snmpRow := models.SwitchInterface{SwitchID: sw.ID, Name: "10GE1/0/1", Kind: "sfp", OperStatus: "up",
		IfIndex: &index, LastInBps: &bps}
	require.NoError(t, db.Create(&snmpRow).Error)
	require.NoError(t, db.Create(&models.SwitchVLAN{SwitchID: sw.ID, VID: 99, Name: "old"}).Error)
	require.NoError(t, db.Create(&models.SwitchNeighbor{SwitchID: sw.ID, Port: "10GE1/0/9", Kind: "mac"}).Error)

	require.NoError(t, poll.Inventory(sw, time.Now()))

	reloaded := reloadSwitch(t, db, sw)
	assert.Equal(t, "CE6850-48S4Q-EI", *reloaded.Model)
	assert.Equal(t, "V100R006C00SPC600", *reloaded.Version)
	var merged models.SwitchInterface
	require.NoError(t, db.First(&merged, "id = ?", snmpRow.ID).Error)
	assert.Equal(t, "trunk", merged.Mode)
	assert.Equal(t, "10,20", merged.AllowedVLANs)
	assert.Equal(t, 7, *merged.IfIndex, "the SNMP reading survives the merge")
	assert.InDelta(t, 1234.0, *merged.LastInBps, 0.01)
	var added models.SwitchInterface
	require.NoError(t, db.First(&added, "switch_id = ? AND name = ?", sw.ID, "10GE1/0/2").Error)
	assert.Equal(t, "access", added.Mode)
	assert.Equal(t, 30, *added.PVID)
	assert.False(t, added.AdminUp)
	assert.True(t, *added.STPEdge)

	var vlans []models.SwitchVLAN
	require.NoError(t, db.Where("switch_id = ?", sw.ID).Order("vid").Find(&vlans).Error)
	require.Len(t, vlans, 2)
	assert.Equal(t, "mgmt", vlans[0].Name)
	var neighbors []models.SwitchNeighbor
	require.NoError(t, db.Where("switch_id = ?", sw.ID).Find(&neighbors).Error)
	require.Len(t, neighbors, 1)
	assert.Equal(t, "SW-PEER", neighbors[0].RemoteName)
	assert.Equal(t, 1, reader.closed)
}

func TestSwitchInventoryKeepsEarlierRowsWhenTheSwitchStopsAnswering(t *testing.T) {
	reader := inventoryReader()
	reader.neighborsErr = errors.New("read timeout")
	poll, sw, db := newSwitchPollForTest(t, reader, nil)
	old := models.SwitchInterface{SwitchID: sw.ID, Name: "10GE1/0/1", Kind: "sfp", OperStatus: "up", Mode: "access"}
	require.NoError(t, db.Create(&old).Error)
	require.NoError(t, db.Create(&models.SwitchVLAN{SwitchID: sw.ID, VID: 99, Name: "old"}).Error)
	require.NoError(t, db.Create(&models.SwitchNeighbor{SwitchID: sw.ID, Port: "10GE1/0/9", Kind: "mac"}).Error)

	require.Error(t, poll.Inventory(sw, time.Now()))

	var kept models.SwitchInterface
	require.NoError(t, db.First(&kept, "id = ?", old.ID).Error)
	assert.Equal(t, "access", kept.Mode)
	var ports, vlans, neighbors int64
	db.Model(&models.SwitchInterface{}).Where("switch_id = ?", sw.ID).Count(&ports)
	db.Model(&models.SwitchVLAN{}).Where("switch_id = ? AND vid = 99", sw.ID).Count(&vlans)
	db.Model(&models.SwitchNeighbor{}).Where("switch_id = ?", sw.ID).Count(&neighbors)
	assert.Equal(t, []int64{1, 1, 1}, []int64{ports, vlans, neighbors})
	assert.Equal(t, models.SwitchStatusUnknown, reloadSwitch(t, db, sw).Status, "only SNMP decides down")
	assert.Equal(t, 1, reader.closed)
}

func TestSwitchHostKeyMismatchSetsTheFlag(t *testing.T) {
	presented := "SHA256:b2Zmc2V0LWtleS1mb3ItdGVzdHMtb25seS0xMjM0NTY3"
	cause := errors.Join(errors.New("switchdriver: open 192.0.2.10"), shell.ErrHostKeyChanged{Presented: presented})
	poll, sw, db := newSwitchPollForTest(t, nil, cause)

	err := poll.Inventory(sw, time.Now())

	var changed shell.ErrHostKeyChanged
	require.ErrorAs(t, err, &changed)
	reloaded := reloadSwitch(t, db, sw)
	assert.True(t, reloaded.HostKeyChanged)
	require.NotNil(t, reloaded.PresentedFingerprint)
	assert.Equal(t, presented, *reloaded.PresentedFingerprint)
}

func TestSwitchUnpinnedHostKeyIsRecordedWithoutTheFlag(t *testing.T) {
	presented := "SHA256:Zmlyc3Qta2V5LWZvci10ZXN0cy1vbmx5LTEyMzQ1Njc4OQ"
	poll, sw, db := newSwitchPollForTest(t, nil, shell.ErrHostKeyUnpinned{Presented: presented})

	require.Error(t, poll.Backup(sw, time.Now()))

	reloaded := reloadSwitch(t, db, sw)
	assert.False(t, reloaded.HostKeyChanged)
	require.NotNil(t, reloaded.PresentedFingerprint)
	assert.Equal(t, presented, *reloaded.PresentedFingerprint)
}

func TestSwitchBackupStoresMaskedTextOnceForTheSameConfig(t *testing.T) {
	raw := "sysname SW-TEST\nsnmp-agent community read cipher %^%#secret%^%#\n"
	poll, sw, db := newSwitchPollForTest(t, &fakeSwitchReader{config: raw}, nil)
	now := time.Now()

	require.NoError(t, poll.Backup(sw, now))
	require.NoError(t, poll.Backup(sw, now.Add(time.Hour)))

	var backups []models.SwitchConfigBackup
	require.NoError(t, db.Where("switch_id = ?", sw.ID).Find(&backups).Error)
	require.Len(t, backups, 1)
	sum := sha256.Sum256([]byte(raw))
	assert.Equal(t, hex.EncodeToString(sum[:]), backups[0].RawSHA256)
	assert.Equal(t, "daily", backups[0].Reason)
	assert.NotContains(t, backups[0].Masked, "secret")
	assert.Contains(t, backups[0].Masked, "snmp-agent community read cipher ***")
	assert.Equal(t, len(backups[0].Masked), backups[0].Size)
}

func seedBackup(t *testing.T, db *gorm.DB, sw *models.Switch, at time.Time, reason, hash string) {
	t.Helper()
	b := models.SwitchConfigBackup{SwitchID: sw.ID, TakenAt: at, Reason: reason, RawSHA256: hash, Masked: hash}
	require.NoError(t, db.Create(&b).Error)
}

func backupHashes(t *testing.T, db *gorm.DB, sw *models.Switch) []string {
	t.Helper()
	var hashes []string
	require.NoError(t, db.Model(&models.SwitchConfigBackup{}).Where("switch_id = ?", sw.ID).
		Order("taken_at").Pluck("raw_sha256", &hashes).Error)
	return hashes
}

func TestSwitchBackupPrunesDailyBackupsOlderThan90Days(t *testing.T) {
	raw := "sysname SW-TEST\n"
	poll, sw, db := newSwitchPollForTest(t, &fakeSwitchReader{config: raw}, nil)
	now := time.Now()
	seedBackup(t, db, sw, now.Add(-100*24*time.Hour), "daily", "old-daily")
	seedBackup(t, db, sw, now.Add(-99*24*time.Hour), "manual", "old-manual")
	seedBackup(t, db, sw, now.Add(-80*24*time.Hour), "daily", "recent-daily")

	require.NoError(t, poll.Backup(sw, now))

	sum := sha256.Sum256([]byte(raw))
	assert.Equal(t, []string{"old-manual", "recent-daily", hex.EncodeToString(sum[:])}, backupHashes(t, db, sw))
}

// A config that never changes is stored once; pruning must not take that one
// backup away when it turns 90 days old.
func TestSwitchBackupKeepsTheLatestEvenWhenOld(t *testing.T) {
	raw := "sysname SW-TEST\n"
	poll, sw, db := newSwitchPollForTest(t, &fakeSwitchReader{config: raw}, nil)
	now := time.Now()
	sum := sha256.Sum256([]byte(raw))
	seedBackup(t, db, sw, now.Add(-100*24*time.Hour), "daily", hex.EncodeToString(sum[:]))

	require.NoError(t, poll.Backup(sw, now))

	assert.Equal(t, []string{hex.EncodeToString(sum[:])}, backupHashes(t, db, sw))
}
