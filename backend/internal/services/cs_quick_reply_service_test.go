package services

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQuickReplyRoundTrip(t *testing.T) {
	db := setupTestDB(t)
	svc := NewCSQuickReplyService(db)
	author := uuid.New()

	created, err := svc.Create("Cek LOS", "Mohon cek lampu LOS pada modem, apakah menyala merah?", author)
	require.NoError(t, err)
	assert.Equal(t, "Cek LOS", created.Title)

	updated, err := svc.Update(created.ID, "Cek LOS", "Mohon foto lampu pada modem.")
	require.NoError(t, err)
	assert.Equal(t, "Mohon foto lampu pada modem.", updated.Body)

	list, err := svc.List()
	require.NoError(t, err)
	require.Len(t, list, 1)

	require.NoError(t, svc.Delete(created.ID))
	list, err = svc.List()
	require.NoError(t, err)
	assert.Empty(t, list)
}

// A template with no body inserts nothing, which looks to the CS like the
// button is broken.
func TestQuickReplyRefusesAnEmptyBody(t *testing.T) {
	db := setupTestDB(t)
	svc := NewCSQuickReplyService(db)

	_, err := svc.Create("Kosong", "   ", uuid.New())
	assert.ErrorIs(t, err, ErrValidation)

	_, err = svc.Create("  ", "isi", uuid.New())
	assert.ErrorIs(t, err, ErrValidation)
}
