package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// cariuTrapVarbinds is a trap Cariu actually sent, transcribed from the
// receiver's own log. Inventing this shape would have tested a guess.
func cariuTrapVarbinds() []Varbind {
	return []Varbind{
		{OID: ".1.3.6.1.2.1.1.3.0", Value: "1237674500"},
		{OID: ".1.3.6.1.4.1.3902.1082.500.10.2.3.3.1.2.285280518.3", Text: "258281282012-NYIMAS HURI MEGA RIZKI"},
		{OID: ".1.3.6.1.4.1.3902.1082.500.10.2.3.3.1.1.285280518.3", Text: "ALL"},
		{OID: ".1.3.6.1.4.1.3902.1082.500.10.2.3.3.1.3.285280518.3", Text: "ONU-6:3"},
		{OID: ".1.3.6.1.4.1.3902.1082.500.10.2.3.3.1.18.285280518.3", Text: "1,ZTEGCACC7172"},
	}
}

func TestIdentifyReadsTheONUATrapIsAbout(t *testing.T) {
	identity := identify(cariuTrapVarbinds())

	require.Equal(t, "ZTEGCACC7172", identity.SerialNumber)
	require.Equal(t, "ONU-6:3", identity.Label)
	require.Equal(t, "258281282012-NYIMAS HURI MEGA RIZKI", identity.Name)
	require.Equal(t, int64(285280518), identity.IfIndex)
	require.Equal(t, 3, identity.ONUID)
}

func TestIdentifyTakesTheSerialWithoutItsLeadingField(t *testing.T) {
	// The value arrives as "1,ZTEGCACC7172": the leading field is the ONU's
	// authentication mode, and the serial is what identifies the box.
	identity := identify([]Varbind{
		{OID: ".1.3.6.1.4.1.3902.1082.500.10.2.3.3.1.18.285280518.3", Text: "1,ZTEGCACC7172"},
	})

	require.Equal(t, "ZTEGCACC7172", identity.SerialNumber)
}

func TestIdentifyIgnoresVarbindsFromOtherTables(t *testing.T) {
	// Every trap carries sysUpTime, and the admin login notifications this OLT
	// also sends carry a username and an address. Neither names an ONU.
	identity := identify([]Varbind{
		{OID: ".1.3.6.1.2.1.1.3.0", Value: "1237676300"},
		{OID: ".1.3.6.1.4.1.3902.1082.20.10.2.3.3.1.3.3", Text: "jejejods"},
		{OID: ".1.3.6.1.4.1.3902.1082.20.10.2.3.3.1.8.3", Text: "172.30.30.1"},
	})

	require.Empty(t, identity.SerialNumber)
	require.Empty(t, identity.Label)
	require.Zero(t, identity.IfIndex)
}

func TestIdentifyOnATrapNamingNoONU(t *testing.T) {
	identity := identify(nil)

	require.Empty(t, identity.SerialNumber)
	require.Zero(t, identity.ONUID)
}

func TestIdentifyKeepsThePositionFromTheFirstONUColumn(t *testing.T) {
	// Every ONU column of one trap carries the same index. Taking it from
	// whichever column arrived last would make the position depend on varbind
	// order, which the agent does not promise.
	identity := identify([]Varbind{
		{OID: ".1.3.6.1.4.1.3902.1082.500.10.2.3.3.1.3.285280526.19", Text: "ONU-14:19"},
		{OID: ".1.3.6.1.4.1.3902.1082.500.10.2.3.3.1.18.285280526.19", Text: "1,RTEGC609B645"},
	})

	require.Equal(t, int64(285280526), identity.IfIndex)
	require.Equal(t, 19, identity.ONUID)
	require.Equal(t, "RTEGC609B645", identity.SerialNumber)
}

func TestSplitONUVarbindRejectsAMalformedIndex(t *testing.T) {
	// A column with the wrong number of index parts is not this table's shape,
	// and reading it as one would attribute a trap to an ONU at random.
	_, _, _, ok := splitONUVarbind(".1.3.6.1.4.1.3902.1082.500.10.2.3.3.1.18.285280518")
	require.False(t, ok)

	_, _, _, ok = splitONUVarbind(".1.3.6.1.4.1.3902.1082.500.10.2.3.3.1.18.abc.3")
	require.False(t, ok)
}

func TestSplitONUVarbindAcceptsAnOIDWithoutItsLeadingDot(t *testing.T) {
	column, ifIndex, onuID, ok := splitONUVarbind("1.3.6.1.4.1.3902.1082.500.10.2.3.3.1.18.285280518.3")

	require.True(t, ok)
	require.Equal(t, onuColumnSerial, column)
	require.Equal(t, int64(285280518), ifIndex)
	require.Equal(t, 3, onuID)
}
