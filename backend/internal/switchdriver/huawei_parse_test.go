package switchdriver

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "huawei", name))
	require.NoError(t, err)
	return string(data)
}

func boolPtr(b bool) *bool { return &b }

func TestParseVersion(t *testing.T) {
	identity, err := parseVersion(fixture(t, "version.txt"))
	require.NoError(t, err)
	assert.Equal(t, Identity{Model: "CE6850-48S4Q-EI", Version: "V100R006C00SPC600"}, identity)
}

func TestParseVersionRejectsUnknownOutput(t *testing.T) {
	_, err := parseVersion("Error: Unrecognized command found at '^' position.")
	assert.Error(t, err)
}

func TestParseConfigPorts(t *testing.T) {
	ports := parseConfigPorts(fixture(t, "current-configuration.txt"))

	want := []Port{
		{Name: "Vlanif10", Kind: KindVirtual, AdminUp: true},
		{Name: "MEth0/0/0", Kind: KindRJ, AdminUp: true},
		{Name: "Eth-Trunk1", Alias: "TO-SW-B", Kind: KindVirtual, Mode: ModeTrunk, AdminUp: true,
			PVID: 1, AllowedVLANs: "10,20,30", STPDisabled: boolPtr(true)},
		{Name: "10GE1/0/1", Alias: "UPLINK-CORE", Kind: KindSFP, Mode: ModeTrunk, AdminUp: true,
			PVID: 1, AllowedVLANs: "10,20,30,3359-3361", STPDisabled: boolPtr(true)},
		{Name: "10GE1/0/2", Alias: "CUSTOMER A", Kind: KindSFP, Mode: ModeAccess, AdminUp: true,
			PVID: 20, STPDisabled: boolPtr(true)},
		{Name: "10GE1/0/3", Kind: KindSFP, Mode: ModeHybrid, AdminUp: true,
			PVID: 30, AllowedVLANs: "10,20,30", STPDisabled: boolPtr(false)},
		{Name: "10GE1/0/4", Kind: KindSFP, Mode: ModeAccess, AdminUp: false,
			PVID: 10, STPDisabled: boolPtr(false), STPEdge: boolPtr(true)},
		{Name: "10GE1/0/5", Alias: "TO-SW-B-1", Kind: KindSFP, AdminUp: true},
		{Name: "10GE1/0/6", Kind: KindSFP, Mode: ModeAccess, AdminUp: true,
			PVID: 1, STPDisabled: boolPtr(false)},
		{Name: "40GE1/0/1", Kind: KindQSFP, Mode: ModeAccess, AdminUp: true,
			PVID: 1, STPDisabled: boolPtr(false)},
		{Name: "LoopBack0", Kind: KindVirtual, AdminUp: true},
		{Name: "NULL0", Kind: KindVirtual, AdminUp: true},
	}
	assert.Equal(t, want, ports)
}

func TestTrunkAllowAllAndRemovals(t *testing.T) {
	config := "interface 10GE1/0/9\n" +
		" port link-type trunk\n" +
		" port trunk pvid vlan 10\n" +
		" port trunk allow-pass vlan all\n" +
		" undo port trunk allow-pass vlan 2 to 4093\n" +
		"#\n"
	ports := parseConfigPorts(config)
	require.Len(t, ports, 1)
	assert.Equal(t, 10, ports[0].PVID)
	assert.Equal(t, "1,4094", ports[0].AllowedVLANs)
}

func TestL3PortHasNoMode(t *testing.T) {
	ports := parseConfigPorts("interface 10GE1/0/9\n undo portswitch\n ip address 192.0.2.9 255.255.255.252\n#\n")
	require.Len(t, ports, 1)
	assert.Equal(t, "", ports[0].Mode)
	assert.Nil(t, ports[0].STPDisabled)
}

// A QinQ port (dot1q-tunnel) or any other link type outside the three modes
// is stored with no mode rather than the raw word, which the column and the
// UI do not know.
func TestUnknownLinkTypeHasNoMode(t *testing.T) {
	ports := parseConfigPorts("interface 10GE1/0/9\n port link-type dot1q-tunnel\n port default vlan 100\n#\n")

	require.Len(t, ports, 1)
	assert.Equal(t, Port{Name: "10GE1/0/9", Kind: KindSFP, AdminUp: true, STPDisabled: boolPtr(false)}, ports[0])
}

func TestPortKind(t *testing.T) {
	cases := []struct {
		name  string
		fiber bool
		want  string
	}{
		{"GE1/0/1", false, KindRJ},
		{"GE1/0/1", true, KindSFP},
		{"10GE1/0/1", false, KindSFP},
		{"25GE1/0/1", false, KindSFP},
		{"40GE1/0/1", false, KindQSFP},
		{"100GE1/0/1", false, KindQSFP},
		{"MEth0/0/0", false, KindRJ},
		{"Eth-Trunk1", false, KindVirtual},
		{"Vlanif10", false, KindVirtual},
		{"NULL0", false, KindVirtual},
		{"LoopBack0", false, KindVirtual},
		{"Nve1", false, KindVirtual},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, portKind(c.name, c.fiber), c.name)
	}
}

func TestParseInterfaceBrief(t *testing.T) {
	admin := parseInterfaceBrief(fixture(t, "interface-brief.txt"))
	assert.Len(t, admin, 12)
	assert.False(t, admin["10GE1/0/4"])
	assert.True(t, admin["40GE1/0/1"], "oper down is not admin down")
	assert.True(t, admin["10GE1/0/5"], "indented trunk members are read")
	_, legend := admin["PHY:"]
	assert.False(t, legend)
}

func TestParseSTPBrief(t *testing.T) {
	edges := parseSTPBrief(fixture(t, "stp-brief.txt"))
	assert.Equal(t, map[string]bool{"10GE1/0/3": false, "10GE1/0/6": true}, edges)
}

func TestMergePorts(t *testing.T) {
	ports := mergePorts(
		parseConfigPorts(fixture(t, "current-configuration.txt")),
		parseInterfaceBrief(fixture(t, "interface-brief.txt")),
		parseSTPBrief(fixture(t, "stp-brief.txt")),
	)
	byName := map[string]Port{}
	for _, p := range ports {
		byName[p.Name] = p
	}
	assert.Equal(t, boolPtr(false), byName["10GE1/0/3"].STPEdge)
	assert.Equal(t, boolPtr(true), byName["10GE1/0/6"].STPEdge)
	assert.Equal(t, boolPtr(true), byName["10GE1/0/4"].STPEdge, "config value kept when STP does not list the port")
	assert.Nil(t, byName["40GE1/0/1"].STPEdge)
	assert.False(t, byName["10GE1/0/4"].AdminUp)
	assert.True(t, byName["40GE1/0/1"].AdminUp)
}

func TestMergePortsTakesAdminStateFromBrief(t *testing.T) {
	ports := mergePorts([]Port{{Name: "10GE1/0/1", AdminUp: true}}, map[string]bool{"10GE1/0/1": false}, nil)
	assert.False(t, ports[0].AdminUp)
}

func TestParseVLANs(t *testing.T) {
	vlans := parseVLANs(fixture(t, "vlan.txt"))
	assert.Equal(t, []VLAN{
		{VID: 1, Name: "VLAN 0001"},
		{VID: 10, Name: "MGMT"},
		{VID: 20, Name: "CUSTOMER A"},
		{VID: 30, Name: ""},
		{VID: 3359, Name: "TRANSIT-1"},
	}, vlans)
}

func TestParseLLDPBrief(t *testing.T) {
	neighbors := parseLLDPBrief(fixture(t, "lldp-neighbor-brief.txt"))
	assert.Equal(t, []Neighbor{
		{Port: "10GE1/0/1", Kind: NeighborLLDP, RemoteName: "SW-TEST-B", RemotePort: "10GE1/0/3"},
		{Port: "10GE1/0/3", Kind: NeighborLLDP, RemoteName: "SW-TEST-C", RemotePort: "TFGigabitEthernet 0/25"},
		{Port: "10GE1/0/6", Kind: NeighborLLDP, RemoteName: "RT-TEST-1", RemotePort: "sfp-sfpplus1"},
	}, neighbors)
}

func TestParseMACTable(t *testing.T) {
	neighbors := parseMACTable(fixture(t, "mac-address.txt"))
	assert.Equal(t, []Neighbor{
		{Port: "Eth-Trunk1", Kind: NeighborMAC, MAC: "02:00:5E:00:0A:01", VID: 10},
		{Port: "10GE1/0/2", Kind: NeighborMAC, MAC: "02:00:5E:00:14:01", VID: 20},
		{Port: "10GE1/0/3", Kind: NeighborMAC, MAC: "02:00:5E:00:1E:01", VID: 30},
	}, neighbors)
}

func TestVLANListRoundTrip(t *testing.T) {
	set := vlanSet{}
	set.apply([]string{"30", "10", "3359", "to", "3361", "11"}, true)
	set.apply([]string{"11"}, false)
	assert.Equal(t, "10,30,3359-3361", set.String())
}
