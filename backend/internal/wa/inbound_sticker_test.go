package wa

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

func customerSendsASticker() *events.Message {
	return &events.Message{
		Info: types.MessageInfo{
			ID:        "3EB0STICKER",
			PushName:  "Budi",
			Timestamp: time.Now(),
			MessageSource: types.MessageSource{
				Chat:   types.JID{User: "628111", Server: types.DefaultUserServer},
				Sender: types.JID{User: "628111222333", Server: types.DefaultUserServer},
			},
		},
		Message: &waE2E.Message{StickerMessage: &waE2E.StickerMessage{
			Mimetype: proto.String("image/webp"),
		}},
	}
}

// A sticker used to be dropped before a thread was ever created, so the CS was
// never told the customer had written and the customer sat on two ticks and
// silence. It has to land like any other attachment.
func TestACustomersStickerIsStoredInTheirThread(t *testing.T) {
	handler, messages, _, convID := inboundSetup(t)

	require.NoError(t, handler.handle(context.Background(), customerSendsASticker()))

	history, err := messages.History(convID, 10, 0)
	require.NoError(t, err)
	require.Len(t, history, 1, "the sticker has to reach the thread")
	assert.Equal(t, models.MessageKindSticker, history[0].Kind)
	assert.Equal(t, models.MessageIn, history[0].Direction)
}
