package services

import (
	"fmt"
	"net"

	"github.com/google/uuid"
)

const (
	minKeepaliveSeconds = 10
	maxKeepaliveSeconds = 120
)

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

func ValidateAllowedIPs(candidate []string, others []PeerNetwork, tunnelSubnet string, reserved []string) error {
	if len(candidate) == 0 {
		return fmt.Errorf("at least one local subnet is required")
	}

	parsed := make([]*net.IPNet, 0, len(candidate))
	for _, entry := range candidate {
		network, err := parseCIDR(entry)
		if err != nil {
			return err
		}
		if ones, bits := network.Mask.Size(); ones == 0 && bits > 0 {
			return fmt.Errorf("%s is a default route: it would send all VPS traffic into one site", entry)
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
				return fmt.Errorf("%s overlaps the %s %s", entry.String(), label, subnet)
			}
		}
	}
	return nil
}

func rejectOverlapWithPeers(candidate []*net.IPNet, others []PeerNetwork) error {
	for _, other := range others {
		for _, entry := range other.AllowedIPs {
			network, err := parseCIDR(entry)
			if err != nil {
				continue // a stored value that no longer parses cannot be routed anyway
			}
			for _, own := range candidate {
				if networksOverlap(own, network) {
					return fmt.Errorf("%s overlaps %s, already used by site %s", own.String(), entry, other.SiteName)
				}
			}
		}
	}
	return nil
}

func ValidateTunnelAddress(address, tunnelSubnet, serverAddress string, taken []string) error {
	ip := net.ParseIP(address)
	if ip == nil {
		return fmt.Errorf("%q is not a valid IP address", address)
	}

	network, err := parseCIDR(tunnelSubnet)
	if err != nil {
		return err
	}
	if !network.Contains(ip) {
		return fmt.Errorf("%s is outside the tunnel subnet %s", address, tunnelSubnet)
	}
	if address == serverAddress {
		return fmt.Errorf("%s is the server address", address)
	}
	for _, used := range taken {
		if used == address {
			return fmt.Errorf("%s is already assigned to another site", address)
		}
	}
	return nil
}

func ValidateKeepalive(seconds int) error {
	if seconds < minKeepaliveSeconds || seconds > maxKeepaliveSeconds {
		return fmt.Errorf("keepalive must be between %d and %d seconds", minKeepaliveSeconds, maxKeepaliveSeconds)
	}
	return nil
}

func parseCIDR(value string) (*net.IPNet, error) {
	_, network, err := net.ParseCIDR(value)
	if err != nil {
		return nil, fmt.Errorf("%q is not a valid subnet in CIDR form, for example 10.10.10.0/24", value)
	}
	return network, nil
}

func networksOverlap(a, b *net.IPNet) bool {
	return a.Contains(b.IP) || b.Contains(a.IP)
}
