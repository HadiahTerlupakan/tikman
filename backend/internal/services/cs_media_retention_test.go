package services

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
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

// A path that will not come off the disk — here a directory with something in
// it, in production a permission or a mount — used to end the sweep where it
// stood, holding every expired attachment behind it until the next day.
func TestSweepKeepsGoingPastAPathItCannotRemove(t *testing.T) {
	db := setupTestDB(t)
	conversations := NewCSConversationService(db)
	messages := NewCSMessageService(db, conversations)
	account := csAccount(t, db)

	conv, err := conversations.FindOrCreate(IncomingPeer{
		WAAccountID: account.ID, JID: "628111@s.whatsapp.net", Phone: "628111222333", Name: "Budi",
	})
	require.NoError(t, err)

	root := t.TempDir()
	stuckPath := filepath.Join("2026", "01", "stuck")
	require.NoError(t, os.MkdirAll(filepath.Join(root, stuckPath), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, stuckPath, "inside"), []byte("x"), 0o644))

	removablePath := filepath.Join("2026", "01", "removable.jpg")
	require.NoError(t, os.WriteFile(filepath.Join(root, removablePath), []byte("x"), 0o644))

	expired := time.Now().Add(-100 * 24 * time.Hour)
	// The stuck row is stored first so it is the one the loop reaches first.
	for i, rel := range []string{stuckPath, removablePath} {
		_, _, err = messages.SaveInbound(InboundMessage{
			ConversationID: conv.ID, WAMessageID: "3EB0STUCK" + strconv.Itoa(i),
			Kind:  models.MessageKindImage,
			Media: &MediaFile{Path: rel, Mime: "image/jpeg", Filename: "x.jpg", Size: 1},
			At:    expired,
		})
		require.NoError(t, err)
	}

	cleared, err := NewCSMediaRetention(db, root, 90).Sweep()
	require.Error(t, err, "the path it could not clear is still reported")
	assert.Equal(t, 1, cleared)

	_, statErr := os.Stat(filepath.Join(root, removablePath))
	assert.True(t, os.IsNotExist(statErr), "the file behind the stuck one is gone")
}

// Channel attachments are written by the same storeUpload into the same media
// root as chat ones. Left out of the sweep they are the one thing on that disk
// nothing ever removes, in a module whose sweeper exists because attachments
// cost gigabytes a month.
func TestSweepDeletesAnOldChannelAttachment(t *testing.T) {
	db := setupTestDB(t)
	account := csAccount(t, db)

	root := t.TempDir()
	oldPath := filepath.Join("2026", "01", "pengumuman-lama.jpg")
	freshPath := filepath.Join("2026", "09", "pengumuman-baru.jpg")
	for _, rel := range []string{oldPath, freshPath} {
		require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(root, rel)), 0o750))
		require.NoError(t, os.WriteFile(filepath.Join(root, rel), []byte("x"), 0o600))
	}

	old := channelPostWithMedia(account.ID, oldPath, time.Now().Add(-100*24*time.Hour))
	require.NoError(t, db.Create(&old).Error)
	fresh := channelPostWithMedia(account.ID, freshPath, time.Now())
	require.NoError(t, db.Create(&fresh).Error)

	cleared, err := NewCSMediaRetention(db, root, 90).Sweep()
	require.NoError(t, err)
	assert.Equal(t, 1, cleared)

	_, statErr := os.Stat(filepath.Join(root, oldPath))
	assert.True(t, os.IsNotExist(statErr), "the aged attachment is gone")
	_, statErr = os.Stat(filepath.Join(root, freshPath))
	assert.NoError(t, statErr, "a recent one stays")

	var stored models.WABroadcastPost
	require.NoError(t, db.First(&stored, "id = ?", old.ID).Error)
	assert.Empty(t, stored.MediaPath, "the row stops pointing at a file")
	assert.Equal(t, "pengumuman.jpg", stored.MediaFilename, "but still names what was announced")
}

func channelPostWithMedia(accountID uuid.UUID, path string, at time.Time) models.WABroadcastPost {
	jid := "120363000000000001@newsletter"
	return models.WABroadcastPost{
		WAAccountID:    accountID,
		Destination:    models.DestinationChannel,
		DestinationJID: &jid,
		SenderUserID:   uuid.New(),
		Kind:           models.MessageKindImage,
		Status:         models.BroadcastSent,
		MediaPath:      path,
		MediaMime:      "image/jpeg",
		MediaFilename:  "pengumuman.jpg",
		MediaSize:      1,
		CreatedAt:      at,
	}
}

// A status attachment lands in the same root through the same upload, so the
// sweep has to age it out the same way — and it is the shape the sweep has
// never seen: destination_jid is NULL, which migration 49 requires to stay
// NULL, so clearing the file must not disturb the row's destination.
func TestSweepDeletesAnOldStatusAttachment(t *testing.T) {
	db := setupTestDB(t)
	account := csAccount(t, db)

	root := t.TempDir()
	rel := filepath.Join("2026", "01", "status-lama.jpg")
	require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(root, rel)), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(root, rel), []byte("x"), 0o600))

	old := statusPostWithMedia(account.ID, rel, time.Now().Add(-100*24*time.Hour))
	require.NoError(t, db.Create(&old).Error)

	cleared, err := NewCSMediaRetention(db, root, 90).Sweep()
	require.NoError(t, err)
	assert.Equal(t, 1, cleared)

	_, statErr := os.Stat(filepath.Join(root, rel))
	assert.True(t, os.IsNotExist(statErr), "the aged status attachment is gone")

	var stored models.WABroadcastPost
	require.NoError(t, db.First(&stored, "id = ?", old.ID).Error)
	assert.Empty(t, stored.MediaPath, "the row stops pointing at a file")
	assert.Equal(t, models.DestinationStatus, stored.Destination)
	assert.Nil(t, stored.DestinationJID, "a status still names no channel afterwards")
}

func statusPostWithMedia(accountID uuid.UUID, path string, at time.Time) models.WABroadcastPost {
	return models.WABroadcastPost{
		WAAccountID:   accountID,
		Destination:   models.DestinationStatus,
		SenderUserID:  uuid.New(),
		Kind:          models.MessageKindImage,
		Status:        models.BroadcastSent,
		MediaPath:     path,
		MediaMime:     "image/jpeg",
		MediaFilename: "status.jpg",
		MediaSize:     1,
		CreatedAt:     at,
	}
}
