package netflow

import (
	"encoding/binary"
	"net/netip"
)

// Information elements flowd reads (IANA IPFIX registry).
const (
	ieOctetDeltaCount        = 1
	iePacketDeltaCount       = 2
	ieSourceIPv4Address      = 8
	ieIngressInterface       = 10
	ieDestinationIPv4Address = 12
	ieEgressInterface        = 14
)

const (
	enterpriseBit  = 0x8000
	variableLength = 0xFFFF
	// longVarLenMarker as a variable-length field's first byte means the next
	// two bytes carry the length (RFC 7011 section 7).
	longVarLenMarker = 255
	fieldSpecLen     = 4
	enterpriseLen    = 4
)

type templateField struct {
	id     uint16 // 0 for an enterprise field: never one flowd reads
	length uint16
}

type template struct {
	fields []templateField
	// usable is false for options templates and for templates without IPv4
	// addresses and a byte count (RouterOS's IPv6 template); their records
	// are skipped.
	usable bool
}

// learnTemplates reads every template record of a set. A malformed record
// ends the set: its length is unknown, so nothing after it can be found.
func (t *Templates) learnTemplates(domain uint32, body []byte, options bool, res *Result) {
	header := 4
	if options {
		header = 6
	}
	for len(body) >= header {
		id := binary.BigEndian.Uint16(body)
		if id < minDataSetID {
			return // set padding
		}
		fields, used, ok := readFields(body[header:], int(binary.BigEndian.Uint16(body[2:])))
		if !ok {
			res.RejectedTemplates++
			return
		}
		t.store(templateKey{domain: domain, id: id}, fields, !options, res)
		body = body[header+used:]
	}
}

func readFields(b []byte, count int) ([]templateField, int, bool) {
	if count*fieldSpecLen > len(b) {
		return nil, 0, false
	}
	fields := make([]templateField, 0, count)
	off := 0
	for i := 0; i < count; i++ {
		if off+fieldSpecLen > len(b) {
			return nil, 0, false
		}
		id, length := binary.BigEndian.Uint16(b[off:]), binary.BigEndian.Uint16(b[off+2:])
		off += fieldSpecLen
		if id&enterpriseBit != 0 {
			off += enterpriseLen
			id = 0
		}
		if length == 0 || off > len(b) {
			return nil, 0, false
		}
		fields = append(fields, templateField{id: id, length: length})
	}
	return fields, off, true
}

// store learns, replaces or (with no fields) withdraws a template. A rejected
// template also forgets the old layout under its key: decoding new data with
// a layout the router no longer uses would misread every record.
func (t *Templates) store(key templateKey, fields []templateField, data bool, res *Result) {
	if len(fields) == 0 {
		delete(t.byKey, key)
		return
	}
	tmpl, ok := newTemplate(fields, data)
	if !ok {
		delete(t.byKey, key)
		res.RejectedTemplates++
		return
	}
	if _, known := t.byKey[key]; !known && len(t.byKey) >= MaxTemplatesPerExporter {
		res.RejectedTemplates++
		return
	}
	t.byKey[key] = tmpl
}

// lengthValid accepts what RFC 7011 allows for the elements flowd reads:
// addresses are exactly four bytes, counters and indexes may use reduced-size
// encoding. Variable length fails both, as it should.
func lengthValid(f templateField) bool {
	switch f.id {
	case ieSourceIPv4Address, ieDestinationIPv4Address:
		return f.length == 4
	case ieOctetDeltaCount, iePacketDeltaCount:
		return f.length <= 8
	case ieIngressInterface, ieEgressInterface:
		return f.length <= 4
	}
	return true
}

func newTemplate(fields []templateField, data bool) (*template, bool) {
	seen := map[uint16]bool{}
	for _, f := range fields {
		if !lengthValid(f) {
			return nil, false
		}
		seen[f.id] = true
	}
	usable := data && seen[ieSourceIPv4Address] && seen[ieDestinationIPv4Address] && seen[ieOctetDeltaCount]
	return &template{fields: fields, usable: usable}, true
}

// decodeRecords reads records until the set ends. A record running past the
// set ends it; bytes too short for a record are padding.
func (tmpl *template) decodeRecords(body []byte) []Flow {
	if !tmpl.usable {
		return nil
	}
	var flows []Flow
	for len(body) > 0 {
		flow, used, ok := tmpl.decodeRecord(body)
		if !ok {
			break
		}
		flows = append(flows, flow)
		body = body[used:]
	}
	return flows
}

func (tmpl *template) decodeRecord(b []byte) (Flow, int, bool) {
	var flow Flow
	off := 0
	for _, field := range tmpl.fields {
		n, header, ok := fieldLength(field, b[off:])
		if !ok || off+header+n > len(b) {
			return Flow{}, 0, false
		}
		flow.set(field.id, b[off+header:off+header+n])
		off += header + n
	}
	return flow, off, true
}

func fieldLength(field templateField, b []byte) (n, header int, ok bool) {
	if field.length != variableLength {
		return int(field.length), 0, true
	}
	if len(b) < 1 {
		return 0, 0, false
	}
	if b[0] < longVarLenMarker {
		return int(b[0]), 1, true
	}
	if len(b) < 3 {
		return 0, 0, false
	}
	return int(binary.BigEndian.Uint16(b[1:])), 3, true
}

// set stores one field. Address lengths were checked when the template was
// learned, so the conversion to [4]byte cannot fail.
func (f *Flow) set(id uint16, v []byte) {
	switch id {
	case ieSourceIPv4Address:
		f.Src = netip.AddrFrom4([4]byte(v))
	case ieDestinationIPv4Address:
		f.Dst = netip.AddrFrom4([4]byte(v))
	case ieIngressInterface:
		f.Ingress = uint32(beUint(v))
	case ieEgressInterface:
		f.Egress = uint32(beUint(v))
	case ieOctetDeltaCount:
		f.Bytes = beUint(v)
	case iePacketDeltaCount:
		f.Packets = beUint(v)
	}
}

func beUint(v []byte) uint64 {
	var n uint64
	for _, b := range v {
		n = n<<8 | uint64(b)
	}
	return n
}
