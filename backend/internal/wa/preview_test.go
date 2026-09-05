package wa

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/wa/linkpreview"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
)

// Without a preview the message keeps the plain shape it has always had.
func TestNoPreviewLeavesAPlainConversation(t *testing.T) {
	msg := buildTextMessage("halo pak", nil, nil)

	assert.Equal(t, "halo pak", msg.GetConversation())
	assert.Nil(t, msg.GetExtendedTextMessage())
}

// The card is carried by the extended form, and the text has to survive the
// change of shape — a message that loses its body to gain a picture is worse
// than no preview at all.
func TestAPreviewCarriesTheTextAndTheCard(t *testing.T) {
	p := &linkpreview.Preview{
		URL:         "https://example.com",
		Title:       "Judul",
		Description: "Deskripsi",
		Thumbnail:   []byte{0xff, 0xd8, 0xff},
	}

	msg := buildTextMessage("lihat https://example.com ya", nil, p)

	ext := msg.GetExtendedTextMessage()
	require.NotNil(t, ext)
	assert.Equal(t, "lihat https://example.com ya", ext.GetText())
	assert.Equal(t, "https://example.com", ext.GetMatchedText())
	assert.Equal(t, waE2E.ExtendedTextMessage_NONE, ext.GetPreviewType())
	assert.Equal(t, "Judul", ext.GetTitle())
	assert.Equal(t, "Deskripsi", ext.GetDescription())
	assert.Equal(t, []byte{0xff, 0xd8, 0xff}, ext.GetJPEGThumbnail())
	assert.Empty(t, msg.GetConversation())
}

// A quote and a preview have to coexist: both live on the extended form, and
// an earlier version of this code could carry only one.
func TestAQuotedReplyKeepsItsQuoteAlongsideThePreview(t *testing.T) {
	p := &linkpreview.Preview{URL: "https://example.com", Title: "Judul"}
	chat := types.NewJID("628111", types.DefaultUserServer)
	self := types.NewJID("628999", types.DefaultUserServer)
	ctx := buildContextInfo(&Quote{StanzaID: "ABC", Body: "pesan lama"}, chat, self)
	require.NotNil(t, ctx)

	msg := buildTextMessage("balasan", ctx, p)

	ext := msg.GetExtendedTextMessage()
	require.NotNil(t, ext)
	assert.Equal(t, "Judul", ext.GetTitle())
	assert.Equal(t, "ABC", ext.GetContextInfo().GetStanzaID())
}
