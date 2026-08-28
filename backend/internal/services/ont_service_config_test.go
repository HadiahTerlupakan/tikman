package services

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/utils"
	"gorm.io/datatypes"
)

func seedONTWithService(t *testing.T, service connectivity.ZTEONUService, password string) (*ONTService, uuid.UUID) {
	t.Helper()

	db := setupTestDB(t)
	encoded, err := json.Marshal(service)
	require.NoError(t, err)

	ont := &models.ONT{
		ID: uuid.New(), OLTID: uuid.New(), PortID: 1, ONTID: 1,
		SerialNumber: "HWTCB403E8A0", ServiceConfig: datatypes.JSON(encoded),
	}
	if password != "" {
		sealed, err := utils.Encrypt(password, testEncryptionKey)
		require.NoError(t, err)
		ont.PPPoEPassword = sealed
	}
	require.NoError(t, db.Create(ont).Error)

	return NewONTServiceWithEncryption(db, testEncryptionKey), ont.ID
}

// Reconfiguring a service has to resend the password, so it has to survive the
// round trip through storage.
func TestONTService_GetServiceConfigDecryptsThePassword(t *testing.T) {
	service, ontID := seedONTWithService(t,
		connectivity.ZTEONUService{VLANID: 214, PPPoEUsername: "258179206252"}, "12345")

	got, _, err := service.GetServiceConfig(ontID)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, 214, got.VLANID)
	assert.Equal(t, "258179206252", got.PPPoEUsername)
	assert.Equal(t, "12345", got.PPPoEPassword)
}

// The password is a separate encrypted column; the stored JSON must not hold a
// readable copy of it.
func TestONTService_ServiceConfigJSONHasNoReadablePassword(t *testing.T) {
	db := setupTestDB(t)
	encoded, err := json.Marshal(connectivity.ZTEONUService{
		PPPoEUsername: "258179206252", PPPoEPassword: "12345",
	})
	require.NoError(t, err)

	ont := &models.ONT{
		ID: uuid.New(), OLTID: uuid.New(), PortID: 1, ONTID: 1,
		SerialNumber: "HWTCB403E8A0", ServiceConfig: datatypes.JSON(encoded),
	}
	require.NoError(t, db.Create(ont).Error)

	var stored models.ONT
	require.NoError(t, db.First(&stored, "id = ?", ont.ID).Error)
	assert.NotContains(t, string(stored.ServiceConfig), "12345")
}

// Without a key the rest of the service still pre-fills the form.
func TestONTService_GetServiceConfigWithoutAKeyOmitsThePassword(t *testing.T) {
	withKey, ontID := seedONTWithService(t,
		connectivity.ZTEONUService{VLANID: 214}, "12345")
	withoutKey := NewONTService(withKey.db)

	got, _, err := withoutKey.GetServiceConfig(ontID)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, 214, got.VLANID)
	assert.Empty(t, got.PPPoEPassword)
}

func TestONTService_GetServiceConfigIsEmptyBeforeThePollCoversTheONT(t *testing.T) {
	db := setupTestDB(t)
	ont := &models.ONT{ID: uuid.New(), OLTID: uuid.New(), PortID: 1, ONTID: 2, SerialNumber: "HWTCB403E8A1"}
	require.NoError(t, db.Create(ont).Error)

	got, _, err := NewONTServiceWithEncryption(db, testEncryptionKey).GetServiceConfig(ont.ID)

	require.NoError(t, err)
	assert.Nil(t, got)
}
