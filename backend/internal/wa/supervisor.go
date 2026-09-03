// Keeping a session up: the reconnection loop and the two signals that drive
// it. Separate from client.go because the lifecycle of a connection is its own
// concern, and this process now runs one of these per CS number.
package wa

import (
	"context"
	"errors"
	"time"

	"go.mau.fi/whatsmeow"

	"go.uber.org/zap"
)

func (c *Client) signalDropped() {
	select {
	case c.dropped <- struct{}{}:
	default:
	}
}

func (c *Client) signalPaired() {
	select {
	case c.paired <- struct{}{}:
	default:
	}
}

// supervise puts the session back after every drop until ctx ends. A drop
// while the store is unpaired is not something reconnecting can fix — there
// is no device to reconnect — so supervise waits for a pairing to succeed
// instead of returning. Returning here was the bug: it left nothing running
// to recover the very next drop once the account did get paired.
func (c *Client) supervise(ctx context.Context) {
	delay := minReconnectDelay
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.dropped:
		}

		delay = c.reconnect(ctx, delay)
		if delay != 0 {
			continue
		}

		select {
		case <-ctx.Done():
			return
		case <-c.paired:
			delay = minReconnectDelay
		}
	}
}

// reconnect retries until the socket is back, answering the delay the next drop
// should start from, or zero when reconnecting can no longer help.
func (c *Client) reconnect(ctx context.Context, delay time.Duration) time.Duration {
	for !c.NeedsPairing() {
		select {
		case <-ctx.Done():
			return 0
		case <-time.After(delay):
		}

		err := c.wa.Connect()
		if err == nil || errors.Is(err, whatsmeow.ErrAlreadyConnected) {
			return minReconnectDelay
		}
		c.logger.Warn("Could not reopen the WhatsApp session",
			zap.Duration("waited", delay), zap.Error(err))
		delay = min(delay*2, maxReconnectDelay)
	}
	return 0 // logged out: only a new pairing can bring this session back
}
