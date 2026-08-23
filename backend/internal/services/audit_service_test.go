package services

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/tikman/olt-provisioning/internal/models"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestAuditDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	// models.AutoMigrate rather than a local AutoMigrate(&models.AuditLog{}):
	// migrating the model here passes whether or not AuditLog is registered in
	// models.AutoMigrate, which is what let audit_logs ship missing from every
	// real database while these tests stayed green.
	err = models.AutoMigrate(db)
	assert.NoError(t, err)

	return db
}

func TestAuditService_Log(t *testing.T) {
	db := setupTestAuditDB(t)
	logger := zap.NewNop()
	service := NewAuditService(db, logger)

	userID := uuid.New()
	resourceID := uuid.New()

	err := service.Log(
		userID,
		"create",
		"site",
		resourceID,
		nil,
		map[string]interface{}{
			"name":     "Test Site",
			"location": "Test Location",
		},
		"192.168.1.1",
		"TestAgent/1.0",
	)

	assert.NoError(t, err)

	// Verify the log was created
	var log models.AuditLog
	err = db.Where("user_id = ?", userID).First(&log).Error
	assert.NoError(t, err)
	assert.Equal(t, "create", log.Action)
	assert.Equal(t, "site", log.ResourceType)
	assert.Equal(t, resourceID, *log.ResourceID)
	assert.Equal(t, "192.168.1.1", log.IPAddress)
	assert.Equal(t, "TestAgent/1.0", log.UserAgent)
	assert.NotNil(t, log.NewValue)
}

func TestAuditService_LogUpdate(t *testing.T) {
	db := setupTestAuditDB(t)
	logger := zap.NewNop()
	service := NewAuditService(db, logger)

	userID := uuid.New()
	resourceID := uuid.New()

	oldValue := map[string]interface{}{
		"name": "Old Name",
	}
	newValue := map[string]interface{}{
		"name": "New Name",
	}

	err := service.Log(
		userID,
		"update",
		"user",
		resourceID,
		oldValue,
		newValue,
		"10.0.0.1",
		"Mozilla/5.0",
	)

	assert.NoError(t, err)

	var log models.AuditLog
	err = db.Where("action = ?", "update").First(&log).Error
	assert.NoError(t, err)
	assert.NotNil(t, log.OldValue)
	assert.NotNil(t, log.NewValue)
}
