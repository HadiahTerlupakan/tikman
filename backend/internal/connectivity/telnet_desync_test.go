package connectivity

import (
	"bufio"
	"net"
	"strings"
	"testing"
	"time"
)

// commanderOnPipe gives a commander a live socket without a login handshake,
// so the read behaviour can be exercised on its own.
func commanderOnPipe(t *testing.T, hostname string) (*TelnetCommander, net.Conn) {
	t.Helper()
	ours, theirs := net.Pipe()
	t.Cleanup(func() {
		_ = ours.Close()
		_ = theirs.Close()
	})

	return &TelnetCommander{
		conn:     ours,
		reader:   bufio.NewReader(ours),
		hostname: hostname,
		timeout:  time.Second,
	}, theirs
}

// A command whose prompt never returns must fail. Reporting it as complete sent
// the next command into the tail of the previous one's output, and every reply
// after that was matched against the wrong command — it surfaced as "name ..."
// being refused at the exec prompt, five commands after the slow one.
func TestWaitForPromptFailsWhenTheOLTIsStillAnswering(t *testing.T) {
	commander, device := commanderOnPipe(t, "OLT")

	go func() {
		// Output starts arriving and stops mid-flight: no prompt.
		_, _ = device.Write([]byte("onu 15 type ALL sn HWTCB403E8A0\r\n."))
	}()

	output, err := commander.waitForPrompt(300 * time.Millisecond)

	if err == nil {
		t.Fatal("a read that never saw the prompt was reported as complete")
	}
	if !strings.Contains(err.Error(), "no prompt") {
		t.Errorf("error = %v, want it to name the missing prompt", err)
	}
	if !strings.Contains(output, "onu 15") {
		t.Errorf("output = %q, want what did arrive to be kept for the caller", output)
	}
}

// The prompt arriving is the whole point: that case must still succeed, and
// promptly rather than after the ceiling.
func TestWaitForPromptSucceedsOnThePrompt(t *testing.T) {
	commander, device := commanderOnPipe(t, "OLT")

	go func() {
		_, _ = device.Write([]byte("name ini test nms\r\nOLT(config-if)#"))
	}()

	started := time.Now()
	output, err := commander.waitForPrompt(5 * time.Second)

	if err != nil {
		t.Fatalf("waitForPrompt: %v", err)
	}
	if !strings.Contains(output, "name ini test nms") {
		t.Errorf("output = %q", output)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Errorf("waited %v; the ceiling is not meant to be spent", elapsed)
	}
}
