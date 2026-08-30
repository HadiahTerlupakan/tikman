package main

import (
	"net"
	"testing"

	"github.com/google/uuid"
	"github.com/gosnmp/gosnmp"
	"github.com/stretchr/testify/require"
)

func knownOLT(id uuid.UUID, ip string) oltFinder {
	return func(candidate string) (uuid.UUID, bool) {
		if candidate == ip {
			return id, true
		}
		return uuid.Nil, false
	}
}

func from(ip string) *net.UDPAddr {
	return &net.UDPAddr{IP: net.ParseIP(ip), Port: 162}
}

func packetWith(vars ...gosnmp.SnmpPDU) *gosnmp.SnmpPacket {
	return &gosnmp.SnmpPacket{Variables: vars}
}

func trapOIDVar(oid string) gosnmp.SnmpPDU {
	return gosnmp.SnmpPDU{Name: snmpTrapOID, Type: gosnmp.ObjectIdentifier, Value: oid}
}

func TestParseTrapIdentifiesTheOLTBySourceAddress(t *testing.T) {
	oltID := uuid.New()

	trap, err := parseTrap(
		packetWith(trapOIDVar(".1.3.6.1.4.1.3902.1015.1.1")),
		from("172.30.30.3"),
		knownOLT(oltID, "172.30.30.3"),
	)

	require.NoError(t, err)
	require.Equal(t, oltID, trap.OLTID)
	require.Equal(t, "172.30.30.3", trap.Source)
	require.Equal(t, ".1.3.6.1.4.1.3902.1015.1.1", trap.OID)
}

func TestParseTrapRefusesADeviceNoOLTClaims(t *testing.T) {
	// Port 162 takes anything that can reach it. Acting on a notification from
	// an unknown device would let whatever sent it write a subscriber's status.
	_, err := parseTrap(
		packetWith(trapOIDVar(".1.3.6.1.4.1.3902.1015.1.1")),
		from("10.0.0.9"),
		knownOLT(uuid.New(), "172.30.30.3"),
	)

	require.Error(t, err)
	require.Contains(t, err.Error(), "not a known OLT")
}

func TestParseTrapRefusesAPacketWithNoNotificationOID(t *testing.T) {
	// Without it there is nothing saying what happened, and every field that
	// follows is unanchored.
	_, err := parseTrap(
		packetWith(gosnmp.SnmpPDU{Name: ".1.3.6.1.2.1.1.3.0", Type: gosnmp.TimeTicks, Value: uint32(42)}),
		from("172.30.30.3"),
		knownOLT(uuid.New(), "172.30.30.3"),
	)

	require.Error(t, err)
	require.Contains(t, err.Error(), "no notification OID")
}

func TestParseTrapMatchesTheNotificationOIDWithoutItsLeadingDot(t *testing.T) {
	// Agents differ on whether they write the leading dot. Comparing on
	// punctuation would drop the one varbind that names what happened.
	trap, err := parseTrap(
		packetWith(gosnmp.SnmpPDU{
			Name: "1.3.6.1.6.3.1.1.4.1.0", Type: gosnmp.ObjectIdentifier,
			Value: ".1.3.6.1.4.1.3902.1015.1.1",
		}),
		from("172.30.30.3"),
		knownOLT(uuid.New(), "172.30.30.3"),
	)

	require.NoError(t, err)
	require.Equal(t, ".1.3.6.1.4.1.3902.1015.1.1", trap.OID)
}

func TestParseTrapKeepsEveryOtherVarbind(t *testing.T) {
	// The notification OIDs this hardware sends are not known here yet, so the
	// trap is recorded whole. Dropping fields we do not recognise is what would
	// stop them ever being recognised.
	trap, err := parseTrap(
		packetWith(
			trapOIDVar(".1.3.6.1.4.1.3902.1015.1.1"),
			gosnmp.SnmpPDU{Name: ".1.3.6.1.4.1.3902.1015.2.1", Type: gosnmp.Integer, Value: 3},
			gosnmp.SnmpPDU{Name: ".1.3.6.1.4.1.3902.1015.2.2", Type: gosnmp.OctetString, Value: []byte("ZTEG")},
		),
		from("172.30.30.3"),
		knownOLT(uuid.New(), "172.30.30.3"),
	)

	require.NoError(t, err)
	require.Len(t, trap.Varbinds, 2)
	require.Equal(t, "3", trap.Varbinds[0].Value)
	require.Contains(t, trap.Varbinds[1].Value, "ZTEG")
	require.Contains(t, trap.Varbinds[1].Value, "hex:5a544547")
}

func TestDescribeRendersEveryVarbindForTheLog(t *testing.T) {
	trap := Trap{Varbinds: []Varbind{
		{OID: ".1.2.3", Value: "3"},
		{OID: ".1.2.4", Value: "up"},
	}}

	require.Equal(t, ".1.2.3=3 .1.2.4=up", trap.describe())
}

func TestParseTrapRefusesAPacketWithNoSource(t *testing.T) {
	_, err := parseTrap(packetWith(trapOIDVar(".1.2.3")), nil, knownOLT(uuid.New(), "172.30.30.3"))

	require.Error(t, err)
}
