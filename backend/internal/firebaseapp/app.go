// Package firebaseapp builds the one Firebase app the backend uses.
//
// Push, custom tokens and the presence mirror are three clients off a single
// service account; constructing an app per consumer would open three
// connections to the same project and give each its own failure mode.
package firebaseapp

import (
	"context"
	"encoding/base64"
	"fmt"

	firebase "firebase.google.com/go/v4"
	"google.golang.org/api/option"
)

// New builds the Firebase app from a base64-encoded service account JSON key.
//
// An empty key returns (nil, nil), never an error: a fresh checkout, or a
// deploy made before the Firebase project exists, must still start. Callers
// read a nil app as "Firebase is not configured".
func New(ctx context.Context, serviceAccountJSONB64 string) (*firebase.App, error) {
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
	return app, nil
}
