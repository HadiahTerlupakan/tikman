package connectivity

import (
	"fmt"
	"time"

	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/utils"
)

// CommanderFactory creates vendor-specific OLTCommander instances bound to a
// particular OLT's address and credentials. This avoids using a single global
// commander for OLTs that have different IPs, ports, or credentials.
type CommanderFactory struct {
	// Default timeout used when creating a commander. Can be overridden per call.
	defaultTimeout time.Duration
	encryptionKey  string
}

// NewCommanderFactory constructs a CommanderFactory with a default timeout.
func NewCommanderFactory(defaultTimeout time.Duration) *CommanderFactory {
	if defaultTimeout <= 0 {
		defaultTimeout = 5 * time.Second
	}
	return &CommanderFactory{defaultTimeout: defaultTimeout}
}

// NewCommanderFactoryWithEncryption constructs a factory that decrypts stored OLT credentials.
func NewCommanderFactoryWithEncryption(defaultTimeout time.Duration, encryptionKey string) *CommanderFactory {
	factory := NewCommanderFactory(defaultTimeout)
	factory.encryptionKey = encryptionKey
	return factory
}

// ForOLT returns a Telnet commander for compatibility with existing callers.
func (f *CommanderFactory) ForOLT(model models.OLTModel, host string, port int, username, password string) (OLTCommander, error) {
	return f.ForOLTWithProtocol(model, host, models.OLTProtocolTelnet, port, username, password)
}

// ForOLTWithProtocol creates a commander using the OLT's configured CLI protocol.
func (f *CommanderFactory) ForOLTWithProtocol(model models.OLTModel, host string, protocol models.OLTProtocol, port int, username, password string) (OLTCommander, error) {
	if host == "" {
		return nil, fmt.Errorf("host is required for OLT commander")
	}
	if port <= 0 {
		if protocol == models.OLTProtocolSSH {
			port = 22
		} else {
			port = 23
		}
	}

	if f.encryptionKey != "" {
		decrypted, err := utils.Decrypt(password, f.encryptionKey)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt OLT password: %w", err)
		}
		password = decrypted
	}

	timeout := f.defaultTimeout
	var commander OLTCommander
	var err error
	switch protocol {
	case models.OLTProtocolSSH:
		commander, err = NewSSHCommander(host, port, username, password, timeout)
	case models.OLTProtocolTelnet:
		commander, err = NewTelnetCommander(host, port, username, password, timeout)
	default:
		return nil, fmt.Errorf("unsupported OLT protocol: %s", protocol)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create commander for OLT %s: %w", host, err)
	}

	_ = model // Reserved for vendor-specific branching.
	return commander, nil
}
