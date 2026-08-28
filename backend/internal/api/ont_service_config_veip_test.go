package api

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/connectivity"
)

// The stored service carried use_veip and the response dropped it, so the
// configure form could only ever open with the toggle off — and saving from
// there removed VEIP from an ONU that had it on the OLT.
func TestServiceConfigPayloadCarriesVEIP(t *testing.T) {
	service := connectivity.ZTEONUService{
		ONUType: "HG8245H5", VLANID: 214, VLANMode: "tag",
		ServiceType: "bridge", TCONTProfile: "1G",
		WANMode: "setup_via_ont", UseVEIP: true,
	}

	encoded, err := json.Marshal(serviceConfigPayload(service))
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(encoded, &payload))
	assert.Equal(t, true, payload["use_veip"])
	assert.Equal(t, float64(214), payload["vlan_id"])
}

// An ONU without VEIP must report it as off rather than omit the field, or the
// form has nothing to distinguish "off" from "unknown".
func TestServiceConfigPayloadReportsVEIPOff(t *testing.T) {
	encoded, err := json.Marshal(serviceConfigPayload(connectivity.ZTEONUService{}))
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(encoded, &payload))

	value, present := payload["use_veip"]
	require.True(t, present, "the field has to be there even when false")
	assert.Equal(t, false, value)
}
