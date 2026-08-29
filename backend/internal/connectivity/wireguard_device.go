package connectivity

import "time"

// TunnelPeerConfig is one peer as the kernel needs it.
type TunnelPeerConfig struct {
	PublicKey    string
	PresharedKey string
	AllowedIPs   []string
	Keepalive    time.Duration
}

// TunnelConfig is the complete desired state of the interface. There is no
// partial form on purpose: the service always applies everything.
type TunnelConfig struct {
	InterfaceName string
	PrivateKey    string
	Address       string
	ListenPort    int
	Peers         []TunnelPeerConfig
}

// TunnelPeerStatus is one peer's live state as read back from the kernel.
type TunnelPeerStatus struct {
	PublicKey       string
	LastHandshakeAt *time.Time
	RxBytes         int64
	TxBytes         int64
}

// TunnelDevice is the boundary between decisions and the kernel. Everything
// above it is tested; the implementation below it needs root and a Linux host.
type TunnelDevice interface {
	Apply(cfg TunnelConfig) error
	Status(interfaceName string) ([]TunnelPeerStatus, error)
}
