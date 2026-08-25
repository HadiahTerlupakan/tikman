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
	db       *gorm.DB // Read-only: OLT credentials loaded here
	resolver DriverResolver
	logger   *zap.Logger
}

// NewSnapshotService creates a snapshot service with vendor driver resolver
func NewSnapshotService(db *gorm.DB, resolver DriverResolver, logger *zap.Logger) *SnapshotService {
	return &SnapshotService{
		db:       db,
		resolver: resolver,
		logger:   logger,
	}
}

// CaptureBeforeSnapshot reads current ONT config via SNMP and maps to vendor-specific snapshot
// This is called BEFORE provisioning to establish baseline for comparison
func (s *SnapshotService) CaptureBeforeSnapshot(ont models.ONT) (*ConfigSnapshot, error) {
	loc := connectivity.ONTLocation{
		Slot:  intSlotOrDefault(ont.Slot),
		Port:  ont.PortID,
		ONTID: ont.ONTID,
	}

	// Load OLT info to get credentials and model
	var olt models.OLT
	if err := s.db.First(&olt, "id = ?", ont.OLTID).Error; err != nil {
		return nil, fmt.Errorf("failed to load OLT %s: %w", ont.OLTID, err)
	}

	driver, err := s.resolver(olt.Model)
	if err != nil {
		return nil, fmt.Errorf("resolve driver for model %s: %w", olt.Model, err)
	}

	// Fetch inventory (identity data: serial, name, device type, IP)
	inventoryMap, err := driver.Inventory(olt.IPAddress, olt.SNMPCommunity, olt.SNMPPort, []connectivity.ONTLocation{loc})
	if err != nil {
		return nil, fmt.Errorf("inventory fetch failed: %w", err)
	}

	inventory, ok := inventoryMap[loc]
	if !ok {
		return nil, fmt.Errorf("inventory not found for ONT location %+v", loc)
	}

	// Fetch metrics (software version, optical power, traffic counters)
	metrics, err := driver.QueryONTMetrics(olt.IPAddress, olt.SNMPCommunity, olt.SNMPPort, loc.Slot, loc.Port, loc.ONTID)
	if err != nil && err != connectivity.ErrUnsupported {
		return nil, fmt.Errorf("metrics query failed: %w", err)
	}

	// Build raw readings for post-hoc debugging
	raw := make(map[string]interface{})
	raw["olt_ip"] = olt.IPAddress
	if metrics != nil {
		raw["rx_power"] = metrics.RxPower
		raw["tx_power"] = metrics.TxPower
	}

	snap := &ConfigSnapshot{
		OntID:       ont.ID,
		Timestamp:   time.Now(),
		RawReadings: raw,
	}

	// Map to vendor-specific structure based on OLT model
	switch olt.Model {
	case models.OLTModelZTEC300, models.OLTModelZTEC320:
		snap.ZTE = &ZTESnapshot{
			SerialNumber:    inventory.SerialNumber,
			Name:            inventory.Name,
			DeviceType:      inventory.DeviceType,
			HardwareVersion: inventory.HardwareVersion,
			IPAddr:          inventory.IPAddress,
			// Bandwidth/VLAN are not exposed via SNMP, so they stay nil
			// ServiceMode defaults to "bridge" until CLI shows otherwise
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
			// VLAN and profile not stored per-ONU in HSGQ SNMP MIB
			VLANID:    nil,
			ProfileID: 0,
		}
	}

	return snap, nil
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
	var diffs []string

	// Handle nil cases
	if before == nil && after == nil {
		return nil
	}
	if before == nil || after == nil {
		return []string{"one snapshot is nil"}
	}

	// Check ZTE fields
	if (before.ZTE != nil) != (after.ZTE != nil) {
		diffs = append(diffs, "zte structure")
	} else if before.ZTE != nil && after.ZTE != nil {
		if before.ZTE.SerialNumber != after.ZTE.SerialNumber {
			diffs = append(diffs, "zte.serial_number")
		}
		if before.ZTE.Name != after.ZTE.Name {
			diffs = append(diffs, "zte.name")
		}
		if before.ZTE.DeviceType != after.ZTE.DeviceType {
			diffs = append(diffs, "zte.device_type")
		}
		if before.ZTE.HardwareVersion != after.ZTE.HardwareVersion {
			diffs = append(diffs, "zte.hardware_version")
		}
		if before.ZTE.ServiceMode != after.ZTE.ServiceMode {
			diffs = append(diffs, "zte.service_mode")
		}
		if before.ZTE.IPAddr != after.ZTE.IPAddr {
			diffs = append(diffs, "zte.ip_addr")
		}
		// Compare pointer fields (optional): only if both non-nil AND values differ
		if needPtrCompare(before.ZTE.Bandwidth, after.ZTE.Bandwidth) {
			diffs = append(diffs, "zte.bandwidth")
		}
		if needPtrCompare(before.ZTE.VLAN, after.ZTE.VLAN) {
			diffs = append(diffs, "zte.vlan")
		}
	}

	// Check HSGQ fields
	if (before.HSGQ != nil) != (after.HSGQ != nil) {
		diffs = append(diffs, "hsgq structure")
	} else if before.HSGQ != nil && after.HSGQ != nil {
		if before.HSGQ.SerialNumber != after.HSGQ.SerialNumber {
			diffs = append(diffs, "hsgq.serial_number")
		}
		if before.HSGQ.PortConfig != after.HSGQ.PortConfig {
			diffs = append(diffs, "hsgq.port_config")
		}
		if needPtrCompare(before.HSGQ.VLANID, after.HSGQ.VLANID) {
			diffs = append(diffs, "hsgq.vlan_id")
		}
		if before.HSGQ.ProfileID != after.HSGQ.ProfileID {
			diffs = append(diffs, "hsgq.profile_id")
		}
	}

	return diffs
}

// RollbackTo restores previous config using stored snapshot
// Note: This requires vendor-specific write commands (Telnet/SNMP SET) which are implemented in Phase 2
// For now, return unimplemented error with clear explanation for developers
func (s *SnapshotService) RollbackTo(ctx context.Context, ont models.ONT, snapshot *ConfigSnapshot) error {
	return fmt.Errorf("rollback not yet implemented — requires CLI command executor integration; see Phase 2 design doc for ZTE/HSGQ write protocols")
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
