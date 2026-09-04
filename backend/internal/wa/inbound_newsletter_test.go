package wa

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/services"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

// channelPosts builds an update arriving from a WhatsApp Channel. The sender is
// the channel itself, which is what a follower's client receives.
func channelPosts(channelUser, text string) *events.Message {
	channel := types.JID{User: channelUser, Server: types.NewsletterServer}
	return &events.Message{
		Info: types.MessageInfo{
			ID:        "3EB0CHANNEL",
			PushName:  channelUser,
			Timestamp: time.Now(),
			MessageSource: types.MessageSource{
				Chat:   channel,
				Sender: channel,
			},
		},
		Message: &waE2E.Message{Conversation: proto.String(text)},
	}
}

// A channel the number follows is not a customer. Its updates used to reach
// FindOrCreate and open a thread named after the channel's JID, which a CS then
// had to recognise as not-a-person and ignore.
func TestAFollowedChannelsUpdateOpensNoThread(t *testing.T) {
	handler, _, conversations, _ := inboundSetup(t)
	before, err := conversations.List(services.ConversationFilter{})
	require.NoError(t, err)

	err = handler.handle(context.Background(),
		channelPosts("120363204165958923", "*Majelis Tafsir Al-Qur'an Arraudhah*"))

	require.NoError(t, err)
	after, err := conversations.List(services.ConversationFilter{})
	require.NoError(t, err)
	assert.Len(t, after, len(before), "a channel update must not open a thread")
}

// WhatsApp delivers our own channel update back to the number that posted it,
// and it does not arrive flagged as ours: the sender is the channel, not this
// device, so IsFromMe is false and cannot be what stops it. Before the
// newsletter guard this echo opened a thread that looked like a new customer —
// and a CS answering it published their greeting to the channel's subscribers.
func TestOurOwnChannelUpdateComingBackOpensNoThread(t *testing.T) {
	handler, _, conversations, _ := inboundSetup(t)
	before, err := conversations.List(services.ConversationFilter{})
	require.NoError(t, err)

	echo := channelPosts("120363399457624066", "Selamat datang di chanel SBL Network")
	require.False(t, echo.Info.IsFromMe, "the echo arrives as the channel, not as us")

	require.NoError(t, handler.handle(context.Background(), echo))

	after, err := conversations.List(services.ConversationFilter{})
	require.NoError(t, err)
	assert.Len(t, after, len(before), "our own channel update must not open a thread")
}
