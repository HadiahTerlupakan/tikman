package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupValidatorTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)
	err = db.AutoMigrate(&models.OLT{})
	assert.NoError(t, err)
	return db
}

func TestValidationResult_StructureExists(t *testing.T) {
	result := &ValidationResult{
		Success:      false,
		PassedTests:  []string{"Ping"},
		FailedTest:   "SSH",
		FailedReason: "authentication failed",
	}
	assert.False(t, result.Success)
	assert.Equal(t, []string{"Ping"}, result.PassedTests)
	assert.Equal(t, "SSH", result.FailedTest)
	assert.Equal(t, "authentication failed", result.FailedReason)
}

func TestNewOLTValidatorService(t *testing.T) {
	db := setupValidatorTestDB(t)
	service := NewOLTValidatorService(db)
	assert.NotNil(t, service)
}

func TestValidateIPNotDuplicate_NoDuplicate(t *testing.T) {
	db := setupValidatorTestDB(t)
	service := NewOLTValidatorService(db)

	err := service.ValidateIPNotDuplicate("10.0.0.1")
	assert.NoError(t, err)
}

func TestValidateIPNotDuplicate_Duplicate(t *testing.T) {
	db := setupValidatorTestDB(t)
	service := NewOLTValidatorService(db)

	// Create an OLT with IP 10.0.0.1
	olt := &models.OLT{
		IPAddress: "10.0.0.1",
		Name:      "Test OLT",
		Username:  "admin",
		Password:  "encrypted",
		Status:    models.OLTStatusOffline,
	}
	db.Create(olt)

	// Try to validate the same IP
	err := service.ValidateIPNotDuplicate("10.0.0.1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

// Note: These tests verify the orchestration logic.
// Actual connectivity tests are unit tested in connectivity package.
// For these tests to pass, you need a mock OLT or skip connectivity by using localhost
func TestValidateCreate_LocalhostSSH(t *testing.T) {
	db := setupValidatorTestDB(t)
	service := NewOLTValidatorService(db)

	// This test will fail on Ping (localhost may not respond to ping unprivileged)
	// or SSH (no SSH server on test machine), which is expected behavior
	result, err := service.ValidateCreate(
		"127.0.0.1",
		"testuser",
		"testpass",
		22,
		23,
		161,
		"public",
		models.OLTProtocolSSH,
	)

	// We expect this to fail in test environment, but validates the flow works
	assert.NoError(t, err)
	assert.NotNil(t, result)
	// Result may be success=false due to no actual OLT available
}
