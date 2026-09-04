package wa

import (
	"context"
	"errors"
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
	refuse string
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
	_ context.Context, _ string, _ models.MessageKind, _, _, _, caption string,
) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, caption)
	return "3EB0MEDIA", nil
}

func channelDrainSetup(t *testing.T) (*ChannelDrainer, *fakeChannelSender, *services.CSChannelPostService, models.WAAccount) {
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
	posts := services.NewCSChannelPostService(db)
	sender := &fakeChannelSender{}
	return NewChannelDrainer(account.ID, posts, sender, nil, t.TempDir(), 0), sender, posts, account
}

func queuePost(t *testing.T, posts *services.CSChannelPostService, account models.WAAccount, body string) uuid.UUID {
	t.Helper()
	post, err := posts.Queue(services.ChannelPost{
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
	drainer, sender, posts, account := channelDrainSetup(t)
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
	drainer, sender, posts, account := channelDrainSetup(t)
	sender.refuse = "ditolak"
	queuePost(t, posts, account, "ditolak")

	_, err := drainer.Drain(context.Background(), 10)
	require.NoError(t, err)

	history, err := posts.ListFor("120363000000000001@newsletter", 10)
	require.NoError(t, err)
	require.Len(t, history, 1)
	assert.Equal(t, models.ChannelPostFailed, history[0].Status)
	assert.Contains(t, history[0].FailReason, "not authorized")
}

// ClaimQueued reads without locking, so two drains racing would hand both the
// same row and subscribers would receive the announcement twice.
func TestTwoDrainsNeverSendTheSameUpdateTwice(t *testing.T) {
	drainer, sender, posts, account := channelDrainSetup(t)
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
	drainer, _, posts, account := channelDrainSetup(t)
	queuePost(t, posts, account, "sudah terkirim")
	_, err := drainer.Drain(context.Background(), 10)
	require.NoError(t, err)

	waiting, err := posts.ClaimQueued(account.ID, 10)
	require.NoError(t, err)
	assert.Empty(t, waiting)
}
