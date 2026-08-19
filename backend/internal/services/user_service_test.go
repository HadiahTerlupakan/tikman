package services

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = models.AutoMigrate(db)
	require.NoError(t, err)

	return db
}

func TestUserService_Create(t *testing.T) {
	db := setupTestDB(t)
	service := NewUserService(db)

	user, err := service.Create("testuser", "test@example.com", "password123", models.UserRoleAdmin)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, user.ID)
	assert.Equal(t, "testuser", user.Username)
	assert.NotEqual(t, "password123", user.PasswordHash)
}

func TestUserService_GetByUsername(t *testing.T) {
	db := setupTestDB(t)
	service := NewUserService(db)

	created, err := service.Create("testuser", "test@example.com", "password123", models.UserRoleAdmin)
	require.NoError(t, err)

	found, err := service.GetByUsername("testuser")
	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)
}

func TestUserService_List(t *testing.T) {
	db := setupTestDB(t)
	service := NewUserService(db)

	_, _ = service.Create("user1", "user1@example.com", "pass", models.UserRoleAdmin)
	_, _ = service.Create("user2", "user2@example.com", "pass", models.UserRoleTechnician)

	users, err := service.List()
	require.NoError(t, err)
	assert.Len(t, users, 2)
}

func TestUserService_Delete(t *testing.T) {
	db := setupTestDB(t)
	service := NewUserService(db)

	user, err := service.Create("testuser", "test@example.com", "password123", models.UserRoleAdmin)
	require.NoError(t, err)

	err = service.Delete(user.ID)
	require.NoError(t, err)

	_, err = service.GetByID(user.ID)
	assert.Error(t, err)
}

func TestUserService_Update(t *testing.T) {
	db := setupTestDB(t)
	service := NewUserService(db)

	user, err := service.Create("testuser", "test@example.com", "password123", models.UserRoleAdmin)
	require.NoError(t, err)

	// Test updating email
	updates := map[string]interface{}{
		"email": "newemail@example.com",
	}
	err = service.Update(user.ID, updates)
	require.NoError(t, err)

	updated, err := service.GetByID(user.ID)
	require.NoError(t, err)
	assert.Equal(t, "newemail@example.com", updated.Email)

	// Test updating password (should be hashed)
	updates = map[string]interface{}{
		"password": "newpassword456",
	}
	err = service.Update(user.ID, updates)
	require.NoError(t, err)

	updated, err = service.GetByID(user.ID)
	require.NoError(t, err)
	assert.NotEqual(t, "newpassword456", updated.PasswordHash)

	// Test updating role
	updates = map[string]interface{}{
		"role": models.UserRoleTechnician,
	}
	err = service.Update(user.ID, updates)
	require.NoError(t, err)

	updated, err = service.GetByID(user.ID)
	require.NoError(t, err)
	assert.Equal(t, models.UserRoleTechnician, updated.Role)
}

func TestUserService_VerifyPassword(t *testing.T) {
	db := setupTestDB(t)
	service := NewUserService(db)

	user, err := service.Create("testuser", "test@example.com", "password123", models.UserRoleAdmin)
	require.NoError(t, err)

	// Test correct password
	err = service.VerifyPassword(user, "password123")
	assert.NoError(t, err)

	// Test incorrect password
	err = service.VerifyPassword(user, "wrongpassword")
	assert.Error(t, err)
}
