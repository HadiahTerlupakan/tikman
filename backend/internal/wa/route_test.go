package wa

import (
	"testing"

	"github.com/google/uuid"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"go.uber.org/zap"
)

// This process holds every CS number at once. whatsmeow calls route on its own
// goroutine, so a panic that escapes it takes the process down and silences
// every other number over one malformed message from one customer.
//
// The nil inbound handler is the cheapest way to make handling genuinely
// panic; what is under test is that route absorbs it, not what caused it.
func TestRouteSurvivesAPanicWhileHandlingOneMessage(t *testing.T) {
	client := &Client{
		accountID: uuid.New(),
		logger:    zap.NewNop(),
		inbound:   nil,
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("a panic escaped route and would have taken every other number with it: %v", r)
		}
	}()

	client.route(&events.Message{Info: types.MessageInfo{ID: "3EB0A"}})
}
