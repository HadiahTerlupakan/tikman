package wa

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"go.mau.fi/whatsmeow/types/events"
	"go.uber.org/zap"
)

// When WhatsApp logs a number out it deletes that device, and every later
// Connect on it fails with "invalid use of deleted device" - there is no way
// back to a pairable state inside this process. cmd/wa already knows that for
// the admin-initiated logout, where ControlDisconnect drops the session so the
// next rescan opens a fresh one. Nothing did the same when WhatsApp was the
// one ending it, so the process kept the dead client and the number could
// never be paired again, however many times an admin tried - which is exactly
// what happened to a production number.
//
// The signal is what lets cmd/wa treat both logouts the same way.
func TestRouteSignalsALoggedOutSessionSoItCanBeRebuilt(t *testing.T) {
	client := &Client{
		accountID: uuid.New(),
		logger:    zap.NewNop(),
		ctx:       context.Background(),
		dropped:   make(chan struct{}, 1),
		loggedOut: make(chan struct{}, 1),
	}

	client.route(&events.LoggedOut{})

	select {
	case <-client.LoggedOut():
	default:
		t.Fatal("a logged-out session raised no signal, so nothing would rebuild it and the number stays unpairable")
	}
}

// The db is nil here, so recording the status panics and route's recover
// catches it. The rebuild signal must already be out by then: a database blip
// must not be able to leave a number permanently unpairable, which it would if
// the signal sat behind the write.
func TestRouteSignalsALoggedOutSessionEvenWhenRecordingTheStatusFails(t *testing.T) {
	client := &Client{
		accountID: uuid.New(),
		logger:    zap.NewNop(),
		ctx:       context.Background(),
		db:        nil,
		dropped:   make(chan struct{}, 1),
		loggedOut: make(chan struct{}, 1),
	}

	client.route(&events.LoggedOut{})

	select {
	case <-client.LoggedOut():
	default:
		t.Fatal("the rebuild signal was lost because the status write failed first")
	}
}
