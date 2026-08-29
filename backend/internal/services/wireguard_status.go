package services

import "time"

// PeerHandshakeGrace is how long after the last handshake a peer still counts as
// connected. WireGuard rehandshakes roughly every two minutes, so this leaves
// room for one missed exchange before a healthy site is called down.
const PeerHandshakeGrace = 3 * time.Minute

// PeerConnected determines whether a WireGuard peer is connected based on the
// time since its last handshake. A peer is considered connected if it has
// handshaken within the grace period, and disconnected if the grace period
// has been exceeded, or if it has never handshaken at all.
func PeerConnected(lastHandshake *time.Time, now time.Time) bool {
	if lastHandshake == nil || lastHandshake.IsZero() {
		return false
	}
	return now.Sub(*lastHandshake) < PeerHandshakeGrace
}
