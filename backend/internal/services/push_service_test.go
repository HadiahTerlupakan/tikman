package services

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

func TestPushServiceSubscribeUpsertsByToken(t *testing.T) {
	db := setupTestDB(t)
	service := NewPushService(db)
	user := uuid.New()

	require.NoError(t, service.Subscribe(user, "token-a"))
	require.NoError(t, service.Subscribe(user, "token-a"))

	var count int64
	require.NoError(t, db.Model(&models.PushSubscription{}).Where("fcm_token = ?", "token-a").Count(&count).Error)
	assert.Equal(t, int64(1), count, "re-registering the same token must not duplicate it")
}

// A device that logs in as a different user on a shared machine should be
// heard by whoever is using it now, not whoever registered it first.
func TestPushServiceSubscribeReassignsAnExistingTokenToANewUser(t *testing.T) {
	db := setupTestDB(t)
	service := NewPushService(db)
	first, second := uuid.New(), uuid.New()

	require.NoError(t, service.Subscribe(first, "shared-token"))
	require.NoError(t, service.Subscribe(second, "shared-token"))

	var row models.PushSubscription
	require.NoError(t, db.Where("fcm_token = ?", "shared-token").First(&row).Error)
	assert.Equal(t, second, row.UserID)
}

func TestPushServiceUnsubscribeOnlyRemovesTheCallersOwnToken(t *testing.T) {
	db := setupTestDB(t)
	service := NewPushService(db)
	owner, someoneElse := uuid.New(), uuid.New()
	require.NoError(t, service.Subscribe(owner, "token-a"))

	// someoneElse does not own "token-a" — this must not delete it.
	require.NoError(t, service.Unsubscribe(someoneElse, "token-a"))
	var count int64
	require.NoError(t, db.Model(&models.PushSubscription{}).Where("fcm_token = ?", "token-a").Count(&count).Error)
	assert.Equal(t, int64(1), count)

	require.NoError(t, service.Unsubscribe(owner, "token-a"))
	require.NoError(t, db.Model(&models.PushSubscription{}).Where("fcm_token = ?", "token-a").Count(&count).Error)
	assert.Equal(t, int64(0), count)
}

func TestPushServiceTokensForRolesFiltersByRole(t *testing.T) {
	db := setupTestDB(t)
	push := NewPushService(db)
	users := NewUserService(db)

	admin, err := users.Create("admin1", "admin1@example.com", "password123", "", models.UserRoleAdmin)
	require.NoError(t, err)
	viewer, err := users.Create("viewer1", "viewer1@example.com", "password123", "", models.UserRoleViewer)
	require.NoError(t, err)

	require.NoError(t, push.Subscribe(admin.ID, "admin-token"))
	require.NoError(t, push.Subscribe(viewer.ID, "viewer-token"))

	tokens, err := push.TokensForRoles(models.UserRoleAdmin, models.UserRoleCS, models.UserRoleTechnician)
	require.NoError(t, err)
	assert.Equal(t, []string{"admin-token"}, tokens)
}

func TestPushServiceRemoveTokensDeletesEveryRowNamed(t *testing.T) {
	db := setupTestDB(t)
	service := NewPushService(db)
	user := uuid.New()
	require.NoError(t, service.Subscribe(user, "dead-token"))
	require.NoError(t, service.Subscribe(user, "live-token"))

	require.NoError(t, service.RemoveTokens([]string{"dead-token"}))

	var remaining []string
	require.NoError(t, db.Model(&models.PushSubscription{}).Pluck("fcm_token", &remaining).Error)
	assert.Equal(t, []string{"live-token"}, remaining)
}
