package push

import (
	"context"
	"encoding/base64"
	"fmt"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"
)

// Client sends push notifications through Firebase Cloud Messaging. It
// satisfies services.PushSender, which is declared there rather than here so
// that the wa, worker and trapd binaries — which import internal/services but
// never send a push — do not link the Firebase SDK.
type Client struct {
	fcm *messaging.Client
}

// NewClient builds a Client from a base64-encoded service account JSON key.
// An empty serviceAccountJSONB64 returns (nil, nil), never an error — a
// fresh checkout or a deploy before the user's Firebase project exists must
// still start normally. The caller treats a nil *Client as "push is not
// configured" (see cmd/api/main.go).
func NewClient(ctx context.Context, serviceAccountJSONB64 string) (*Client, error) {
	if serviceAccountJSONB64 == "" {
		return nil, nil
	}

	raw, err := base64.StdEncoding.DecodeString(serviceAccountJSONB64)
	if err != nil {
		return nil, fmt.Errorf("decode FIREBASE_SERVICE_ACCOUNT_JSON_B64: %w", err)
	}

	app, err := firebase.NewApp(ctx, nil, option.WithAuthCredentialsJSON(option.ServiceAccount, raw))
	if err != nil {
		return nil, fmt.Errorf("initialize firebase app: %w", err)
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
func (c *Client) SendEach(ctx context.Context, tokens []string, title, body string, data map[string]string) ([]string, error) {
	payload := make(map[string]string, len(data)+2)
	for k, v := range data {
		payload[k] = v
	}
	payload["title"] = title
	payload["body"] = body

	messages := make([]*messaging.Message, len(tokens))
	for i, token := range tokens {
		messages[i] = &messaging.Message{
			Token: token,
			Data:  payload,
		}
	}

	resp, err := c.fcm.SendEach(ctx, messages)
	if err != nil {
		return nil, err
	}

	var invalid []string
	for i, r := range resp.Responses {
		if !r.Success && messaging.IsUnregistered(r.Error) {
			invalid = append(invalid, tokens[i])
		}
	}
	return invalid, nil
}
