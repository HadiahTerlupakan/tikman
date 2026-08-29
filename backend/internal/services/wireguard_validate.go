package services

import (
	"errors"
	"fmt"
	"net"

	"github.com/google/uuid"
)

const (
	minKeepaliveSeconds = 10
	maxKeepaliveSeconds = 120
)

// ErrValidation marks a failure caused by the caller's input rather than by
// something going wrong inside the system, so the HTTP layer can answer 400
// instead of reporting a server fault.
var ErrValidation = errors.New("invalid configuration")

// DefaultReservedSubnets covers the Docker bridge range. Routing it into a
// tunnel would cut the API off from postgres and redis.
var DefaultReservedSubnets = []string{"172.16.0.0/12"}

// PeerNetwork is the subset of an existing peer that validation needs. The site
// name travels with it so a conflict can be reported by name.
type PeerNetwork struct {
	PeerID     uuid.UUID
	SiteName   string
	AllowedIPs []string
}

// ValidateAllowedIPs rejects site subnets that cannot be routed safely: a
// default route, an overlap with the tunnel subnet or a reserved subnet, or an
// overlap with a subnet another site already claims.
func ValidateAllowedIPs(candidate []string, others []PeerNetwork, tunnelSubnet string, reserved []string) error {
	if len(candidate) == 0 {
		return fmt.Errorf("%w: at least one local subnet is required", ErrValidation)
	}

	parsed := make([]*net.IPNet, 0, len(candidate))
	for _, entry := range candidate {
		network, err := parseCIDR(entry)
		if err != nil {
			return err
		}
		if ones, bits := network.Mask.Size(); ones == 0 && bits > 0 {
			return fmt.Errorf("%w: %s is a default route: it would send all VPS traffic into one site", ErrValidation, entry)
		}
		parsed = append(parsed, network)
	}

	if err := rejectOverlapWith(parsed, []string{tunnelSubnet}, "tunnel subnet"); err != nil {
		return err
	}
	if err := rejectOverlapWith(parsed, reserved, "reserved subnet"); err != nil {
		return err
	}
	return rejectOverlapWithPeers(parsed, others)
}

func rejectOverlapWith(candidate []*net.IPNet, subnets []string, label string) error {
	for _, subnet := range subnets {
		network, err := parseCIDR(subnet)
		if err != nil {
			return err
		}
		for _, entry := range candidate {
			if networksOverlap(entry, network) {
				return fmt.Errorf("%w: %s overlaps the %s %s", ErrValidation, entry.String(), label, subnet)
			}
		}
	}
	return nil
}

func rejectOverlapWithPeers(candidate []*net.IPNet, others []PeerNetwork) error {
	for _, other := range others {
		if err := rejectOverlapWithPeer(candidate, other); err != nil {
			return err
		}
	}
	return nil
}

func rejectOverlapWithPeer(candidate []*net.IPNet, other PeerNetwork) error {
	for _, entry := range other.AllowedIPs {
		network, err := parseCIDR(entry)
		if err != nil {
			continue // a stored value that no longer parses cannot be routed anyway
		}
		for _, own := range candidate {
			if networksOverlap(own, network) {
				return fmt.Errorf("%w: %s overlaps %s, already used by site %s", ErrValidation, own.String(), entry, other.SiteName)
			}
		}
	}
	return nil
}

// ValidateTunnelAddress rejects an address outside the tunnel subnet, the
// server's own address, and one already assigned to another site.
func ValidateTunnelAddress(address, tunnelSubnet, serverAddress string, taken []string) error {
	ip := net.ParseIP(address)
	if ip == nil {
		return fmt.Errorf("%w: %q is not a valid IP address", ErrValidation, address)
	}

	network, err := parseCIDR(tunnelSubnet)
	if err != nil {
		return err
	}
	if !network.Contains(ip) {
		return fmt.Errorf("%w: %s is outside the tunnel subnet %s", ErrValidation, address, tunnelSubnet)
	}
	if address == serverAddress {
		return fmt.Errorf("%w: %s is the server address", ErrValidation, address)
	}
	for _, used := range taken {
		if used == address {
			return fmt.Errorf("%w: %s is already assigned to another site", ErrValidation, address)
		}
	}
	return nil
}

// ValidateKeepalive bounds the interval that keeps the site's NAT mapping open.
// Outside this range the tunnel either wastes traffic or goes stale before the
// server notices.
func ValidateKeepalive(seconds int) error {
	if seconds < minKeepaliveSeconds || seconds > maxKeepaliveSeconds {
		return fmt.Errorf("%w: keepalive must be between %d and %d seconds", ErrValidation, minKeepaliveSeconds, maxKeepaliveSeconds)
	}
	return nil
}

func parseCIDR(value string) (*net.IPNet, error) {
	_, network, err := net.ParseCIDR(value)
	if err != nil {
		return nil, fmt.Errorf("%w: %q is not a valid subnet in CIDR form, for example 10.10.10.0/24", ErrValidation, value)
	}
	return network, nil
}

func networksOverlap(a, b *net.IPNet) bool {
	return a.Contains(b.IP) || b.Contains(a.IP)
}
