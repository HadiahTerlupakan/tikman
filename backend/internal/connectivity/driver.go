package connectivity

import (
	"context"
	"errors"
	"fmt"

	"github.com/tikman/olt-provisioning/internal/models"
)

// ErrUnsupported reports that an OLT model exposes no known OID for a read
// path. Callers must treat it as "no data available", never as a zero reading:
// a fabricated 0 dBm or an invented traffic rate is worse than a gap.
var ErrUnsupported = errors.New("read path not supported by this OLT model")

// Canonical ONT phase states. Every driver maps its vendor's raw status onto
// this vocabulary, which the rest of the application already speaks. Zero means
// the driver could not map the value and refuses to guess.
const (
	PhaseStateUnknown   = 0
	PhaseStateLOS       = 1
	PhaseStateOnline    = 3
	PhaseStateDyingGasp = 4
	PhaseStateOffline   = 6
)

// ONTInventory is the identity data a driver can read for a single ONT. An
// empty field means the vendor exposes no OID for it, not that the ONT lacks
// the value.
type ONTInventory struct {
	SerialNumber    string
	Name            string
	Description     string
	DeviceType      string
	HardwareVersion string
	SoftwareVersion string
	IPAddress       string
	MACAddress      string
}

// Driver adapts one OLT model's SNMP read dialect: its OIDs, its ifIndex
// arithmetic, its value scaling and its sentinels. Implementations are
// stateless and open their own connection per call, matching how the callers
// already work.
//
// Read paths only. Provisioning (CLI over SSH/telnet) is not covered here, and
// PingTest/SSHTest/TelnetTest/SNMPTest stay vendor-agnostic.
type Driver interface {
	Model() models.OLTModel

	// WalkStatuses returns the canonical phase state of every ONT the OLT
	// knows about. This is the enumeration primitive: the discovery paths use
	// its keys as the list of ONTs that exist.
	WalkStatuses(ipAddress, community string, snmpPort int) (map[ONTLocation]int, error)

	// WalkMetrics returns optical and counter readings per ONT. Fields a
	// vendor does not expose stay at their zero value; RxPower/TxPower are nil
	// when there is no signal.
	WalkMetrics(ipAddress, community string, snmpPort int) (map[ONTLocation]ONTMetrics, error)

	// Inventory reads identity attributes for the given ONT locations, which
	// come from WalkStatuses.
	Inventory(ipAddress, community string, snmpPort int, locations []ONTLocation) (map[ONTLocation]ONTInventory, error)

	// QueryONTMetrics reads one ONT's metrics on demand, for the realtime view.
	QueryONTMetrics(ipAddress, community string, snmpPort, slot, port, ontID int) (*ONTMetrics, error)

	// WalkTrafficRates returns live rate gauges per ONT, or ErrUnsupported.
	WalkTrafficRates(ipAddress, community string, snmpPort int) (map[ONTLocation]ONUTrafficRates, error)

	// QueryTrafficRates reads one ONT's rate gauges, or ErrUnsupported.
	QueryTrafficRates(ipAddress, community string, snmpPort, slot, port, ontID int) (*ONUTrafficRates, error)

	// WalkUnconfigured lists ONUs detected optically but not yet provisioned,
	// or ErrUnsupported when the vendor's autofind table is unknown.
	WalkUnconfigured(ctx context.Context, ipAddress, community string, snmpPort int) ([]UnconfiguredONU, error)
}

// drivers holds one instance per supported model, populated by each driver
// file's init. Registration happens at process start, before any goroutine
// reads the map, so no lock is needed.
var drivers = make(map[models.OLTModel]Driver)

// Register adds a driver to the registry. A second registration for the same
// model is a programming error and panics rather than silently shadowing.
func Register(d Driver) {
	model := d.Model()
	if _, exists := drivers[model]; exists {
		panic(fmt.Sprintf("connectivity: driver for model %q registered twice", model))
	}
	drivers[model] = d
}

// DriverFor resolves the driver for an OLT model. An unknown model is an error,
// never a fallback: applying one vendor's decoding to another's raw values
// produces readings that look plausible and are wrong.
func DriverFor(model models.OLTModel) (Driver, error) {
	if model == "" {
		return nil, fmt.Errorf("OLT model is not set; pick one of %v", SupportedModels())
	}
	d, ok := drivers[model]
	if !ok {
		return nil, fmt.Errorf("unsupported OLT model %q; supported models are %v", model, SupportedModels())
	}
	return d, nil
}

// SupportedModels lists the registered models in declaration order, for error
// messages and API validation.
func SupportedModels() []models.OLTModel {
	all := []models.OLTModel{models.OLTModelZTEC300, models.OLTModelZTEC320, models.OLTModelHSGQ}
	registered := make([]models.OLTModel, 0, len(all))
	for _, m := range all {
		if _, ok := drivers[m]; ok {
			registered = append(registered, m)
		}
	}
	return registered
}
