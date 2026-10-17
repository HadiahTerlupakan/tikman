package wa

import (
	"context"
	"sync"
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

var teamGroup = types.JID{User: "120363000000000001", Server: types.GroupServer}

type groupInboundEnv struct {
	handler       *inboundHandler
	groups        *services.WAGroupService
	groupMessages *services.WAGroupMessageService
	conversations *services.CSConversationService
	seen          *seenGroups
}

type seenGroups struct {
	mu   sync.Mutex
	jids []types.JID
}

func (s *seenGroups) record(jid types.JID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jids = append(s.jids, jid)
}

func groupInboundSetup(t *testing.T) groupInboundEnv {
	t.Helper()
	db, messages, conversations, _ := drainSetup(t)
	handler := testInboundHandler(t, db, messages, conversations)
	env := groupInboundEnv{
		handler:       handler,
		groups:        services.NewWAGroupService(db),
		groupMessages: services.NewWAGroupMessageService(db),
		conversations: conversations,
		seen:          &seenGroups{},
	}
	handler.groups = env.groups
	handler.groupMessages = env.groupMessages
	handler.groupSeen = env.seen.record
	return env
}

func memberWrites(id, text string, mentions ...string) *events.Message {
	ext := &waE2E.ExtendedTextMessage{Text: proto.String(text)}
	if len(mentions) > 0 {
		ext.ContextInfo = &waE2E.ContextInfo{MentionedJID: mentions}
	}
	return &events.Message{
		Info: types.MessageInfo{
			ID:        id,
			PushName:  "Rina",
			Timestamp: time.Now(),
			MessageSource: types.MessageSource{
				Chat:      teamGroup,
				Sender:    types.JID{User: "99887766554433", Server: types.HiddenUserServer},
				SenderAlt: types.JID{User: "628122223333", Server: types.DefaultUserServer},
				IsGroup:   true,
			},
		},
		Message: &waE2E.Message{ExtendedTextMessage: ext},
	}
}

// The point of the feature: group traffic lands in its own tables and never
// in the customer inbox.
func TestAGroupMessageIsStoredWithTheGroupAndNotInTheInbox(t *testing.T) {
	env := groupInboundSetup(t)
	before, err := env.conversations.List(services.ConversationFilter{})
	require.NoError(t, err)

	require.NoError(t, env.handler.handle(context.Background(), memberWrites("W1", "ODP 12 putus")))

	after, err := env.conversations.List(services.ConversationFilter{})
	require.NoError(t, err)
	assert.Len(t, after, len(before), "a group must not open a customer thread")

	groups, err := env.groups.List(services.GroupFilter{})
	require.NoError(t, err)
	require.Len(t, groups, 1)
	assert.Equal(t, teamGroup.String(), groups[0].JID)
	assert.Equal(t, 1, groups[0].UnreadCount)

	history, err := env.groupMessages.History(groups[0].ID, 10, 0)
	require.NoError(t, err)
	require.Len(t, history, 1)
	assert.Equal(t, "ODP 12 putus", history[0].Body)
	assert.Equal(t, "Rina", history[0].SenderPushName)
	assert.Empty(t, history[0].SenderName, "nobody saved her, so a phone would show ~Rina")
	assert.Equal(t, "628122223333", history[0].SenderPhone, "the number comes from SenderAlt when the sender is a LID")
	assert.Equal(t, models.MessageIn, history[0].Direction)
}

// A group nobody has synced yet has no name; the handler has to say so, or
// the list shows it as a bare number until the hourly sync.
func TestAMessageFromAnUnknownGroupAsksForItsDetails(t *testing.T) {
	env := groupInboundSetup(t)

	require.NoError(t, env.handler.handle(context.Background(), memberWrites("W1", "halo")))
	require.NoError(t, env.handler.handle(context.Background(), memberWrites("W2", "halo lagi")))

	require.Eventually(t, func() bool {
		env.seen.mu.Lock()
		defer env.seen.mu.Unlock()
		return len(env.seen.jids) == 1
	}, time.Second, 10*time.Millisecond, "asked once, for the first message only")
}

func TestAMentionIsShownAsTheMembersName(t *testing.T) {
	env := groupInboundSetup(t)
	_, err := env.groups.Upsert(env.handler.accountID, services.GroupSnapshot{
		JID: teamGroup.String(), Name: "Teknisi",
		Participants: []models.WAGroupParticipant{{
			JID: "55511122233344@lid", LID: "55511122233344@lid", Phone: "628133334444",
			Name: "Pak Joko", Role: models.GroupRoleMember,
		}},
	})
	require.NoError(t, err)

	evt := memberWrites("W1", "@55511122233344 tolong cek", "55511122233344@lid")
	require.NoError(t, env.handler.handle(context.Background(), evt))

	groups, err := env.groups.List(services.GroupFilter{})
	require.NoError(t, err)
	history, err := env.groupMessages.History(groups[0].ID, 10, 0)
	require.NoError(t, err)
	require.Len(t, history, 1)
	assert.Equal(t, "@Pak Joko tolong cek", history[0].Body)
}

func TestWhatThePhoneWritesInAGroupIsStoredAsOurs(t *testing.T) {
	env := groupInboundSetup(t)
	evt := memberWrites("W1", "siap meluncur")
	evt.Info.IsFromMe = true

	require.NoError(t, env.handler.handle(context.Background(), evt))

	groups, err := env.groups.List(services.GroupFilter{})
	require.NoError(t, err)
	require.Len(t, groups, 1)
	assert.Zero(t, groups[0].UnreadCount)
	history, err := env.groupMessages.History(groups[0].ID, 10, 0)
	require.NoError(t, err)
	require.Len(t, history, 1)
	assert.Equal(t, models.MessageOut, history[0].Direction)
}

// whatsmeow marks status updates IsGroup as well; they are not a group anyone
// can read here.
func TestAStatusUpdateIsNotTakenForAGroup(t *testing.T) {
	env := groupInboundSetup(t)
	evt := memberWrites("W1", "promo")
	evt.Info.Chat = types.StatusBroadcastJID

	require.NoError(t, env.handler.handle(context.Background(), evt))

	groups, err := env.groups.List(services.GroupFilter{})
	require.NoError(t, err)
	assert.Empty(t, groups)
}
