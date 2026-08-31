package main

import (
	"fmt"
	"net"
	"strings"

	"github.com/google/uuid"
	"github.com/gosnmp/gosnmp"
)

// snmpTrapOID is the varbind every v2c trap carries naming what happened. Its
// value is the notification's own OID.
const snmpTrapOID = ".1.3.6.1.6.3.1.1.4.1.0"

// Trap is one notification, reduced to what this system can act on.
type Trap struct {
	// OLTID is the OLT the trap came from, matched by source address. A trap
	// from an address no OLT claims is not ours to interpret.
	OLTID uuid.UUID
	// Source is the address it arrived from, kept for the log so an unmatched
	// trap can be traced to a device.
	Source string
	// OID names the notification.
	OID string
	// Community is the v2c community the trap arrived with. ZTE packs the
	// event's own severity into it, which is the device stating what the
	// notification means rather than us inferring it from the OID.
	Community string
	// Varbinds are the values it carried, in arrival order.
	Varbinds []Varbind
}

// Varbind is one value from a trap.
type Varbind struct {
	OID string
	// Value is the display form, which shows an octet string both as text and as
	// hex because ZTE packs some fields printably and others not.
	Value string
	// Text is the value as a plain string when it was one, so a parser reads the
	// field rather than the rendering of it.
	Text string
}

// describe renders a trap for the log.
//
// Every field is included rather than only the ones we recognise, because the
// notification OIDs this hardware sends are not documented here and this is how
// they get learned: from the device, not from a guess about it.
func (t Trap) describe() string {
	parts := make([]string, 0, len(t.Varbinds))
	for _, vb := range t.Varbinds {
		parts = append(parts, fmt.Sprintf("%s=%s", vb.OID, vb.Value))
	}
	return strings.Join(parts, " ")
}

// oltFinder resolves a trap's source address to the OLT that sent it.
type oltFinder func(ip string) (uuid.UUID, bool)

// parseTrap reduces a received packet to a Trap, or reports why it cannot.
//
// A trap from an address no OLT claims is refused rather than guessed at: acting
// on a notification from an unknown device would let anything that can reach
// port 162 write to a subscriber's status.
func parseTrap(packet *gosnmp.SnmpPacket, addr *net.UDPAddr, findOLT oltFinder) (Trap, error) {
	if addr == nil {
		return Trap{}, fmt.Errorf("trap with no source address")
	}
	source := addr.IP.String()

	oltID, known := findOLT(source)
	if !known {
		return Trap{}, fmt.Errorf("trap from %s, which is not a known OLT", source)
	}

	trap := Trap{
		OLTID:     oltID,
		Source:    source,
		Community: packet.Community,
		Varbinds:  make([]Varbind, 0, len(packet.Variables)),
	}
	for _, variable := range packet.Variables {
		display, text := renderValue(variable)
		if normaliseOID(variable.Name) == normaliseOID(snmpTrapOID) {
			trap.OID = display
			continue
		}
		trap.Varbinds = append(trap.Varbinds, Varbind{OID: variable.Name, Value: display, Text: text})
	}

	if trap.OID == "" {
		return Trap{}, fmt.Errorf("trap from %s carries no notification OID", source)
	}
	return trap, nil
}

// normaliseOID makes the leading dot optional, which agents are inconsistent
// about and which would otherwise make an OID comparison fail on punctuation.
func normaliseOID(oid string) string {
	return strings.TrimPrefix(oid, ".")
}

// renderValue turns a varbind into a display form and, where it was one, the
// plain string behind it.
//
// Octet strings are displayed as text and as hex: ZTE packs serials and
// positions into octet strings that are printable in some traps and binary in
// others, and showing one form only would mean a second capture to read the
// other. The plain text is returned separately so a parser reads the field
// rather than the rendering of it.
func renderValue(variable gosnmp.SnmpPDU) (display, text string) {
	switch value := variable.Value.(type) {
	case []byte:
		text = string(value)
		return fmt.Sprintf("%q(hex:%x)", text, value), text
	case string:
		return value, value
	default:
		return fmt.Sprintf("%v", value), ""
	}
}
