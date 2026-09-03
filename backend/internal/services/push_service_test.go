package services

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

func TestPushServiceSubscribeUpsertsByFID(t *testing.T) {
	db := setupTestDB(t)
	service := NewPushService(db)
	user := uuid.New()

	require.NoError(t, service.Subscribe(user, "fid-a"))
	require.NoError(t, service.Subscribe(user, "fid-a"))

	var count int64
	require.NoError(t, db.Model(&models.PushSubscription{}).Where("fid = ?", "fid-a").Count(&count).Error)
	assert.Equal(t, int64(1), count, "re-registering the same FID must not duplicate it")
}

// A device that logs in as a different user on a shared machine should be
// heard by whoever is using it now, not whoever registered it first.
func TestPushServiceSubscribeReassignsAnExistingFIDToANewUser(t *testing.T) {
	db := setupTestDB(t)
	service := NewPushService(db)
	first, second := uuid.New(), uuid.New()

	require.NoError(t, service.Subscribe(first, "shared-fid"))
	require.NoError(t, service.Subscribe(second, "shared-fid"))

	var row models.PushSubscription
	require.NoError(t, db.Where("fid = ?", "shared-fid").First(&row).Error)
	assert.Equal(t, second, row.UserID)
}

func TestPushServiceUnsubscribeOnlyRemovesTheCallersOwnFID(t *testing.T) {
	db := setupTestDB(t)
	service := NewPushService(db)
	owner, someoneElse := uuid.New(), uuid.New()
	require.NoError(t, service.Subscribe(owner, "fid-a"))

	// someoneElse does not own "fid-a" — this must not delete it.
	require.NoError(t, service.Unsubscribe(someoneElse, "fid-a"))
	var count int64
	require.NoError(t, db.Model(&models.PushSubscription{}).Where("fid = ?", "fid-a").Count(&count).Error)
	assert.Equal(t, int64(1), count)

	require.NoError(t, service.Unsubscribe(owner, "fid-a"))
	require.NoError(t, db.Model(&models.PushSubscription{}).Where("fid = ?", "fid-a").Count(&count).Error)
	assert.Equal(t, int64(0), count)
}

func TestPushServiceFIDsForRolesFiltersByRole(t *testing.T) {
	db := setupTestDB(t)
	push := NewPushService(db)
	users := NewUserService(db)

	admin, err := users.Create("admin1", "admin1@example.com", "password123", "", models.UserRoleAdmin)
	require.NoError(t, err)
	viewer, err := users.Create("viewer1", "viewer1@example.com", "password123", "", models.UserRoleViewer)
	require.NoError(t, err)

	require.NoError(t, push.Subscribe(admin.ID, "admin-fid"))
	require.NoError(t, push.Subscribe(viewer.ID, "viewer-fid"))

	fids, err := push.FIDsForRoles(models.UserRoleAdmin, models.UserRoleCS, models.UserRoleTechnician)
	require.NoError(t, err)
	assert.Equal(t, []string{"admin-fid"}, fids)
}

func TestPushServiceRemoveFIDsDeletesEveryRowNamed(t *testing.T) {
	db := setupTestDB(t)
	service := NewPushService(db)
	user := uuid.New()
	require.NoError(t, service.Subscribe(user, "dead-fid"))
	require.NoError(t, service.Subscribe(user, "live-fid"))

	require.NoError(t, service.RemoveFIDs([]string{"dead-fid"}))

	var remaining []string
	require.NoError(t, db.Model(&models.PushSubscription{}).Pluck("fid", &remaining).Error)
	assert.Equal(t, []string{"live-fid"}, remaining)
}
