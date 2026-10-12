// Package netflow turns IPFIX from a border router into per-minute traffic
// summaries: parse, place on the router's topology, name the far side's AS,
// and add up. Nothing here touches the network or the database.
package netflow

import (
	"encoding/binary"
	"errors"
	"net/netip"
)

// MaxPacketBytes is the most flowd reads of one datagram. IPFIX over UDP
// cannot be longer, and a fixed buffer keeps a sender from choosing the size.
const MaxPacketBytes = 64 << 10

// MaxTemplatesPerExporter bounds one router's template table. RouterOS uses a
// handful; anything near this is a sender inventing IDs to grow memory.
const MaxTemplatesPerExporter = 256

const (
	ipfixVersion     = 10
	messageHeaderLen = 16
	setHeaderLen     = 4
	templateSetID    = 2
	optionsSetID     = 3
	minDataSetID     = 256
)

var (
	// ErrNotIPFIX means the datagram is not an IPFIX message: another NetFlow
	// version, or noise on the port.
	ErrNotIPFIX = errors.New("not an IPFIX message")
	// ErrTruncated means a length inside the message points past its end.
	ErrTruncated = errors.New("truncated IPFIX message")
)

// Flow is one IPv4 flow record, with counts as exported, before sampling.
type Flow struct {
	Src, Dst        netip.Addr
	Ingress, Egress uint32
	Bytes, Packets  uint64
}

// Result is what one message held.
type Result struct {
	Flows []Flow
	// MissingTemplate counts data sets whose template has not arrived yet.
	MissingTemplate int
	// RejectedTemplates counts templates refused for bad lengths or the cap.
	RejectedTemplates int
}

type templateKey struct {
	domain uint32
	id     uint16
}

// Templates is one exporter's template table. It is not safe for concurrent
// use; flowd decodes every packet on the one goroutine reading the socket.
type Templates struct {
	byKey map[templateKey]*template
}

// NewTemplates returns an empty table.
func NewTemplates() *Templates {
	return &Templates{byKey: map[templateKey]*template{}}
}

// Decode reads one message, learning its templates and returning its IPv4
// flows. Data whose template is unknown is dropped, not held: RouterOS
// re-sends templates periodically, and holding data for one that never comes
// is memory the sender controls.
func (t *Templates) Decode(packet []byte) (Result, error) {
	var res Result
	if len(packet) < messageHeaderLen || binary.BigEndian.Uint16(packet) != ipfixVersion {
		return res, ErrNotIPFIX
	}
	length := int(binary.BigEndian.Uint16(packet[2:]))
	if length < messageHeaderLen || length > len(packet) {
		return res, ErrTruncated
	}
	domain := binary.BigEndian.Uint32(packet[12:])
	for rest := packet[messageHeaderLen:length]; len(rest) > 0; {
		if len(rest) < setHeaderLen {
			return res, ErrTruncated
		}
		setLen := int(binary.BigEndian.Uint16(rest[2:]))
		if setLen < setHeaderLen || setLen > len(rest) {
			return res, ErrTruncated
		}
		t.decodeSet(domain, binary.BigEndian.Uint16(rest), rest[setHeaderLen:setLen], &res)
		rest = rest[setLen:]
	}
	return res, nil
}

func (t *Templates) decodeSet(domain uint32, setID uint16, body []byte, res *Result) {
	switch {
	case setID == templateSetID:
		t.learnTemplates(domain, body, false, res)
	case setID == optionsSetID:
		t.learnTemplates(domain, body, true, res)
	case setID >= minDataSetID:
		t.decodeData(templateKey{domain: domain, id: setID}, body, res)
	}
}

func (t *Templates) decodeData(key templateKey, body []byte, res *Result) {
	tmpl, ok := t.byKey[key]
	if !ok {
		res.MissingTemplate++
		return
	}
	res.Flows = append(res.Flows, tmpl.decodeRecords(body)...)
}
