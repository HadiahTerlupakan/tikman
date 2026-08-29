package connectivity

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// silentAgent accepts UDP packets and never answers, which is how a device
// that has stopped replying looks from here.
func silentAgent(t *testing.T) int {
	t.Helper()
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return conn.LocalAddr().(*net.UDPAddr).Port
}

// The client is built on context.Background(), which never cancels, and one
// Get sat in it for over ten minutes. The worker runs its cycles one after
// another, so that call froze every ONT's status and metrics until the process
// was killed.
func TestBatchedReadsGiveUpOnASilentDevice(t *testing.T) {
	original := zteReadDeadline
	zteReadDeadline = 300 * time.Millisecond
	t.Cleanup(func() { zteReadDeadline = original })

	port := silentAgent(t)
	locations := []ONTLocation{{Slot: 3, Port: 1, ONTID: 1}, {Slot: 3, Port: 1, ONTID: 2}}

	for _, read := range []struct {
		name string
		call func() error
	}{
		{"inventory", func() error {
			_, err := queryZTEInventoryFor("127.0.0.1", "public", port, locations)
			return err
		}},
		{"metrics", func() error {
			_, err := zteDriver{}.QueryMetricsFor("127.0.0.1", "public", port, locations)
			return err
		}},
		{"status", func() error {
			_, err := zteDriver{}.QueryStatusFor("127.0.0.1", "public", port, locations)
			return err
		}},
	} {
		t.Run(read.name, func(t *testing.T) {
			done := make(chan struct{})
			go func() {
				defer close(done)
				_ = read.call()
			}()

			select {
			case <-done:
			case <-time.After(10 * time.Second):
				assert.Fail(t, "the read never gave up; it would freeze the worker's cycle")
			}
		})
	}
}
