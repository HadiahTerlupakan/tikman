package main

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupWorkerOLTStatusTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, models.AutoMigrate(db))
	return db
}

func TestUpdateOLTConnectionStatusOnline(t *testing.T) {
	db := setupWorkerOLTStatusTestDB(t)
	olt := models.OLT{
		ID:                uuid.New(),
		SiteID:            uuid.New(),
		Name:              "olt-1",
		IPAddress:         "192.0.2.10",
		PreferredProtocol: models.OLTProtocolSSH,
		Username:          "admin",
		Password:          "secret",
		Status:            models.OLTStatusOffline,
	}
	require.NoError(t, db.Create(&olt).Error)

	require.NoError(t, updateOLTConnectionStatus(db, olt.ID, models.OLTStatusOnline, zap.NewNop()))

	var updated models.OLT
	require.NoError(t, db.First(&updated, "id = ?", olt.ID).Error)
	assert.Equal(t, models.OLTStatusOnline, updated.Status)
	require.NotNil(t, updated.LastSeen)
	assert.WithinDuration(t, time.Now(), *updated.LastSeen, time.Second)
}

func TestUpdateOLTConnectionStatusOfflineKeepsLastSeen(t *testing.T) {
	db := setupWorkerOLTStatusTestDB(t)
	lastSeen := time.Now().Add(-time.Hour)
	olt := models.OLT{
		ID:                uuid.New(),
		SiteID:            uuid.New(),
		Name:              "olt-1",
		IPAddress:         "192.0.2.10",
		PreferredProtocol: models.OLTProtocolSSH,
		Username:          "admin",
		Password:          "secret",
		Status:            models.OLTStatusOnline,
		LastSeen:          &lastSeen,
	}
	require.NoError(t, db.Create(&olt).Error)

	require.NoError(t, updateOLTConnectionStatus(db, olt.ID, models.OLTStatusOffline, zap.NewNop()))

	var updated models.OLT
	require.NoError(t, db.First(&updated, "id = ?", olt.ID).Error)
	assert.Equal(t, models.OLTStatusOffline, updated.Status)
	require.NotNil(t, updated.LastSeen)
	assert.True(t, updated.LastSeen.Equal(lastSeen))
}
