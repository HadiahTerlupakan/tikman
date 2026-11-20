// Package switchdriver reads managed switches through one interface, with a
// driver per vendor chosen from a registry. Phase 1 drivers only read.
package switchdriver

import (
	"context"
	"errors"
	"fmt"

	"github.com/tikman/olt-provisioning/internal/models"
)

// Port kinds, as the faceplate draws them.
const (
	KindRJ      = "rj"
	KindSFP     = "sfp"
	KindQSFP    = "qsfp"
	KindVirtual = "virtual"
)

// Port VLAN modes. A port that carries no switched VLAN has an empty mode.
const (
	ModeAccess = "access"
	ModeTrunk  = "trunk"
	ModeHybrid = "hybrid"
)

// Neighbour kinds.
const (
	NeighborLLDP = "lldp"
	NeighborMAC  = "mac"
)

// ErrVendorNotSupported means no driver is registered for a switch's vendor.
var ErrVendorNotSupported = errors.New("switchdriver: vendor not supported")

// Identity is what the switch says it is.
type Identity struct {
	Model   string
	Version string
}

// Port is one interface with its VLAN and STP settings. STPDisabled is nil
// for a port that does not switch; STPEdge is nil when nothing says.
type Port struct {
	Name         string
	Alias        string
	Kind         string
	Mode         string
	AdminUp      bool
	PVID         int
	AllowedVLANs string
	STPDisabled  *bool
	STPEdge      *bool
}

// VLAN is a VLAN defined on the switch.
type VLAN struct {
	VID  int
	Name string
}

// Neighbor is an LLDP neighbour or a learned MAC address behind a port.
type Neighbor struct {
	Port           string
	Kind           string
	MAC            string
	VID            int
	RemoteName     string
	RemotePort     string
	RemotePlatform string
}

// Reader reads one switch over an open session. Nothing it does changes the
// switch.
type Reader interface {
	Identify(ctx context.Context) (Identity, error)
	ReadPorts(ctx context.Context) ([]Port, error)
	ReadVLANs(ctx context.Context) ([]VLAN, error)
	ReadNeighbors(ctx context.Context) ([]Neighbor, error)
	// Backup returns the raw config text; the caller masks it.
	Backup(ctx context.Context) (string, error)
	Close() error
}

// Opener connects to a switch of one vendor.
type Opener func(ctx context.Context, sw *models.Switch, community, password string) (Reader, error)

// openers is filled by each driver file's init, before any goroutine reads
// it, so no lock is needed.
var openers = make(map[models.SwitchVendor]Opener)

// Register adds a vendor's driver. A second registration for the same vendor
// is a programming error and panics rather than silently shadowing.
func Register(vendor models.SwitchVendor, open Opener) {
	if _, exists := openers[vendor]; exists {
		panic(fmt.Sprintf("switchdriver: vendor %q registered twice", vendor))
	}
	openers[vendor] = open
}

// Open connects to sw with the driver for its vendor. community and password
// are the decrypted secrets.
func Open(ctx context.Context, sw *models.Switch, community, password string) (Reader, error) {
	open, ok := openers[sw.Vendor]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrVendorNotSupported, sw.Vendor)
	}
	return open(ctx, sw, community, password)
}
