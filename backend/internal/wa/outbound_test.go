package wa

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type fakeSender struct {
	mu   sync.Mutex
	sent []string
	err  error
	// failOn refuses exactly one message body, so a drain can be watched
	// carrying on past a number WhatsApp will not accept.
	failOn string
	// delay holds a send open. Without it the goroutines in the concurrency
	// test finish before they can overlap, and the test passes whether the
	// drain lock is there or not — proving nothing.
	delay time.Duration
	// quotes records what each send was told to quote, in the order sent.
	quotes []*Quote
	// reads records each read receipt as the chat it went to and the ids it
	// named.
	reads []readReceipt
}

type readReceipt struct {
	chatJID string
	ids     []string
}

func (f *fakeSender) MarkRead(_ context.Context, chatJID string, ids []string, _ time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.reads = append(f.reads, readReceipt{chatJID: chatJID, ids: ids})
	return nil
}

// The sleep sits outside the lock on purpose: taking it first would serialise
// the fake itself and hide the very overlap the test is trying to create.
func (f *fakeSender) send(record string, fail bool, quote *Quote) (string, error) {
	if f.delay > 0 {
		time.Sleep(f.delay)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return "", f.err
	}
	if fail {
		return "", errors.New("nomor tidak terdaftar di WhatsApp")
	}
	f.sent = append(f.sent, record)
	f.quotes = append(f.quotes, quote)
	return "3EB0" + record, nil
}

func (f *fakeSender) SendText(_ context.Context, _, body string, quote *Quote) (string, error) {
	return f.send(body, f.failOn != "" && body == f.failOn, quote)
}

func (f *fakeSender) SendMedia(
	_ context.Context, _ string, _ models.MessageKind, path, _, _, _ string, quote *Quote,
) (string, error) {
	return f.send(path, false, quote)
}

func drainSetup(t *testing.T) (*gorm.DB, *services.CSMessageService, *services.CSConversationService, *models.CSConversation) {
	// Logger discarded: FindOrCreate's first lookup is an expected miss, and
	// GORM logs every one of them as an error, burying real failures in noise.
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)

	// One connection, and this is load-bearing for the concurrency test. Every
	// new connection to an unshared :memory: database gets its own empty copy,
	// so goroutines that force the pool to grow end up querying tables that do
	// not exist there. Those failures are swallowed, and the race the test
	// exists to catch hides behind them instead of failing the test.
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

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

	_, err := messages.Queue(conv.ID, uuid.New(), models.MessageKindText, "sudah kami cek", nil, nil)
	require.NoError(t, err)

	n, err := NewDrainer(conv.WAAccountID, messages, conversations, sender, nil, t.TempDir(), 0).Drain(context.Background(), 10)
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

	_, err := messages.Queue(conv.ID, uuid.New(), models.MessageKindText, "halo", nil, nil)
	require.NoError(t, err)

	n, err := NewDrainer(conv.WAAccountID, messages, conversations, sender, nil, t.TempDir(), 0).Drain(context.Background(), 10)
	require.NoError(t, err, "one bad message must not stop the drain")
	assert.Equal(t, 0, n)

	history, err := messages.History(conv.ID, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, models.MessageFailed, history[0].Status)
	assert.Contains(t, history[0].FailReason, "nomor tidak terdaftar")
}

// One number WhatsApp refuses must not hold up every other customer's reply.
func TestDrainKeepsGoingPastAMessageWhatsAppRefuses(t *testing.T) {
	_, messages, conversations, conv := drainSetup(t)
	sender := &fakeSender{failOn: "kedua"}

	for _, body := range []string{"pertama", "kedua", "ketiga"} {
		_, err := messages.Queue(conv.ID, uuid.New(), models.MessageKindText, body, nil, nil)
		require.NoError(t, err)
	}

	n, err := NewDrainer(conv.WAAccountID, messages, conversations, sender, nil, t.TempDir(), 0).Drain(context.Background(), 10)
	require.NoError(t, err)

	assert.Equal(t, 2, n)
	assert.Equal(t, []string{"pertama", "ketiga"}, sender.sent,
		"the refusal in the middle does not swallow the message behind it")

	history, err := messages.History(conv.ID, 10, 0)
	require.NoError(t, err)
	byBody := map[string]models.MessageStatus{}
	for _, msg := range history {
		byBody[msg.Body] = msg.Status
	}
	assert.Equal(t, models.MessageSent, byBody["pertama"])
	assert.Equal(t, models.MessageFailed, byBody["kedua"])
	assert.Equal(t, models.MessageSent, byBody["ketiga"])
}

// Draining twice must not send the same reply twice: a customer receiving the
// same answer repeatedly is worse than a slow answer.
func TestDrainDoesNotSendTheSameMessageTwice(t *testing.T) {
	_, messages, conversations, conv := drainSetup(t)
	sender := &fakeSender{}
	drainer := NewDrainer(conv.WAAccountID, messages, conversations, sender, nil, t.TempDir(), 0)

	_, err := messages.Queue(conv.ID, uuid.New(), models.MessageKindText, "halo", nil, nil)
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
	// The delay is what gives the goroutines room to overlap. Without it they
	// finish one after another and the test passes even with the lock removed.
	sender := &fakeSender{delay: 20 * time.Millisecond}
	drainer := NewDrainer(conv.WAAccountID, messages, conversations, sender, nil, t.TempDir(), 0)

	_, err := messages.Queue(conv.ID, uuid.New(), models.MessageKindText, "halo", nil, nil)
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
