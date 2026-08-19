package auth

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"

	"github.com/redis/go-redis/v9"
)

func setupTestRedis(t *testing.T) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available, skipping test")
	}

	return client
}

func TestSessionStore_CreateAndGet(t *testing.T) {
	client := setupTestRedis(t)
	defer func() { _ = client.Close() }()

	store := NewStore(client, 24*time.Hour)
	userID := uuid.New()

	token, err := store.Create(userID, models.UserRoleAdmin)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	data, err := store.Get(token)
	require.NoError(t, err)
	assert.Equal(t, userID, data.UserID)
	assert.Equal(t, models.UserRoleAdmin, data.Role)
}

func TestSessionStore_GetInvalidToken(t *testing.T) {
	client := setupTestRedis(t)
	defer func() { _ = client.Close() }()

	store := NewStore(client, 24*time.Hour)

	data, err := store.Get("invalid-token")
	assert.Error(t, err)
	assert.Nil(t, data)
}

func TestSessionStore_Delete(t *testing.T) {
	client := setupTestRedis(t)
	defer func() { _ = client.Close() }()

	store := NewStore(client, 24*time.Hour)
	userID := uuid.New()

	token, err := store.Create(userID, models.UserRoleAdmin)
	require.NoError(t, err)

	err = store.Delete(token)
	require.NoError(t, err)

	data, err := store.Get(token)
	assert.Error(t, err)
	assert.Nil(t, data)
}

func TestSessionStore_Refresh(t *testing.T) {
	client := setupTestRedis(t)
	defer func() { _ = client.Close() }()

	store := NewStore(client, 1*time.Second)
	userID := uuid.New()

	token, err := store.Create(userID, models.UserRoleAdmin)
	require.NoError(t, err)

	time.Sleep(500 * time.Millisecond)

	err = store.Refresh(token)
	require.NoError(t, err)

	time.Sleep(700 * time.Millisecond)

	data, err := store.Get(token)
	require.NoError(t, err)
	assert.Equal(t, userID, data.UserID)
}
