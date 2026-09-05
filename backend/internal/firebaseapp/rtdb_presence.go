package firebaseapp

import (
	"context"
	"fmt"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/db"
	"github.com/google/uuid"
)

// presencePath is the node the browser writes its own presence to. It must
// match the security rules and the browser's path exactly.
const presencePath = "cs-presence"

// RTDBPresence reads the presence set out of the Realtime Database.
type RTDBPresence struct {
	ref *db.Ref
}

// NewRTDBPresence constructs an RTDBPresence. A nil app or an empty URL
// returns (nil, nil): presence is simply not mirrored.
func NewRTDBPresence(ctx context.Context, app *firebase.App, databaseURL string) (*RTDBPresence, error) {
	if app == nil || databaseURL == "" {
		return nil, nil
	}
	client, err := app.DatabaseWithURL(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("initialize firebase database client: %w", err)
	}
	return &RTDBPresence{ref: client.NewRef(presencePath)}, nil
}

// Present lists the agents holding the inbox open.
//
// GetShallow returns the keys without their values, which is all this needs
// and keeps the response one entry per agent however much the nodes grow.
func (p *RTDBPresence) Present(ctx context.Context) ([]uuid.UUID, error) {
	var shallow map[string]bool
	if err := p.ref.GetShallow(ctx, &shallow); err != nil {
		return nil, fmt.Errorf("read %s: %w", presencePath, err)
	}

	ids := make([]uuid.UUID, 0, len(shallow))
	for key := range shallow {
		id, err := uuid.Parse(key)
		if err != nil {
			// A node whose key is not a user id was not written by this app.
			// Skipping it is right; failing the whole pass over it would let
			// one stray key stop the rotation.
			continue
		}
		ids = append(ids, id)
	}
	return ids, nil
}
