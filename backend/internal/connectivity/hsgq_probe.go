package connectivity

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/gosnmp/gosnmp"
)

// Phase 2 of the multi-vendor spec resolves the four HSGQ unknowns that phase 1
// deliberately refused to guess. Three of them cannot be answered from a desk:
// they need raw values from the operator's own OLT. This probe collects that
// evidence and does the correlation that turns it into an answer, so the fix is
// "run this, then change a constant" rather than "read a hex dump and hope".
//
// It is a read-only diagnostic. It never writes to the OLT and is not wired
// into monitoring.

// hsgqProbeTimeout bounds the five sequential walks a probe performs. Generous
// compared with the monitoring path because this runs interactively against one
// OLT, but still bounded so an unresponsive device cannot hang the operator's
// terminal indefinitely.
const hsgqProbeTimeout = 60 * time.Second

// IF-MIB labels. On most OLTs one of these spells the PON position outright
// ("gpon-onu_1/1/1:1"), which resolves the ifIndex layout from the device's own
// naming instead of reverse-engineering its bit packing.
// ifNameOID is IF-MIB's ifName. ifDescrOID lives in driver_hsgq.go, where the
// traffic counters that depend on the same index mapping are read.
const ifNameOID = ".1.3.6.1.2.1.31.1.1.1.1"

// HSGQProbeRow is one ONU exactly as the OLT reported it, before any decoding.
// Raw values are kept because the point of the probe is to inspect what the
// vendor actually sends, not what the current decoders make of it.
type HSGQProbeRow struct {
	IfIndex   uint32
	IfLabel   string // ifName, or ifDescr when ifName is empty
	Name      string
	StatusRaw int64
	HasStatus bool
	RxRaw     int64
	HasRx     bool
	MACRaw    []byte
}

// HSGQSentinel is a raw optical value that cannot be a real GPON level, with
// how many ONUs reported it.
type HSGQSentinel struct {
	Raw   int64
	Count int
}

// HSGQProbeReport answers as much of the four unknowns as the collected data
// supports. Fields left unset mean the data was inconclusive, which is a valid
// outcome: an unresolved unknown is better than a confidently wrong constant.
type HSGQProbeReport struct {
	Rows []HSGQProbeRow

	// OnlineStatusValue is the raw status that means "online", valid only when
	// PolarityConfident. Unknown #2 in the spec.
	OnlineStatusValue int64
	PolarityConfident bool

	// SentinelCandidates are the no-signal values, most frequent first.
	// Unknown #4 in the spec.
	SentinelCandidates []HSGQSentinel
}

// ProbeHSGQ reads the HSGQ tables needed to resolve the phase 1 unknowns.
func ProbeHSGQ(ctx context.Context, ipAddress, community string, snmpPort int) (HSGQProbeReport, error) {
	ctx, cancel := context.WithTimeout(ctx, hsgqProbeTimeout)
	defer cancel()

	client, err := newSNMPClientWithContext(ctx, ipAddress, community, snmpPort)
	if err != nil {
		return HSGQProbeReport{}, err
	}
	defer func() { _ = client.Conn.Close() }()

	rows := make(map[uint32]*HSGQProbeRow)
	at := func(ifIndex uint32) *HSGQProbeRow {
		row, ok := rows[ifIndex]
		if !ok {
			row = &HSGQProbeRow{IfIndex: ifIndex}
			rows[ifIndex] = row
		}
		return row
	}

	// The status table enumerates the ONUs; everything else annotates them.
	if err := hsgqWalkColumn(client, hsgqONUStatus, "", func(ifIndex uint32, pdu gosnmp.SnmpPDU) {
		if raw, ok := toInt64(pdu.Value); ok {
			row := at(ifIndex)
			row.StatusRaw, row.HasStatus = raw, true
		}
	}); err != nil {
		return HSGQProbeReport{}, fmt.Errorf("HSGQ status walk failed: %w", err)
	}

	// An empty status table is the headline result, not an empty report: it
	// means these third-party OIDs do not match the operator's firmware, which
	// is the risk the spec called out.
	if len(rows) == 0 {
		return HSGQProbeReport{}, fmt.Errorf("HSGQ status table %s returned no rows, so these OIDs do not match this firmware: %w", hsgqONUStatus, ErrUnsupported)
	}

	// Optical level, ONU end of the row. Needed to correlate against status.
	if err := hsgqWalkColumn(client, hsgqRxPower, hsgqONUTail, func(ifIndex uint32, pdu gosnmp.SnmpPDU) {
		if raw, ok := toInt64(pdu.Value); ok {
			row := at(ifIndex)
			row.RxRaw, row.HasRx = raw, true
		}
	}); err != nil {
		return HSGQProbeReport{}, fmt.Errorf("HSGQ rx power walk failed: %w", err)
	}

	// MAC is the ONU identity on EPON. Kept raw so the CLI can show the bytes
	// alongside the decoded form.
	_ = hsgqWalkColumn(client, hsgqONUMAC, "", func(ifIndex uint32, pdu gosnmp.SnmpPDU) {
		if raw, ok := pdu.Value.([]byte); ok {
			at(ifIndex).MACRaw = raw
		}
	})

	_ = hsgqWalkColumn(client, hsgqONUName, "", func(ifIndex uint32, pdu gosnmp.SnmpPDU) {
		if name := printableText(pdu.Value); name != "" {
			at(ifIndex).Name = name
		}
	})

	// IF-MIB labels resolve unknown #1. ifDescr first so a present ifName wins.
	for _, oid := range []string{ifDescrOID, ifNameOID} {
		_ = hsgqWalkColumn(client, oid, "", func(ifIndex uint32, pdu gosnmp.SnmpPDU) {
			if _, known := rows[ifIndex]; !known {
				return // an uplink or PON port, not one of our ONUs
			}
			if label := printableText(pdu.Value); label != "" {
				at(ifIndex).IfLabel = label
			}
		})
	}

	flat := make([]HSGQProbeRow, 0, len(rows))
	for _, row := range rows {
		flat = append(flat, *row)
	}

	return analyseHSGQProbe(flat), nil
}

// analyseHSGQProbe turns collected rows into a report. Pure, so the inference
// is tested without an OLT.
func analyseHSGQProbe(rows []HSGQProbeRow) HSGQProbeReport {
	sort.Slice(rows, func(i, j int) bool { return rows[i].IfIndex < rows[j].IfIndex })

	online, confident := inferHSGQOnlineStatus(rows)

	return HSGQProbeReport{
		Rows:               rows,
		OnlineStatusValue:  online,
		PolarityConfident:  confident,
		SentinelCandidates: hsgqSentinelCandidates(rows),
	}
}

// inferHSGQOnlineStatus decides which raw status means "online" by correlating
// status against optical presence: an ONU reporting a plausible rx level has
// light on its fibre and is therefore up.
//
// It reports confident only when every row of exactly one status value has a
// reading and every row of the others has none. A partial correlation returns
// not-confident, because a mixed group is how an inverted polarity gets locked
// in - and inverted polarity reports a healthy network during an outage.
func inferHSGQOnlineStatus(rows []HSGQProbeRow) (int64, bool) {
	type tally struct{ total, withSignal int }

	byStatus := make(map[int64]*tally)
	for _, row := range rows {
		if !row.HasStatus {
			continue
		}
		counts, ok := byStatus[row.StatusRaw]
		if !ok {
			counts = &tally{}
			byStatus[row.StatusRaw] = counts
		}
		counts.total++
		if row.HasRx && hsgqDecodePower(row.RxRaw) != nil {
			counts.withSignal++
		}
	}

	// One status value observed proves nothing: every ONU being up (or the OID
	// returning a constant) looks identical here.
	if len(byStatus) < 2 {
		return 0, false
	}

	var online int64
	matches := 0
	for status, counts := range byStatus {
		switch {
		case counts.withSignal == counts.total:
			online, matches = status, matches+1
		case counts.withSignal == 0:
			// Consistent with offline; nothing to conclude on its own.
		default:
			return 0, false // mixed group, correlation is not clean
		}
	}

	if matches != 1 {
		return 0, false
	}

	return online, true
}

// hsgqSentinelCandidates lists raw rx values that cannot be real GPON levels,
// most frequent first. A vendor's no-signal sentinel shows up as a single value
// repeated across every dark ONU, which is what distinguishes it from noise.
func hsgqSentinelCandidates(rows []HSGQProbeRow) []HSGQSentinel {
	counts := make(map[int64]int)
	for _, row := range rows {
		if row.HasRx && hsgqDecodePower(row.RxRaw) == nil {
			counts[row.RxRaw]++
		}
	}

	if len(counts) == 0 {
		return nil
	}

	candidates := make([]HSGQSentinel, 0, len(counts))
	for raw, count := range counts {
		candidates = append(candidates, HSGQSentinel{Raw: raw, Count: count})
	}
	// Frequency first, then value, so equal counts still order deterministically.
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Count != candidates[j].Count {
			return candidates[i].Count > candidates[j].Count
		}
		return candidates[i].Raw < candidates[j].Raw
	})

	return candidates
}

// MACRendering returns the decoded MAC and its raw bytes. Unlike a GPON serial
// there is no byte-order ambiguity here, so one rendering is enough; the raw
// bytes are still shown because a column that is not actually a MAC shows up as
// a wrong length rather than as plausible-looking hex.
func (r HSGQProbeRow) MACRendering() (mac, hex string) {
	for _, b := range r.MACRaw {
		hex += fmt.Sprintf("%02X ", b)
	}

	return hsgqFormatMAC(r.MACRaw), hex
}

// IfIndexBreakdown renders the ifIndex in the forms a packed layout is visible
// in. Paired with IfLabel it is what resolves unknown #1: the label says which
// PON the ONU is on, the hex shows where that number sits in the index.
func (r HSGQProbeRow) IfIndexBreakdown() string {
	return fmt.Sprintf("%d (0x%08X, bytes %d.%d.%d.%d)",
		r.IfIndex, r.IfIndex,
		byte(r.IfIndex>>24), byte(r.IfIndex>>16), byte(r.IfIndex>>8), byte(r.IfIndex))
}
