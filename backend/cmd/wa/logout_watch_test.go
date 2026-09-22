package main

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// An admin pressing Disconnect already drops the session, because the logout
// deletes the device and nothing can pair on a deleted one. WhatsApp can end
// the link on its own, which deletes the device just the same, and nothing
// watched for that - so the process kept a client that answered every later
// Connect with "invalid use of deleted device" and the number could not be
// paired again without restarting the container.
func TestWatchForLogoutDropsTheSessionWhenWhatsAppEndsTheLink(t *testing.T) {
	loggedOut := make(chan struct{}, 1)
	dropped := make(chan struct{}, 1)

	loggedOut <- struct{}{}
	watchForLogout(context.Background(), loggedOut, func() { dropped <- struct{}{} })

	select {
	case <-dropped:
	case <-time.After(time.Second):
		t.Fatal("the session was not dropped, so the number stays unpairable until the process restarts")
	}
}

// Shutdown is not a logout. Dropping a session on the way down would have the
// next start rebuild it around a new device and lose a perfectly good pairing.
func TestWatchForLogoutLeavesTheSessionAloneOnShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	calls := 0
	watchForLogout(ctx, make(chan struct{}), func() { calls++ })

	assert.Zero(t, calls, "a shutdown dropped the session as if WhatsApp had ended the link")
}
