package shell

import (
	"context"
	"errors"
	"testing"
	"time"
)

type nopCloser struct{}

func (nopCloser) Close() error { return nil }

// floodingSession is a session whose device never stops printing, the way a
// VTY with a log flood behaves. Every command written is sent to sent.
func floodingSession(t *testing.T) (*Session, <-chan string) {
	t.Helper()
	sent := make(chan string, 8)
	session := newSession(func(b []byte) error {
		sent <- string(b)
		return nil
	}, nopCloser{}, telnetEOL)
	go func() {
		for session.emit([]byte("%Nov 20 10:00:00 SW-TEST IFNET/4/LINK_STATE: flap\r\n")) {
			time.Sleep(5 * time.Millisecond)
		}
	}()
	t.Cleanup(func() { _ = session.Close() })
	return session, sent
}

func TestRunGivesUpOnAFloodingSwitchWhenItsContextEnds(t *testing.T) {
	session, _ := floodingSession(t)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		_, err := session.Run(ctx, "display version")
		done <- err
	}()

	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("err = %v, want deadline exceeded", err)
		}
	case <-time.After(pendingDrainLimit + time.Second):
		t.Fatal("Run kept draining after its context ended")
	}
}

func TestRunStopsDrainingAFloodAndSendsTheCommand(t *testing.T) {
	session, sent := floodingSession(t)
	go func() { _, _ = session.Run(testContext(t), "display version") }()

	select {
	case line := <-sent:
		if line != "display version"+telnetEOL {
			t.Fatalf("sent %q, want the command", line)
		}
	case <-time.After(pendingDrainLimit + time.Second):
		t.Fatal("the command was never sent while the switch kept printing")
	}
}
