// Command wgstat prints the WireGuard state the kernel actually holds.
//
// The tunnel is configured over netlink rather than from a file, so there is no
// config on disk to read and the `wg` tool is not installed. That left the one
// piece of state that decides whether a packet is accepted — each peer's
// allowed IPs — invisible while diagnosing why traps never arrived.
//
// The received-bytes counter is the other half: WireGuard counts a packet as
// received when it decrypts, and only then checks the source against the peer's
// allowed IPs. A peer whose counter climbs while nothing reaches the interface
// is a peer whose packets are being refused by that check, which looks exactly
// like silence from anywhere else in the system.
package main

import (
	"fmt"
	"log"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

func main() {
	client, err := wgctrl.New()
	if err != nil {
		log.Fatalf("open wireguard control: %v", err)
	}
	defer func() { _ = client.Close() }()

	devices, err := client.Devices()
	if err != nil {
		log.Fatalf("list devices: %v", err)
	}
	if len(devices) == 0 {
		log.Fatal("no WireGuard devices in this network namespace")
	}

	for _, device := range devices {
		fmt.Printf("interface %s (port %d, %d peers)\n\n", device.Name, device.ListenPort, len(device.Peers))
		for _, peer := range device.Peers {
			describePeer(peer)
		}
	}
}

func describePeer(peer wgtypes.Peer) {
	allowed := make([]string, 0, len(peer.AllowedIPs))
	for _, network := range peer.AllowedIPs {
		allowed = append(allowed, network.String())
	}

	endpoint := "none"
	if peer.Endpoint != nil {
		endpoint = peer.Endpoint.String()
	}

	handshake := "never"
	if !peer.LastHandshakeTime.IsZero() {
		handshake = fmt.Sprintf("%.0fs ago", time.Since(peer.LastHandshakeTime).Seconds())
	}

	fmt.Printf("peer %s…\n", peer.PublicKey.String()[:12])
	fmt.Printf("  endpoint    %s\n", endpoint)
	fmt.Printf("  allowed-ips %v\n", allowed)
	fmt.Printf("  handshake   %s\n", handshake)
	fmt.Printf("  rx          %d bytes\n", peer.ReceiveBytes)
	fmt.Printf("  tx          %d bytes\n\n", peer.TransmitBytes)
}
