package wa

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// fakeChannelSender stands in for the WhatsApp connection. refuse names the
// body it rejects, so a test can make exactly one post fail.
type fakeChannelSender struct {
	mu     sync.Mutex
	sent   []string
	media  []mediaCall
	refuse string
}

// mediaCall is what the drainer handed the sender for one attachment, so a
// test can check the path it built and the fields it forwarded.
type mediaCall struct {
	kind     models.MessageKind
	path     string
	mime     string
	filename string
	caption  string
}

func (f *fakeChannelSender) SendChannelText(_ context.Context, _, body string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if body == f.refuse {
		return "", errors.New("not authorized to post")
	}
	f.sent = append(f.sent, body)
	return "3EB0" + body, nil
}

func (f *fakeChannelSender) SendChannelMedia(
	_ context.Context, _ string, kind models.MessageKind, path, mime, filename, caption string,
) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, caption)
	f.media = append(f.media, mediaCall{
		kind: kind, path: path, mime: mime, filename: filename, caption: caption,
	})
	return "3EB0MEDIA", nil
}

func channelDrainSetup(t *testing.T) (*BroadcastDrainer, *fakeChannelSender, *services.CSBroadcastPostService, models.WAAccount, string) {
	t.Helper()
	// Built here rather than borrowed from the services package, matching
	// drainSetup in outbound_test.go — including the single connection, which
	// is load-bearing for the concurrency test below: every new connection to
	// an unshared :memory: database gets its own empty copy, so a goroutine
	// that grows the pool would query tables that do not exist there and the
	// race would hide behind swallowed failures.
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, models.AutoMigrate(db))

	account := models.WAAccount{Label: "CS Utama", Status: models.WAAccountConnected}
	require.NoError(t, db.Create(&account).Error)
	posts := services.NewCSBroadcastPostService(db)
	sender := &fakeChannelSender{}
	root := t.TempDir()
	return NewBroadcastDrainer(account.ID, posts, sender, nil, root, 0), sender, posts, account, root
}

func queuePost(t *testing.T, posts *services.CSBroadcastPostService, account models.WAAccount, body string) uuid.UUID {
	t.Helper()
	post, err := posts.Queue(services.BroadcastPost{
		WAAccountID:  account.ID,
		ChannelJID:   "120363000000000001@newsletter",
		SenderUserID: uuid.New(),
		Kind:         models.MessageKindText,
		Body:         body,
	})
	require.NoError(t, err)
	return post.ID
}

// One channel refusing an update must not hold up the announcements queued
// behind it.
func TestDrainKeepsGoingAfterWhatsAppRefusesOne(t *testing.T) {
	drainer, sender, posts, account, _ := channelDrainSetup(t)
	sender.refuse = "ditolak"
	queuePost(t, posts, account, "ditolak")
	queuePost(t, posts, account, "berhasil")

	sent, err := drainer.Drain(context.Background(), 10)

	require.NoError(t, err)
	assert.Equal(t, 1, sent)
	assert.Equal(t, []string{"berhasil"}, sender.sent)
}

// The refusal has to survive on the row, because the history is the only place
// the sender ever learns their announcement did not go.
func TestARefusedUpdateKeepsItsReason(t *testing.T) {
	drainer, sender, posts, account, _ := channelDrainSetup(t)
	sender.refuse = "ditolak"
	queuePost(t, posts, account, "ditolak")

	_, err := drainer.Drain(context.Background(), 10)
	require.NoError(t, err)

	history, err := posts.ListFor("120363000000000001@newsletter", 10)
	require.NoError(t, err)
	require.Len(t, history, 1)
	assert.Equal(t, models.BroadcastFailed, history[0].Status)
	assert.Contains(t, history[0].FailReason, "not authorized")
}

// ClaimQueued reads without locking, so two drains racing would hand both the
// same row and subscribers would receive the announcement twice.
func TestTwoDrainsNeverSendTheSameUpdateTwice(t *testing.T) {
	drainer, sender, posts, account, _ := channelDrainSetup(t)
	queuePost(t, posts, account, "sekali saja")

	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = drainer.Drain(context.Background(), 10)
		}()
	}
	wg.Wait()

	assert.Equal(t, []string{"sekali saja"}, sender.sent)
}

// A sent update stops being claimable, so a later sweep does not repeat it.
func TestASentUpdateLeavesTheQueue(t *testing.T) {
	drainer, _, posts, account, _ := channelDrainSetup(t)
	queuePost(t, posts, account, "sudah terkirim")
	_, err := drainer.Drain(context.Background(), 10)
	require.NoError(t, err)

	waiting, err := posts.ClaimQueued(account.ID, 10)
	require.NoError(t, err)
	assert.Empty(t, waiting)
}

// The drainer builds the absolute path itself, joining the media root onto the
// relative path storeUpload wrote. Nothing exercised that branch: if the two
// ever disagree the read fails inside the wa process, and all the sender ever
// sees is "Gagal".
func TestAMediaUpdateIsSentFromWhereTheUploadWasStored(t *testing.T) {
	drainer, sender, posts, account, root := channelDrainSetup(t)

	rel := filepath.Join("2026", "09", "pengumuman.jpg")
	require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(root, rel)), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(root, rel), []byte("x"), 0o600))

	_, err := posts.Queue(services.BroadcastPost{
		WAAccountID:  account.ID,
		ChannelJID:   "120363000000000001@newsletter",
		SenderUserID: uuid.New(),
		Kind:         models.MessageKindImage,
		Body:         "Ada pemeliharaan malam ini",
		Media: &services.MediaFile{
			Path: rel, Mime: "image/jpeg", Filename: "pengumuman.jpg", Size: 1,
		},
	})
	require.NoError(t, err)

	sent, err := drainer.Drain(context.Background(), 10)
	require.NoError(t, err)
	assert.Equal(t, 1, sent)

	require.Len(t, sender.media, 1)
	call := sender.media[0]
	assert.Equal(t, filepath.Join(root, rel), call.path)
	assert.FileExists(t, call.path, "the path the drainer built is the file that was stored")
	assert.Equal(t, models.MessageKindImage, call.kind)
	assert.Equal(t, "image/jpeg", call.mime)
	assert.Equal(t, "pengumuman.jpg", call.filename)
	assert.Equal(t, "Ada pemeliharaan malam ini", call.caption, "the body travels as the caption")

	history, err := posts.ListFor("120363000000000001@newsletter", 10)
	require.NoError(t, err)
	require.Len(t, history, 1)
	assert.Equal(t, models.BroadcastSent, history[0].Status)
	require.NotNil(t, history[0].WAMessageID)
	assert.Equal(t, "3EB0MEDIA", *history[0].WAMessageID)
}
