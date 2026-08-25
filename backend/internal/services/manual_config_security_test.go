package services

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
)

func TestValidateManualConfigRejectsUnknownAndUnsafeValues(t *testing.T) {
	tests := []struct {
		name   string
		config map[string]interface{}
		want   string
	}{
		{name: "unknown key", config: map[string]interface{}{"profile": "1"}, want: "not allowed"},
		{name: "command injection", config: map[string]interface{}{"vlan": "100; reboot"}, want: "vlan"},
		{name: "invalid vlan range", config: map[string]interface{}{"vlan": 4095}, want: "vlan"},
		{name: "non scalar value", config: map[string]interface{}{"bandwidth": map[string]interface{}{"value": "100M"}}, want: "bandwidth"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateManualConfig(tt.config)
			require.Error(t, err)
			assert.Contains(t, strings.ToLower(err.Error()), tt.want)
		})
	}
}

func TestValidateManualConfigAcceptsSupportedValues(t *testing.T) {
	err := validateManualConfig(map[string]interface{}{"bandwidth": "100M", "vlan": "100"})
	require.NoError(t, err)
}

func TestBuildZTECommandsFormatsNumericManualValues(t *testing.T) {
	cmds := (&OntProvisioningService{}).buildZTECommands(models.ONT{PortID: 3, ONTID: 7}, map[string]interface{}{
		"bandwidth": float64(100),
		"vlan":      float64(100),
	})

	assert.Contains(t, cmds, "ont traffic band-width 100 7")
	assert.Contains(t, cmds, "ont service vlan add 100 7")
	for _, cmd := range cmds {
		assert.NotContains(t, cmd, "%!")
	}
}

func TestProvisioningErrorReturnedToCallerRedactsManualSecrets(t *testing.T) {
	assert.Equal(t, "command failed password=<redacted>", redactProvisioningError(
		"command failed password=super-secret",
		map[string]interface{}{"password": "super-secret"},
	))
}

func TestPersistedConfigRedactsSensitiveNestedValues(t *testing.T) {
	persisted := redactManualConfig(map[string]interface{}{
		"bandwidth": "100M",
		"auth":      map[string]interface{}{"password": "super-secret"},
	})

	assert.Equal(t, redactedValue, persisted["auth"].(map[string]interface{})["password"])
	assert.Equal(t, "100M", persisted["bandwidth"])
}

func TestRedactManualConfigRemovesSensitiveValuesWithoutMutatingRuntimeConfig(t *testing.T) {
	runtime := map[string]interface{}{
		"bandwidth": "100M",
		"auth":      map[string]interface{}{"password": "super-secret"},
	}

	redacted := redactManualConfig(runtime)

	assert.Equal(t, "super-secret", runtime["auth"].(map[string]interface{})["password"])
	assert.Equal(t, redactedValue, redacted["auth"].(map[string]interface{})["password"])
	assert.Equal(t, "100M", redacted["bandwidth"])
}

func TestRedactProvisioningErrorRemovesRuntimeSecrets(t *testing.T) {
	err := redactProvisioningError(
		"command failed for username=operator password=super-secret",
		map[string]interface{}{"username": "operator", "password": "super-secret"},
	)

	assert.NotContains(t, err, "super-secret")
	assert.Contains(t, err, redactedValue)
	assert.Contains(t, err, "operator")
}

func TestProvisionOntRejectsUnsafeManualConfigBeforeCommands(t *testing.T) {
	cmdr := &fakeCommander{}
	driver := &fakeDriver{
		model:     models.OLTModelZTEC300,
		inventory: connectivity.ONTInventory{SerialNumber: "ZTEGC0A1B2C3"},
	}
	svc, fixtures := newProvisioningService(t, models.OLTModelZTEC300, cmdr, driver)

	_, err := svc.ProvisionOnt(context.Background(), fixtures.ont.ID, uuid.New(), ProvisionConfig{
		ManualConfig: map[string]interface{}{"vlan": "100; reboot"},
	})
	require.Error(t, err)
	assert.Empty(t, cmdr.commands)
}
