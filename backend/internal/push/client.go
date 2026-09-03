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
// satisfies services.PushSender, which is declared there rather than here on
// purpose — see the note in this task, and keep this package free of any
// interface so internal/services never has to import it.
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

// SendEach implements Sender.
func (c *Client) SendEach(ctx context.Context, tokens []string, title, body string, data map[string]string) ([]string, error) {
	messages := make([]*messaging.Message, len(tokens))
	for i, token := range tokens {
		messages[i] = &messaging.Message{
			Token: token,
			Notification: &messaging.Notification{
				Title: title,
				Body:  body,
			},
			Data: data,
		}
	}

	resp, err := c.fcm.SendEach(ctx, messages)
	if err != nil {
		return nil, err
	}

	var invalid []string
	for i, r := range resp.Responses {
		if !r.Success && messaging.IsRegistrationTokenNotRegistered(r.Error) {
			invalid = append(invalid, tokens[i])
		}
	}
	return invalid, nil
}
