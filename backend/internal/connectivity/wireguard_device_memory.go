package connectivity

// MemoryTunnelDevice records what would have been applied. It is the device the
// tests use, so service behaviour can be asserted without a kernel.
type MemoryTunnelDevice struct {
	Applied  TunnelConfig
	Statuses []TunnelPeerStatus
	ApplyErr error
}

// Apply records cfg as the applied state, or returns ApplyErr if the test set
// one to simulate the kernel refusing the configuration.
func (d *MemoryTunnelDevice) Apply(cfg TunnelConfig) error {
	if d.ApplyErr != nil {
		return d.ApplyErr
	}
	d.Applied = cfg
	return nil
}

// Status returns the statuses the test preloaded.
func (d *MemoryTunnelDevice) Status(string) ([]TunnelPeerStatus, error) {
	return d.Statuses, nil
}
