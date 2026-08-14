package services

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

const testEncryptionKey = "12345678901234567890123456789012" // 32 bytes

func TestOLTService_Create(t *testing.T) {
	db := setupTestDB(t)
	siteService := NewSiteService(db)
	oltService := NewOLTService(db, testEncryptionKey)

	// Create test site
	site, err := siteService.Create("Test Site", "Test Location", "Test Description")
	require.NoError(t, err)

	// Create OLT
	olt, err := oltService.Create(
		site.ID,
		"Test OLT",
		"192.168.1.1",
		"admin",
		"password123",
		22,
		23,
		161,
		"public",
		models.OLTProtocolSSH,
	)

	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, olt.ID)
	assert.Equal(t, "Test OLT", olt.Name)
	assert.Equal(t, "192.168.1.1", olt.IPAddress)
	assert.Equal(t, "admin", olt.Username)
	assert.NotEqual(t, "password123", olt.Password) // Should be encrypted
	assert.Equal(t, models.OLTStatusOffline, olt.Status)
	assert.Equal(t, 22, olt.SSHPort)
	assert.Equal(t, 23, olt.TelnetPort)
	assert.Equal(t, 161, olt.SNMPPort)
	assert.Equal(t, "public", olt.SNMPCommunity)
	assert.Equal(t, models.OLTProtocolSSH, olt.PreferredProtocol)
}

func TestOLTService_Create_InvalidSiteID(t *testing.T) {
	db := setupTestDB(t)
	oltService := NewOLTService(db, testEncryptionKey)

	// Try to create OLT with non-existent site ID
	invalidSiteID := uuid.New()
	_, err := oltService.Create(
		invalidSiteID,
		"Test OLT",
		"192.168.1.1",
		"admin",
		"password123",
		22,
		23,
		161,
		"public",
		models.OLTProtocolSSH,
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "site not found")
}

func TestOLTService_GetByID(t *testing.T) {
	db := setupTestDB(t)
	siteService := NewSiteService(db)
	oltService := NewOLTService(db, testEncryptionKey)

	site, err := siteService.Create("Test Site", "Test Location", "Test Description")
	require.NoError(t, err)

	created, err := oltService.Create(
		site.ID,
		"Test OLT",
		"192.168.1.1",
		"admin",
		"password123",
		22,
		23,
		161,
		"public",
		models.OLTProtocolSSH,
	)
	require.NoError(t, err)

	found, err := oltService.GetByID(created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)
	assert.Equal(t, created.Name, found.Name)
	assert.Equal(t, site.ID, found.SiteID)
}

func TestOLTService_List(t *testing.T) {
	db := setupTestDB(t)
	siteService := NewSiteService(db)
	oltService := NewOLTService(db, testEncryptionKey)

	site, err := siteService.Create("Test Site", "Test Location", "Test Description")
	require.NoError(t, err)

	oltService.Create(site.ID, "OLT1", "192.168.1.1", "admin", "pass", 22, 23, 161, "public", models.OLTProtocolSSH)
	oltService.Create(site.ID, "OLT2", "192.168.1.2", "admin", "pass", 22, 23, 161, "public", models.OLTProtocolTelnet)

	olts, err := oltService.List()
	require.NoError(t, err)
	assert.Len(t, olts, 2)
}

func TestOLTService_Update(t *testing.T) {
	db := setupTestDB(t)
	siteService := NewSiteService(db)
	oltService := NewOLTService(db, testEncryptionKey)

	site, err := siteService.Create("Test Site", "Test Location", "Test Description")
	require.NoError(t, err)

	olt, err := oltService.Create(
		site.ID,
		"Test OLT",
		"192.168.1.1",
		"admin",
		"password123",
		22,
		23,
		161,
		"public",
		models.OLTProtocolSSH,
	)
	require.NoError(t, err)

	// Test updating name
	updates := map[string]interface{}{
		"name": "Updated OLT",
	}
	err = oltService.Update(olt.ID, updates)
	require.NoError(t, err)

	updated, err := oltService.GetByID(olt.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated OLT", updated.Name)

	// Test updating password (should be encrypted)
	updates = map[string]interface{}{
		"password": "newpassword456",
	}
	err = oltService.Update(olt.ID, updates)
	require.NoError(t, err)

	updated, err = oltService.GetByID(olt.ID)
	require.NoError(t, err)
	assert.NotEqual(t, "newpassword456", updated.Password)

	// Verify password can be decrypted
	decrypted, err := oltService.DecryptPassword(updated.Password)
	require.NoError(t, err)
	assert.Equal(t, "newpassword456", decrypted)
}

func TestOLTService_Delete(t *testing.T) {
	db := setupTestDB(t)
	siteService := NewSiteService(db)
	oltService := NewOLTService(db, testEncryptionKey)

	site, err := siteService.Create("Test Site", "Test Location", "Test Description")
	require.NoError(t, err)

	olt, err := oltService.Create(
		site.ID,
		"Test OLT",
		"192.168.1.1",
		"admin",
		"password123",
		22,
		23,
		161,
		"public",
		models.OLTProtocolSSH,
	)
	require.NoError(t, err)

	err = oltService.Delete(olt.ID)
	require.NoError(t, err)

	_, err = oltService.GetByID(olt.ID)
	assert.Error(t, err)
}

func TestOLTService_UpdateStatus(t *testing.T) {
	db := setupTestDB(t)
	siteService := NewSiteService(db)
	oltService := NewOLTService(db, testEncryptionKey)

	site, err := siteService.Create("Test Site", "Test Location", "Test Description")
	require.NoError(t, err)

	olt, err := oltService.Create(
		site.ID,
		"Test OLT",
		"192.168.1.1",
		"admin",
		"password123",
		22,
		23,
		161,
		"public",
		models.OLTProtocolSSH,
	)
	require.NoError(t, err)
	assert.Equal(t, models.OLTStatusOffline, olt.Status)

	// Update status to online
	err = oltService.UpdateStatus(olt.ID, models.OLTStatusOnline)
	require.NoError(t, err)

	updated, err := oltService.GetByID(olt.ID)
	require.NoError(t, err)
	assert.Equal(t, models.OLTStatusOnline, updated.Status)
	assert.NotNil(t, updated.LastSeen)
}

func TestOLTService_DecryptPassword(t *testing.T) {
	db := setupTestDB(t)
	siteService := NewSiteService(db)
	oltService := NewOLTService(db, testEncryptionKey)

	site, err := siteService.Create("Test Site", "Test Location", "Test Description")
	require.NoError(t, err)

	plainPassword := "password123"
	olt, err := oltService.Create(
		site.ID,
		"Test OLT",
		"192.168.1.1",
		"admin",
		plainPassword,
		22,
		23,
		161,
		"public",
		models.OLTProtocolSSH,
	)
	require.NoError(t, err)

	// Decrypt password
	decrypted, err := oltService.DecryptPassword(olt.Password)
	require.NoError(t, err)
	assert.Equal(t, plainPassword, decrypted)
}
