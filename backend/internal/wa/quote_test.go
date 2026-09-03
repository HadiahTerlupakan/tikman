package wa

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

var (
	customer = types.JID{User: "628111222333", Server: types.DefaultUserServer}
	ourself  = types.JID{User: "628999888777", Server: types.DefaultUserServer}
)

// WhatsApp finds the quoted message by participant, so quoting the customer
// must name the customer. Naming ourselves draws an empty grey box on their
// phone — the quote silently stops working while our own screen looks right.
func TestQuotingTheCustomerNamesTheCustomer(t *testing.T) {
	ctx := buildContextInfo(&Quote{
		StanzaID: "3EB0AAA", FromMe: false,
		Body: "internet saya mati", Kind: models.MessageKindText,
	}, customer, ourself)

	require.NotNil(t, ctx)
	assert.Equal(t, "3EB0AAA", ctx.GetStanzaID())
	assert.Equal(t, "628111222333@s.whatsapp.net", ctx.GetParticipant())
	assert.Equal(t, "internet saya mati", ctx.GetQuotedMessage().GetConversation())
}

func TestQuotingOurOwnReplyNamesThisInbox(t *testing.T) {
	ctx := buildContextInfo(&Quote{
		StanzaID: "3EB0BBB", FromMe: true,
		Body: "sudah kami cek", Kind: models.MessageKindText,
	}, customer, ourself)

	require.NotNil(t, ctx)
	assert.Equal(t, "628999888777@s.whatsapp.net", ctx.GetParticipant())
}

// A device suffix belongs to one phone or laptop, not to the person. Leaving it
// on the participant is another way to name someone WhatsApp cannot match.
func TestQuoteStripsTheDeviceFromTheParticipant(t *testing.T) {
	withDevice := types.JID{User: "628111222333", Server: types.DefaultUserServer, Device: 3}

	ctx := buildContextInfo(&Quote{StanzaID: "3EB0CCC", Kind: models.MessageKindText}, withDevice, ourself)

	require.NotNil(t, ctx)
	assert.Equal(t, "628111222333@s.whatsapp.net", ctx.GetParticipant())
}

func TestNoQuoteMeansNoContext(t *testing.T) {
	assert.Nil(t, buildContextInfo(nil, customer, ourself))
}

// The original protobuf is not kept, so a quoted photo is rebuilt as a photo
// carrying its caption. Rebuilt as plain text it would quote the caption as if
// the customer had typed it, with no sign a picture was ever involved.
func TestQuotedPhotoIsRebuiltAsAPhoto(t *testing.T) {
	ctx := buildContextInfo(&Quote{
		StanzaID: "3EB0DDD", Body: "ini modemnya", Kind: models.MessageKindImage,
	}, customer, ourself)

	quoted := ctx.GetQuotedMessage()
	require.NotNil(t, quoted.GetImageMessage())
	assert.Equal(t, "ini modemnya", quoted.GetImageMessage().GetCaption())
	assert.Empty(t, quoted.GetConversation())
}

// A customer's reply arrives quoting a stanza id, and it hangs off whichever
// shape they replied with — a photo reply carries it on the photo.
func TestQuotedStanzaIDIsFoundOnEveryShapeThisInboxStores(t *testing.T) {
	ctx := &waE2E.ContextInfo{StanzaID: proto.String("3EB0EEE")}

	shapes := map[string]*waE2E.Message{
		"text":     {ExtendedTextMessage: &waE2E.ExtendedTextMessage{ContextInfo: ctx}},
		"image":    {ImageMessage: &waE2E.ImageMessage{ContextInfo: ctx}},
		"video":    {VideoMessage: &waE2E.VideoMessage{ContextInfo: ctx}},
		"audio":    {AudioMessage: &waE2E.AudioMessage{ContextInfo: ctx}},
		"document": {DocumentMessage: &waE2E.DocumentMessage{ContextInfo: ctx}},
	}
	for shape, msg := range shapes {
		assert.Equal(t, "3EB0EEE", quotedStanzaID(msg), shape)
	}
}

func TestAMessageQuotingNothingHasNoStanzaID(t *testing.T) {
	assert.Empty(t, quotedStanzaID(&waE2E.Message{Conversation: proto.String("halo")}))
}
