package wa

import (
	"context"

	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
)

// deviceFor picks one account's device out of the shared session store, or a
// fresh one when that account has never been paired.
//
// Matched on the number rather than by JID lookup: the store keys devices by
// AD-JID — the number plus which linked device it is — and what an account row
// holds is the plain number. The number is the identity that matters anyway,
// since one CS number is one device here.
//
// A stored number with no device left (logged out, or the store rebuilt) falls
// through to a new device, which is exactly the state pairing needs. Before
// this, every session called GetFirstDevice and all of them would have opened
// the same one.
func deviceFor(ctx context.Context, container *sqlstore.Container, accountJID string) (*store.Device, error) {
	if accountJID == "" {
		return container.NewDevice(), nil
	}
	want, err := types.ParseJID(accountJID)
	if err != nil {
		return container.NewDevice(), nil
	}

	devices, err := container.GetAllDevices(ctx)
	if err != nil {
		return nil, err
	}
	for _, device := range devices {
		if device.ID != nil && device.ID.User == want.User {
			return device, nil
		}
	}
	return container.NewDevice(), nil
}
