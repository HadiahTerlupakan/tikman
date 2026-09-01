package main

import (
	"strconv"
	"strings"
)

// The columns a ZTE ONU trap carries, under the table this OLT's traps index
// into. The table itself is the one the poller already reads — the repository
// documents .1.18 as the serial and .1.2 as the name — so a trap and a poll
// name the same ONU the same way.
const (
	onuTrapTable  = "1.3.6.1.4.1.3902.1082.500.10.2.3.3.1"
	onuColumnName = "2"
	// onuColumnLabel carries the OLT's own position string, "ONU-<port>:<onu>".
	onuColumnLabel = "3"
	// onuColumnSerial carries the serial with a leading field, as "1,ZTEGCA...".
	onuColumnSerial = "18"
)

// onuIdentity is which ONU a trap is about, as the trap itself reported it.
type onuIdentity struct {
	SerialNumber string
	Label        string
	Name         string
	IfIndex      int64
	ONUID        int
}

// identify reads the ONU a trap names.
//
// Nothing is resolved against stored ONTs here. A trap naming an ONU we have no
// row for is evidence — of a device the poller has not seen, or of a serial
// held differently — and turning it into our own terms would destroy that.
func identify(varbinds []Varbind) onuIdentity {
	var identity onuIdentity

	for _, vb := range varbinds {
		column, ifIndex, onuID, ok := splitONUVarbind(vb.OID)
		if !ok {
			continue
		}

		// Every ONU column of a trap carries the same index, so the first one
		// seen settles the position and the rest only fill in labels.
		if identity.IfIndex == 0 {
			identity.IfIndex, identity.ONUID = ifIndex, onuID
		}

		switch column {
		case onuColumnSerial:
			identity.SerialNumber = trailingField(vb.Text)
		case onuColumnLabel:
			identity.Label = vb.Text
		case onuColumnName:
			identity.Name = vb.Text
		}
	}

	return identity
}

// splitONUVarbind reads a varbind OID of the form <table>.<column>.<ifIndex>.<onuID>.
func splitONUVarbind(oid string) (column string, ifIndex int64, onuID int, ok bool) {
	trimmed := strings.TrimPrefix(oid, ".")
	if !strings.HasPrefix(trimmed, onuTrapTable+".") {
		return "", 0, 0, false
	}

	parts := strings.Split(strings.TrimPrefix(trimmed, onuTrapTable+"."), ".")
	if len(parts) != 3 {
		return "", 0, 0, false
	}

	index, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return "", 0, 0, false
	}
	id, err := strconv.Atoi(parts[2])
	if err != nil {
		return "", 0, 0, false
	}
	return parts[0], index, id, true
}

// trailingField takes the serial out of a value like "1,ZTEGC0000001". The
// leading field is the ONU's authentication mode; the serial is what identifies
// the box.
func trailingField(value string) string {
	if value == "" {
		return ""
	}
	parts := strings.Split(value, ",")
	return strings.TrimSpace(parts[len(parts)-1])
}
