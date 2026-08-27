package services

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/datatypes"
)

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
