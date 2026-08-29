package services

import (
	"fmt"
	"net"
	"time"

	"github.com/google/uuid"
)

// reachabilityTimeout is short on purpose: the operator is watching the button.
// A device behind a working tunnel answers in milliseconds, and one that is not
// there will not start answering if we wait longer.
const reachabilityTimeout = 3 * time.Second

// ReachabilityResult explains what happened rather than only whether it worked.
// "Unreachable" has two very different causes here — an address the tunnel was
// never told to carry, and a device that simply did not answer — and the
// operator fixes them in different places.
type ReachabilityResult struct {
	Reachable bool
	// Routed reports whether the address falls inside the subnets this site's
	// tunnel carries. When false, no packet was sent: it could not have arrived.
	Routed  bool
	Message string
}

// TestPeerReachability pings an address through a site's tunnel. It checks the
// routing claim first, because a mistyped subnet and an unplugged device look
// identical from a failed ping alone.
func (s *WireGuardService) TestPeerReachability(peerID uuid.UUID, address string) (*ReachabilityResult, error) {
	peer, err := s.GetPeer(peerID)
	if err != nil {
		return nil, err
	}

	ip := net.ParseIP(address)
	if ip == nil {
		return nil, fmt.Errorf("%w: %q is not a valid IP address", ErrValidation, address)
	}

	allowedIPs, err := peer.AllowedIPsList()
	if err != nil {
		return nil, err
	}

	if !addressWithinSubnets(ip, allowedIPs) {
		return &ReachabilityResult{
			Routed: false,
			Message: fmt.Sprintf(
				"%s is outside this site's subnets (%s), so nothing was sent — the tunnel only carries those. Correct the subnet, or the address.",
				address, joinOrNone(allowedIPs),
			),
		}, nil
	}

	if !peer.Enabled {
		return &ReachabilityResult{
			Routed:  true,
			Message: "This site's tunnel is switched off, so the address cannot be reached until it is enabled.",
		}, nil
	}

	if err := s.pingHost(address, reachabilityTimeout); err != nil {
		return &ReachabilityResult{
			Routed: true,
			Message: fmt.Sprintf(
				"%s did not answer. The tunnel carries this subnet, so check that the device is on and that it can route back — its gateway is usually the router at the site.",
				address,
			),
		}, nil
	}

	return &ReachabilityResult{
		Reachable: true,
		Routed:    true,
		Message:   fmt.Sprintf("%s answered through the tunnel.", address),
	}, nil
}

func addressWithinSubnets(ip net.IP, subnets []string) bool {
	for _, entry := range subnets {
		network, err := parseCIDR(entry)
		if err != nil {
			continue
		}
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

func joinOrNone(subnets []string) string {
	if len(subnets) == 0 {
		return "none configured"
	}
	out := subnets[0]
	for _, entry := range subnets[1:] {
		out += ", " + entry
	}
	return out
}
