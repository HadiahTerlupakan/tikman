package services

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

// The message stays; only the file goes. A CS reading old history should see
// that the customer sent a photo, even when the photo itself is long gone.
func TestSweepDeletesOldFilesButKeepsTheMessages(t *testing.T) {
	db := setupTestDB(t)
	conversations := NewCSConversationService(db)
	messages := NewCSMessageService(db, conversations)
	account := csAccount(t, db)

	conv, err := conversations.FindOrCreate(IncomingPeer{
		WAAccountID: account.ID, JID: "628111@s.whatsapp.net", Phone: "628111222333", Name: "Budi",
	})
	require.NoError(t, err)

	root := t.TempDir()
	oldPath := filepath.Join("2026", "01", "old.jpg")
	freshPath := filepath.Join("2026", "09", "fresh.jpg")
	for _, rel := range []string{oldPath, freshPath} {
		require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(root, rel)), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(root, rel), []byte("x"), 0o644))
	}

	old, _, err := messages.SaveInbound(InboundMessage{
		ConversationID: conv.ID, WAMessageID: "3EB0OLD", Kind: models.MessageKindImage,
		Media: &MediaFile{Path: oldPath, Mime: "image/jpeg", Filename: "old.jpg", Size: 1},
		At:    time.Now().Add(-100 * 24 * time.Hour),
	})
	require.NoError(t, err)

	_, _, err = messages.SaveInbound(InboundMessage{
		ConversationID: conv.ID, WAMessageID: "3EB0FRESH", Kind: models.MessageKindImage,
		Media: &MediaFile{Path: freshPath, Mime: "image/jpeg", Filename: "fresh.jpg", Size: 1},
		At:    time.Now(),
	})
	require.NoError(t, err)

	removed, err := NewCSMediaRetention(db, root, 90).Sweep()
	require.NoError(t, err)
	assert.Equal(t, 1, removed)

	_, err = os.Stat(filepath.Join(root, oldPath))
	assert.True(t, os.IsNotExist(err), "the old file is gone")
	_, err = os.Stat(filepath.Join(root, freshPath))
	assert.NoError(t, err, "the recent file stays")

	var stored models.CSMessage
	require.NoError(t, db.First(&stored, "id = ?", old.ID).Error)
	assert.Equal(t, models.MessageKindImage, stored.Kind, "the message survives its file")
	assert.Empty(t, stored.MediaPath, "but no longer points at one")
}

// Somebody clearing disk space by hand must not jam the sweep forever. If a
// missing file aborted it, every row behind that one would keep its path and
// the disk would never drain again.
func TestSweepStepsOverAFileSomebodyAlreadyDeleted(t *testing.T) {
	db := setupTestDB(t)
	conversations := NewCSConversationService(db)
	messages := NewCSMessageService(db, conversations)
	account := csAccount(t, db)

	conv, err := conversations.FindOrCreate(IncomingPeer{
		WAAccountID: account.ID, JID: "628111@s.whatsapp.net", Phone: "628111222333", Name: "Budi",
	})
	require.NoError(t, err)

	root := t.TempDir()
	// Two expired rows. Only the second still has its file on disk; the first
	// points at a path that was never written.
	for i, rel := range []string{filepath.Join("2026", "01", "vanished.jpg"), filepath.Join("2026", "01", "present.jpg")} {
		if i == 1 {
			require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(root, rel)), 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(root, rel), []byte("x"), 0o644))
		}
		_, _, err := messages.SaveInbound(InboundMessage{
			ConversationID: conv.ID,
			WAMessageID:    "3EB0GONE" + strconv.Itoa(i),
			Kind:           models.MessageKindImage,
			Media:          &MediaFile{Path: rel, Mime: "image/jpeg", Filename: "x.jpg", Size: 1},
			At:             time.Now().Add(-100 * 24 * time.Hour),
		})
		require.NoError(t, err)
	}

	cleared, err := NewCSMediaRetention(db, root, 90).Sweep()
	require.NoError(t, err, "a file that is already gone is not a failure")
	assert.Equal(t, 2, cleared, "both rows are cleared, including the one whose file had vanished")

	var stillPointing int64
	require.NoError(t, db.Model(&models.CSMessage{}).Where("media_path <> ''").Count(&stillPointing).Error)
	assert.Zero(t, stillPointing, "no row is left pointing at a file")
}
