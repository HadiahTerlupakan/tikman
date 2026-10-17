package wa

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waCommon"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

func (env groupInboundEnv) only(t *testing.T) models.WAGroupMessage {
	t.Helper()
	groups, err := env.groups.List(services.GroupFilter{})
	require.NoError(t, err)
	require.Len(t, groups, 1)
	history, err := env.groupMessages.History(groups[0].ID, 10, 0)
	require.NoError(t, err)
	require.Len(t, history, 1)
	return history[0]
}

func memberSends(id string, msg *waE2E.Message) *events.Message {
	evt := memberWrites(id, "")
	evt.Message = msg
	return evt
}

func key(id string) *waCommon.MessageKey { return &waCommon.MessageKey{ID: proto.String(id)} }

func TestAReactionLandsUnderTheMessageItNames(t *testing.T) {
	env := groupInboundSetup(t)
	require.NoError(t, env.handler.handle(context.Background(), memberWrites("W1", "ODP 3 nyala")))

	reaction := &waE2E.Message{ReactionMessage: &waE2E.ReactionMessage{Key: key("W1"), Text: proto.String("👍")}}
	require.NoError(t, env.handler.handle(context.Background(), memberSends("W2", reaction)))

	msg := env.only(t)
	require.Len(t, msg.Reactions, 1, "a reaction is not a message of its own")
	assert.Equal(t, "👍", msg.Reactions[0].Emoji)
	assert.Equal(t, "Rina", msg.Reactions[0].Reactors[0].Name)
}

func TestADeletionAndAnEditFromTheSenderReachTheStoredMessage(t *testing.T) {
	env := groupInboundSetup(t)
	require.NoError(t, env.handler.handle(context.Background(), memberWrites("W1", "jam 2")))

	edit := &waE2E.Message{ProtocolMessage: &waE2E.ProtocolMessage{
		Type: waE2E.ProtocolMessage_MESSAGE_EDIT.Enum(), Key: key("W1"),
		EditedMessage: &waE2E.Message{Conversation: proto.String("jam 3")},
	}}
	require.NoError(t, env.handler.handle(context.Background(), memberSends("W2", edit)))
	edited := env.only(t)
	assert.Equal(t, "jam 3", edited.Body)
	assert.True(t, edited.Edited)

	revoke := &waE2E.Message{ProtocolMessage: &waE2E.ProtocolMessage{
		Type: waE2E.ProtocolMessage_REVOKE.Enum(), Key: key("W1"),
	}}
	require.NoError(t, env.handler.handle(context.Background(), memberSends("W3", revoke)))
	deleted := env.only(t)
	assert.True(t, deleted.Deleted)
	assert.Empty(t, deleted.Body)
}

func TestALocationIsStoredWithItsCoordinates(t *testing.T) {
	env := groupInboundSetup(t)
	loc := &waE2E.Message{LocationMessage: &waE2E.LocationMessage{
		DegreesLatitude: proto.Float64(-6.5), DegreesLongitude: proto.Float64(106.9), Name: proto.String("ODP Cariu 3"),
	}}

	require.NoError(t, env.handler.handle(context.Background(), memberSends("W1", loc)))

	msg := env.only(t)
	assert.Equal(t, models.MessageKindLocation, msg.Kind)
	assert.Equal(t, "Lokasi: ODP Cariu 3", msg.Body)
	assert.JSONEq(t, `{"latitude":-6.5,"longitude":106.9,"name":"ODP Cariu 3","address":"","url":""}`, string(msg.ExtraData))
}

func TestAContactCardKeepsItsNumbers(t *testing.T) {
	env := groupInboundSetup(t)
	card := &waE2E.Message{ContactMessage: &waE2E.ContactMessage{
		DisplayName: proto.String("Pak Joko"),
		Vcard:       proto.String("BEGIN:VCARD\nFN:Pak Joko\nTEL;type=CELL;waid=628133334444:+62 813-3333-4444\nEND:VCARD"),
	}}

	require.NoError(t, env.handler.handle(context.Background(), memberSends("W1", card)))

	msg := env.only(t)
	assert.Equal(t, models.MessageKindContact, msg.Kind)
	assert.JSONEq(t, `{"contacts":[{"display_name":"Pak Joko","phones":["+62 813-3333-4444"]}]}`, string(msg.ExtraData))
}

func TestAPollIsStoredWithItsOptions(t *testing.T) {
	env := groupInboundSetup(t)
	poll := &waE2E.Message{PollCreationMessageV3: &waE2E.PollCreationMessage{
		Name: proto.String("Lembur?"), SelectableOptionsCount: proto.Uint32(1),
		Options: []*waE2E.PollCreationMessage_Option{{OptionName: proto.String("Ya")}, {OptionName: proto.String("Tidak")}},
	}}

	require.NoError(t, env.handler.handle(context.Background(), memberSends("W1", poll)))

	msg := env.only(t)
	assert.Equal(t, models.MessageKindPoll, msg.Kind)
	extra, err := services.DecodePoll(&msg)
	require.NoError(t, err)
	assert.Equal(t, "Lembur?", extra.Question)
	assert.Len(t, extra.Options, 2)
}

// A vote names its options by hash; reading it back depends on hashing the
// names exactly as WhatsApp does.
func TestChosenOptionsTurnsHashesBackIntoNames(t *testing.T) {
	poll := services.PollExtra{Options: []services.PollOption{{Name: "Ya"}, {Name: "Tidak"}}}
	hashes := whatsmeow.HashPollOptions([]string{"Tidak"})

	assert.Equal(t, []string{"Tidak"}, chosenOptions(poll, hashes))
}

func TestAForwardedMessageIsMarkedSo(t *testing.T) {
	env := groupInboundSetup(t)
	fwd := &waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{
		Text: proto.String("info gangguan"), ContextInfo: &waE2E.ContextInfo{IsForwarded: proto.Bool(true)},
	}}

	require.NoError(t, env.handler.handle(context.Background(), memberSends("W1", fwd)))

	assert.True(t, env.only(t).Forwarded)
}

func TestGroupChangesAreWordedTheWayAPhoneWordsThem(t *testing.T) {
	budi := types.JID{User: "628111", Server: types.DefaultUserServer}
	rina := types.JID{User: "628222", Server: types.DefaultUserServer}
	members := []models.WAGroupParticipant{
		{JID: budi.String(), Name: "Budi"}, {JID: rina.String(), PushName: "Rina"},
	}
	label := memberLabeller(members, selfAddresses{phone: ourPhone})

	cases := []struct {
		name string
		evt  events.GroupInfo
		want []string
	}{
		{"adding someone", events.GroupInfo{Sender: &budi, Join: []types.JID{rina}}, []string{"Budi menambahkan Rina"}},
		{"leaving", events.GroupInfo{Sender: &rina, Leave: []types.JID{rina}}, []string{"Rina keluar"}},
		{"removing us", events.GroupInfo{Sender: &budi, Leave: []types.JID{ourPhone}}, []string{"Budi mengeluarkan Anda"}},
		{"a stranger joining by link", events.GroupInfo{
			Sender: &types.JID{User: "628333", Server: types.DefaultUserServer},
			Join:   []types.JID{{User: "628333", Server: types.DefaultUserServer}}, JoinReason: "invite",
		}, []string{"+628333 bergabung lewat tautan undangan"}},
		{"renaming", events.GroupInfo{Sender: &budi, Name: &types.GroupName{Name: "Teknisi Cariu"}},
			[]string{`Budi mengubah nama grup menjadi "Teknisi Cariu"`}},
		{"admins only", events.GroupInfo{Sender: &budi, Announce: &types.GroupAnnounce{IsAnnounce: true}},
			[]string{"Budi mengubah setelan grup: hanya admin yang dapat mengirim pesan"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			evt := tc.evt
			assert.Equal(t, tc.want, systemNotices(&evt, label))
		})
	}
}

func TestOurOwnReactionAndDeletionAreRecordedOnceWhatsAppAccepts(t *testing.T) {
	env := groupDrainSetup(t)
	root := t.TempDir()
	mine, err := env.messages.Queue(services.GroupOutgoing{
		GroupID: env.group.ID, SenderUserID: uuid.New(), Kind: models.MessageKindText, Body: "salah ketik",
	})
	require.NoError(t, err)
	require.NoError(t, env.messages.MarkSent(mine.ID, "WMINE", nil))
	queue := func(kind models.GroupActionKind, args services.GroupActionArgs) {
		args.MessageID = mine.ID.String()
		queueAction(t, env, services.GroupActionRequest{Action: kind, Args: args})
	}
	admin := &fakeGroupAdmin{}
	runner := NewGroupActionRunner(env.account.ID, services.NewWAGroupActionService(env.db), env.groups,
		env.messages, admin, &recordingAnnouncer{}, root)

	queue(models.GroupActionReact, services.GroupActionArgs{Emoji: "🙏"})
	queue(models.GroupActionRevoke, services.GroupActionArgs{})
	_, err = runner.Run(context.Background(), 10)
	require.NoError(t, err)

	require.Len(t, admin.calls, 2)
	assert.Equal(t, "react:🙏:WMINE:", admin.calls[0].name)
	assert.Equal(t, "revoke:WMINE:", admin.calls[1].name, "our own message is revoked with no sender")
	stored, err := env.messages.Get(mine.ID)
	require.NoError(t, err)
	assert.True(t, stored.Deleted)
}

func TestHistoryIsAskedForFromTheOldestMessage(t *testing.T) {
	env := groupDrainSetup(t)
	older := time.Now().Add(-time.Hour)
	for i, at := range []time.Time{time.Now(), older} {
		_, _, err := env.messages.SaveArrived(services.GroupArrival{
			GroupID: env.group.ID, WAMessageID: []string{"WNEW", "WOLD"}[i], Direction: models.MessageIn,
			Kind: models.MessageKindText, Body: "x", At: at,
		})
		require.NoError(t, err)
	}
	queueAction(t, env, services.GroupActionRequest{Action: models.GroupActionHistory})
	admin := &fakeGroupAdmin{}

	runActions(t, env, admin, &recordingAnnouncer{})

	require.Len(t, admin.calls, 1)
	assert.Equal(t, "history:WOLD", admin.calls[0].name)
}

func TestAPollIsSentAsAPoll(t *testing.T) {
	env := groupDrainSetup(t)
	extra, err := services.NewPollExtra("Lembur?", []string{"Ya", "Tidak"}, 1)
	require.NoError(t, err)
	_, err = env.messages.Queue(services.GroupOutgoing{
		GroupID: env.group.ID, SenderUserID: uuid.New(), Kind: models.MessageKindPoll, Body: "Polling: Lembur?", Extra: extra,
	})
	require.NoError(t, err)
	sender := &fakeGroupSender{}

	_, err = NewGroupDrainer(env.account.ID, env.messages, sender, nil, t.TempDir(), 0).Drain(context.Background(), 10)
	require.NoError(t, err)

	assert.Equal(t, [][]string{{"Lembur?", "Ya", "Tidak"}}, sender.polls)
}
