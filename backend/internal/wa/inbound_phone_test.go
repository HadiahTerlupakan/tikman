package wa

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

// customerNumber is the address inboundSetup opens its thread under.
var customerNumber = types.JID{User: "628111", Server: types.DefaultUserServer}

// customerLID is the same customer as WhatsApp's privacy identifier names them.
var customerLID = types.JID{User: "111222333444555", Server: types.HiddenUserServer}

// phoneSends builds what whatsmeow hands over when the phone holding the number
// sends a message: IsFromMe, the recipient as the chat, and the recipient's
// other address in RecipientAlt.
func phoneSends(id, text string, chat, recipientAlt types.JID) *events.Message {
	return &events.Message{
		Info: types.MessageInfo{
			ID:        id,
			PushName:  "CS Utama",
			Timestamp: time.Now(),
			MessageSource: types.MessageSource{
				Chat:         chat,
				Sender:       types.JID{User: "628999000111", Server: types.DefaultUserServer},
				IsFromMe:     true,
				RecipientAlt: recipientAlt,
			},
		},
		Message: &waE2E.Message{Conversation: proto.String(text)},
	}
}

func TestAMessageTypedOnThePhoneLandsInTheCustomersThread(t *testing.T) {
	handler, messages, _, convID := inboundSetup(t)

	evt := phoneSends("3EB0PHONE", "sudah kami cek", customerNumber, types.EmptyJID)
	require.NoError(t, handler.handle(context.Background(), evt))

	history, err := messages.History(convID, 10, 0)
	require.NoError(t, err)
	require.Len(t, history, 1)
	assert.Equal(t, "sudah kami cek", history[0].Body)
	assert.Equal(t, models.MessageOut, history[0].Direction)
	assert.Nil(t, history[0].SenderUserID)
}

// The phone's copy may name the customer by LID while the thread was opened
// under their number. Looking only at Chat would drop the reply.
func TestAPhoneMessageAddressedByLIDFindsTheThreadOpenedUnderTheNumber(t *testing.T) {
	handler, messages, _, convID := inboundSetup(t)

	evt := phoneSends("3EB0LID", "sudah kami cek", customerLID, customerNumber)
	require.NoError(t, handler.handle(context.Background(), evt))

	history, err := messages.History(convID, 10, 0)
	require.NoError(t, err)
	assert.Len(t, history, 1)
}

// WhatsApp re-delivers on every reconnect; the reply must show once.
func TestAPhoneMessageDeliveredTwiceShowsOnce(t *testing.T) {
	handler, messages, _, convID := inboundSetup(t)
	evt := phoneSends("3EB0TWICE", "sudah kami cek", customerNumber, types.EmptyJID)

	require.NoError(t, handler.handle(context.Background(), evt))
	require.NoError(t, handler.handle(context.Background(), evt))

	history, err := messages.History(convID, 10, 0)
	require.NoError(t, err)
	assert.Len(t, history, 1)
}

// The number's phone also carries chats that are not customers'. One the phone
// starts with somebody who has no thread must not open one.
func TestAPhoneMessageToSomeoneWithNoThreadOpensNothing(t *testing.T) {
	handler, messages, conversations, convID := inboundSetup(t)
	before, err := conversations.List(services.ConversationFilter{})
	require.NoError(t, err)

	stranger := types.JID{User: "628777000111", Server: types.DefaultUserServer}
	require.NoError(t, handler.handle(context.Background(),
		phoneSends("3EB0STRANGER", "nanti saya telepon", stranger, types.EmptyJID)))

	after, err := conversations.List(services.ConversationFilter{})
	require.NoError(t, err)
	assert.Len(t, after, len(before))
	history, err := messages.History(convID, 10, 0)
	require.NoError(t, err)
	assert.Empty(t, history)
}

// Letting IsFromMe through must not let the phone's groups and channels in with
// it.
func TestAPhoneMessageToAGroupOrChannelIsStillIgnored(t *testing.T) {
	cases := []struct {
		name    string
		chat    types.JID
		isGroup bool
	}{
		{"group", types.JID{User: "120363000000000000", Server: types.GroupServer}, true},
		{"channel", types.JID{User: "120363399457624066", Server: types.NewsletterServer}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler, messages, conversations, convID := inboundSetup(t)
			before, err := conversations.List(services.ConversationFilter{})
			require.NoError(t, err)

			evt := phoneSends("3EB0"+tc.name, "rapat jam 3", tc.chat, types.EmptyJID)
			evt.Info.IsGroup = tc.isGroup
			require.NoError(t, handler.handle(context.Background(), evt))

			after, err := conversations.List(services.ConversationFilter{})
			require.NoError(t, err)
			assert.Len(t, after, len(before))
			history, err := messages.History(convID, 10, 0)
			require.NoError(t, err)
			assert.Empty(t, history)
		})
	}
}
