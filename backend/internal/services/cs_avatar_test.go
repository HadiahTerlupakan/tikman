package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

func avatarSetup(t *testing.T) (*CSConversationService, models.WAAccount) {
	t.Helper()
	db := setupTestDB(t)
	return NewCSConversationService(db), csAccount(t, db)
}

func avatarPeer(t *testing.T, s *CSConversationService, acc models.WAAccount, phone string) *models.CSConversation {
	t.Helper()
	conv, err := s.FindOrCreate(IncomingPeer{
		WAAccountID: acc.ID, JID: phone + "@s.whatsapp.net", Phone: phone, Name: "Budi",
	})
	require.NoError(t, err)
	return conv
}

// A conversation nobody has asked about must be looked at before one that was
// asked about last week: a customer who just wrote in is the one whose face a
// CS is about to need.
func TestStaleAvatarsPutsTheNeverCheckedFirst(t *testing.T) {
	conversations, acc := avatarSetup(t)

	checked := avatarPeer(t, conversations, acc, "628111222333")
	fresh := avatarPeer(t, conversations, acc, "628111222444")
	never := avatarPeer(t, conversations, acc, "628111222555")

	require.NoError(t, conversations.SetAvatarChecked(checked.ID, time.Now().Add(-30*24*time.Hour)))
	require.NoError(t, conversations.SetAvatarChecked(fresh.ID, time.Now()))

	due, err := conversations.StaleAvatars(acc.ID, 10, time.Now().Add(-7*24*time.Hour))
	require.NoError(t, err)

	ids := []string{}
	for _, c := range due {
		ids = append(ids, c.ID.String())
	}
	assert.Equal(t, []string{never.ID.String(), checked.ID.String()}, ids,
		"never-checked first, then oldest; one checked today is not due")
}

func TestStaleAvatarsHonoursItsLimit(t *testing.T) {
	conversations, acc := avatarSetup(t)
	for _, phone := range []string{"628111222333", "628111222444", "628111222555"} {
		avatarPeer(t, conversations, acc, phone)
	}

	due, err := conversations.StaleAvatars(acc.ID, 2, time.Now())
	require.NoError(t, err)
	assert.Len(t, due, 2)
}

func TestSetAvatarStoresThePhotoAndStopsItBeingDueAgain(t *testing.T) {
	conversations, acc := avatarSetup(t)
	conv := avatarPeer(t, conversations, acc, "628111222333")

	replaced, err := conversations.SetAvatar(conv.ID, "PIC1", "2026/09/a.jpg")
	require.NoError(t, err)
	assert.Empty(t, replaced, "there was nothing to replace")

	stored, err := conversations.Get(conv.ID)
	require.NoError(t, err)
	assert.Equal(t, "2026/09/a.jpg", stored.AvatarPath)
	assert.Equal(t, "PIC1", stored.AvatarID)
	assert.True(t, stored.HasAvatar)

	due, err := conversations.StaleAvatars(acc.ID, 10, time.Now().Add(-time.Hour))
	require.NoError(t, err)
	assert.Empty(t, due)
}

// A customer who changes their photo leaves the old file referenced by nothing,
// and nothing else sweeps it: CSMediaRetention works from message rows.
func TestSetAvatarAnswersTheFileItReplaced(t *testing.T) {
	conversations, acc := avatarSetup(t)
	conv := avatarPeer(t, conversations, acc, "628111222333")

	_, err := conversations.SetAvatar(conv.ID, "PIC1", "2026/09/a.jpg")
	require.NoError(t, err)

	replaced, err := conversations.SetAvatar(conv.ID, "PIC2", "2026/09/b.jpg")
	require.NoError(t, err)
	assert.Equal(t, "2026/09/a.jpg", replaced)
}

// Taking a photo down is as real a change as putting one up. Left alone, the
// inbox would keep showing a face the customer has removed.
func TestSetAvatarToNothingForgetsThePhoto(t *testing.T) {
	conversations, acc := avatarSetup(t)
	conv := avatarPeer(t, conversations, acc, "628111222333")

	_, err := conversations.SetAvatar(conv.ID, "PIC1", "2026/09/a.jpg")
	require.NoError(t, err)

	replaced, err := conversations.SetAvatar(conv.ID, "", "")
	require.NoError(t, err)
	assert.Equal(t, "2026/09/a.jpg", replaced)

	stored, err := conversations.Get(conv.ID)
	require.NoError(t, err)
	assert.Empty(t, stored.AvatarPath)
	assert.Empty(t, stored.AvatarID)
	assert.False(t, stored.HasAvatar)
}

// The common case by far: the customer hides their photo. Recording the
// attempt is the whole point — without it the sweep asks WhatsApp about the
// same person every time it runs.
func TestSetAvatarCheckedStopsAskingAgainWithoutStoringAPhoto(t *testing.T) {
	conversations, acc := avatarSetup(t)
	conv := avatarPeer(t, conversations, acc, "628111222333")

	require.NoError(t, conversations.SetAvatarChecked(conv.ID, time.Now()))

	due, err := conversations.StaleAvatars(acc.ID, 10, time.Now().Add(-time.Hour))
	require.NoError(t, err)
	assert.Empty(t, due)

	stored, err := conversations.Get(conv.ID)
	require.NoError(t, err)
	assert.False(t, stored.HasAvatar)
}
