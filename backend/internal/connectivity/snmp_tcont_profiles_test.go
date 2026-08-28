package connectivity

import (
	"testing"

	"github.com/gosnmp/gosnmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The two profiles this OLT actually has, at the profile IDs it indexes them
// by, with the figures its CLI reports for the same profiles.
func tcontPDUs() []gosnmp.SnmpPDU {
	return []gosnmp.SnmpPDU{
		{Name: zteTcontProfileName + ".1879048193", Type: gosnmp.OctetString, Value: []byte("default")},
		{Name: zteTcontProfileName + ".1879048194", Type: gosnmp.OctetString, Value: []byte("1G")},
		{Name: zteTcontProfileFixed + ".1879048193", Type: gosnmp.Integer, Value: 10000},
		{Name: zteTcontProfileFixed + ".1879048194", Type: gosnmp.Integer, Value: 0},
		{Name: zteTcontProfileAssured + ".1879048193", Type: gosnmp.Integer, Value: 0},
		{Name: zteTcontProfileAssured + ".1879048194", Type: gosnmp.Integer, Value: 512},
		{Name: zteTcontProfileMax + ".1879048193", Type: gosnmp.Integer, Value: 0},
		{Name: zteTcontProfileMax + ".1879048194", Type: gosnmp.Integer, Value: 1024000},
		{Name: zteTcontProfileType + ".1879048193", Type: gosnmp.Integer, Value: 1},
		{Name: zteTcontProfileType + ".1879048194", Type: gosnmp.Integer, Value: 3},
	}
}

func TestWalkTcontProfiles(t *testing.T) {
	_, snmpPort := newUncfgAgent(t, tcontPDUs())

	profiles, err := WalkTcontProfiles("127.0.0.1", "public", snmpPort)
	require.NoError(t, err)

	assert.Equal(t, []ZTETcontProfile{
		{Name: "default", Type: 1, FixedBW: 10000, AssuredBW: 0, MaxBW: 0},
		{Name: "1G", Type: 3, FixedBW: 0, AssuredBW: 512, MaxBW: 1024000},
	}, profiles)
}

// The bandwidth columns are separate walks keyed by profile ID. Pairing them by
// arrival order instead would give one profile another's speed.
func TestWalkTcontProfilesKeepsBandwidthWithItsProfile(t *testing.T) {
	_, snmpPort := newUncfgAgent(t, tcontPDUs())

	profiles, err := WalkTcontProfiles("127.0.0.1", "public", snmpPort)
	require.NoError(t, err)
	require.Len(t, profiles, 2)

	assert.Equal(t, 1024000, profiles[1].MaxBW, "1G must keep its own maximum")
	assert.Equal(t, 0, profiles[0].MaxBW, "default grants no maximum")
}

// A bandwidth row whose profile the name column never listed has nothing to
// attach to, and must not invent a nameless profile.
func TestWalkTcontProfilesIgnoresOrphanBandwidths(t *testing.T) {
	_, snmpPort := newUncfgAgent(t, []gosnmp.SnmpPDU{
		{Name: zteTcontProfileName + ".1879048193", Type: gosnmp.OctetString, Value: []byte("default")},
		{Name: zteTcontProfileMax + ".1879048193", Type: gosnmp.Integer, Value: 10000},
		{Name: zteTcontProfileMax + ".1879048999", Type: gosnmp.Integer, Value: 99},
	})

	profiles, err := WalkTcontProfiles("127.0.0.1", "public", snmpPort)
	require.NoError(t, err)

	assert.Equal(t, []ZTETcontProfile{{Name: "default", MaxBW: 10000}}, profiles)
}
