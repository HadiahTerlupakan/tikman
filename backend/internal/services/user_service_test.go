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

	service.Create("user1", "user1@example.com", "pass", models.UserRoleAdmin)
	service.Create("user2", "user2@example.com", "pass", models.UserRoleTechnician)

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
