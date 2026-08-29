package services

import (
	"fmt"
	"strings"
)

// MikroTikInterfaceName is the interface the generated MikroTik commands create.
// It is fixed so a regenerated config overwrites the same interface instead of
// leaving a second one behind.
const MikroTikInterfaceName = "wg-tikman"

// mikroTikListenPort is RouterOS's own default. The site never receives an
// inbound handshake, so the value only has to be free.
const mikroTikListenPort = 13231

// PeerConfigInput carries everything the site side needs. The private key is
// the peer's own, decrypted only while rendering.
type PeerConfigInput struct {
	PeerPrivateKey string
	// PeerAddress is a bare IP with no prefix, for example "10.88.0.5". The
	// prefix comes from TunnelSubnet, so a CIDR value here would render an
	// address carrying two prefixes and a config the site cannot load.
	PeerAddress     string
	ServerPublicKey string
	EndpointHost    string
	// TunnelSubnet is in CIDR form, for example "10.88.0.0/24". It supplies the
	// prefix for PeerAddress and is the only route the site sends to the VPS.
	TunnelSubnet string
	ListenPort   int
	Keepalive    int
	// AllowedIPs are the site's own local subnets in CIDR form. Each one gets
	// its own masquerade rule so the OLT needs no route back to the tunnel.
	AllowedIPs []string
}

// RenderWGQuickConfig renders the wg-quick configuration format for Linux sites.
func RenderWGQuickConfig(in PeerConfigInput) string {
	var b strings.Builder

	fmt.Fprintf(&b, "[Interface]\nPrivateKey = %s\nAddress = %s\n", in.PeerPrivateKey, addressWithSubnetPrefix(in.PeerAddress, in.TunnelSubnet))

	// Forwarding has to be enabled and the packet has to survive the FORWARD
	// chain before it ever reaches POSTROUTING. Most distributions ship
	// net.ipv4.ip_forward=0, and a box with Docker installed also has a FORWARD
	// policy of DROP, so without these lines the masquerade rules below never
	// run: the tunnel looks alive while the OLT stays unreachable.
	//
	// The reply needs its own rule. conntrack un-SNATs it before the routing
	// decision, so the FORWARD chain sees in=LAN out=%i and the -i rule above
	// does not match it. Matching on ESTABLISHED,RELATED rather than accepting
	// everything leaving %i keeps the site's LAN from initiating into the tunnel.
	b.WriteString("PostUp = sysctl -w net.ipv4.ip_forward=1\n")
	b.WriteString("PostUp = iptables -A FORWARD -i %i -j ACCEPT\n")
	b.WriteString("PostUp = iptables -A FORWARD -o %i -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT\n")
	for _, subnet := range in.AllowedIPs {
		fmt.Fprintf(&b, "PostUp = iptables -t nat -A POSTROUTING -s %s -d %s -j MASQUERADE\n", in.TunnelSubnet, subnet)
	}
	for _, subnet := range in.AllowedIPs {
		fmt.Fprintf(&b, "PostDown = iptables -t nat -D POSTROUTING -s %s -d %s -j MASQUERADE\n", in.TunnelSubnet, subnet)
	}
	// ip_forward is deliberately not turned back off: the box may be routing for
	// something else that was relying on it long before this tunnel existed.
	b.WriteString("PostDown = iptables -D FORWARD -i %i -j ACCEPT\n")
	b.WriteString("PostDown = iptables -D FORWARD -o %i -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT\n")
	fmt.Fprintf(&b, "\n[Peer]\nPublicKey = %s\nEndpoint = %s:%d\nAllowedIPs = %s\nPersistentKeepalive = %d\n",
		in.ServerPublicKey, in.EndpointHost, in.ListenPort, in.TunnelSubnet, in.Keepalive)

	return b.String()
}

// RenderMikroTikConfig renders the MikroTik RouterOS command format for site configurations.
func RenderMikroTikConfig(in PeerConfigInput) string {
	var b strings.Builder

	fmt.Fprintf(&b, "/interface/wireguard/add name=%s private-key=%q listen-port=%d\n",
		MikroTikInterfaceName, in.PeerPrivateKey, mikroTikListenPort)
	fmt.Fprintf(&b, "/ip/address/add address=%s interface=%s\n",
		addressWithSubnetPrefix(in.PeerAddress, in.TunnelSubnet), MikroTikInterfaceName)
	fmt.Fprintf(&b, "/interface/wireguard/peers/add interface=%s public-key=%q endpoint-address=%s endpoint-port=%d allowed-address=%s persistent-keepalive=%ds\n",
		MikroTikInterfaceName, in.ServerPublicKey, in.EndpointHost, in.ListenPort, in.TunnelSubnet, in.Keepalive)

	// Source NAT written without an interface name: the operator would have to
	// look up the LAN interface otherwise, and without it the OLT needs a route
	// back to the tunnel subnet that it almost never has.
	for _, subnet := range in.AllowedIPs {
		fmt.Fprintf(&b, "/ip/firewall/nat/add chain=srcnat src-address=%s dst-address=%s action=masquerade comment=\"TikMan VPN\"\n",
			in.TunnelSubnet, subnet)
	}

	return b.String()
}

// addressWithSubnetPrefix extracts the prefix from subnet and appends it to address.
// It is used to convert a bare IP (e.g. 10.88.0.5) and tunnel subnet (e.g. 10.88.0.0/24)
// into a CIDR address (e.g. 10.88.0.5/24).
func addressWithSubnetPrefix(address, subnet string) string {
	parts := strings.SplitN(subnet, "/", 2)
	if len(parts) != 2 {
		return address
	}
	return fmt.Sprintf("%s/%s", address, parts[1])
}
