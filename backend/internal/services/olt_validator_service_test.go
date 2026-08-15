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
