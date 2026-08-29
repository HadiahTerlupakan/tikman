package connectivity

import (
	"net"
	"strings"
)

// LocalSubnets reports the networks this process can already reach directly.
// Routing one of them into a tunnel would take the API away from its own
// database or from the host it lives on, so the service refuses a site subnet
// that overlaps one.
//
// It is read from the live interfaces rather than a constant because the
// networks Docker hands out differ per host, and a constant wide enough to
// cover them all — 172.16.0.0/12, say — also swallows the private range many
// ISPs use for their own management networks.
func LocalSubnets(excludeInterface string) []string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil
	}

	var subnets []string
	for _, iface := range interfaces {
		if iface.Name == excludeInterface || strings.HasPrefix(iface.Name, "lo") {
			continue
		}
		addresses, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, address := range addresses {
			network, ok := address.(*net.IPNet)
			if !ok || network.IP.To4() == nil {
				continue
			}
			subnets = append(subnets, (&net.IPNet{IP: network.IP.Mask(network.Mask), Mask: network.Mask}).String())
		}
	}
	return subnets
}
