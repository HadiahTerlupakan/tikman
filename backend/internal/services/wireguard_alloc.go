package services

import (
	"fmt"
	"net"
	"sort"
)

// AllocateTunnelAddress returns the lowest address in the tunnel subnet that is
// neither the server's nor already assigned. Gaps are reused so a deleted site
// does not push the range upward forever.
func AllocateTunnelAddress(tunnelSubnet, serverAddress string, taken []string) (string, error) {
	network, err := parseCIDR(tunnelSubnet)
	if err != nil {
		return "", err
	}

	used := make(map[string]bool, len(taken)+1)
	used[serverAddress] = true
	for _, address := range taken {
		used[address] = true
	}

	for candidate := nextIP(network.IP); network.Contains(candidate); candidate = nextIP(candidate) {
		if isBroadcast(candidate, network) {
			break
		}
		if !used[candidate.String()] {
			return candidate.String(), nil
		}
	}
	return "", fmt.Errorf("%w: no free address left in tunnel subnet %s", ErrValidation, tunnelSubnet)
}

// SuggestAllowedIPs turns the OLT addresses already registered for a site into
// /24 subnets. It is a suggestion the operator confirms, not a discovery: the
// real prefix length is only known at the site.
func SuggestAllowedIPs(oltAddresses []string) []string {
	seen := make(map[string]bool)
	for _, address := range oltAddresses {
		ip := net.ParseIP(address).To4()
		if ip == nil {
			continue
		}
		masked := ip.Mask(net.CIDRMask(24, 32))
		seen[fmt.Sprintf("%s/24", masked.String())] = true
	}

	subnets := make([]string, 0, len(seen))
	for subnet := range seen {
		subnets = append(subnets, subnet)
	}
	sort.Strings(subnets)
	return subnets
}

func nextIP(ip net.IP) net.IP {
	next := make(net.IP, len(ip))
	copy(next, ip)
	for i := len(next) - 1; i >= 0; i-- {
		next[i]++
		if next[i] != 0 {
			break
		}
	}
	return next
}

func isBroadcast(ip net.IP, network *net.IPNet) bool {
	broadcast := make(net.IP, len(network.IP))
	for i := range network.IP {
		broadcast[i] = network.IP[i] | ^network.Mask[i]
	}
	return ip.Equal(broadcast)
}
