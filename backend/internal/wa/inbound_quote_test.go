package wa

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

// nobodyOnline is the rotation with no agents in it, which is what makes
// AssignOne a no-op — this test is about storage, not about who answers.
type nobodyOnline struct{}

func (nobodyOnline) MarkOnline(context.Context, uuid.UUID) error { return nil }
func (nobodyOnline) Online(context.Context) ([]uuid.UUID, error) { return nil, nil }
func (nobodyOnline) NextTurn(context.Context) (uint64, error)    { return 0, nil }

// inboundSetup builds the handler over a real database. The Redis client points
// at a port nothing listens on: publishing is a nudge to the browsers and its
// failure is logged and ignored, so a dead client exercises the same path a
// running one does without needing Redis here.
func inboundSetup(t *testing.T) (*inboundHandler, *services.CSMessageService, *services.CSConversationService, uuid.UUID) {
	t.Helper()
	db, messages, conversations, conv := drainSetup(t)

	dead := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", DialTimeout: time.Millisecond})
	t.Cleanup(func() { _ = dead.Close() })

	var account models.WAAccount
	require.NoError(t, db.First(&account).Error)

	return &inboundHandler{
		accountID:     account.ID,
		conversations: conversations,
		messages:      messages,
		assignment:    services.NewCSAssignmentService(db, conversations, nobodyOnline{}),
		publisher:     NewPublisher(dead),
		logger:        zap.NewNop(),
	}, messages, conversations, conv.ID
}

func customerSays(text string, quoting *waE2E.ContextInfo) *events.Message {
	from := types.JID{User: "628111222333", Server: types.DefaultUserServer}
	return &events.Message{
		Info: types.MessageInfo{
			ID:        "3EB0INCOMING",
			PushName:  "Budi",
			Timestamp: time.Now(),
			MessageSource: types.MessageSource{
				Chat:   types.JID{User: "628111", Server: types.DefaultUserServer},
				Sender: from,
			},
		},
		Message: &waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text: proto.String(text), ContextInfo: quoting,
		}},
	}
}

// The whole inbound half of quoting rests on one line in handle: reading the
// stanza id off the event and carrying it into storage. Without it a customer's
// reply arrives looking like an unrelated new message, and the CS has to guess
// which of their own answers it was about.
func TestAnIncomingReplyIsStoredAgainstTheMessageItQuotes(t *testing.T) {
	handler, messages, _, convID := inboundSetup(t)

	ours, err := messages.Queue(convID, uuid.New(), models.MessageKindText, "sudah kami cek", nil, nil)
	require.NoError(t, err)
	require.NoError(t, messages.MarkSent(ours.ID, "3EB0OURS"))

	evt := customerSays("masih mati juga", &waE2E.ContextInfo{StanzaID: proto.String("3EB0OURS")})
	require.NoError(t, handler.handle(context.Background(), evt))

	history, err := messages.History(convID, 10, 0)
	require.NoError(t, err)
	require.Len(t, history, 2)

	arrived := history[0]
	assert.Equal(t, "masih mati juga", arrived.Body)
	require.NotNil(t, arrived.ReplyToID, "the reply must point at the message it quotes")
	assert.Equal(t, ours.ID, *arrived.ReplyToID)
	require.NotNil(t, arrived.ReplyTo)
	assert.Equal(t, "sudah kami cek", arrived.ReplyTo.Body)
}

func TestAnIncomingMessageQuotingNothingIsStoredWithoutAQuote(t *testing.T) {
	handler, messages, _, convID := inboundSetup(t)

	require.NoError(t, handler.handle(context.Background(), customerSays("internet saya mati", nil)))

	history, err := messages.History(convID, 10, 0)
	require.NoError(t, err)
	require.Len(t, history, 1)
	assert.Nil(t, history[0].ReplyToID)
}

// A customer can quote a message this inbox has never held — one from before
// the number was linked, or one retention has swept. Their complaint still has
// to land.
func TestAnIncomingReplyToAnUnknownMessageIsStillStored(t *testing.T) {
	handler, messages, _, convID := inboundSetup(t)

	evt := customerSays("masih mati juga", &waE2E.ContextInfo{StanzaID: proto.String("3EB0NEVERSEEN")})
	require.NoError(t, handler.handle(context.Background(), evt))

	history, err := messages.History(convID, 10, 0)
	require.NoError(t, err)
	require.Len(t, history, 1)
	assert.Equal(t, "masih mati juga", history[0].Body)
	assert.Nil(t, history[0].ReplyToID)
}
