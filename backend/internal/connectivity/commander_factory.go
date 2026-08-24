package connectivity

import (
	"fmt"
	"time"

	"github.com/tikman/olt-provisioning/internal/models"
)

// CommanderFactory creates vendor-specific OLTCommander instances bound to a
// particular OLT's address and credentials. This avoids using a single global
// commander for OLTs that have different IPs, ports, or credentials.
type CommanderFactory struct {
	// Default timeout used when creating a commander. Can be overridden per call.
	defaultTimeout time.Duration
}

// NewCommanderFactory constructs a CommanderFactory with a default timeout.
func NewCommanderFactory(defaultTimeout time.Duration) *CommanderFactory {
	if defaultTimeout <= 0 {
		defaultTimeout = 5 * time.Second
	}
	return &CommanderFactory{defaultTimeout: defaultTimeout}
}

// ForOLT returns a commander suitable for the OLT's vendor and credentials.
// Currently supports Telnet-based ZTE/HSGQ; SNMP SET support can be added here.
func (f *CommanderFactory) ForOLT(model models.OLTModel, host string, port int, username, password string) (OLTCommander, error) {
	if host == "" {
		return nil, fmt.Errorf("host is required for OLT commander")
	}
	if port <= 0 {
		port = 23
	}

	timeout := f.defaultTimeout

	// Choose the right commander implementation based on the vendor model.
	// For now, both ZTE and HSGQ use Telnet; future models may use SSH or SNMP.
	commander, err := NewTelnetCommander(host, port, username, password, timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to create commander for OLT %s: %w", host, err)
	}

	_ = model // Reserved for vendor-specific branching (ZTE SNMP, HSGQ Telnet, etc.)
	return commander, nil
}
