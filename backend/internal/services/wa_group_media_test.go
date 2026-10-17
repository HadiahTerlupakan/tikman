package services

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

func writeMedia(t *testing.T, root, rel string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(root, rel)), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, rel), []byte("x"), 0o644))
}

func groupPhoto(f groupFixture, waID, rel string, at time.Time) GroupArrival {
	photo := arrival(f.group.ID, waID, "")
	photo.Kind = models.MessageKindImage
	photo.Media = &MediaFile{Path: rel, Mime: "image/jpeg", Filename: "foto.jpg", Size: 1}
	photo.At = at
	return photo
}

// Groups are where most photos arrive; the retention sweep has to reach them
// or the disk fills from there.
func TestSweepDeletesAnOldGroupAttachmentButKeepsTheMessage(t *testing.T) {
	f := newGroupFixture(t)
	root := t.TempDir()
	oldPath := filepath.Join("2026", "01", "grup-lama.jpg")
	writeMedia(t, root, oldPath)
	old, _, err := f.messages.SaveArrived(groupPhoto(f, "W1", oldPath, time.Now().Add(-100*24*time.Hour)))
	require.NoError(t, err)

	removed, err := NewCSMediaRetention(f.db, root, 90).Sweep()
	require.NoError(t, err)
	assert.Equal(t, 1, removed)

	_, err = os.Stat(filepath.Join(root, oldPath))
	assert.True(t, os.IsNotExist(err))
	kept, err := f.messages.Get(old.ID)
	require.NoError(t, err)
	assert.Empty(t, kept.MediaPath)
	assert.Equal(t, "foto.jpg", kept.MediaFilename, "the history still says a photo was sent")
}

func TestDeleteAccountRemovesItsGroupsAndTheirFiles(t *testing.T) {
	f := newGroupFixture(t)
	root := t.TempDir()
	rel := filepath.Join("2026", "10", "grup.jpg")
	writeMedia(t, root, rel)
	_, _, err := f.messages.SaveArrived(groupPhoto(f, "W1", rel, time.Now()))
	require.NoError(t, err)

	require.NoError(t, NewCSPurgeService(f.db, root).DeleteAccount(f.account.ID))

	_, err = os.Stat(filepath.Join(root, rel))
	assert.True(t, os.IsNotExist(err), "a cascade cannot remove a file, so the purge must")
	for _, table := range []any{&models.WAGroup{}, &models.WAGroupMessage{}, &models.WAGroupParticipant{}} {
		var left int64
		require.NoError(t, f.db.Model(table).Count(&left).Error)
		assert.Zero(t, left)
	}
}
