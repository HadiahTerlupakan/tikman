package services

import (
	"context"

	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/switchdriver"
)

// SwitchProber opens a switch that is not stored yet, to check what the form
// holds before it is saved.
type SwitchProber struct {
	open switchdriver.Opener
}

// NewSwitchProber creates a prober that talks to real switches.
func NewSwitchProber() *SwitchProber {
	return &SwitchProber{open: switchdriver.Open}
}

// Probe logs in read-only and asks the switch what it is. Nothing is stored.
// An SSH switch with no pinned key fails with shell.ErrHostKeyUnpinned, which
// carries the fingerprint for the form to show.
func (p *SwitchProber) Probe(ctx context.Context, in SwitchInput) (switchdriver.Identity, error) {
	if err := validateSwitchInput(in); err != nil {
		return switchdriver.Identity{}, err
	}
	sw := &models.Switch{
		Host: in.Host, Vendor: in.Vendor, Protocol: in.Protocol, Username: in.Username,
		Port: portOr(in.Port, defaultSwitchPort(in.Protocol)), SNMPPort: portOr(in.SNMPPort, DefaultSwitchSNMPPort),
	}
	reader, err := p.open(ctx, sw, in.SNMPCommunity, in.Password)
	if err != nil {
		return switchdriver.Identity{}, err
	}
	defer func() { _ = reader.Close() }()
	return reader.Identify(ctx)
}
