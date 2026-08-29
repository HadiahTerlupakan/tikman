package services

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func peerForReachability(t *testing.T) (*WireGuardService, uuid.UUID) {
	t.Helper()

	service, _, db := newWireGuardService(t)
	_, err := service.EnsureServer("vpn.contoh.id")
	require.NoError(t, err)
	site := createTestSite(t, db, "Site A")

	peer, err := service.CreatePeer(site.ID, "Site A", []string{"10.10.10.0/24"}, "")
	require.NoError(t, err)
	return service, peer.ID
}

func TestReachabilitySendsNothingForAnAddressTheTunnelDoesNotCarry(t *testing.T) {
	service, peerID := peerForReachability(t)

	pinged := false
	service.pingHost = func(string, time.Duration) error {
		pinged = true
		return nil
	}

	result, err := service.TestPeerReachability(peerID, "192.168.1.5")
	require.NoError(t, err)

	require.False(t, result.Routed)
	require.False(t, result.Reachable)
	require.False(t, pinged, "an address outside the tunnel's subnets must not be probed at all")
	// The message has to name both halves, since either one can be the mistake.
	require.Contains(t, result.Message, "192.168.1.5")
	require.Contains(t, result.Message, "10.10.10.0/24")
}

func TestReachabilityReportsAnAnswerThroughTheTunnel(t *testing.T) {
	service, peerID := peerForReachability(t)
	service.pingHost = func(string, time.Duration) error { return nil }

	result, err := service.TestPeerReachability(peerID, "10.10.10.5")
	require.NoError(t, err)

	require.True(t, result.Routed)
	require.True(t, result.Reachable)
	require.Contains(t, result.Message, "10.10.10.5")
}

func TestReachabilitySeparatesSilenceFromMisrouting(t *testing.T) {
	service, peerID := peerForReachability(t)
	service.pingHost = func(string, time.Duration) error {
		return errors.New("timeout")
	}

	result, err := service.TestPeerReachability(peerID, "10.10.10.5")
	require.NoError(t, err)

	// Routed stays true: the operator should look at the device, not the subnet.
	require.True(t, result.Routed)
	require.False(t, result.Reachable)
	require.Contains(t, result.Message, "did not answer")
}

func TestReachabilitySaysWhenTheTunnelIsSwitchedOff(t *testing.T) {
	service, peerID := peerForReachability(t)
	service.pingHost = func(string, time.Duration) error { return nil }

	disabled := false
	_, err := service.UpdatePeer(peerID, nil, nil, &disabled)
	require.NoError(t, err)

	result, err := service.TestPeerReachability(peerID, "10.10.10.5")
	require.NoError(t, err)

	require.True(t, result.Routed)
	require.False(t, result.Reachable)
	require.Contains(t, result.Message, "switched off")
}

func TestReachabilityRejectsSomethingThatIsNotAnAddress(t *testing.T) {
	service, peerID := peerForReachability(t)

	_, err := service.TestPeerReachability(peerID, "bukan-ip")
	require.Error(t, err)
	require.ErrorIs(t, err, ErrValidation)
}

func TestReachabilityReportsAnUnknownPeer(t *testing.T) {
	service, _ := peerForReachability(t)

	_, err := service.TestPeerReachability(uuid.New(), "10.10.10.5")
	require.Error(t, err)
	require.ErrorIs(t, err, ErrPeerNotFound)
}
