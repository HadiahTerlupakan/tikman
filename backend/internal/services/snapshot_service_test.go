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
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// fakeDriver satisfies connectivity.Driver with canned data so snapshot tests
// never touch a network. inventory and metrics are looked up by ONT location;
// invErr/metricsErr simulate an unreachable OLT.
type fakeDriver struct {
	model            models.OLTModel
	inventory        connectivity.ONTInventory
	metrics          *connectivity.ONTMetrics
	invErr           error
	metricsErr       error
	inventoryByONTID map[int]connectivity.ONTInventory
}

func (d *fakeDriver) Model() models.OLTModel { return d.model }

func (d *fakeDriver) WalkStatuses(string, string, int) (map[connectivity.ONTLocation]int, error) {
	return nil, connectivity.ErrUnsupported
}

func (d *fakeDriver) WalkMetrics(string, string, int) (map[connectivity.ONTLocation]connectivity.ONTMetrics, error) {
	return nil, connectivity.ErrUnsupported
}

func (d *fakeDriver) Inventory(_ string, _ string, _ int, locations []connectivity.ONTLocation) (map[connectivity.ONTLocation]connectivity.ONTInventory, error) {
	if d.invErr != nil {
		return nil, d.invErr
	}
	out := make(map[connectivity.ONTLocation]connectivity.ONTInventory, len(locations))
	for _, loc := range locations {
		if inventory, ok := d.inventoryByONTID[loc.ONTID]; ok {
			out[loc] = inventory
			continue
		}
		out[loc] = d.inventory
	}
	return out, nil
}

func (d *fakeDriver) InventoryByPort(ip string, community string, snmpPort int, locations []connectivity.ONTLocation, report func([]connectivity.ONTLocation, map[connectivity.ONTLocation]connectivity.ONTInventory)) error {
	inventory, err := d.Inventory(ip, community, snmpPort, locations)
	if err != nil {
		return err
	}
	report(locations, inventory)
	return nil
}

func (d *fakeDriver) QueryONTMetrics(string, string, int, int, int, int) (*connectivity.ONTMetrics, error) {
	if d.metricsErr != nil {
		return nil, d.metricsErr
	}
	return d.metrics, nil
}

func (d *fakeDriver) WalkTrafficRates(string, string, int) (map[connectivity.ONTLocation]connectivity.ONUTrafficRates, error) {
	return nil, connectivity.ErrUnsupported
}

func (d *fakeDriver) QueryTrafficRates(string, string, int, int, int, int) (*connectivity.ONUTrafficRates, error) {
	return nil, connectivity.ErrUnsupported
}

func (d *fakeDriver) WalkUnconfigured(context.Context, string, string, int) ([]connectivity.UnconfiguredONU, error) {
	return nil, connectivity.ErrUnsupported
}

func setupSnapshotTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, models.AutoMigrate(db))
	return db
}

// seedOLTAndONT persists the OLT (whose Model picks the read dialect) and the
// ONT the snapshot is captured for. The ONT carries only identity; everything
// else is read live from the driver.
func seedOLTAndONT(t *testing.T, db *gorm.DB, model models.OLTModel) (models.OLT, models.ONT) {
	t.Helper()
	olt := models.OLT{
		SiteID:        uuid.New(),
		Name:          "test-olt",
		IPAddress:     "10.0.0.1",
		SNMPPort:      161,
		SNMPCommunity: "public",
		Model:         model,
		Username:      "admin",
		Password:      "secret",
	}
	require.NoError(t, db.Create(&olt).Error)

	slot := 1
	ont := models.ONT{
		OLTID:        olt.ID,
		PortID:       3,
		ONTID:        7,
		Slot:         &slot,
		SerialNumber: "ZTEGC0A1B2C3",
		Status:       models.ONTStatusOnline,
	}
	require.NoError(t, db.Create(&ont).Error)
	return olt, ont
}

func newSnapshotService(db *gorm.DB, driver connectivity.Driver) *SnapshotService {
	resolver := func(models.OLTModel) (connectivity.Driver, error) { return driver, nil }
	return NewSnapshotService(db, resolver, zap.NewNop())
}

func TestSnapshotService_CaptureBeforeSnapshot_ZTE(t *testing.T) {
	db := setupSnapshotTestDB(t)
	olt, ont := seedOLTAndONT(t, db, models.OLTModelZTEC300)

	driver := &fakeDriver{
		model: models.OLTModelZTEC300,
		inventory: connectivity.ONTInventory{
			SerialNumber:    "ZTEGC0A1B2C3",
			Name:            "customer-42",
			DeviceType:      "F660",
			HardwareVersion: "V9.0",
			IPAddress:       "192.168.100.7",
		},
		metrics: &connectivity.ONTMetrics{SoftwareVersion: "F660V9"},
	}
	svc := newSnapshotService(db, driver)

	snap, err := svc.CaptureBeforeSnapshot(ont)
	require.NoError(t, err)
	require.NotNil(t, snap)

	assert.Equal(t, ont.ID, snap.OntID)
	assert.False(t, snap.Timestamp.IsZero())
	require.NotNil(t, snap.ZTE)
	assert.Nil(t, snap.HSGQ)
	assert.Equal(t, "ZTEGC0A1B2C3", snap.ZTE.SerialNumber)
	assert.Equal(t, "customer-42", snap.ZTE.Name)
	assert.Equal(t, "F660", snap.ZTE.DeviceType)
	assert.Equal(t, "V9.0", snap.ZTE.HardwareVersion)
	assert.Equal(t, "192.168.100.7", snap.ZTE.IPAddr)
	// Bandwidth/VLAN live only in CLI config, not in SNMP reads: they stay nil
	// so Compare never flags them as drift.
	assert.Nil(t, snap.ZTE.Bandwidth)
	assert.Nil(t, snap.ZTE.VLAN)

	// RawReadings preserve the unmapped values for post-hoc debugging.
	assert.NotEmpty(t, snap.RawReadings)
	assert.Equal(t, olt.IPAddress, snap.RawReadings["olt_ip"])
}

func TestSnapshotService_CaptureBeforeSnapshot_HSGQ(t *testing.T) {
	db := setupSnapshotTestDB(t)
	_, ont := seedOLTAndONT(t, db, models.OLTModelHSGQ)
	// HSGQ is EPON: identity is the MAC, which the driver also fills into
	// SerialNumber. The ONT row must match what the driver reports.
	ont.SerialNumber = "AABBCCDDEEFF"
	require.NoError(t, db.Save(&ont).Error)

	driver := &fakeDriver{
		model: models.OLTModelHSGQ,
		inventory: connectivity.ONTInventory{
			SerialNumber: "AABBCCDDEEFF",
			Name:         "ONU03/07",
			MACAddress:   "AA:BB:CC:DD:EE:FF",
		},
		metrics: &connectivity.ONTMetrics{},
	}
	svc := newSnapshotService(db, driver)

	snap, err := svc.CaptureBeforeSnapshot(ont)
	require.NoError(t, err)
	require.NotNil(t, snap)

	require.NotNil(t, snap.HSGQ)
	assert.Nil(t, snap.ZTE)
	assert.Equal(t, "AABBCCDDEEFF", snap.HSGQ.SerialNumber)
	assert.Equal(t, "gpon-onu-port/3/7", snap.HSGQ.PortConfig)
	// HSGQ's SNMP tree exposes no per-ONU VLAN or profile table, so both stay
	// unset rather than being fabricated.
	assert.Nil(t, snap.HSGQ.VLANID)
	assert.Equal(t, 0, snap.HSGQ.ProfileID)
}

func TestSnapshotService_CaptureBeforeSnapshot_SNMPTimeout(t *testing.T) {
	db := setupSnapshotTestDB(t)
	_, ont := seedOLTAndONT(t, db, models.OLTModelZTEC300)

	driver := &fakeDriver{
		model:  models.OLTModelZTEC300,
		invErr: errors.New("request timeout"),
	}
	svc := newSnapshotService(db, driver)

	snap, err := svc.CaptureBeforeSnapshot(ont)
	require.Error(t, err)
	assert.Nil(t, snap)
	assert.Contains(t, err.Error(), "timeout")
}

func TestSnapshotService_CaptureBeforeSnapshot_UnknownModel(t *testing.T) {
	db := setupSnapshotTestDB(t)
	_, ont := seedOLTAndONT(t, db, models.OLTModel("huawei_ma5800"))

	resolver := func(model models.OLTModel) (connectivity.Driver, error) {
		return connectivity.DriverFor(model)
	}
	svc := NewSnapshotService(db, resolver, zap.NewNop())

	_, err := svc.CaptureBeforeSnapshot(ont)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "huawei_ma5800")
}

func TestSnapshotService_CaptureAfterSnapshot_MatchesBefore(t *testing.T) {
	db := setupSnapshotTestDB(t)
	_, ont := seedOLTAndONT(t, db, models.OLTModelZTEC320)

	driver := &fakeDriver{
		model: models.OLTModelZTEC320,
		inventory: connectivity.ONTInventory{
			SerialNumber: "ZTEGC0A1B2C3",
			Name:         "customer-42",
			DeviceType:   "F660",
		},
		metrics: &connectivity.ONTMetrics{},
	}
	svc := newSnapshotService(db, driver)

	before, err := svc.CaptureBeforeSnapshot(ont)
	require.NoError(t, err)
	after, err := svc.CaptureAfterSnapshot(ont)
	require.NoError(t, err)

	// Same reads, different instants: only the timestamp may differ.
	assert.Empty(t, svc.Compare(before, after))
	assert.NotEqual(t, before.Timestamp, after.Timestamp)
}

func TestSnapshotService_Compare(t *testing.T) {
	db := setupSnapshotTestDB(t)
	svc := newSnapshotService(db, &fakeDriver{model: models.OLTModelZTEC300})

	zte := func(mut func(*ZTESnapshot)) *ConfigSnapshot {
		s := &ConfigSnapshot{
			OntID: uuid.New(),
			ZTE: &ZTESnapshot{
				SerialNumber:    "ZTEGC0A1B2C3",
				Name:            "customer-42",
				DeviceType:      "F660",
				HardwareVersion: "V9.0",
				ServiceMode:     "ipoe",
				IPAddr:          "192.168.100.7",
			},
		}
		if mut != nil {
			mut(s.ZTE)
		}
		return s
	}

	t.Run("identical snapshots produce no diff", func(t *testing.T) {
		assert.Empty(t, svc.Compare(zte(nil), zte(nil)))
	})

	t.Run("single field change is reported", func(t *testing.T) {
		before := zte(nil)
		after := zte(func(s *ZTESnapshot) { s.Name = "customer-43" })
		assert.Equal(t, []string{"zte.name"}, svc.Compare(before, after))
	})

	t.Run("multiple changes are all reported", func(t *testing.T) {
		before := zte(nil)
		after := zte(func(s *ZTESnapshot) {
			s.Name = "customer-43"
			s.ServiceMode = "bridge"
		})
		assert.ElementsMatch(t, []string{"zte.name", "zte.service_mode"}, svc.Compare(before, after))
	})

	t.Run("nil optional fields are not drift", func(t *testing.T) {
		vlan := "100"
		before := zte(func(s *ZTESnapshot) { s.VLAN = nil })
		after := zte(func(s *ZTESnapshot) { s.VLAN = &vlan })
		assert.Empty(t, svc.Compare(before, after))
	})

	t.Run("set optional field that changes is drift", func(t *testing.T) {
		v100, v200 := "100", "200"
		before := zte(func(s *ZTESnapshot) { s.VLAN = &v100 })
		after := zte(func(s *ZTESnapshot) { s.VLAN = &v200 })
		assert.Equal(t, []string{"zte.vlan"}, svc.Compare(before, after))
	})

	t.Run("hsGQ snapshots compare independently", func(t *testing.T) {
		pid := 5
		before := &ConfigSnapshot{OntID: uuid.New(), HSGQ: &HSGQSnapshot{SerialNumber: "AABBCCDDEEFF", PortConfig: "gpon-onu-port/3/7", ProfileID: pid, VLANID: nil}}
		after := &ConfigSnapshot{OntID: before.OntID, HSGQ: &HSGQSnapshot{SerialNumber: "AABBCCDDEEFF", PortConfig: "gpon-onu-port/3/7", ProfileID: 6, VLANID: nil}}
		assert.Equal(t, []string{"hsgq.profile_id"}, svc.Compare(before, after))
	})

	t.Run("nil before snapshot means everything differs", func(t *testing.T) {
		assert.NotEmpty(t, svc.Compare(nil, zte(nil)))
	})
}

func TestSnapshotService_RollbackTo_NotYetImplemented(t *testing.T) {
	db := setupSnapshotTestDB(t)
	_, ont := seedOLTAndONT(t, db, models.OLTModelZTEC300)
	svc := newSnapshotService(db, &fakeDriver{model: models.OLTModelZTEC300})

	snap := &ConfigSnapshot{OntID: ont.ID, ZTE: &ZTESnapshot{SerialNumber: ont.SerialNumber}}
	err := svc.RollbackTo(context.Background(), ont, snap)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rollback command factory is not configured")
}
