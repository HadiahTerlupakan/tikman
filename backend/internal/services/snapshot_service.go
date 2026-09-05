package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ZTESnapshot captures ZTE OLT ONU configuration for rollback verification
type ZTESnapshot struct {
	SerialNumber    string  `json:"serial_number"`
	Name            string  `json:"name,omitempty"`
	DeviceType      string  `json:"device_type,omitempty"`
	HardwareVersion string  `json:"hardware_version,omitempty"`
	Bandwidth       *string `json:"bandwidth,omitempty"` // Optional if not configured
	VLAN            *string `json:"vlan,omitempty"`      // Optional if not configured
	ServiceMode     string  `json:"service_mode"`        // e.g., "ipoe", "bridge"
	IPAddr          string  `json:"ip_addr,omitempty"`   // Assigned IP address
}

// HSGQSnapshot captures HSGQ OLT ONU configuration for rollback verification
type HSGQSnapshot struct {
	SerialNumber string `json:"serial_number"`
	PortConfig   string `json:"port_config,omitempty"` // e.g., "gpon-onu-port/3/7"
	VLANID       *int   `json:"vlan_id,omitempty"`     // Not exposed by HSGQ SNMP
	ProfileID    int    `json:"profile_id"`            // Profile ID from config
}

// ConfigSnapshot captures ONT configuration at a point in time
// Used for before/after comparison and rollback restoration
type ConfigSnapshot struct {
	OntID       uuid.UUID              `json:"ont_id"`
	Timestamp   time.Time              `json:"timestamp"`
	ZTE         *ZTESnapshot           `json:"zte,omitempty"`
	HSGQ        *HSGQSnapshot          `json:"hsgq,omitempty"`
	RawReadings map[string]interface{} `json:"raw_readings,omitempty"`
}

// DriverResolver resolves an OLT model to its connectivity driver
type DriverResolver func(models.OLTModel) (connectivity.Driver, error)

// SnapshotService captures ONT configuration snapshots before/after provisioning
// for validation against written config and rollback restoration on failure
type SnapshotService struct {
	db               *gorm.DB // Read-only: OLT credentials loaded here
	resolver         DriverResolver
	logger           *zap.Logger
	commanderFactory CommanderFactory
}

// NewSnapshotService creates a snapshot service with vendor driver resolver
func NewSnapshotService(db *gorm.DB, resolver DriverResolver, logger *zap.Logger) *SnapshotService {
	return &SnapshotService{db: db, resolver: resolver, logger: logger}
}

// NewSnapshotServiceWithCommander adds the command boundary needed for generic rollback.
func NewSnapshotServiceWithCommander(db *gorm.DB, resolver DriverResolver, logger *zap.Logger, factory CommanderFactory) *SnapshotService {
	service := NewSnapshotService(db, resolver, logger)
	service.commanderFactory = factory
	return service
}

// CaptureBeforeSnapshot reads current ONT config via SNMP and maps to vendor-specific snapshot
// This is called BEFORE provisioning to establish baseline for comparison
func (s *SnapshotService) CaptureBeforeSnapshot(ont models.ONT) (*ConfigSnapshot, error) {
	loc := connectivity.ONTLocation{
		Slot:  intSlotOrDefault(ont.Slot),
		Port:  ont.PortID,
		ONTID: ont.ONTID,
	}

	var olt models.OLT
	if err := s.db.First(&olt, "id = ?", ont.OLTID).Error; err != nil {
		return nil, fmt.Errorf("failed to load OLT %s: %w", ont.OLTID, err)
	}

	driver, err := s.resolver(olt.Model)
	if err != nil {
		return nil, fmt.Errorf("resolve driver for model %s: %w", olt.Model, err)
	}

	inventoryMap, err := driver.Inventory(olt.IPAddress, olt.SNMPCommunity, olt.SNMPPort, []connectivity.ONTLocation{loc})
	if err != nil {
		return nil, fmt.Errorf("inventory fetch failed: %w", err)
	}
	inventory, ok := inventoryMap[loc]
	if !ok {
		return nil, fmt.Errorf("inventory not found for ONT location %+v", loc)
	}

	// ErrUnsupported is not a failure: a driver without a metrics table still
	// yields a usable identity snapshot.
	metrics, err := driver.QueryONTMetrics(olt.IPAddress, olt.SNMPCommunity, olt.SNMPPort, loc.Slot, loc.Port, loc.ONTID)
	if err != nil && err != connectivity.ErrUnsupported {
		return nil, fmt.Errorf("metrics query failed: %w", err)
	}

	snap := &ConfigSnapshot{
		OntID:       ont.ID,
		Timestamp:   time.Now(),
		RawReadings: rawReadings(olt, metrics),
	}
	applyVendorSnapshot(snap, olt.Model, ont, inventory, metrics)

	return snap, nil
}

// rawReadings keeps what the walk actually returned, for reading back after a
// provision went wrong.
func rawReadings(olt models.OLT, metrics *connectivity.ONTMetrics) map[string]interface{} {
	raw := map[string]interface{}{"olt_ip": olt.IPAddress}
	if metrics != nil {
		raw["rx_power"] = metrics.RxPower
		raw["tx_power"] = metrics.TxPower
	}
	return raw
}

// applyVendorSnapshot fills in the half of the snapshot that only makes sense
// for the chassis it came from.
func applyVendorSnapshot(snap *ConfigSnapshot, model models.OLTModel, ont models.ONT,
	inventory connectivity.ONTInventory, metrics *connectivity.ONTMetrics) {

	switch model {
	case models.OLTModelZTEC300, models.OLTModelZTEC320:
		snap.ZTE = &ZTESnapshot{
			SerialNumber:    inventory.SerialNumber,
			Name:            inventory.Name,
			DeviceType:      inventory.DeviceType,
			HardwareVersion: inventory.HardwareVersion,
			IPAddr:          inventory.IPAddress,
			// Bandwidth/VLAN are not exposed via SNMP, so they stay nil.
			// ServiceMode defaults to "bridge" until CLI shows otherwise.
			ServiceMode: "bridge",
		}
		if metrics != nil && metrics.SoftwareVersion != "" {
			snap.ZTE.ServiceMode = metrics.SoftwareVersion
		}

	default: // HSGQ or unknown
		snap.HSGQ = &HSGQSnapshot{
			SerialNumber: inventory.SerialNumber,
			// HSGQ names its ONU ports by PON position: <port>/<onu-id>.
			// Slot is not part of the HSGQ naming scheme.
			PortConfig: fmt.Sprintf("gpon-onu-port/%d/%d", ont.PortID, ont.ONTID),
			// VLAN and profile are not stored per-ONU in the HSGQ SNMP MIB.
			VLANID:    nil,
			ProfileID: 0,
		}
	}
}

// CaptureAfterSnapshot reads back config after provisioning attempt
// Identical implementation to CaptureBeforeSnapshot but used for comparison
func (s *SnapshotService) CaptureAfterSnapshot(ont models.ONT) (*ConfigSnapshot, error) {
	return s.CaptureBeforeSnapshot(ont)
}

// Compare returns list of field names where snapshots differ
// Nil optional fields are ignored - only non-nil changes trigger drift detection
// Empty slice means no differences detected (config verified match)
func (s *SnapshotService) Compare(before, after *ConfigSnapshot) []string {
	if before == nil && after == nil {
		return nil
	}
	if before == nil || after == nil {
		return []string{"one snapshot is nil"}
	}

	return append(compareZTE(before.ZTE, after.ZTE), compareHSGQ(before.HSGQ, after.HSGQ)...)
}

func compareZTE(before, after *ZTESnapshot) []string {
	if (before != nil) != (after != nil) {
		return []string{"zte structure"}
	}
	if before == nil {
		return nil
	}

	var diffs []string
	fields := []struct {
		name          string
		before, after string
	}{
		{"zte.serial_number", before.SerialNumber, after.SerialNumber},
		{"zte.name", before.Name, after.Name},
		{"zte.device_type", before.DeviceType, after.DeviceType},
		{"zte.hardware_version", before.HardwareVersion, after.HardwareVersion},
		{"zte.service_mode", before.ServiceMode, after.ServiceMode},
		{"zte.ip_addr", before.IPAddr, after.IPAddr},
	}
	for _, f := range fields {
		if f.before != f.after {
			diffs = append(diffs, f.name)
		}
	}

	// Optional fields count as changed only when both sides carry a value and
	// the values differ: a field the walk did not return is not drift.
	if needPtrCompare(before.Bandwidth, after.Bandwidth) {
		diffs = append(diffs, "zte.bandwidth")
	}
	if needPtrCompare(before.VLAN, after.VLAN) {
		diffs = append(diffs, "zte.vlan")
	}
	return diffs
}

func compareHSGQ(before, after *HSGQSnapshot) []string {
	if (before != nil) != (after != nil) {
		return []string{"hsgq structure"}
	}
	if before == nil {
		return nil
	}

	var diffs []string
	if before.SerialNumber != after.SerialNumber {
		diffs = append(diffs, "hsgq.serial_number")
	}
	if before.PortConfig != after.PortConfig {
		diffs = append(diffs, "hsgq.port_config")
	}
	if needPtrCompare(before.VLANID, after.VLANID) {
		diffs = append(diffs, "hsgq.vlan_id")
	}
	if before.ProfileID != after.ProfileID {
		diffs = append(diffs, "hsgq.profile_id")
	}
	return diffs
}

// RollbackTo restores previous config using stored snapshot
// Note: This requires vendor-specific write commands (Telnet/SNMP SET) which are implemented in Phase 2
// For now, return unimplemented error with clear explanation for developers
func (s *SnapshotService) RollbackTo(ctx context.Context, ont models.ONT, snapshot *ConfigSnapshot) error {
	if s.commanderFactory == nil {
		return fmt.Errorf("rollback command factory is not configured")
	}
	var olt models.OLT
	if err := s.db.First(&olt, "id = ?", ont.OLTID).Error; err != nil {
		return fmt.Errorf("load OLT for rollback: %w", err)
	}
	engine := NewRollbackEngineForOLTs(s.commanderFactory, s.logger)
	return engine.RollbackToSnapshotForOLT(ctx, olt, ont, snapshot)
}

// Helper functions

func intSlotOrDefault(slot *int) int {
	if slot != nil {
		return *slot
	}
	return 0
}

// needPtrCompare reports drift only when both optional fields are set and
// differ. A field that appears (nil → set) or disappears (set → nil) between
// snapshots is not drift: the vendor read path simply does not expose it
// consistently, and treating absence as a change would roll back every job.
func needPtrCompare[T comparable](a, b *T) bool {
	if a == nil || b == nil {
		return false
	}
	return *a != *b
}
