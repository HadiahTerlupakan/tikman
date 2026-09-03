package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tikman/olt-provisioning/internal/models"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupSeedTest(t *testing.T) (*gorm.DB, *zap.Logger) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	err = db.AutoMigrate(&models.User{})
	assert.NoError(t, err)

	logger := zap.NewNop()
	return db, logger
}

func TestCreateDefaultAdmin_NoUsers(t *testing.T) {
	db, logger := setupSeedTest(t)

	err := CreateDefaultAdmin(db, logger)
	assert.NoError(t, err)

	// Verify admin user was created
	var user models.User
	err = db.First(&user, "username = ?", "admin").Error
	assert.NoError(t, err)
	assert.Equal(t, "admin", user.Username)
	assert.Equal(t, "admin@tikman.local", user.Email)
	assert.Equal(t, models.UserRoleAdmin, user.Role)

	// Verify password is correct
	service := NewUserService(db)
	err = service.VerifyPassword(&user, "admin123")
	assert.NoError(t, err)
}

func TestCreateDefaultAdmin_UsersExist(t *testing.T) {
	db, logger := setupSeedTest(t)

	// Create a user first
	service := NewUserService(db)
	_, err := service.Create("existing", "existing@example.com", "password123", "", models.UserRoleViewer)
	assert.NoError(t, err)

	// Try to create default admin
	err = CreateDefaultAdmin(db, logger)
	assert.NoError(t, err)

	// Verify admin user was NOT created
	var count int64
	err = db.Model(&models.User{}).Where("username = ?", "admin").Count(&count).Error
	assert.NoError(t, err)
	assert.Equal(t, int64(0), count)

	// Verify only one user exists
	err = db.Model(&models.User{}).Count(&count).Error
	assert.NoError(t, err)
	assert.Equal(t, int64(1), count)
}
