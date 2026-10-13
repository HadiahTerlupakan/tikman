package middleware

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"strconv"
	"strings"
)

// routeTablePath is the kernel's IPv4 routing table. Reading it needs neither
// netlink nor any capability, and it exists inside every container the API
// runs in.
const routeTablePath = "/proc/net/route"

// routeTableFields is how many columns a /proc/net/route row has up to Mask,
// the last one read here.
const routeTableFields = 8

// dockerBridgeRange caps what may be trusted. The compose network sits inside
// it, while a default route leaving through some other network — the API run
// straight on a host, say — must not hand trust to that host's whole LAN.
var dockerBridgeRange = netip.MustParsePrefix("172.16.0.0/12")

type routeEntry struct {
	iface   string
	network netip.Prefix
	gateway netip.Addr
}

// TrustedProxies lists the addresses whose X-Forwarded-For gin may believe:
// the network the API's default route leaves through — the compose network,
// where nginx and cloudflared sit — minus that network's gateway.
//
// The gateway is left out because docker-proxy uses it as the source of every
// connection it relays to a published port, from the host itself or from an
// IPv6 client of an IPv4-only bridge. The rest of 172.16.0.0/12 is left out
// because WireGuard site subnets routed through wg0 can sit there, and their
// hosts reach the API with their own addresses.
func TrustedProxies() ([]string, error) {
	file, err := os.Open(routeTablePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return trustedProxiesFromRoutes(file)
}

func trustedProxiesFromRoutes(table io.Reader) ([]string, error) {
	routes, err := parseRouteTable(table)
	if err != nil {
		return nil, err
	}
	defaultRoute, ok := findDefaultRoute(routes)
	if !ok {
		return nil, errors.New("no IPv4 default route")
	}
	subnet, ok := findLinkSubnet(routes, defaultRoute)
	if !ok {
		return nil, fmt.Errorf("no subnet on %s holds the gateway %s", defaultRoute.iface, defaultRoute.gateway)
	}
	if subnet.Bits() < dockerBridgeRange.Bits() || !dockerBridgeRange.Contains(subnet.Addr()) {
		return nil, fmt.Errorf("%s lies outside %s", subnet, dockerBridgeRange)
	}
	return excludeAddress(subnet, defaultRoute.gateway), nil
}

func parseRouteTable(table io.Reader) ([]routeEntry, error) {
	scanner := bufio.NewScanner(table)
	scanner.Scan() // the header row
	var routes []routeEntry
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < routeTableFields {
			continue
		}
		route, err := parseRouteRow(fields)
		if err != nil {
			return nil, err
		}
		routes = append(routes, route)
	}
	return routes, scanner.Err()
}

func parseRouteRow(fields []string) (routeEntry, error) {
	destination, err := parseRouteAddr(fields[1])
	if err != nil {
		return routeEntry{}, err
	}
	gateway, err := parseRouteAddr(fields[2])
	if err != nil {
		return routeEntry{}, err
	}
	mask, err := parseRouteAddr(fields[7])
	if err != nil {
		return routeEntry{}, err
	}
	ones, bits := net.IPMask(mask.AsSlice()).Size()
	if bits == 0 {
		return routeEntry{}, fmt.Errorf("route mask %s is not contiguous", fields[7])
	}
	return routeEntry{
		iface:   fields[0],
		network: netip.PrefixFrom(destination, ones).Masked(),
		gateway: gateway,
	}, nil
}

// parseRouteAddr decodes an address column: the kernel prints the
// network-order address as a host-order integer.
func parseRouteAddr(field string) (netip.Addr, error) {
	value, err := strconv.ParseUint(field, 16, 32)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("route address %q: %w", field, err)
	}
	var octets [4]byte
	binary.NativeEndian.PutUint32(octets[:], uint32(value))
	return netip.AddrFrom4(octets), nil
}

func findDefaultRoute(routes []routeEntry) (routeEntry, bool) {
	for _, route := range routes {
		if route.network.Bits() == 0 && !route.gateway.IsUnspecified() {
			return route, true
		}
	}
	return routeEntry{}, false
}

func findLinkSubnet(routes []routeEntry, defaultRoute routeEntry) (netip.Prefix, bool) {
	for _, route := range routes {
		if route.iface == defaultRoute.iface && route.gateway.IsUnspecified() &&
			route.network.Bits() > 0 && route.network.Contains(defaultRoute.gateway) {
			return route.network, true
		}
	}
	return netip.Prefix{}, false
}

// excludeAddress covers every address in subnet but one with CIDR blocks: at
// each prefix length below the subnet's, the half that does not hold addr.
func excludeAddress(subnet netip.Prefix, addr netip.Addr) []string {
	octets := addr.As4()
	value := binary.BigEndian.Uint32(octets[:])
	blocks := make([]string, 0, 32-subnet.Bits())
	for bits := subnet.Bits() + 1; bits <= 32; bits++ {
		binary.BigEndian.PutUint32(octets[:], value^(1<<(32-bits)))
		blocks = append(blocks, netip.PrefixFrom(netip.AddrFrom4(octets), bits).Masked().String())
	}
	return blocks
}
