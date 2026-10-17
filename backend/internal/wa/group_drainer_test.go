package wa

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"github.com/tikman/olt-provisioning/internal/wa/linkpreview"
	"gorm.io/gorm"
)

type groupSend struct {
	jid      string
	body     string
	quote    *Quote
	mentions []string
}

type fakeGroupSender struct {
	sends []groupSend
	err   error
}

func (f *fakeGroupSender) SendGroupText(
	_ context.Context, jid, body string, quote *Quote, mentions []string, _ *linkpreview.Preview,
) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	f.sends = append(f.sends, groupSend{jid: jid, body: body, quote: quote, mentions: mentions})
	return "3EB0G" + body, nil
}

func (f *fakeGroupSender) SendGroupMedia(
	_ context.Context, jid string, _ models.MessageKind, path, _, _, _ string, quote *Quote, mentions []string,
) (string, error) {
	return f.SendGroupText(context.Background(), jid, path, quote, mentions, nil)
}

type groupDrainEnv struct {
	db       *gorm.DB
	account  models.WAAccount
	group    *models.WAGroup
	groups   *services.WAGroupService
	messages *services.WAGroupMessageService
}

func groupDrainSetup(t *testing.T) groupDrainEnv {
	t.Helper()
	db, _, _, _ := drainSetup(t)
	var account models.WAAccount
	require.NoError(t, db.First(&account).Error)
	groups := services.NewWAGroupService(db)
	group, err := groups.Upsert(account.ID, services.GroupSnapshot{JID: teamGroup.String(), Name: "Teknisi"})
	require.NoError(t, err)
	return groupDrainEnv{db: db, account: account, group: group, groups: groups, messages: services.NewWAGroupMessageService(db)}
}

func TestGroupDrainSendsToTheGroupAndMarksItSent(t *testing.T) {
	env := groupDrainSetup(t)
	queued, err := env.messages.Queue(services.GroupOutgoing{
		GroupID: env.group.ID, SenderUserID: uuid.New(), Kind: models.MessageKindText, Body: "OTW",
	})
	require.NoError(t, err)
	sender := &fakeGroupSender{}
	announced := &recordingAnnouncer{}

	sent, err := NewGroupDrainer(env.account.ID, env.messages, sender, announced, t.TempDir(), 0).Drain(context.Background(), 10)
	require.NoError(t, err)

	assert.Equal(t, 1, sent)
	require.Len(t, sender.sends, 1)
	assert.Equal(t, teamGroup.String(), sender.sends[0].jid)
	stored, err := env.messages.Get(queued.ID)
	require.NoError(t, err)
	assert.Equal(t, models.MessageSent, stored.Status)
	require.Len(t, announced.events, 1)
	assert.Equal(t, EventGroupMessage, announced.events[0].Type)
	assert.Equal(t, env.group.ID.String(), announced.events[0].GroupID)
}

// In a group the chat is not the author. A quote naming the group as its
// participant reaches every phone as an empty grey box.
func TestAGroupReplyQuotesTheMemberWhoWroteTheMessage(t *testing.T) {
	env := groupDrainSetup(t)
	original, _, err := env.messages.SaveArrived(services.GroupArrival{
		GroupID: env.group.ID, WAMessageID: "W1", Direction: models.MessageIn,
		SenderJID: "99887766554433@lid", Kind: models.MessageKindText, Body: "ONT LOS", At: time.Now(),
	})
	require.NoError(t, err)
	_, err = env.messages.Queue(services.GroupOutgoing{
		GroupID: env.group.ID, SenderUserID: uuid.New(), Kind: models.MessageKindText, Body: "dicek", ReplyToID: &original.ID,
	})
	require.NoError(t, err)
	sender := &fakeGroupSender{}

	_, err = NewGroupDrainer(env.account.ID, env.messages, sender, nil, t.TempDir(), 0).Drain(context.Background(), 10)
	require.NoError(t, err)

	require.Len(t, sender.sends, 1)
	require.NotNil(t, sender.sends[0].quote)
	assert.Equal(t, "W1", sender.sends[0].quote.StanzaID)
	assert.Equal(t, "99887766554433@lid", sender.sends[0].quote.Participant)
}

func TestARefusedGroupMessageIsMarkedFailedWithWhatsAppsReason(t *testing.T) {
	env := groupDrainSetup(t)
	queued, err := env.messages.Queue(services.GroupOutgoing{
		GroupID: env.group.ID, SenderUserID: uuid.New(), Kind: models.MessageKindText, Body: "x",
	})
	require.NoError(t, err)
	sender := &fakeGroupSender{err: errors.New("not-authorized")}

	sent, err := NewGroupDrainer(env.account.ID, env.messages, sender, nil, t.TempDir(), 0).Drain(context.Background(), 10)
	require.NoError(t, err)

	assert.Zero(t, sent)
	stored, err := env.messages.Get(queued.ID)
	require.NoError(t, err)
	assert.Equal(t, models.MessageFailed, stored.Status)
	assert.Equal(t, "not-authorized", stored.FailReason)
}

func TestBuildContextInfoNamesTheGroupMemberBeingQuoted(t *testing.T) {
	self := customerNumber
	ctx := buildContextInfo(&Quote{StanzaID: "W1", Participant: "628122223333@s.whatsapp.net", Body: "a"}, teamGroup, self)
	assert.Equal(t, "628122223333@s.whatsapp.net", ctx.GetParticipant())

	ctx = buildContextInfo(&Quote{StanzaID: "W2", FromMe: true, Participant: "628122223333@s.whatsapp.net"}, teamGroup, self)
	assert.Equal(t, self.String(), ctx.GetParticipant(), "our own message is quoted as ours")
}

func TestAGroupMessageTagsTheMembersItMentions(t *testing.T) {
	env := groupDrainSetup(t)
	_, err := env.messages.Queue(services.GroupOutgoing{
		GroupID: env.group.ID, SenderUserID: uuid.New(), Kind: models.MessageKindText,
		Body: "@55511122233344 cek", Mentions: []string{"55511122233344@lid"},
	})
	require.NoError(t, err)
	sender := &fakeGroupSender{}

	_, err = NewGroupDrainer(env.account.ID, env.messages, sender, nil, t.TempDir(), 0).Drain(context.Background(), 10)
	require.NoError(t, err)

	require.Len(t, sender.sends, 1)
	assert.Equal(t, "@55511122233344 cek", sender.sends[0].body, "WhatsApp needs the address in the text")
	assert.Equal(t, []string{"55511122233344@lid"}, sender.sends[0].mentions)
}

// The text alone is not a mention; without MentionedJID nobody is notified.
func TestWithMentionsListsTheTaggedMembersBesideAQuote(t *testing.T) {
	quoted := buildContextInfo(&Quote{StanzaID: "W1", Participant: "628122223333@s.whatsapp.net"}, teamGroup, customerNumber)
	info := withMentions(quoted, []string{"55511122233344@lid"})

	assert.Equal(t, "W1", info.GetStanzaID())
	assert.Equal(t, []string{"55511122233344@lid"}, info.GetMentionedJID())
	assert.Nil(t, withMentions(nil, nil))
}
