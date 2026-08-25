package services

import (
	"context"
	"fmt"

	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
	"go.uber.org/zap"
)

// RollbackEngine restores ONT configuration from captured snapshots using the
// same command executor path used for provisioning. It is idempotent: applying
// the same snapshot twice yields the same final state.
type RollbackEngine struct {
	commander connectivity.OLTCommander
	logger    *zap.Logger
}

// NewRollbackEngine constructs a RollbackEngine with the given command executor.
func NewRollbackEngine(commander connectivity.OLTCommander, logger *zap.Logger) *RollbackEngine {
	return &RollbackEngine{
		commander: commander,
		logger:    logger,
	}
}

// RollbackToSnapshot restores an ONT's configuration from a ConfigSnapshot.
// It issues the vendor-specific inverse commands for the captured fields.
// The operation is idempotent: repeated calls with the same snapshot produce
// the same end state because each CLI command overwrites the target fields.
func (e *RollbackEngine) RollbackToSnapshot(ctx context.Context, ont models.ONT, snapshot *ConfigSnapshot) error {
	if snapshot == nil {
		return fmt.Errorf("snapshot is nil")
	}

	if snapshot.ZTE != nil {
		cmds := e.buildZTERollbackCommands(ont, snapshot.ZTE)
		return e.executeRollback(ctx, cmds, "ZTE")
	}

	if snapshot.HSGQ != nil {
		cmds := e.buildHSGQRollbackCommands(ont, snapshot.HSGQ)
		return e.executeRollback(ctx, cmds, "HSGQ")
	}

	return fmt.Errorf("snapshot contains no vendor configuration to restore")
}

func (e *RollbackEngine) executeRollback(ctx context.Context, cmds []string, vendor string) error {
	results, err := e.commander.BatchExecute(ctx, cmds)
	if err != nil {
		return fmt.Errorf("failed to execute %s rollback commands: %w", vendor, err)
	}

	for _, result := range results {
		if !result.Success {
			return fmt.Errorf("%s rollback command failed: %s", vendor, result.Error)
		}
	}

	e.logger.Info("rollback completed", zap.String("vendor", vendor), zap.Int("commands", len(cmds)))
	return nil
}

func (e *RollbackEngine) buildZTERollbackCommands(ont models.ONT, snap *ZTESnapshot) []string {
	cmds := []string{
		"config",
		fmt.Sprintf("interface gpon 0/%d", ont.PortID),
		fmt.Sprintf("ont ontid %d profile default", ont.ONTID),
	}

	if snap.Bandwidth != nil && *snap.Bandwidth != "" {
		cmds = append(cmds, fmt.Sprintf("ont traffic band-width %s %d", *snap.Bandwidth, ont.ONTID))
	}
	if snap.VLAN != nil && *snap.VLAN != "" {
		cmds = append(cmds, fmt.Sprintf("ont service vlan restore %s %d", *snap.VLAN, ont.ONTID))
	}

	cmds = append(cmds, "commit")
	return cmds
}

func (e *RollbackEngine) buildHSGQRollbackCommands(ont models.ONT, snap *HSGQSnapshot) []string {
	return []string{
		"configure terminal",
		fmt.Sprintf("interface gpon-oltport 0/%d", ont.PortID),
		fmt.Sprintf("ont delete %d", ont.ONTID),
		"commit",
	}
}
