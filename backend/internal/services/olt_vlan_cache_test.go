package services

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/datatypes"
)

// countingCommanderFactory records how often a CLI session was asked for. It
// always fails to connect, which is enough to tell "tried" from "skipped".
type countingCommanderFactory struct {
	calls int
}

func (f *countingCommanderFactory) ForOLT(models.OLTModel, string, int, string, string) (connectivity.OLTCommander, error) {
	f.calls++
	return nil, errors.New("no OLT in tests")
}

func TestOLTService_ListVLANsReadsTheCachedTable(t *testing.T) {
	db := setupTestDB(t)
	service := NewOLTService(db, testEncryptionKey)

	olt := &models.OLT{
		ID: uuid.New(), SiteID: uuid.New(), Name: "Depok", IPAddress: "192.0.2.1",
		Model: models.OLTModelZTEC300, Username: "admin", Password: "pass",
		VLANs: datatypes.JSON(`[{"vlan_id":99,"name":"VLAN0099-PPP"},{"vlan_id":100,"name":"VLAN0100"}]`),
	}
	require.NoError(t, db.Create(olt).Error)

	vlans, updatedAt, err := service.ListVLANs(olt.ID)

	require.NoError(t, err)
	require.Len(t, vlans, 2)
	assert.Equal(t, 99, vlans[0].VLANID)
	assert.Equal(t, "VLAN0099-PPP", vlans[0].Name)
	assert.Equal(t, 100, vlans[1].VLANID)
	assert.Nil(t, updatedAt)
}

// The form falls back to a typed VLAN ID, so an OLT that has never been polled
// has to answer with an empty list rather than an error.
func TestOLTService_ListVLANsIsEmptyBeforeTheFirstPoll(t *testing.T) {
	db := setupTestDB(t)
	service := NewOLTService(db, testEncryptionKey)

	olt := &models.OLT{
		ID: uuid.New(), SiteID: uuid.New(), Name: "Baru", IPAddress: "192.0.2.2",
		Model: models.OLTModelZTEC300, Username: "admin", Password: "pass",
	}
	require.NoError(t, db.Create(olt).Error)

	vlans, _, err := service.ListVLANs(olt.ID)

	require.NoError(t, err)
	assert.Empty(t, vlans)
}

func TestOLTService_ListVLANsRejectsAnUnknownOLT(t *testing.T) {
	db := setupTestDB(t)
	service := NewOLTService(db, testEncryptionKey)

	_, _, err := service.ListVLANs(uuid.New())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "OLT not found")
}

func TestOLTService_ListTCONTProfilesReadsTheCachedNames(t *testing.T) {
	db := setupTestDB(t)
	service := NewOLTService(db, testEncryptionKey)

	olt := &models.OLT{
		ID: uuid.New(), SiteID: uuid.New(), Name: "Depok", IPAddress: "192.0.2.1",
		Model: models.OLTModelZTEC300, Username: "admin", Password: "pass",
		TCONTProfiles: datatypes.JSON(`["default","1G"]`),
	}
	require.NoError(t, db.Create(olt).Error)

	profiles, _, err := service.ListTCONTProfiles(olt.ID)

	require.NoError(t, err)
	assert.Equal(t, []string{"default", "1G"}, profiles)
}

// Reading profiles costs a CLI login, so a poll must not repeat it while the
// cached list is still fresh.
func TestOLTService_RefreshTCONTProfileCacheSkipsAFreshList(t *testing.T) {
	db := setupTestDB(t)
	factory := &countingCommanderFactory{}
	service := NewOLTServiceWithCommander(db, testEncryptionKey, factory)

	justRead := time.Now()
	olt := &models.OLT{
		ID: uuid.New(), SiteID: uuid.New(), Name: "Depok", IPAddress: "192.0.2.1",
		Model: models.OLTModelZTEC300, Username: "admin", Password: "pass",
		TCONTProfiles: datatypes.JSON(`["default"]`), TCONTProfilesUpdatedAt: &justRead,
	}
	require.NoError(t, db.Create(olt).Error)

	service.refreshTCONTProfileCache(olt)

	assert.Zero(t, factory.calls, "a fresh cache must not open a CLI session")
}

func TestOLTService_RefreshTCONTProfileCacheReadsAStaleList(t *testing.T) {
	db := setupTestDB(t)
	factory := &countingCommanderFactory{}
	service := NewOLTServiceWithCommander(db, testEncryptionKey, factory)

	stale := time.Now().Add(-profileCacheTTL - time.Minute)
	olt := &models.OLT{
		ID: uuid.New(), SiteID: uuid.New(), Name: "Depok", IPAddress: "192.0.2.1",
		Model: models.OLTModelZTEC300, Username: "admin", Password: "pass",
		TCONTProfiles: datatypes.JSON(`["default"]`), TCONTProfilesUpdatedAt: &stale,
	}
	require.NoError(t, db.Create(olt).Error)

	service.refreshTCONTProfileCache(olt)

	assert.Equal(t, 1, factory.calls)
}
