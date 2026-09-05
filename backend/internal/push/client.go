package push

import (
	"context"
	"errors"
	"fmt"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
)

// Client sends push notifications through Firebase Cloud Messaging. It
// satisfies services.PushSender, which is declared there rather than here so
// that the wa, worker and trapd binaries — which import internal/services but
// never send a push — do not link the Firebase SDK.
type Client struct {
	fcm *messaging.Client
}

// NewClient builds a Client from the shared Firebase app.
//
// A nil app means Firebase is not configured, and returns (nil, nil) rather
// than an error — the caller treats a nil *Client as "push is not configured"
// (see cmd/api/main.go).
func NewClient(ctx context.Context, app *firebase.App) (*Client, error) {
	if app == nil {
		return nil, nil
	}

	fcm, err := app.Messaging(ctx)
	if err != nil {
		return nil, fmt.Errorf("initialize firebase messaging client: %w", err)
	}

	return &Client{fcm: fcm}, nil
}

// SendEach implements services.PushSender.
//
// The payload is data-only, deliberately: a message carrying a Notification
// block is displayed by the Firebase SW SDK itself *and* handed to
// onBackgroundMessage, so the service worker's own showNotification produced a
// second, differently-behaving copy of every push. Data-only leaves the service
// worker as the single place a notification is built, which is also the only
// way it keeps its icon and its /cs click target.
func (c *Client) SendEach(ctx context.Context, fids []string, title, body string, data map[string]string) ([]string, error) {
	// The SDK rejects an empty batch outright ("messages must not be nil or
	// empty"), so without this a caller with nobody to notify would get an
	// error describing a problem it does not have. PushNotifierService also
	// returns early in that case; this makes the client correct on its own
	// rather than only correct through its one current caller.
	if len(fids) == 0 {
		return nil, nil
	}

	payload := make(map[string]string, len(data)+2)
	for k, v := range data {
		payload[k] = v
	}
	payload["title"] = title
	payload["body"] = body

	messages := make([]*messaging.Message, len(fids))
	for i, fid := range fids {
		messages[i] = &messaging.Message{
			Fid:  fid,
			Data: payload,
		}
	}

	resp, err := c.fcm.SendEach(ctx, messages)
	if err != nil {
		return nil, err
	}

	// SendEach answers per message, and only the whole call failing produces
	// err above. A message FCM rejects on its own — a stale project, a
	// malformed installation id, a key without permission — is reported only
	// here, so dropping these left the one place that knows why a push never
	// arrived throwing the answer away.
	var (
		invalid  []string
		failures []error
	)
	for i, r := range resp.Responses {
		switch {
		case r.Success:
		case messaging.IsUnregistered(r.Error):
			invalid = append(invalid, fids[i])
		default:
			failures = append(failures, fmt.Errorf("fid %s: %w", fids[i], r.Error))
		}
	}
	return invalid, errors.Join(failures...)
}
