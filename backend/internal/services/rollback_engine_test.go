package services

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"go.uber.org/zap"
)

func TestRollbackEngine_RollbackToSnapshot_ZTE(t *testing.T) {
	ont := models.ONT{PortID: 1, ONTID: 5}
	bw := "100M"
	vlan := "100"
	snap := &ConfigSnapshot{
		ZTE: &ZTESnapshot{
			SerialNumber: "ZTEGC0A1B2C3",
			Name:         "customer-42",
			Bandwidth:    &bw,
			VLAN:         &vlan,
			ServiceMode:  "bridge",
		},
	}

	cmdr := &fakeCommander{}
	engine := NewRollbackEngine(cmdr, zap.NewNop())
	err := engine.RollbackToSnapshot(context.Background(), ont, snap)
	require.NoError(t, err)
	assert.Contains(t, cmdr.commands[0], "config")
	assert.Contains(t, cmdr.commands[len(cmdr.commands)-1], "commit")
}

func TestRollbackEngine_RollbackToSnapshot_HSGQ(t *testing.T) {
	ont := models.ONT{PortID: 1, ONTID: 5}
	snap := &ConfigSnapshot{
		HSGQ: &HSGQSnapshot{
			SerialNumber: "HWTCB403E8A0",
			PortConfig:   "gpon-onu-port/1/5",
			ProfileID:    1,
		},
	}

	cmdr := &fakeCommander{}
	engine := NewRollbackEngine(cmdr, zap.NewNop())
	err := engine.RollbackToSnapshot(context.Background(), ont, snap)
	require.NoError(t, err)
	assert.Contains(t, cmdr.commands[0], "configure terminal")
	assert.Contains(t, cmdr.commands[len(cmdr.commands)-1], "commit")
}

func TestRollbackEngine_NilSnapshot(t *testing.T) {
	engine := NewRollbackEngine(&fakeCommander{}, zap.NewNop())
	var snap *ConfigSnapshot
	err := engine.RollbackToSnapshot(context.Background(), models.ONT{}, snap)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "snapshot is nil")
}

func TestRollbackEngine_EmptySnapshot(t *testing.T) {
	engine := NewRollbackEngine(&fakeCommander{}, zap.NewNop())
	snap := &ConfigSnapshot{}
	err := engine.RollbackToSnapshot(context.Background(), models.ONT{}, snap)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no vendor configuration")
}
