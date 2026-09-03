package wa

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/tikman/olt-provisioning/internal/models"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types/events"
	"go.uber.org/zap"
)

// waClientDisplayName is what WhatsApp shows the phone during phone-number
// pairing. The server validates this against its own allowlist of
// "Browser (OS)" strings, so it cannot be an arbitrary label like "TikMan".
const waClientDisplayName = "Chrome (Linux)"

// qrWaitTimeout bounds how long Pair waits for whatsmeow's login handshake to
// produce its first QR event before calling PairPhone anyway. whatsmeow's own
// docs call a one-second sleep "probably" enough; this waits for the actual
// signal instead of guessing, but still gives up rather than hang forever if
// the event never arrives.
const qrWaitTimeout = 5 * time.Second

// Pair answers an admin's "connect" request. A session with a device already
// linked and currently connected just gets its status re-announced — phone
// pairing is for establishing a link that does not exist yet, and asking
// WhatsApp for another code on top of one already in place would spend its
// rate limit on nothing. A session that is linked but not actually connected
// (the drop this pairing flow's own 160-second login window can cause, or any
// other) is not reported as connected — that would show an admin a healthy
// status while nothing reconnects — the supervisor is nudged instead.
//
// An unlinked session (re)opens the connection — the unauthenticated socket
// whatsmeow used at startup lives only around 160 seconds, so it may already
// be gone by the time an admin acts — waits for the login handshake to settle,
// then asks for an eight-character linking code and announces it, so the
// admin who clicked Connect sees it on the same screen instead of a container
// log. Any failure along the way is reported back as disconnected: a stuck
// "pairing" status with nothing to explain it is worse than an honest failure.
func (c *Client) Pair(ctx context.Context, phone string) error {
	if !c.NeedsPairing() {
		if !c.wa.IsConnected() {
			// The row still says "pairing" (the API wrote that before this
			// call even reached the wa process) or whatever it said before a
			// drop. Disconnected is true at this instant, and it clears
			// itself on the next events.Connected — an admin must not be
			// left watching a spinner that nothing is working toward.
			c.setStatus(ctx, models.WAAccountDisconnected)
			c.signalDropped()
			return nil
		}
		c.setStatus(ctx, models.WAAccountConnected)
		return nil
	}

	justConnected := true
	if err := c.wa.Connect(); err != nil {
		if !errors.Is(err, whatsmeow.ErrAlreadyConnected) {
			c.setStatus(ctx, models.WAAccountDisconnected)
			return fmt.Errorf("open a session to pair: %w", err)
		}
		justConnected = false
	}
	if justConnected {
		// PairPhone's own documentation warns that calling it right after
		// Connect races the handshake: Connect returns once the noise
		// handshake is done, before the login exchange that produces pairing
		// state has run. Nothing to wait for on a socket that was already
		// open (ErrAlreadyConnected) — that handshake finished long ago.
		c.waitForQR(ctx)
	}

	code, err := c.wa.PairPhone(ctx, phone, false, whatsmeow.PairClientChrome, waClientDisplayName)
	if err != nil {
		c.setStatus(ctx, models.WAAccountDisconnected)
		return fmt.Errorf("pair by phone: %w", err)
	}

	event := Event{
		Type:          EventAccountStatus,
		WAAccountID:   c.accountID.String(),
		AccountStatus: string(models.WAAccountPairing),
		PairingCode:   code,
	}
	if err := c.publisher.Publish(ctx, event); err != nil {
		c.logger.Warn("Could not announce the WhatsApp pairing code", zap.Error(err))
	}
	return nil
}

// waitForQR blocks until whatsmeow's login handshake produces its first QR
// event, or qrWaitTimeout passes. The handler is scoped to this one wait and
// removed afterward — route already owns the client's steady-state events,
// and this one only needs to see the very next QR.
func (c *Client) waitForQR(ctx context.Context) {
	seen := make(chan struct{}, 1)
	id := c.wa.AddEventHandler(func(evt any) {
		if _, ok := evt.(*events.QR); ok {
			select {
			case seen <- struct{}{}:
			default:
			}
		}
	})
	defer c.wa.RemoveEventHandler(id)

	select {
	case <-seen:
	case <-time.After(qrWaitTimeout):
	case <-ctx.Done():
	}
}

// Unpair gives up the pairing and tells the browsers the number is
// disconnected. The next connect needs a fresh phone-number pairing.
func (c *Client) Unpair(ctx context.Context) error {
	if err := c.Logout(ctx); err != nil {
		return fmt.Errorf("logout: %w", err)
	}
	c.setStatus(ctx, models.WAAccountDisconnected)

	// The stored number goes with the device. Logout deletes the device, so a
	// row still naming it would have this account point at a session that no
	// longer exists — and the inbox would go on showing a number nobody is
	// connected to.
	err := c.db.Model(&models.WAAccount{}).Where("id = ?", c.accountID).
		Update("jid", "").Error
	if err != nil {
		c.logger.Warn("Could not clear the number of a logged-out account",
			zap.String("account_id", c.accountID.String()), zap.Error(err))
	}
	return nil
}
