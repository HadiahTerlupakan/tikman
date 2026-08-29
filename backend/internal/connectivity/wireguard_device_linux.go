//go:build linux

package connectivity

import (
	"fmt"
	"net"

	"github.com/vishvananda/netlink"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// WireGuardDevice drives the kernel interface. It needs CAP_NET_ADMIN, which the
// api container is granted in docker-compose.yml.
type WireGuardDevice struct{}

// NewWireGuardDevice returns a device that drives the real wg0 interface.
func NewWireGuardDevice() *WireGuardDevice {
	return &WireGuardDevice{}
}

// Apply makes the kernel interface match cfg: link, address, WireGuard peers,
// and routes are each brought to the desired state in turn.
func (d *WireGuardDevice) Apply(cfg TunnelConfig) error {
	link, err := ensureLink(cfg.InterfaceName)
	if err != nil {
		return err
	}
	if err := ensureAddress(link, cfg.Address); err != nil {
		return err
	}
	if err := netlink.LinkSetUp(link); err != nil {
		return fmt.Errorf("bring %s up: %w", cfg.InterfaceName, err)
	}
	if err := applyWireGuardConfig(cfg); err != nil {
		return err
	}
	return syncRoutes(link, cfg)
}

func ensureLink(name string) (netlink.Link, error) {
	link, err := netlink.LinkByName(name)
	if err == nil {
		return link, nil
	}

	attrs := netlink.NewLinkAttrs()
	attrs.Name = name
	wg := &netlink.Wireguard{LinkAttrs: attrs}
	if err := netlink.LinkAdd(wg); err != nil {
		return nil, fmt.Errorf("create %s: %w", name, err)
	}
	return netlink.LinkByName(name)
}

func ensureAddress(link netlink.Link, address string) error {
	addr, err := netlink.ParseAddr(address)
	if err != nil {
		return fmt.Errorf("parse address %s: %w", address, err)
	}

	existing, err := netlink.AddrList(link, netlink.FAMILY_V4)
	if err != nil {
		return fmt.Errorf("list addresses: %w", err)
	}

	// The whole list is scanned rather than stopping at the first match: an
	// address left behind by a previous configuration would leave the
	// interface answering on two subnets.
	found := false
	for i := range existing {
		if existing[i].Equal(*addr) {
			found = true
			continue
		}
		if err := netlink.AddrDel(link, &existing[i]); err != nil {
			return fmt.Errorf("remove stale address %s: %w", existing[i].IPNet, err)
		}
	}

	if found {
		return nil
	}
	if err := netlink.AddrAdd(link, addr); err != nil {
		return fmt.Errorf("add address %s: %w", address, err)
	}
	return nil
}

func applyWireGuardConfig(cfg TunnelConfig) error {
	client, err := wgctrl.New()
	if err != nil {
		return fmt.Errorf("open wireguard control: %w", err)
	}
	defer client.Close()

	key, err := wgtypes.ParseKey(cfg.PrivateKey)
	if err != nil {
		return fmt.Errorf("parse server key: %w", err)
	}

	peers := make([]wgtypes.PeerConfig, 0, len(cfg.Peers))
	for _, peer := range cfg.Peers {
		peerKey, err := wgtypes.ParseKey(peer.PublicKey)
		if err != nil {
			return fmt.Errorf("parse peer key: %w", err)
		}
		allowed, err := parseAllowedIPs(peer.AllowedIPs)
		if err != nil {
			return err
		}
		keepalive := peer.Keepalive
		peers = append(peers, wgtypes.PeerConfig{
			PublicKey:                   peerKey,
			ReplaceAllowedIPs:           true,
			AllowedIPs:                  allowed,
			PersistentKeepaliveInterval: &keepalive,
		})
	}

	port := cfg.ListenPort
	return client.ConfigureDevice(cfg.InterfaceName, wgtypes.Config{
		PrivateKey:   &key,
		ListenPort:   &port,
		ReplacePeers: true,
		Peers:        peers,
	})
}

func parseAllowedIPs(entries []string) ([]net.IPNet, error) {
	allowed := make([]net.IPNet, 0, len(entries))
	for _, entry := range entries {
		_, network, err := net.ParseCIDR(entry)
		if err != nil {
			return nil, fmt.Errorf("parse allowed ip %s: %w", entry, err)
		}
		allowed = append(allowed, *network)
	}
	return allowed, nil
}

// syncRoutes makes the routing table match the peers. The kernel's allowed-ips
// do not create routes by themselves, and a route left behind by a deleted peer
// would blackhole that subnet.
func syncRoutes(link netlink.Link, cfg TunnelConfig) error {
	wanted := make(map[string]*net.IPNet)
	for _, peer := range cfg.Peers {
		for _, entry := range peer.AllowedIPs {
			_, network, err := net.ParseCIDR(entry)
			if err != nil {
				return fmt.Errorf("parse route %s: %w", entry, err)
			}
			wanted[network.String()] = network
		}
	}

	// The kernel installs a connected route for the interface's own address
	// when the link comes up. It belongs to no peer and must survive, or the
	// VPS loses its route to every peer's tunnel address.
	_, connected, err := net.ParseCIDR(cfg.Address)
	if err != nil {
		return fmt.Errorf("parse interface address %s: %w", cfg.Address, err)
	}

	existing, err := netlink.RouteList(link, netlink.FAMILY_V4)
	if err != nil {
		return fmt.Errorf("list routes: %w", err)
	}
	for i := range existing {
		route := existing[i]
		if route.Dst == nil || route.Dst.String() == connected.String() {
			continue
		}
		if _, keep := wanted[route.Dst.String()]; keep {
			delete(wanted, route.Dst.String())
			continue
		}
		if err := netlink.RouteDel(&route); err != nil {
			return fmt.Errorf("remove stale route %s: %w", route.Dst, err)
		}
	}

	for _, network := range wanted {
		route := &netlink.Route{LinkIndex: link.Attrs().Index, Dst: network}
		if err := netlink.RouteAdd(route); err != nil {
			return fmt.Errorf("add route %s: %w", network, err)
		}
	}
	return nil
}

// Status reads the live peer state back from the kernel via wgctrl.
func (d *WireGuardDevice) Status(interfaceName string) ([]TunnelPeerStatus, error) {
	client, err := wgctrl.New()
	if err != nil {
		return nil, fmt.Errorf("open wireguard control: %w", err)
	}
	defer client.Close()

	device, err := client.Device(interfaceName)
	if err != nil {
		return nil, fmt.Errorf("read device %s: %w", interfaceName, err)
	}

	statuses := make([]TunnelPeerStatus, 0, len(device.Peers))
	for _, peer := range device.Peers {
		status := TunnelPeerStatus{
			PublicKey: peer.PublicKey.String(),
			RxBytes:   peer.ReceiveBytes,
			TxBytes:   peer.TransmitBytes,
		}
		if !peer.LastHandshakeTime.IsZero() {
			handshake := peer.LastHandshakeTime.UTC()
			status.LastHandshakeAt = &handshake
		}
		statuses = append(statuses, status)
	}
	return statuses, nil
}
