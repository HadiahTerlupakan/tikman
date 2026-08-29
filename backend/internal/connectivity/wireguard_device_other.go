//go:build !linux

package connectivity

import "errors"

// errWireGuardUnsupported keeps development on macOS building. The kernel
// interface only exists on the Linux host the API is deployed to.
var errWireGuardUnsupported = errors.New("wireguard requires linux")

// WireGuardDevice is a non-Linux stand-in so the package builds on macOS. It
// answers every call with errWireGuardUnsupported; the real implementation is
// in wireguard_device_linux.go.
type WireGuardDevice struct{}

// NewWireGuardDevice returns the non-Linux stand-in device.
func NewWireGuardDevice() *WireGuardDevice {
	return &WireGuardDevice{}
}

// Apply always fails: WireGuard is not implemented on this platform.
func (d *WireGuardDevice) Apply(TunnelConfig) error {
	return errWireGuardUnsupported
}

// Status always fails: WireGuard is not implemented on this platform.
func (d *WireGuardDevice) Status(string) ([]TunnelPeerStatus, error) {
	return nil, errWireGuardUnsupported
}
