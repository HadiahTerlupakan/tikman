// Package ipfixtest builds IPFIX messages for tests field by field, so a test
// reads as the packet it sends rather than as a hex blob. It is imported only
// by _test.go files.
package ipfixtest

import (
	"bytes"
	"encoding/binary"
	"net/netip"
)

// Field is one template field. A non-zero Enterprise sets the enterprise bit
// and appends the enterprise number, as RFC 7011 section 3.2 lays it out.
type Field struct {
	ID         uint16
	Length     uint16
	Enterprise uint32
}

// VariableLength is the template length that marks a variable-length field.
const VariableLength = 0xFFFF

// FlowFields is an IPv4 flow template with the fields flowd reads plus two it
// skips (protocolIdentifier, destinationTransportPort).
var FlowFields = []Field{
	{ID: 8, Length: 4}, {ID: 12, Length: 4}, {ID: 10, Length: 4}, {ID: 14, Length: 4},
	{ID: 1, Length: 8}, {ID: 2, Length: 8}, {ID: 4, Length: 1}, {ID: 11, Length: 2},
}

// U8 encodes one byte.
func U8(v uint8) []byte { return []byte{v} }

// U16 encodes big-endian.
func U16(v uint16) []byte { return binary.BigEndian.AppendUint16(nil, v) }

// U32 encodes big-endian.
func U32(v uint32) []byte { return binary.BigEndian.AppendUint32(nil, v) }

// U64 encodes big-endian.
func U64(v uint64) []byte { return binary.BigEndian.AppendUint64(nil, v) }

// IPv4 encodes a dotted IPv4 address.
func IPv4(s string) []byte {
	a := netip.MustParseAddr(s).As4()
	return a[:]
}

// Message wraps sets in an IPFIX header for one observation domain.
func Message(domain uint32, sets ...[]byte) []byte {
	body := bytes.Join(sets, nil)
	return bytes.Join([][]byte{U16(10), U16(uint16(16 + len(body))), U32(1_760_000_000), U32(1), U32(domain), body}, nil)
}

// Set wraps a body under a set ID.
func Set(id uint16, body []byte) []byte {
	return bytes.Join([][]byte{U16(id), U16(uint16(4 + len(body))), body}, nil)
}

// TemplateSet is a template set holding one template; no fields withdraws it.
func TemplateSet(templateID uint16, fields ...Field) []byte {
	header := bytes.Join([][]byte{U16(templateID), U16(uint16(len(fields)))}, nil)
	return Set(2, append(header, encodeFields(fields)...))
}

// OptionsTemplateSet is an options template set holding one template whose
// first scopeCount fields are scope fields.
func OptionsTemplateSet(templateID, scopeCount uint16, fields ...Field) []byte {
	header := bytes.Join([][]byte{U16(templateID), U16(uint16(len(fields))), U16(scopeCount)}, nil)
	return Set(3, append(header, encodeFields(fields)...))
}

func encodeFields(fields []Field) []byte {
	var out []byte
	for _, f := range fields {
		id := f.ID
		if f.Enterprise != 0 {
			id |= 0x8000
		}
		out = append(out, U16(id)...)
		out = append(out, U16(f.Length)...)
		if f.Enterprise != 0 {
			out = append(out, U32(f.Enterprise)...)
		}
	}
	return out
}

// DataSet is a data set of records under a template ID.
func DataSet(templateID uint16, records ...[]byte) []byte {
	return Set(templateID, bytes.Join(records, nil))
}

// FlowRecord is one record in FlowFields' layout.
func FlowRecord(src, dst string, ingress, egress uint32, octets, packets uint64) []byte {
	return bytes.Join([][]byte{IPv4(src), IPv4(dst), U32(ingress), U32(egress),
		U64(octets), U64(packets), U8(6), U16(443)}, nil)
}
