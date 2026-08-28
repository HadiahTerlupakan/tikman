package services

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
)

func registerRequest() models.ZTEGPONRegisterRequest {
	return models.ZTEGPONRegisterRequest{
		Card: 3, PON: 1, SerialNumber: "HWTCB403E8A0", ONUType: "ALL",
		VLANID: 214, VLANMode: models.ZTEVLANModeTag,
		ServiceType:     models.ZTEServiceBridge,
		DownloadProfile: "1G", UploadProfile: "1G",
		UseVEIP: true, PPPoEUsername: "user", PPPoEPassword: "secret",
	}
}

// The form reopens on what the job applied. Waiting for the discovery poll to
// read it back off the OLT left it empty for up to thirty minutes.
func TestRecordZTEServiceStoresWhatWasApplied(t *testing.T) {
	db := setupTestDB(t)
	ont := models.ONT{ID: uuid.New(), OLTID: uuid.New(), PortID: 1, ONTID: 15, SerialNumber: "HWTCB403E8A0"}
	require.NoError(t, db.Create(&ont).Error)

	require.NoError(t, recordZTEService(db, []byte(testEncryptionKey), ont, registerRequest()))

	var stored models.ONT
	require.NoError(t, db.First(&stored, "id = ?", ont.ID).Error)
	require.NotNil(t, stored.ServiceConfigAt)

	var service connectivity.ZTEONUService
	require.NoError(t, json.Unmarshal(stored.ServiceConfig, &service))
	assert.Equal(t, 214, service.VLANID)
	assert.Equal(t, "1G", service.TCONTProfile)
	assert.True(t, service.UseVEIP, "the toggle has to survive a reopen")
}

// The password is a credential: it is sealed in its own column and must never
// appear in the stored service JSON.
func TestRecordZTEServiceKeepsThePasswordOutOfTheJSON(t *testing.T) {
	db := setupTestDB(t)
	ont := models.ONT{ID: uuid.New(), OLTID: uuid.New(), PortID: 1, ONTID: 15, SerialNumber: "HWTCB403E8A0"}
	require.NoError(t, db.Create(&ont).Error)

	require.NoError(t, recordZTEService(db, []byte(testEncryptionKey), ont, registerRequest()))

	var stored models.ONT
	require.NoError(t, db.First(&stored, "id = ?", ont.ID).Error)
	assert.NotContains(t, string(stored.ServiceConfig), "secret")
	assert.NotEmpty(t, stored.PPPoEPassword)
	assert.NotContains(t, stored.PPPoEPassword, "secret")
}
