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
	expected := `[Interface]
PrivateKey = PEERPRIV
Address = 10.88.0.5/24
PostUp = iptables -t nat -A POSTROUTING -s 10.88.0.0/24 -d 10.10.10.0/24 -j MASQUERADE
PostDown = iptables -t nat -D POSTROUTING -s 10.88.0.0/24 -d 10.10.10.0/24 -j MASQUERADE

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
	require.Equal(t, 2, strings.Count(output, "PostUp ="))
	require.Equal(t, 2, strings.Count(output, "PostDown ="))
	require.Contains(t, output, "-d 192.168.88.0/24 -j MASQUERADE")
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
	require.Contains(t, output, "endpoint-address=vpn.contoh.id endpoint-port=51820")
	require.Contains(t, output, "allowed-address=10.88.0.0/24")
	require.Contains(t, output, "persistent-keepalive=25s")
	require.Contains(t, output, "chain=srcnat src-address=10.88.0.0/24 dst-address=10.10.10.0/24 action=masquerade")
}

func TestRenderMikroTikConfigNeedsNoInterfaceName(t *testing.T) {
	output := RenderMikroTikConfig(renderInput())
	require.NotContains(t, output, "out-interface=",
		"the NAT rule must not ask the operator to know the name of the LAN interface")
}
