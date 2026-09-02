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
)

type fakeSender struct {
	mu   sync.Mutex
	sent []string
	err  error
}

func (f *fakeSender) SendText(_ context.Context, _, body string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return "", f.err
	}
	f.sent = append(f.sent, body)
	return "3EB0" + body, nil
}

func (f *fakeSender) SendMedia(_ context.Context, _ string, _ models.MessageKind, path, _, _, _ string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return "", f.err
	}
	f.sent = append(f.sent, path)
	return "3EB0MEDIA", nil
}

func drainSetup(t *testing.T) (*gorm.DB, *services.CSMessageService, *services.CSConversationService, *models.CSConversation) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, models.AutoMigrate(db))

	account := models.WAAccount{Label: "CS Utama", Status: models.WAAccountConnected}
	require.NoError(t, db.Create(&account).Error)

	conversations := services.NewCSConversationService(db)
	conv, err := conversations.FindOrCreate(services.IncomingPeer{
		WAAccountID: account.ID, JID: "628111@s.whatsapp.net", Phone: "628111222333", Name: "Budi",
	})
	require.NoError(t, err)

	return db, services.NewCSMessageService(db, conversations), conversations, conv
}

func TestDrainSendsWhatIsWaitingAndMarksItSent(t *testing.T) {
	_, messages, conversations, conv := drainSetup(t)
	sender := &fakeSender{}

	_, err := messages.Queue(conv.ID, uuid.New(), models.MessageKindText, "sudah kami cek", nil)
	require.NoError(t, err)

	n, err := NewDrainer(messages, conversations, sender, t.TempDir(), 0).Drain(context.Background(), 10)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Equal(t, []string{"sudah kami cek"}, sender.sent)

	history, err := messages.History(conv.ID, 10, 0)
	require.NoError(t, err)
	require.Len(t, history, 1)
	assert.Equal(t, models.MessageSent, history[0].Status)
	require.NotNil(t, history[0].WAMessageID)
}

// A refusal from WhatsApp must end up in front of the CS as a sentence, not as
// a reply that silently never arrives.
func TestDrainRecordsWhyAMessageCouldNotBeSent(t *testing.T) {
	_, messages, conversations, conv := drainSetup(t)
	sender := &fakeSender{err: errors.New("nomor tidak terdaftar di WhatsApp")}

	_, err := messages.Queue(conv.ID, uuid.New(), models.MessageKindText, "halo", nil)
	require.NoError(t, err)

	n, err := NewDrainer(messages, conversations, sender, t.TempDir(), 0).Drain(context.Background(), 10)
	require.NoError(t, err, "one bad message must not stop the drain")
	assert.Equal(t, 0, n)

	history, err := messages.History(conv.ID, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, models.MessageFailed, history[0].Status)
	assert.Contains(t, history[0].FailReason, "nomor tidak terdaftar")
}

// Draining twice must not send the same reply twice: a customer receiving the
// same answer repeatedly is worse than a slow answer.
func TestDrainDoesNotSendTheSameMessageTwice(t *testing.T) {
	_, messages, conversations, conv := drainSetup(t)
	sender := &fakeSender{}
	drainer := NewDrainer(messages, conversations, sender, t.TempDir(), 0)

	_, err := messages.Queue(conv.ID, uuid.New(), models.MessageKindText, "halo", nil)
	require.NoError(t, err)

	_, err = drainer.Drain(context.Background(), 10)
	require.NoError(t, err)
	_, err = drainer.Drain(context.Background(), 10)
	require.NoError(t, err)

	assert.Len(t, sender.sent, 1)
}

// Two things call Drain: the thirty-second ticker and the Redis announcement
// that a CS just hit send. If they overlap, ClaimQueued hands both the same row
// and the customer receives the reply twice.
func TestConcurrentDrainsSendAReplyOnce(t *testing.T) {
	_, messages, conversations, conv := drainSetup(t)
	sender := &fakeSender{}
	drainer := NewDrainer(messages, conversations, sender, t.TempDir(), 0)

	_, err := messages.Queue(conv.ID, uuid.New(), models.MessageKindText, "halo", nil)
	require.NoError(t, err)

	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = drainer.Drain(context.Background(), 10)
		}()
	}
	wg.Wait()

	assert.Len(t, sender.sent, 1, "one queued reply reaches WhatsApp exactly once")
}
