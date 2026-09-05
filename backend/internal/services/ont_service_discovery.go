package services

import (
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/utils"
)

// Registration of what an SNMP walk found on a chassis: the rows a discovery
// turns into, and the fields an already-known ONT may learn from it.

// BulkRegisterResult holds the result of bulk ONT registration
type BulkRegisterResult struct {
	Registered int
	Skipped    int
	Errors     []string
}

// BulkRegisterFromDiscovery registers multiple ONTs from discovery results
// describeSlot renders a slot for an error message an operator has to act on,
// where "unknown" is more useful than an empty gap.
func describeSlot(slot *int) string {
	if slot == nil {
		return "unknown"
	}
	return strconv.Itoa(*slot)
}

// discoveredSlot keeps a slot the OLT did not report out of the row, so an
// unknown slot stays null rather than becoming a real-looking zero.
func discoveredSlot(ont connectivity.DiscoveredONT) *int {
	if ont.Slot <= 0 {
		return nil
	}
	slot := ont.Slot
	return &slot
}

func (s *ONTService) BulkRegisterFromDiscovery(oltID uuid.UUID, discovered []connectivity.DiscoveredONT) *BulkRegisterResult {
	result := &BulkRegisterResult{
		Errors: make([]string, 0),
	}

	for _, ont := range discovered {
		existing := s.rowForDiscovered(oltID, ont)

		if existing != nil {
			updates := discoveryUpdates(ont, existing)
			if len(updates) == 0 {
				result.Skipped++
				continue
			}
			updates["updated_at"] = time.Now()
			if err := s.db.Model(existing).Updates(updates).Error; err == nil {
				result.Registered++
			}
			continue
		}

		err := s.Create(newONTFromDiscovery(oltID, ont))
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Port %d ONT %d: %v", ont.PortID, ont.ONTID, err))
			continue
		}

		result.Registered++
	}

	return result
}

// rowForDiscovered finds the row a discovery record belongs to, by position.
//
// Not by serial: this OLT reports the same serial at two positions in one walk,
// so treating a serial found elsewhere as the same box moved made its row chase
// between the two every cycle. A serial appearing where another row already
// holds it stays an error the log names.
func (s *ONTService) rowForDiscovered(oltID uuid.UUID, ont connectivity.DiscoveredONT) *models.ONT {
	existing, _ := s.GetByOLTAndPosition(oltID, ont.Slot, ont.PortID, ont.ONTID)
	if existing == nil && ont.Slot > 0 {
		// A row registered before the OLT reported card numbers carries a null
		// slot. It is the same ONT, so discovery backfills its card rather than
		// inserting a second row beside it. Only the null case falls back: a row
		// already sitting on a different card is a different subscriber's box.
		existing, _ = s.GetByOLTAndPosition(oltID, 0, ont.PortID, ont.ONTID)
	}
	return existing
}

// newONTFromDiscovery builds the row for an ONU no stored row claims yet.
func newONTFromDiscovery(oltID uuid.UUID, ont connectivity.DiscoveredONT) *models.ONT {
	return &models.ONT{
		OLTID:           oltID,
		Slot:            discoveredSlot(ont),
		PortID:          ont.PortID,
		ONTID:           ont.ONTID,
		SerialNumber:    ont.SerialNumber,
		Name:            ont.Name,
		Description:     ont.Description,
		DeviceType:      ont.DeviceType,
		HardwareVersion: ont.HardwareVersion,
		SoftwareVersion: ont.SoftwareVersion,
		IPAddress:       ont.IPAddress,
		MACAddress:      ont.MACAddress,
		// The discovery walk already read this ONT's phase state, so storing
		// "unknown" here threw away a fact we had and left the ONT list showing
		// UNKNOWN until the next status poll happened to run. Newly registered
		// ONTs sort first by created_at, so those placeholders were exactly the
		// rows an operator saw first after adding an OLT.
		Status: models.ONTStatus(utils.StatusMap(ont.RunState)),
	}
}

// discoveryUpdates returns what the walk knows that the stored row does not.
// An empty map means the row already agrees with the OLT and must not be
// written, which is what keeps updated_at meaningful across repeat scans.
//
// Inventory fields only fill gaps: the walk reads them inconsistently, so an
// operator's correction should outlive the next scan. Name and description are
// the exception, being the OLT's own labels rather than ours.
func discoveryUpdates(ont connectivity.DiscoveredONT, existing *models.ONT) map[string]interface{} {
	updates := map[string]interface{}{}

	if ont.Name != "" && existing.Name != ont.Name {
		updates["name"] = ont.Name
	}
	if ont.Description != "" && existing.Description != ont.Description {
		updates["description"] = ont.Description
	}
	if ont.DeviceType != "" && existing.DeviceType == "" {
		updates["device_type"] = ont.DeviceType
	}
	if ont.HardwareVersion != "" && existing.HardwareVersion == "" {
		updates["hardware_version"] = ont.HardwareVersion
	}
	if ont.SoftwareVersion != "" && existing.SoftwareVersion == "" {
		updates["software_version"] = ont.SoftwareVersion
	}
	if ont.IPAddress != "" && existing.IPAddress == "" {
		updates["ip_address"] = ont.IPAddress
	}
	if ont.MACAddress != "" && existing.MACAddress == "" {
		updates["mac_address"] = ont.MACAddress
	}

	// Backfills rows registered before discovery carried a slot. The auto ONU ID
	// allocator matches on it, and a null one hides the ONT from that lookup,
	// which can hand out an ID already in use.
	if ont.Slot > 0 && existing.Slot == nil {
		updates["slot"] = ont.Slot
	}

	return updates
}
