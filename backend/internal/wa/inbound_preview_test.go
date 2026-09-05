package wa

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"
)

// A customer's own WhatsApp builds the card and sends it with the message, so
// the inbox can show exactly what they saw without fetching anything.
func TestAnIncomingCardIsTakenFromTheMessage(t *testing.T) {
	msg := &waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{
		Text:          proto.String("lihat https://example.com ya"),
		MatchedText:   proto.String("https://example.com"),
		Title:         proto.String("Judul"),
		Description:   proto.String("Deskripsi"),
		JPEGThumbnail: []byte{0xff, 0xd8, 0xff},
	}}

	got := inboundPreview(msg)

	assert.Equal(t, "https://example.com", got.URL)
	assert.Equal(t, "Judul", got.Title)
	assert.Equal(t, "Deskripsi", got.Description)
	assert.Equal(t, []byte{0xff, 0xd8, 0xff}, got.Thumbnail)
}

// A quoted reply also arrives in the extended form but carries no card. It
// must not produce an empty one, or the bubble draws a blank box.
func TestAnExtendedMessageWithoutACardYieldsNothing(t *testing.T) {
	msg := &waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{
		Text: proto.String("balasan biasa"),
	}}

	assert.Nil(t, inboundPreview(msg))
}

func TestAPlainMessageYieldsNothing(t *testing.T) {
	assert.Nil(t, inboundPreview(&waE2E.Message{Conversation: proto.String("halo")}))
	assert.Nil(t, inboundPreview(nil))
}

// A card with a URL but no title is the grey box the send path already
// refuses to build; the inbound side has to refuse it too.
func TestACardWithNoTitleYieldsNothing(t *testing.T) {
	msg := &waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{
		Text:        proto.String("x"),
		MatchedText: proto.String("https://example.com"),
	}}

	assert.Nil(t, inboundPreview(msg))
}
