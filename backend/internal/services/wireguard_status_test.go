package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPeerConnected(t *testing.T) {
	now := time.Date(2026, 8, 29, 10, 0, 0, 0, time.UTC)

	recent := now.Add(-90 * time.Second)
	require.True(t, PeerConnected(&recent, now))

	stale := now.Add(-10 * time.Minute)
	require.False(t, PeerConnected(&stale, now))

	require.False(t, PeerConnected(nil, now), "a peer that never handshook is not connected")
}

func TestPeerConnectedToleratesOneMissedRehandshake(t *testing.T) {
	now := time.Date(2026, 8, 29, 10, 0, 0, 0, time.UTC)
	// WireGuard rehandshakes about every two minutes, so a peer seen 150s ago
	// is healthy and must not be reported as down.
	seen := now.Add(-150 * time.Second)
	require.True(t, PeerConnected(&seen, now))
}
