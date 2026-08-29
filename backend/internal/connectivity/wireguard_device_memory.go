package connectivity

// MemoryTunnelDevice records what would have been applied. It is the device the
// tests use, so service behaviour can be asserted without a kernel.
type MemoryTunnelDevice struct {
	Applied  TunnelConfig
	Statuses []TunnelPeerStatus
	ApplyErr error
	// ApplyErrOnce clears ApplyErr after the first refusal, modelling a kernel
	// that rejects one configuration and accepts the next. That is the case the
	// recovery reconcile after a rollback has to survive.
	ApplyErrOnce bool
}

// Apply records cfg as the applied state, or returns ApplyErr if the test set
// one to simulate the kernel refusing the configuration.
func (d *MemoryTunnelDevice) Apply(cfg TunnelConfig) error {
	if d.ApplyErr != nil {
		err := d.ApplyErr
		if d.ApplyErrOnce {
			d.ApplyErr = nil
		}
		return err
	}
	d.Applied = cfg
	return nil
}

// Status returns the statuses the test preloaded.
func (d *MemoryTunnelDevice) Status(string) ([]TunnelPeerStatus, error) {
	return d.Statuses, nil
}
