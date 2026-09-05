package services

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
)

// Turning a provisioning request into the commands one OLT model understands,
// and running them.

func (s *OntProvisioningService) buildProvisionConfig(config ProvisionConfig, ont *models.ONT, olt *models.OLT) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	if config.TemplateID != nil {
		if s.templates == nil {
			return nil, fmt.Errorf("config template service is not configured")
		}
		template, err := s.templates.GetByID(*config.TemplateID)
		if err != nil {
			return nil, fmt.Errorf("load config template: %w", err)
		}
		if !templateVendorMatchesOLT(template.Vendor, olt.Model) {
			return nil, fmt.Errorf("config template vendor does not match OLT model")
		}
		var fields map[string]interface{}
		if err := json.Unmarshal(template.ConfigFields, &fields); err != nil {
			return nil, fmt.Errorf("decode config template fields: %w", err)
		}
		for k, v := range fields {
			result[k] = v
		}
	}

	// Add manual overrides after validating the complete input. The runtime map
	// remains unredacted so command execution receives the original values.
	for k, v := range config.ManualConfig {
		result[k] = v
	}

	if err := validateManualConfig(result); err != nil {
		return nil, err
	}

	// Add implicit ONT/Olt IDs
	result["ont_id"] = ont.ID
	result["olt_id"] = olt.ID
	result["slot"] = ont.Slot
	result["port"] = ont.PortID
	result["ontid"] = ont.ONTID

	return result, nil
}

func templateVendorMatchesOLT(vendor string, model models.OLTModel) bool {
	switch model {
	case models.OLTModelZTEC300, models.OLTModelZTEC320:
		return vendor == models.VendorZTE
	case models.OLTModelHSGQ:
		return vendor == models.VendorHSGQ
	default:
		return false
	}
}

func (s *OntProvisioningService) executeProvision(ctx context.Context, ont *models.ONT, olt *models.OLT, config map[string]interface{}) (*connectivity.CommandResult, error) {
	// Create a fresh commander for this OLT using the factory
	commander, err := createCommanderForOLT(s.commanderFactory, *olt)
	if err != nil {
		return nil, fmt.Errorf("failed to create commander for OLT %s: %w", olt.IPAddress, err)
	}
	defer closeCommander(commander)

	// Prepare vendor-specific CLI commands based on OLT model
	var cmds []string

	switch olt.Model {
	case models.OLTModelZTEC300, models.OLTModelZTEC320:
		cmds = s.buildZTECommands(*ont, config)
	default:
		cmds = s.buildHSGQCommands(*ont, config)
	}

	results, err := commander.BatchExecute(ctx, cmds)
	if err != nil {
		return nil, fmt.Errorf("command batch execution failed: %w", err)
	}

	// Check results
	for i, r := range results {
		if !r.Success {
			return nil, fmt.Errorf("command %d failed: %s", i, r.Output)
		}
	}

	return results[len(results)-1], nil
}

func (s *OntProvisioningService) buildZTECommands(ont models.ONT, config map[string]interface{}) []string {
	cmds := []string{
		"configure terminal",
		fmt.Sprintf("interface gpon-onu %d:%d", ont.PortID, ont.ONTID),
	}

	// Add bandwidth/vlan config if present
	if bw, ok := config["bandwidth"]; ok {
		cmds = append(cmds, fmt.Sprintf("ont traffic band-width %s %d", manualTokenString(bw), ont.ONTID))
	}
	if vlan, ok := config["vlan"]; ok {
		cmds = append(cmds, fmt.Sprintf("ont service vlan add %s %d", manualTokenString(vlan), ont.ONTID))
	}
	cmds = append(cmds, "commit")
	return cmds
}

func (s *OntProvisioningService) buildHSGQCommands(ont models.ONT, config map[string]interface{}) []string {
	return []string{
		"configure terminal",
		fmt.Sprintf("interface gpon-onu-port/%d/%d", ont.PortID, ont.ONTID),
		"commit",
	}
}

func (s *OntProvisioningService) rollbackOnt(job *models.ProvisioningJob, ont models.ONT, olt models.OLT, snap *ConfigSnapshot) error {
	if s.rollback == nil {
		return fmt.Errorf("rollback engine not configured")
	}
	if rollbacker, ok := interface{}(s.rollback).(oltRollbacker); ok {
		return rollbacker.RollbackToSnapshotForOLT(context.Background(), olt, ont, snap)
	}
	return s.rollback.RollbackToSnapshot(context.Background(), ont, snap)
}

func createCommanderForOLT(factory CommanderFactory, olt models.OLT) (connectivity.OLTCommander, error) {
	protocol := olt.PreferredProtocol
	if protocol == "" {
		protocol = models.OLTProtocolTelnet
	}
	port := olt.TelnetPort
	if protocol == models.OLTProtocolSSH {
		port = olt.SSHPort
	}
	if protocolFactory, ok := factory.(protocolCommanderFactory); ok {
		return protocolFactory.ForOLTWithProtocol(olt.Model, olt.IPAddress, protocol, port, olt.Username, olt.Password)
	}
	return factory.ForOLT(olt.Model, olt.IPAddress, port, olt.Username, olt.Password)
}

// closeCommander closes a commander if it implements io.Closer. Fake
// commanders in tests do not hold connections, so failing the assertion is
// the correct behavior.
func closeCommander(commander connectivity.OLTCommander) {
	if closer, ok := commander.(interface{ Close() error }); ok {
		_ = closer.Close()
	}
}
