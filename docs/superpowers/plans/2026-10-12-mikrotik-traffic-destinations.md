# MikroTik Traffic Destinations (NetFlow/IPFIX) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Show, per border router, where customer traffic goes — destination AS/organisation, from which POP, over which upstream, in which direction — from one minute to years back, plus the top destination IPs of the last 7 days.

**Architecture:** A new `flowd` service (its own container, UDP 2055) parses IPFIX from RouterOS v7 with a stdlib-only parser, places each flow on the router's topology (upstream by ifIndex of BGP-facing interfaces, POP by longest-prefix match over interface addresses and statically routed blocks), names the far side's AS from the iptoasn.com dataset, and writes one summary row per minute per (router, POP, upstream, direction, ASN) — top 20 ASNs plus ASN 0 — into a TimescaleDB hypertable with 5-minute, hourly and daily continuous aggregates. Pure logic lives in `internal/netflow`; DB-backed pieces in `internal/services`; the API and a new **Tujuan Trafik** tab read the rollups with a resolution chosen from the range.

**Tech Stack:** Go 1.26 stdlib (`net/netip`, `compress/gzip`, `encoding/binary`, `sort`, `net/http`), GORM, Gin, PostgreSQL/TimescaleDB, React + TypeScript + Ant Design + React Query + recharts, Vitest.

**Spec:** `docs/superpowers/specs/2026-10-12-mikrotik-traffic-destinations-design.md`

## Global Constraints

- Collector `flowd`: UDP **2055**, IPFIX **RFC 7011** only (version 10), its own container built from `Dockerfile.worker`, **not** `network_mode: service:api`; publishes `2055/udp` in `docker-compose.yml` and `docker-compose.vps.yml`; a named volume caches the ASN dataset.
- IPFIX fields read: `sourceIPv4Address` (8), `destinationIPv4Address` (12), `ingressInterface` (10), `egressInterface` (14), `octetDeltaCount` (1), `packetDeltaCount` (2). `protocolIdentifier` (4) and `destinationTransportPort` (11) are recognised in templates and skipped (nothing stores them — see the ruling in Task 1).
- Limits: UDP datagram read at most **64 KiB**; at most **256 templates per router**; template field lengths validated; a record running past its set is dropped; at most **50,000 aggregation keys per router per minute** (excess folds into ASN 0 and is logged as `dropped_keys`); at most **100,000 distinct destination IPs per POP per 5 minutes** in the top-IP counter (excess counts only in the minute totals).
- Storage: per (router, POP, direction, minute) only the **top 20 ASNs** get their own rows; the rest fold into **`asn = 0`** ("Lainnya"); totals stay exact. Top IPs: **top 100 per (router, POP, direction, 5 min)**.
- Retention: `router_flow_minute` **7 days**; `router_flow_5min` **90 days**; `router_flow_hourly` **2 years**; `router_flow_daily` **forever**; `router_flow_top_ips` **7 days**. Rollups are continuous aggregates with `timescaledb.materialized_only = false`.
- Resolution: the finest source whose retention still covers `from` (1 min ≤ 7 days, 5 min ≤ 90 days, 1 h ≤ 2 years, else daily), with the step widened in whole multiples of the source's step so one series has **≤ 1,500 points**.
- `flow_sampling`: default **100**, range **1–10000**; bytes and packets are multiplied by it.
- **IPv4 only.**
- ASN dataset: `https://iptoasn.com/data/ip2asn-v4.tsv.gz`, downloaded at start when the local copy is missing or older than 24 h, and every 24 h; atomic swap of the in-memory table; a failed or invalid download keeps the previous table and file. Names also stored in `asn_names`.
- Access: all roles (Admin, Technician, Viewer) read every flow endpoint; only Admin changes `flow_enabled` / `flow_sampling` (via `PUT /api/v1/routers/:id`).
- Migration number **58** (`backend/migrations/58_mikrotik_traffic_flows.sql`).
- **No new Go or npm dependencies.** IPFIX parsing, gzip TSV loading and longest-prefix match are stdlib (`net/netip`, `compress/gzip`, `encoding/binary`, `sort`).
- **PUBLIC REPO:** every fixture, test, doc and example uses only 192.0.2.0/24, 198.51.100.0/24, 203.0.113.0/24, 127.0.0.1, `0.0.0.0/0` as "default route", 2001:db8::/32, and documentation AS numbers 64496–64511 (plus RFC 5398's 65536–65551 where a test needs more than 16 distinct ASNs — see the ruling in Task 4). No real provider, POP or router names or IPs.
- Files ≤ 350 lines, functions ≤ 50 lines, ≤ 3 nesting levels (CLAUDE.md). Comments explain why. Exported Go identifiers get short doc comments. No TODOs.
- UI copy is Indonesian, matching the existing router pages; reuse `TrafficRangePicker`, `trafficRange.ts` and `TrafficChart`.
- `internal/api` is tested with `-run` filters only: its WireGuard tests fail environmentally on the dev Mac while `ppp0` is up.
- Postgres-only tests use `TEST_POSTGRES_DSN=postgres://tikman:testpw@localhost:5439/tikman_test?sslmode=disable` (the repo's `freshPostgres` in `internal/database`, `setupPostgresTestDB` in `internal/services`). Service tests use SQLite unless they need TimescaleDB.

## Review Focus

1. **Data before its template / router reboot.** After flowd or the router restarts, data sets arrive before the template that describes them, and a rebooted router may re-send a template ID with another layout. Expected: early data is dropped and counted (never misread), and the newest layout wins. → Task 1 `TestDecodeDropsDataUntilItsTemplateArrives`, `TestDecodeReplacesATemplateResentWithAnotherLayout`.
2. **ifIndex renumbered after a router reboot.** RouterOS renumbers ifIndex; flowd must follow the interfaces poll's stored number at its next mapping reload, and must never attribute traffic to an upstream through a stale index (a disabled interface keeps its last, possibly reassigned, number; two upstreams claiming one index). → Task 7 `TestLoadFollowsARenumberedIfIndex`, `TestLoadNeverGuessesAnUpstreamFromAStaleIfIndex`.
3. **Flows before discovery has upstreams or prefixes** (flow switched on right after adding the router). Expected: nothing crashes; flows that cannot be placed are dropped, flows to unknown local addresses count as POP "Lainnya", and the router is still marked as heard from so the Router tab shows IPFIX is arriving. → Task 3 `TestClassifyCountsAnAddressInNoPOPAsOther`, Task 8 `TestHandleMarksARouterHeardEvenWhenNoFlowCanBePlaced`.
4. **Database slow or down while flows arrive.** Expected: one retry with the next minute, then the batch is dropped and logged; memory never grows with the outage; a minute that was stored is never stored twice by a retry. → Task 8 `TestFlushDropsABatchThatFailsTwice`, `TestRetryDoesNotStoreAMinuteTwice`.
5. **Enterprise and variable-length fields** (a RouterOS update adds vendor fields). Expected: they are skipped by length and never overwrite or shift the standard fields, including an enterprise field whose number equals `sourceIPv4Address`. → Task 1 `TestDecodeSkipsEnterpriseAndVariableLengthFields`.

---

## File Structure

Backend (`backend/`):

| File | Responsibility |
|---|---|
| `internal/netflow/ipfix.go` (new) | message/set walking, `Templates`, `Decode`, `Result`, limits, errors |
| `internal/netflow/ipfix_template.go` (new) | template parsing and validation, record decoding |
| `internal/netflow/ipfixtest/ipfixtest.go` (new) | test-only builder of IPFIX messages with `encoding/binary` (used by `netflow` and `cmd/flowd` tests) |
| `internal/netflow/asn.go` (new) | `ASNTable`: TSV parse, gzip load with inflate cap, binary-search lookup |
| `internal/netflow/asnfetch.go` (new) | `ASNFetcher`: download to temp file, validate, atomic rename, staleness |
| `internal/netflow/prefixes.go` (new) | `PrefixTable`: longest-prefix match over arbitrary IPv4 lengths |
| `internal/netflow/classify.go` (new) | `RouterMap.Classify`: direction, upstream, POP, sampling |
| `internal/netflow/aggregate.go` (new) | `Aggregator`: minute keys with cap, top-20 ASN fold, 5-min top-100 IPs with cap |
| `internal/netflow/*_test.go` (new) | tests |
| `internal/models/flow.go` (new) | `RouterFlowMinute`, `RouterFlowTopIP`, `ASNName`, `RouterRoutedPrefix` |
| `internal/models/mikrotik.go`, `models.go` | flow columns on `MikrotikRouter`; AutoMigrate list |
| `migrations/58_mikrotik_traffic_flows.sql` (new) | checks, FKs, hypertables, rollups, retention |
| `internal/database/migrations_flow_postgres_test.go` (new) | migration 58 on Postgres |
| `internal/mikrotik/types.go`, `client.go`, `rest.go`, `api.go` (+ tests) | `Route`, `StaticRoutes`, `List(path, out, where...)` filter on both transports |
| `internal/services/router_routes.go` (new) | static route → interface mapping |
| `internal/services/router_discovery.go`, `router_poll.go` | discovery reads routes and refills `router_routed_prefixes` |
| `internal/services/flow_mapping.go` (new) | `FlowMappingService.Load`: per-router `netflow.RouterMap` |
| `internal/services/flow_store.go` (new) | `FlowStore`: minute rows, top IPs, last seen, ASN names |
| `internal/services/flow_resolution.go` (new) | source/step selection |
| `internal/services/flow_query.go` (new) | `FlowQueryService`, `FlowFilter`, `Destinations`, ASN names |
| `internal/services/flow_summary.go` (new) | `Summary`, `POPs` |
| `internal/services/flow_top_ips.go` (new) | `TopIPs`, `Status` |
| `internal/services/router_service.go` | `flow_enabled` / `flow_sampling` on update |
| `internal/services/*_test.go` | tests (SQLite and Postgres) |
| `internal/api/router_flow_handler.go` (new) | flow read handlers and filter parsing |
| `internal/api/router_dto.go`, `router_handler.go`, `router_handler_reads.go`, `router_handlers.go`, `router_routes_mikrotik.go` | PUT flow fields, shared range parsing, wiring, routes |
| `internal/api/router_flow_handler_test.go` (new), `router_handler_test.go`, `router_rbac_test.go` | tests |
| `cmd/flowd/main.go` (new) | wiring: config, DB, socket, tickers, signals |
| `cmd/flowd/collector.go` (new) | source filter, decode, classify, aggregate |
| `cmd/flowd/writer.go` (new) | retry-once writer |
| `cmd/flowd/asn.go` (new) | ASN table lifecycle |
| `cmd/flowd/*_test.go` (new) | tests |
| `Dockerfile.worker` | builds and ships `flowd` |

Repository root: `docker-compose.yml`, `docker-compose.vps.yml` (flowd service, 2055/udp, `asn_data` volume); `CLAUDE.md` (entry point, deploy note); `docs/operator_guide.md` (NetFlow section).

Frontend (`frontend/src/`):

| File | Responsibility |
|---|---|
| `domain/entities/RouterFlow.ts` (new), `index.ts` | flow response types |
| `domain/entities/MikrotikRouter.ts` | flow fields on router and update DTO |
| `infrastructure/http/endpoints.ts` | `MIKROTIK_ROUTER_FLOWS` |
| `infrastructure/repositories/RouterFlowRepository.ts` (new), `index.ts` | flow reads |
| `application/hooks/useRouterFlows.ts` (new) | React Query hooks |
| `presentation/components/routers/routerFlows.ts` (+ test, new) | presets, chart pivot, labels, top-IP availability, staleness |
| `presentation/components/routers/TrafficRangePicker.tsx` | optional `presets` prop |
| `presentation/components/routers/TrafficChart.tsx` | palette long enough for 11 series |
| `presentation/components/routers/FlowDestinationsTab.tsx` (+ test, new) | the tab |
| `presentation/components/routers/FlowFilters.tsx`, `FlowSummaryTable.tsx`, `FlowPopTable.tsx`, `FlowTopIpsPanel.tsx` (new) | tab parts |
| `presentation/components/routers/RouterFlowStatus.tsx` (+ test, new) | flow status on the Router tab |
| `presentation/components/routers/RouterInfoTab.tsx`, `RouterFormFields.tsx`, `RouterFormModal.tsx` (+ test), `useRouterTestSteps.ts` | status, NetFlow toggle and sampling |
| `presentation/pages/RouterDetailPage.tsx` | new tab |

---

### Task 1: IPFIX parser

**Files:**
- Create: `backend/internal/netflow/ipfix.go`
- Create: `backend/internal/netflow/ipfix_template.go`
- Create: `backend/internal/netflow/ipfixtest/ipfixtest.go`
- Test: `backend/internal/netflow/ipfix_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces: `netflow.MaxPacketBytes` (`64 << 10`), `netflow.MaxTemplatesPerExporter` (256), `netflow.ErrNotIPFIX`, `netflow.ErrTruncated`, `type Flow struct { Src, Dst netip.Addr; Ingress, Egress uint32; Bytes, Packets uint64 }`, `type Result struct { Flows []Flow; MissingTemplate, RejectedTemplates int }`, `type Templates`, `func NewTemplates() *Templates`, `func (t *Templates) Decode(packet []byte) (Result, error)`. Test helper package `ipfixtest`: `Field{ID, Length uint16; Enterprise uint32}`, `VariableLength`, `FlowFields`, `U8/U16/U32/U64`, `IPv4(string) []byte`, `Message(domain uint32, sets ...[]byte)`, `Set(id, body)`, `TemplateSet(id, fields...)`, `OptionsTemplateSet(id, scopeCount, fields...)`, `DataSet(id, records...)`, `FlowRecord(src, dst string, ingress, egress uint32, octets, packets uint64)`.

Ruling recorded here: `protocolIdentifier` and `destinationTransportPort` are recognised as ordinary template fields and skipped by length. No table or view stores them, so decoding them into `Flow` would be dead code; adding them later is two `case` lines.

- [ ] **Step 1: Write the test builder**

`backend/internal/netflow/ipfixtest/ipfixtest.go`:

```go
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
```

- [ ] **Step 2: Write the failing tests**

`backend/internal/netflow/ipfix_test.go`:

```go
package netflow

import (
	"bytes"
	"errors"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/netflow/ipfixtest"
)

const flowTemplate = 256

func decodeAll(t *testing.T, tmpl *Templates, messages ...[]byte) Result {
	t.Helper()
	var total Result
	for _, m := range messages {
		res, err := tmpl.Decode(m)
		require.NoError(t, err)
		total.Flows = append(total.Flows, res.Flows...)
		total.MissingTemplate += res.MissingTemplate
		total.RejectedTemplates += res.RejectedTemplates
	}
	return total
}

func oneFlow() []byte {
	return ipfixtest.FlowRecord("198.51.100.7", "203.0.113.5", 10, 20, 1500, 3)
}

func TestDecodeReadsFlowsAfterTheirTemplate(t *testing.T) {
	msg := ipfixtest.Message(1,
		ipfixtest.TemplateSet(flowTemplate, ipfixtest.FlowFields...),
		ipfixtest.DataSet(flowTemplate, oneFlow(),
			ipfixtest.FlowRecord("203.0.113.6", "192.0.2.80", 20, 11, 400, 2)))

	res := decodeAll(t, NewTemplates(), msg)

	require.Len(t, res.Flows, 2)
	assert.Equal(t, Flow{Src: netip.MustParseAddr("198.51.100.7"), Dst: netip.MustParseAddr("203.0.113.5"),
		Ingress: 10, Egress: 20, Bytes: 1500, Packets: 3}, res.Flows[0])
	assert.Equal(t, uint32(11), res.Flows[1].Egress)
}

// Review Focus 1: after flowd or the router restarts, data arrives before the
// template describing it. It is dropped and counted, never misread, and read
// normally once the template comes.
func TestDecodeDropsDataUntilItsTemplateArrives(t *testing.T) {
	tmpl := NewTemplates()
	data := ipfixtest.Message(1, ipfixtest.DataSet(flowTemplate, oneFlow()))

	early := decodeAll(t, tmpl, data)
	assert.Empty(t, early.Flows)
	assert.Equal(t, 1, early.MissingTemplate)

	late := decodeAll(t, tmpl, ipfixtest.Message(1, ipfixtest.TemplateSet(flowTemplate, ipfixtest.FlowFields...)), data)
	assert.Len(t, late.Flows, 1)
	assert.Zero(t, late.MissingTemplate)
}

// Review Focus 1: a rebooted router re-sends a template ID with its fields in
// another order and other widths; the new layout replaces the old one.
func TestDecodeReplacesATemplateResentWithAnotherLayout(t *testing.T) {
	tmpl := NewTemplates()
	decodeAll(t, tmpl, ipfixtest.Message(1, ipfixtest.TemplateSet(flowTemplate, ipfixtest.FlowFields...)))

	swapped := []ipfixtest.Field{{ID: 12, Length: 4}, {ID: 8, Length: 4}, {ID: 1, Length: 4},
		{ID: 10, Length: 2}, {ID: 14, Length: 2}}
	record := bytes.Join([][]byte{ipfixtest.IPv4("203.0.113.5"), ipfixtest.IPv4("198.51.100.7"),
		ipfixtest.U32(900), ipfixtest.U16(10), ipfixtest.U16(20)}, nil)
	res := decodeAll(t, tmpl, ipfixtest.Message(1,
		ipfixtest.TemplateSet(flowTemplate, swapped...), ipfixtest.DataSet(flowTemplate, record)))

	require.Len(t, res.Flows, 1)
	assert.Equal(t, netip.MustParseAddr("198.51.100.7"), res.Flows[0].Src)
	assert.Equal(t, netip.MustParseAddr("203.0.113.5"), res.Flows[0].Dst)
	assert.Equal(t, uint64(900), res.Flows[0].Bytes, "a four-byte counter is reduced-size encoding")
	assert.Equal(t, uint32(10), res.Flows[0].Ingress)
}

func TestDecodeKeepsTemplatesPerObservationDomain(t *testing.T) {
	tmpl := NewTemplates()
	decodeAll(t, tmpl, ipfixtest.Message(1, ipfixtest.TemplateSet(flowTemplate, ipfixtest.FlowFields...)))

	res := decodeAll(t, tmpl, ipfixtest.Message(2, ipfixtest.DataSet(flowTemplate, oneFlow())))

	assert.Empty(t, res.Flows)
	assert.Equal(t, 1, res.MissingTemplate)
}

// Review Focus 5: a firmware update adds an enterprise field numbered like
// sourceIPv4Address and a variable-length string, once short and once in the
// three-byte length form. Neither may shift or overwrite the fields read.
func TestDecodeSkipsEnterpriseAndVariableLengthFields(t *testing.T) {
	fields := append([]ipfixtest.Field{
		{ID: 8, Length: 4, Enterprise: 32473},
		{ID: 82, Length: ipfixtest.VariableLength},
	}, ipfixtest.FlowFields...)
	short := bytes.Join([][]byte{ipfixtest.IPv4("192.0.2.200"), {3}, []byte("eth"), oneFlow()}, nil)
	long := bytes.Join([][]byte{ipfixtest.IPv4("192.0.2.201"), {255}, ipfixtest.U16(300),
		bytes.Repeat([]byte("x"), 300), ipfixtest.FlowRecord("198.51.100.8", "203.0.113.6", 10, 20, 700, 1)}, nil)

	res := decodeAll(t, NewTemplates(), ipfixtest.Message(1,
		ipfixtest.TemplateSet(flowTemplate, fields...), ipfixtest.DataSet(flowTemplate, short, long)))

	require.Len(t, res.Flows, 2)
	assert.Equal(t, netip.MustParseAddr("198.51.100.7"), res.Flows[0].Src, "the enterprise field is not sourceIPv4Address")
	assert.Equal(t, netip.MustParseAddr("198.51.100.8"), res.Flows[1].Src)
	assert.Equal(t, uint64(700), res.Flows[1].Bytes)
}

func TestDecodeRefusesWhatIsNotWholeIPFIX(t *testing.T) {
	valid := ipfixtest.Message(1, ipfixtest.TemplateSet(flowTemplate, ipfixtest.FlowFields...))
	v9 := bytes.Clone(valid)
	v9[1] = 9
	longer := bytes.Clone(valid)
	longer[3] += 10
	setPastEnd := ipfixtest.Message(1, ipfixtest.Set(flowTemplate, []byte{1, 2}))
	setPastEnd[19] += 10

	cases := map[string]struct {
		packet []byte
		want   error
	}{
		"short header":                {valid[:10], ErrNotIPFIX},
		"NetFlow v9":                  {v9, ErrNotIPFIX},
		"length past the datagram":    {longer, ErrTruncated},
		"set past the message":        {setPastEnd, ErrTruncated},
		"set shorter than its header": {ipfixtest.Message(1, []byte{0x01, 0x00, 0x00, 0x02}), ErrTruncated},
	}
	for name, c := range cases {
		_, err := NewTemplates().Decode(c.packet)
		assert.True(t, errors.Is(err, c.want), "%s: %v", name, err)
	}
}

func TestDecodeDropsTheHalfRecordAtTheEndOfASet(t *testing.T) {
	cut := append(bytes.Clone(oneFlow()), oneFlow()[:20]...)

	res := decodeAll(t, NewTemplates(), ipfixtest.Message(1,
		ipfixtest.TemplateSet(flowTemplate, ipfixtest.FlowFields...), ipfixtest.Set(flowTemplate, cut)))

	assert.Len(t, res.Flows, 1)
}

func TestDecodeCapsTemplatesPerExporter(t *testing.T) {
	tmpl := NewTemplates()
	var sets [][]byte
	for i := 0; i <= MaxTemplatesPerExporter; i++ {
		sets = append(sets, ipfixtest.TemplateSet(uint16(minDataSetID+i), ipfixtest.FlowFields...))
	}

	res := decodeAll(t, tmpl, ipfixtest.Message(1, sets...))
	assert.Equal(t, 1, res.RejectedTemplates)

	again := decodeAll(t, tmpl, ipfixtest.Message(1, ipfixtest.TemplateSet(minDataSetID, ipfixtest.FlowFields...)))
	assert.Zero(t, again.RejectedTemplates, "re-sending a known template is not a new one")
}

func TestDecodeRejectsTemplatesWithImpossibleLengths(t *testing.T) {
	cases := map[string]ipfixtest.Field{
		"16-byte IPv4 address":    {ID: 8, Length: 16},
		"9-byte counter":          {ID: 1, Length: 9},
		"variable-length address": {ID: 12, Length: ipfixtest.VariableLength},
		"zero-length field":       {ID: 99, Length: 0},
	}
	for name, bad := range cases {
		tmpl := NewTemplates()
		fields := append([]ipfixtest.Field{bad}, ipfixtest.FlowFields...)
		res := decodeAll(t, tmpl, ipfixtest.Message(1, ipfixtest.TemplateSet(flowTemplate, fields...)))
		assert.Equal(t, 1, res.RejectedTemplates, name)

		data := decodeAll(t, tmpl, ipfixtest.Message(1, ipfixtest.DataSet(flowTemplate, oneFlow())))
		assert.Equal(t, 1, data.MissingTemplate, name)
	}
}

// RouterOS sends IPv6 flows under their own template and describes itself in
// options records. Neither is IPv4 traffic, and neither is a missing template.
func TestDecodeSkipsIPv6AndOptionsRecordsQuietly(t *testing.T) {
	ipv6 := []ipfixtest.Field{{ID: 27, Length: 16}, {ID: 28, Length: 16}, {ID: 1, Length: 8}}
	ipv6Record := bytes.Join([][]byte{make([]byte, 32), ipfixtest.U64(100)}, nil)
	options := []ipfixtest.Field{{ID: 149, Length: 4}, {ID: 34, Length: 4}}
	optionsRecord := bytes.Join([][]byte{ipfixtest.U32(1), ipfixtest.U32(100)}, nil)

	res := decodeAll(t, NewTemplates(), ipfixtest.Message(1,
		ipfixtest.TemplateSet(257, ipv6...), ipfixtest.OptionsTemplateSet(258, 1, options...),
		ipfixtest.DataSet(257, ipv6Record), ipfixtest.DataSet(258, optionsRecord)))

	assert.Empty(t, res.Flows)
	assert.Zero(t, res.MissingTemplate)
	assert.Zero(t, res.RejectedTemplates)
}

func TestDecodeForgetsAWithdrawnTemplate(t *testing.T) {
	tmpl := NewTemplates()
	decodeAll(t, tmpl, ipfixtest.Message(1, ipfixtest.TemplateSet(flowTemplate, ipfixtest.FlowFields...)))
	decodeAll(t, tmpl, ipfixtest.Message(1, ipfixtest.TemplateSet(flowTemplate)))

	res := decodeAll(t, tmpl, ipfixtest.Message(1, ipfixtest.DataSet(flowTemplate, oneFlow())))

	assert.Equal(t, 1, res.MissingTemplate)
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `cd backend && go test ./internal/netflow/ -v`
Expected: FAIL to compile — `undefined: Templates`, `undefined: NewTemplates`, `undefined: Flow`.

- [ ] **Step 4: Write the parser**

`backend/internal/netflow/ipfix.go`:

```go
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
```

`backend/internal/netflow/ipfix_template.go`:

```go
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
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `cd backend && go test ./internal/netflow/... -v -race`
Expected: PASS (11 tests).

- [ ] **Step 6: Commit**

```bash
cd backend && gofmt -s -l ./internal/netflow
git add internal/netflow
git commit -m "feat(netflow): stdlib IPFIX parser with template and length limits

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2: ASN dataset — table and downloader

**Files:**
- Create: `backend/internal/netflow/asn.go`
- Create: `backend/internal/netflow/asnfetch.go`
- Test: `backend/internal/netflow/asn_test.go`, `backend/internal/netflow/asnfetch_test.go`

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: `type ASNInfo struct { Name, Country string }`, `type ASNTable`, `func ParseASNTable(r io.Reader) (*ASNTable, error)`, `func LoadASNFile(path string) (*ASNTable, error)`, `func (t *ASNTable) Lookup(ip netip.Addr) uint32` (0 when not found), `func (t *ASNTable) Names() map[uint32]ASNInfo`, `const ASNDatasetURL = "https://iptoasn.com/data/ip2asn-v4.tsv.gz"`, `const ASNMaxAge = 24 * time.Hour`, `type ASNFetcher struct { URL, Path string; Client *http.Client }` with `Stale(now time.Time) bool`, `LoadLocal() (*ASNTable, time.Time, error)`, `Download() (*ASNTable, time.Time, error)`.

The dataset's lines are `range_start<TAB>range_end<TAB>AS_number<TAB>country<TAB>description`; AS 0 marks unannounced space.

- [ ] **Step 1: Write the failing tests**

`backend/internal/netflow/asn_test.go`:

```go
package netflow

import (
	"bytes"
	"compress/gzip"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const asnFixture = "192.0.2.0\t192.0.2.255\t64496\tZZ\tEXAMPLE-NET-A\n" +
	"198.51.100.0\t198.51.100.127\t64497\tZZ\tEXAMPLE-NET-B\n" +
	"198.51.100.128\t198.51.100.255\t0\tNone\tNot routed\n" +
	"203.0.113.0\t203.0.113.255\t64498\tZZ\tEXAMPLE-NET-C\n"

func gzipped(t *testing.T, text string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	_, err := w.Write([]byte(text))
	require.NoError(t, err)
	require.NoError(t, w.Close())
	return buf.Bytes()
}

// Both ends of a range are inside it; one past the end is not.
func TestLookupFindsTheRangeHoldingAnAddress(t *testing.T) {
	table, err := ParseASNTable(strings.NewReader(asnFixture))
	require.NoError(t, err)

	cases := map[string]uint32{
		"192.0.2.0":      64496,
		"192.0.2.255":    64496,
		"198.51.100.127": 64497,
		"198.51.100.128": 0, // announced by nobody
		"203.0.113.77":   64498,
		"127.0.0.1":      0, // before the first range
		"2001:db8::1":    0, // IPv4 only
	}
	for ip, want := range cases {
		assert.Equal(t, want, table.Lookup(netip.MustParseAddr(ip)), ip)
	}
	assert.Equal(t, ASNInfo{Name: "EXAMPLE-NET-A", Country: "ZZ"}, table.Names()[64496])
	assert.NotContains(t, table.Names(), uint32(0))
}

// A malformed line fails the whole table, so a truncated or substituted file
// never replaces a good one.
func TestParseRefusesAMalformedDataset(t *testing.T) {
	cases := map[string]string{
		"AS prefix":         asnFixture + "192.0.2.0\t192.0.2.255\tAS64496\tZZ\tX\n",
		"four columns":      asnFixture + "192.0.2.0\t192.0.2.255\t64496\tZZ\n",
		"end before start":  asnFixture + "192.0.2.255\t192.0.2.0\t64496\tZZ\tX\n",
		"IPv6 range":        asnFixture + "2001:db8::\t2001:db8::ff\t64496\tZZ\tX\n",
		"nothing announced": "198.51.100.128\t198.51.100.255\t0\tNone\tNot routed\n",
		"empty":             "",
	}
	for name, text := range cases {
		_, err := ParseASNTable(strings.NewReader(text))
		assert.Error(t, err, name)
	}
}

func TestLoadASNFileReadsTheGzippedDataset(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ip2asn-v4.tsv.gz")
	require.NoError(t, os.WriteFile(path, gzipped(t, asnFixture), 0o644))

	table, err := LoadASNFile(path)

	require.NoError(t, err)
	assert.Equal(t, uint32(64498), table.Lookup(netip.MustParseAddr("203.0.113.1")))
}
```

`backend/internal/netflow/asnfetch_test.go`:

```go
package netflow

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDownloadStoresAValidDataset(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ip2asn-v4.tsv.gz")
	body := gzipped(t, asnFixture)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body)
	}))
	defer server.Close()

	table, _, err := (&ASNFetcher{URL: server.URL, Path: path, Client: server.Client()}).Download()

	require.NoError(t, err)
	assert.Equal(t, uint32(64497), table.Lookup(netip.MustParseAddr("198.51.100.1")))
	stored, err := LoadASNFile(path)
	require.NoError(t, err)
	assert.Equal(t, uint32(64496), stored.Lookup(netip.MustParseAddr("192.0.2.1")))
}

func TestDownloadFailureLeavesThePreviousCopy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ip2asn-v4.tsv.gz")
	previous := gzipped(t, asnFixture)
	require.NoError(t, os.WriteFile(path, previous, 0o644))
	malformed := gzipped(t, "192.0.2.0\tnot-an-address\t64496\tZZ\tX\n")

	handlers := map[string]http.HandlerFunc{
		"server error":      func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusBadGateway) },
		"not gzip":          func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("<html>maintenance</html>")) },
		"malformed dataset": func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(malformed) },
	}
	for name, handler := range handlers {
		server := httptest.NewServer(handler)
		_, _, err := (&ASNFetcher{URL: server.URL, Path: path, Client: server.Client()}).Download()
		server.Close()

		assert.Error(t, err, name)
		kept, readErr := os.ReadFile(path)
		require.NoError(t, readErr)
		assert.Equal(t, previous, kept, name)
	}
	leftovers, err := filepath.Glob(filepath.Join(dir, ".ip2asn-*"))
	require.NoError(t, err)
	assert.Empty(t, leftovers, "a failed download leaves no temporary file")
}

func TestStaleFollowsTheCopysAge(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ip2asn-v4.tsv.gz")
	fetcher := &ASNFetcher{Path: path}
	now := time.Now()
	assert.True(t, fetcher.Stale(now), "no copy yet")

	require.NoError(t, os.WriteFile(path, gzipped(t, asnFixture), 0o644))
	assert.False(t, fetcher.Stale(now))

	old := now.Add(-ASNMaxAge - time.Hour)
	require.NoError(t, os.Chtimes(path, old, old))
	assert.True(t, fetcher.Stale(now))
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd backend && go test ./internal/netflow/ -run 'Lookup|Parse|LoadASN|Download|Stale' -v`
Expected: FAIL to compile — `undefined: ParseASNTable`, `undefined: ASNFetcher`.

- [ ] **Step 3: Write the table**

`backend/internal/netflow/asn.go`:

```go
package netflow

import (
	"bufio"
	"compress/gzip"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"os"
	"sort"
	"strconv"
	"strings"
)

// asnColumns is start, end, AS number, country, description.
const asnColumns = 5

// maxASNInflated bounds the decompressed dataset, a few tens of megabytes
// today, so a substituted file cannot inflate without limit.
const maxASNInflated = 256 << 20

var errASNTooLarge = errors.New("ip2asn dataset inflates past its size limit")

// ASNInfo names an autonomous system.
type ASNInfo struct {
	Name    string
	Country string
}

type asnRange struct {
	start, end uint32
	asn        uint32
}

// ASNTable maps an IPv4 address to the AS announcing it, from iptoasn.com's
// ip2asn-v4 dataset: sorted ranges, searched by binary search.
type ASNTable struct {
	ranges []asnRange
	names  map[uint32]ASNInfo
}

// ParseASNTable reads the uncompressed TSV. Any malformed line fails the whole
// table, so a truncated or substituted download never replaces a good one.
func ParseASNTable(r io.Reader) (*ASNTable, error) {
	t := &ASNTable{names: map[uint32]ASNInfo{}}
	scanner := bufio.NewScanner(r)
	for line := 1; scanner.Scan(); line++ {
		if err := t.addLine(scanner.Text()); err != nil {
			return nil, fmt.Errorf("ip2asn line %d: %w", line, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(t.ranges) == 0 {
		return nil, errors.New("ip2asn: no announced ranges")
	}
	sort.Slice(t.ranges, func(i, j int) bool { return t.ranges[i].start < t.ranges[j].start })
	return t, nil
}

func (t *ASNTable) addLine(line string) error {
	cols := strings.Split(line, "\t")
	if len(cols) != asnColumns {
		return fmt.Errorf("want %d tab-separated columns, got %d", asnColumns, len(cols))
	}
	start, errStart := parseIPv4(cols[0])
	end, errEnd := parseIPv4(cols[1])
	asn, errASN := strconv.ParseUint(cols[2], 10, 32)
	if err := errors.Join(errStart, errEnd, errASN); err != nil {
		return err
	}
	if end < start {
		return errors.New("range ends before it starts")
	}
	// AS 0 is space nobody announces. Leaving it out makes it "not found",
	// which Lookup reports as 0 anyway.
	if asn == 0 {
		return nil
	}
	t.ranges = append(t.ranges, asnRange{start: start, end: end, asn: uint32(asn)})
	t.names[uint32(asn)] = ASNInfo{Name: cols[4], Country: cols[3]}
	return nil
}

func parseIPv4(s string) (uint32, error) {
	addr, err := netip.ParseAddr(s)
	if err != nil || !addr.Is4() {
		return 0, fmt.Errorf("%q is not an IPv4 address", s)
	}
	b := addr.As4()
	return binary.BigEndian.Uint32(b[:]), nil
}

// Lookup returns the AS announcing ip, or 0 when none does.
func (t *ASNTable) Lookup(ip netip.Addr) uint32 {
	if !ip.Is4() {
		return 0
	}
	b := ip.As4()
	v := binary.BigEndian.Uint32(b[:])
	i := sort.Search(len(t.ranges), func(i int) bool { return t.ranges[i].start > v }) - 1
	if i < 0 || v > t.ranges[i].end {
		return 0
	}
	return t.ranges[i].asn
}

// Names returns every AS the table knows, for asn_names.
func (t *ASNTable) Names() map[uint32]ASNInfo { return t.names }

// LoadASNFile reads a gzipped dataset from disk.
func LoadASNFile(path string) (*ASNTable, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	gz, err := gzip.NewReader(file)
	if err != nil {
		return nil, fmt.Errorf("ip2asn: %w", err)
	}
	defer func() { _ = gz.Close() }()
	return ParseASNTable(&cappedReader{r: gz, left: maxASNInflated})
}

// cappedReader fails rather than ends at its limit: a silent end could fall
// on a line boundary and pass a truncated table off as whole.
type cappedReader struct {
	r    io.Reader
	left int64
}

func (c *cappedReader) Read(p []byte) (int, error) {
	if c.left <= 0 {
		return 0, errASNTooLarge
	}
	if int64(len(p)) > c.left {
		p = p[:c.left]
	}
	n, err := c.r.Read(p)
	c.left -= int64(n)
	return n, err
}
```

- [ ] **Step 4: Write the downloader**

`backend/internal/netflow/asnfetch.go`:

```go
package netflow

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// ASNDatasetURL is iptoasn.com's IPv4 dataset, rebuilt daily.
const ASNDatasetURL = "https://iptoasn.com/data/ip2asn-v4.tsv.gz"

// ASNMaxAge is how old the copy on disk may be before flowd downloads again.
const ASNMaxAge = 24 * time.Hour

const (
	// asnDownloadTimeout covers the whole transfer of a file of a few MB.
	asnDownloadTimeout = 2 * time.Minute
	// maxASNDownload bounds what a misbehaving server can make flowd write.
	maxASNDownload = 64 << 20
)

// ASNFetcher keeps the dataset on disk and loads it.
type ASNFetcher struct {
	URL    string
	Path   string
	Client *http.Client
}

// Stale reports whether the copy on disk is missing or older than ASNMaxAge.
func (f *ASNFetcher) Stale(now time.Time) bool {
	info, err := os.Stat(f.Path)
	return err != nil || now.Sub(info.ModTime()) > ASNMaxAge
}

// LoadLocal reads the copy on disk and reports when it was downloaded.
func (f *ASNFetcher) LoadLocal() (*ASNTable, time.Time, error) {
	info, err := os.Stat(f.Path)
	if err != nil {
		return nil, time.Time{}, err
	}
	table, err := LoadASNFile(f.Path)
	return table, info.ModTime(), err
}

// Download fetches the dataset into a temporary file beside Path, parses it,
// and only then renames it over Path. A failed or invalid download leaves the
// previous copy, and the table built from it, as they were.
func (f *ASNFetcher) Download() (*ASNTable, time.Time, error) {
	tmp, err := os.CreateTemp(filepath.Dir(f.Path), ".ip2asn-*")
	if err != nil {
		return nil, time.Time{}, err
	}
	// After a successful rename this finds nothing to remove.
	defer func() { _ = os.Remove(tmp.Name()) }()

	fetchErr := f.fetchTo(tmp)
	if closeErr := tmp.Close(); fetchErr == nil {
		fetchErr = closeErr
	}
	if fetchErr != nil {
		return nil, time.Time{}, fetchErr
	}
	table, err := LoadASNFile(tmp.Name())
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("downloaded dataset is invalid: %w", err)
	}
	if err := os.Rename(tmp.Name(), f.Path); err != nil {
		return nil, time.Time{}, err
	}
	return table, time.Now(), nil
}

func (f *ASNFetcher) fetchTo(w io.Writer) error {
	ctx, cancel := context.WithTimeout(context.Background(), asnDownloadTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.URL, nil)
	if err != nil {
		return err
	}
	resp, err := f.Client.Do(req)
	if err != nil {
		return fmt.Errorf("GET %s: %w", f.URL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: HTTP %d", f.URL, resp.StatusCode)
	}
	n, err := io.Copy(w, io.LimitReader(resp.Body, maxASNDownload+1))
	if err != nil {
		return err
	}
	if n > maxASNDownload {
		return fmt.Errorf("dataset larger than %d bytes", maxASNDownload)
	}
	return nil
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `cd backend && go test ./internal/netflow/... -v -race`
Expected: PASS (Task 1's tests plus 6 new ones).

- [ ] **Step 6: Commit**

```bash
cd backend && gofmt -s -l ./internal/netflow
git add internal/netflow
git commit -m "feat(netflow): iptoasn dataset table with validated, atomic download

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 3: POP prefixes and flow classification

**Files:**
- Create: `backend/internal/netflow/prefixes.go`
- Create: `backend/internal/netflow/classify.go`
- Test: `backend/internal/netflow/classify_test.go`

**Interfaces:**
- Consumes: `netflow.Flow` (Task 1).
- Produces: `type PrefixEntry struct { Prefix netip.Prefix; InterfaceID uuid.UUID }`, `func NewPrefixTable(entries []PrefixEntry) *PrefixTable`, `func (t *PrefixTable) Lookup(ip netip.Addr) (uuid.UUID, bool)`, `type Direction string` with `Download = "download"`, `Upload = "upload"`, `type RouterMap struct { RouterID uuid.UUID; Sampling uint64; Upstreams map[uint32]uuid.UUID; POPs *PrefixTable }`, `type Classified struct { POP, Upstream uuid.UUID; Direction Direction; Remote netip.Addr; Bytes, Packets uint64 }`, `func (m *RouterMap) Classify(f Flow) (Classified, bool)`. `POP == uuid.Nil` means "Lainnya" (stored as NULL).

Ruling recorded here: a flow whose other-side interface index is 0 is the router's own traffic (BGP, management, ICMP to the router) — the spec's "ke router sendiri" — and is dropped.

- [ ] **Step 1: Write the failing tests**

`backend/internal/netflow/classify_test.go`:

```go
package netflow

import (
	"net/netip"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	popA     = uuid.MustParse("00000000-0000-0000-0000-0000000000a1")
	popB     = uuid.MustParse("00000000-0000-0000-0000-0000000000a2")
	popWide  = uuid.MustParse("00000000-0000-0000-0000-0000000000a3")
	transit  = uuid.MustParse("00000000-0000-0000-0000-0000000000b1")
	ix       = uuid.MustParse("00000000-0000-0000-0000-0000000000b2")
	routerID = uuid.MustParse("00000000-0000-0000-0000-0000000000c1")
)

func addr(s string) netip.Addr { return netip.MustParseAddr(s) }

func testPrefixes() *PrefixTable {
	return NewPrefixTable([]PrefixEntry{
		// An interface address: host bits set, as /ip/address stores it.
		{Prefix: netip.MustParsePrefix("192.0.2.33/29"), InterfaceID: popA},
		// A second block on the same interface.
		{Prefix: netip.MustParsePrefix("192.0.2.57/30"), InterfaceID: popA},
		{Prefix: netip.MustParsePrefix("203.0.113.0/24"), InterfaceID: popWide},
		// A block routed statically to a POP router, inside the /24.
		{Prefix: netip.MustParsePrefix("203.0.113.64/27"), InterfaceID: popB},
		{Prefix: netip.MustParsePrefix("2001:db8::/32"), InterfaceID: popB},
	})
}

func TestLongestPrefixWinsWhateverTheLengths(t *testing.T) {
	table := testPrefixes()
	cases := map[string]uuid.UUID{
		"192.0.2.38":   popA,     // the /29
		"192.0.2.58":   popA,     // the /30
		"192.0.2.60":   uuid.Nil, // just past the /30
		"203.0.113.70": popB,     // the /27 beats the /24 holding it
		"203.0.113.96": popWide,  // only the /24
		"2001:db8::1":  uuid.Nil, // IPv4 only
	}
	for ip, want := range cases {
		got, ok := table.Lookup(addr(ip))
		assert.Equal(t, want, got, ip)
		assert.Equal(t, want != uuid.Nil, ok, ip)
	}
}

func testRouterMap() *RouterMap {
	return &RouterMap{RouterID: routerID, Sampling: 100,
		Upstreams: map[uint32]uuid.UUID{10: transit, 11: ix}, POPs: testPrefixes()}
}

func TestClassifyPlacesDownloadsAndUploads(t *testing.T) {
	m := testRouterMap()

	down, ok := m.Classify(Flow{Src: addr("198.51.100.7"), Dst: addr("203.0.113.70"),
		Ingress: 10, Egress: 30, Bytes: 1500, Packets: 1})
	require.True(t, ok)
	assert.Equal(t, Classified{POP: popB, Upstream: transit, Direction: Download,
		Remote: addr("198.51.100.7"), Bytes: 150_000, Packets: 100}, down)

	up, ok := m.Classify(Flow{Src: addr("192.0.2.38"), Dst: addr("198.51.100.9"),
		Ingress: 30, Egress: 11, Bytes: 80, Packets: 1})
	require.True(t, ok)
	assert.Equal(t, Classified{POP: popA, Upstream: ix, Direction: Upload,
		Remote: addr("198.51.100.9"), Bytes: 8000, Packets: 100}, up)
}

func TestClassifyDropsTrafficThatNeverCrossesAnUpstream(t *testing.T) {
	m := testRouterMap()
	cases := map[string]Flow{
		"POP to POP":             {Src: addr("192.0.2.38"), Dst: addr("203.0.113.70"), Ingress: 30, Egress: 31, Bytes: 1},
		"to the router itself":   {Src: addr("198.51.100.7"), Dst: addr("198.51.100.0"), Ingress: 10, Egress: 0, Bytes: 1},
		"from the router itself": {Src: addr("198.51.100.0"), Dst: addr("198.51.100.7"), Ingress: 0, Egress: 10, Bytes: 1},
	}
	for name, f := range cases {
		_, ok := m.Classify(f)
		assert.False(t, ok, name)
	}
}

// Review Focus 3: before discovery has stored a POP's blocks, its traffic is
// still counted, under "Lainnya", rather than lost.
func TestClassifyCountsAnAddressInNoPOPAsOther(t *testing.T) {
	c, ok := testRouterMap().Classify(Flow{Src: addr("198.51.100.7"), Dst: addr("192.0.2.200"),
		Ingress: 10, Egress: 30, Bytes: 10, Packets: 1})

	require.True(t, ok)
	assert.Equal(t, uuid.Nil, c.POP)
	assert.Equal(t, transit, c.Upstream)
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd backend && go test ./internal/netflow/ -run 'LongestPrefix|Classify' -v`
Expected: FAIL to compile — `undefined: NewPrefixTable`, `undefined: RouterMap`.

- [ ] **Step 3: Write the prefix table**

`backend/internal/netflow/prefixes.go`:

```go
package netflow

import (
	"net/netip"
	"sort"

	"github.com/google/uuid"
)

// PrefixEntry is one block a POP interface owns: an address on it, or a block
// routed statically to a device behind it.
type PrefixEntry struct {
	Prefix      netip.Prefix
	InterfaceID uuid.UUID
}

// PrefixTable finds the most specific block holding an address. Blocks have
// whatever length the operator gave each POP, so every length present is
// tried, longest first: at most 33 map lookups however many blocks exist.
type PrefixTable struct {
	byLength map[int]map[netip.Prefix]uuid.UUID
	lengths  []int
}

// NewPrefixTable indexes IPv4 entries; IPv6 ones are ignored. When two
// interfaces list the same block the first entry keeps it.
func NewPrefixTable(entries []PrefixEntry) *PrefixTable {
	t := &PrefixTable{byLength: map[int]map[netip.Prefix]uuid.UUID{}}
	for _, e := range entries {
		if !e.Prefix.Addr().Is4() {
			continue
		}
		p := e.Prefix.Masked()
		blocks, ok := t.byLength[p.Bits()]
		if !ok {
			blocks = map[netip.Prefix]uuid.UUID{}
			t.byLength[p.Bits()] = blocks
			t.lengths = append(t.lengths, p.Bits())
		}
		if _, taken := blocks[p]; !taken {
			blocks[p] = e.InterfaceID
		}
	}
	sort.Sort(sort.Reverse(sort.IntSlice(t.lengths)))
	return t
}

// Lookup returns the interface owning the most specific block holding ip.
func (t *PrefixTable) Lookup(ip netip.Addr) (uuid.UUID, bool) {
	if !ip.Is4() {
		return uuid.Nil, false
	}
	for _, bits := range t.lengths {
		p, err := ip.Prefix(bits)
		if err != nil {
			continue
		}
		if id, ok := t.byLength[bits][p]; ok {
			return id, true
		}
	}
	return uuid.Nil, false
}
```

- [ ] **Step 4: Write the classifier**

`backend/internal/netflow/classify.go`:

```go
package netflow

import (
	"net/netip"

	"github.com/google/uuid"
)

// Direction is which way traffic crossed the upstream.
type Direction string

// Each packet is counted at the interface it entered: in from an upstream is
// a download, in from a POP and out to an upstream is an upload.
const (
	Download Direction = "download"
	Upload   Direction = "upload"
)

// RouterMap is what flowd knows about one exporting router.
type RouterMap struct {
	RouterID uuid.UUID
	// Sampling is the router's 1:N packet sampling; counts are scaled by it.
	Sampling uint64
	// Upstreams maps the ifIndex of each BGP-facing interface to its ID.
	Upstreams map[uint32]uuid.UUID
	// POPs holds the blocks of every interface that is not an upstream.
	POPs *PrefixTable
}

// Classified is a flow placed on the router's topology. POP is uuid.Nil when
// the local address is in no POP's blocks ("Lainnya").
type Classified struct {
	POP       uuid.UUID
	Upstream  uuid.UUID
	Direction Direction
	Remote    netip.Addr
	Bytes     uint64
	Packets   uint64
}

// Classify places a flow, or reports false for one that never crossed an
// upstream (POP to POP) or that the router itself sent or received: ifIndex 0
// on the far side means no interface, which is the router's own stack.
func (m *RouterMap) Classify(f Flow) (Classified, bool) {
	c := Classified{Bytes: f.Bytes * m.Sampling, Packets: f.Packets * m.Sampling}
	var local netip.Addr
	var otherSide uint32
	if up, ok := m.Upstreams[f.Ingress]; ok {
		c.Upstream, c.Direction, local, c.Remote, otherSide = up, Download, f.Dst, f.Src, f.Egress
	} else if up, ok := m.Upstreams[f.Egress]; ok {
		c.Upstream, c.Direction, local, c.Remote, otherSide = up, Upload, f.Src, f.Dst, f.Ingress
	} else {
		return Classified{}, false
	}
	if otherSide == 0 {
		return Classified{}, false
	}
	c.POP, _ = m.POPs.Lookup(local)
	return c, true
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `cd backend && go test ./internal/netflow/... -v -race`
Expected: PASS (all earlier tests plus 4 new ones).

- [ ] **Step 6: Commit**

```bash
cd backend && gofmt -s -l ./internal/netflow
git add internal/netflow
git commit -m "feat(netflow): longest-prefix POP match and flow direction/upstream mapping

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 4: Minute aggregation, top ASNs, top IPs and the key caps

**Files:**
- Create: `backend/internal/netflow/aggregate.go`
- Test: `backend/internal/netflow/aggregate_test.go`

**Interfaces:**
- Consumes: `Classified`, `Direction`, `Download` (Task 3); test vars `popA`, `popB`, `transit`, `ix` from `classify_test.go`.
- Produces: constants `MaxKeysPerRouter = 50_000`, `MaxIPsPerPOP = 100_000`, `TopASNs = 20`, `TopIPs = 100`, `OtherASN = 0` (untyped), `TopIPWindow = 5 * time.Minute`; `type MinuteKey struct { RouterID, POP, Upstream uuid.UUID; Direction Direction; ASN uint32 }`; `type MinuteRow struct { Time time.Time; MinuteKey; Bytes, Packets uint64 }`; `type TopIPRow struct { Time time.Time; RouterID, POP uuid.UUID; Direction Direction; IP netip.Addr; ASN uint32; Bytes uint64 }`; `func NewAggregator(now time.Time) *Aggregator`; `(*Aggregator).Add(routerID uuid.UUID, c Classified, asn uint32)`; `(*Aggregator).CloseMinute(now time.Time) ([]MinuteRow, map[uuid.UUID]uint64)` (rows, folded keys per router); `(*Aggregator).WindowDue(now time.Time) bool`; `(*Aggregator).CloseWindow(now time.Time) []TopIPRow`.

Rulings recorded here:
- Flows are bucketed by **arrival minute** at flowd, not by the flow's own timestamps: the router's clock is not trusted and the minute boundary is flowd's. The operator guide (Task 13) sets `active-flow-timeout=1m` so a long download is exported every minute instead of in one lump after RouterOS's 30-minute default.
- `dropped_keys` is a **log field** written each minute a router hits the cap; the spec names no table for it.
- The top-ASN tests need 25 distinct AS numbers; 64496–64511 has 16, so they also use RFC 5398's other documentation block 65536–65551. The 100,000-address cap test uses 2001:db8::/32, the only documentation space that large (the aggregator does not care about address family; `Classify` alone restricts flows to IPv4).

- [ ] **Step 1: Write the failing tests**

`backend/internal/netflow/aggregate_test.go`:

```go
package netflow

import (
	"fmt"
	"net/netip"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	routerA = uuid.MustParse("00000000-0000-0000-0000-0000000000d1")
	routerB = uuid.MustParse("00000000-0000-0000-0000-0000000000d2")
	minute0 = time.Date(2026, 10, 12, 3, 0, 0, 0, time.UTC)
)

// docASN returns the i-th documentation AS number (RFC 5398): 64496-64511,
// then 65536-65551.
func docASN(i int) uint32 {
	if i < 16 {
		return 64496 + uint32(i)
	}
	return 65536 + uint32(i-16)
}

func download(pop, upstream uuid.UUID, remote string, bytes uint64) Classified {
	return Classified{POP: pop, Upstream: upstream, Direction: Download,
		Remote: netip.MustParseAddr(remote), Bytes: bytes, Packets: 1}
}

func sumBytes(rows []MinuteRow) uint64 {
	var total uint64
	for _, r := range rows {
		total += r.Bytes
	}
	return total
}

func asnsUnder(rows []MinuteRow, pop uuid.UUID) []uint32 {
	var asns []uint32
	for _, r := range rows {
		if r.POP == pop {
			asns = append(asns, r.ASN)
		}
	}
	return asns
}

func TestCloseMinuteKeepsTheTopASNsAndPreservesTheTotal(t *testing.T) {
	agg := NewAggregator(minute0.Add(10 * time.Second))
	var total uint64
	for i := 0; i < TopASNs+5; i++ {
		bytes := uint64(i+1) * 1000
		// Split over two upstreams: the ranking is per ASN, not per row.
		agg.Add(routerA, download(popA, transit, "198.51.100.7", bytes), docASN(i))
		agg.Add(routerA, download(popA, ix, "198.51.100.8", bytes), docASN(i))
		total += 2 * bytes
	}

	rows, dropped := agg.CloseMinute(minute0.Add(time.Minute))

	assert.Empty(t, dropped)
	assert.Equal(t, total, sumBytes(rows), "folding into ASN 0 loses no bytes")
	kept := map[uint32]bool{}
	var other uint64
	for _, r := range rows {
		assert.Equal(t, minute0, r.Time)
		if r.ASN == OtherASN {
			other += r.Bytes
			continue
		}
		kept[r.ASN] = true
	}
	assert.Len(t, kept, TopASNs)
	for i := 0; i < 5; i++ {
		assert.False(t, kept[docASN(i)], "the five smallest fold into ASN 0")
	}
	assert.Equal(t, uint64(2*(1+2+3+4+5)*1000), other)
	assert.Len(t, rows, 2*(TopASNs+1), "one row per upstream for each kept ASN and for ASN 0")
}

// Each POP has its own top: a small destination of a quiet POP is not
// crowded out by a busy POP's.
func TestTopASNsAreRankedPerPOP(t *testing.T) {
	agg := NewAggregator(minute0)
	for i := 0; i < TopASNs; i++ {
		agg.Add(routerA, download(popA, transit, "198.51.100.7", 1_000_000), docASN(i))
	}
	agg.Add(routerA, download(popB, transit, "198.51.100.7", 10), docASN(TopASNs))

	rows, _ := agg.CloseMinute(minute0.Add(time.Minute))

	assert.Equal(t, []uint32{docASN(TopASNs)}, asnsUnder(rows, popB))
}

func TestKeyCapFoldsNewKeysIntoOtherPerRouter(t *testing.T) {
	agg := NewAggregator(minute0)
	pops := make([]uuid.UUID, MaxKeysPerRouter)
	for i := range pops {
		pops[i] = uuid.New()
		agg.Add(routerA, download(pops[i], transit, "198.51.100.7", 1), docASN(0))
	}
	agg.Add(routerA, download(pops[0], transit, "198.51.100.7", 5), docASN(1))
	agg.Add(routerB, download(popA, transit, "198.51.100.7", 7), docASN(1))

	rows, dropped := agg.CloseMinute(minute0.Add(time.Minute))

	assert.Equal(t, map[uuid.UUID]uint64{routerA: 1}, dropped)
	assert.Equal(t, uint64(MaxKeysPerRouter+5+7), sumBytes(rows))
	assert.Contains(t, rows, MinuteRow{Time: minute0, MinuteKey: MinuteKey{RouterID: routerA, POP: pops[0],
		Upstream: transit, Direction: Download, ASN: OtherASN}, Bytes: 5, Packets: 1})
	assert.Equal(t, []uint32{docASN(1)}, asnsUnder(rows, popA), "another router's keys are not capped by this one's")
}

func TestCloseWindowKeepsTheHundredLargestAddresses(t *testing.T) {
	agg := NewAggregator(minute0)
	for i := 0; i < 150; i++ {
		agg.Add(routerA, download(popA, transit, fmt.Sprintf("203.0.113.%d", i), uint64(i+1)), docASN(0))
	}
	assert.False(t, agg.WindowDue(minute0.Add(4*time.Minute)))
	require.True(t, agg.WindowDue(minute0.Add(TopIPWindow)))

	rows := agg.CloseWindow(minute0.Add(TopIPWindow))

	require.Len(t, rows, TopIPs)
	for _, r := range rows {
		assert.Greater(t, r.Bytes, uint64(50), "only the hundred largest are kept")
		assert.Equal(t, minute0, r.Time)
		assert.Equal(t, docASN(0), r.ASN)
	}
	assert.False(t, agg.WindowDue(minute0.Add(TopIPWindow+time.Minute)), "the next window is open")
}

// Past the cap a new address is not tracked, however large; its bytes still
// count in the minute totals.
func TestIPCounterStopsAtTheCapPerPOP(t *testing.T) {
	agg := NewAggregator(minute0)
	remote := netip.MustParseAddr("2001:db8::")
	for i := 0; i < MaxIPsPerPOP; i++ {
		agg.Add(routerA, Classified{POP: popA, Upstream: transit, Direction: Download, Remote: remote, Bytes: 1}, 0)
		remote = remote.Next()
	}
	late := netip.MustParseAddr("2001:db8:ffff::1")
	agg.Add(routerA, Classified{POP: popA, Upstream: transit, Direction: Download, Remote: late, Bytes: 1 << 40}, 0)

	for _, r := range agg.CloseWindow(minute0.Add(TopIPWindow)) {
		assert.NotEqual(t, late, r.IP)
	}
	minute, _ := agg.CloseMinute(minute0.Add(time.Minute))
	assert.Equal(t, uint64(MaxIPsPerPOP)+1<<40, sumBytes(minute))
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd backend && go test ./internal/netflow/ -run 'CloseMinute|TopASNs|KeyCap|CloseWindow|IPCounter' -v`
Expected: FAIL to compile — `undefined: NewAggregator`, `undefined: MinuteRow`.

- [ ] **Step 3: Write the aggregator**

`backend/internal/netflow/aggregate.go`:

```go
package netflow

import (
	"net/netip"
	"sort"
	"time"

	"github.com/google/uuid"
)

const (
	// MaxKeysPerRouter bounds one router's distinct keys in a minute, so a
	// flood of destinations cannot grow memory without limit.
	MaxKeysPerRouter = 50_000
	// MaxIPsPerPOP bounds the distinct far-side addresses counted per POP
	// per window.
	MaxIPsPerPOP = 100_000
	// TopASNs is how many ASNs per (router, POP, direction, minute) keep a
	// row of their own.
	TopASNs = 20
	// TopIPs is how many addresses per (router, POP, direction, window) are
	// stored.
	TopIPs = 100
	// OtherASN stands for both "no dataset range holds it" and "outside the
	// top": the UI calls it Lainnya.
	OtherASN = 0
	// TopIPWindow is the top-IP counters' window.
	TopIPWindow = 5 * time.Minute
)

// MinuteKey identifies one router_flow_minute row.
type MinuteKey struct {
	RouterID  uuid.UUID
	POP       uuid.UUID
	Upstream  uuid.UUID
	Direction Direction
	ASN       uint32
}

// MinuteRow is one closed minute's total for a key.
type MinuteRow struct {
	Time time.Time
	MinuteKey
	Bytes   uint64
	Packets uint64
}

// TopIPRow is one far-side address's total in a window.
type TopIPRow struct {
	Time      time.Time
	RouterID  uuid.UUID
	POP       uuid.UUID
	Direction Direction
	IP        netip.Addr
	ASN       uint32
	Bytes     uint64
}

type counter struct{ bytes, packets uint64 }

type groupKey struct {
	RouterID  uuid.UUID
	POP       uuid.UUID
	Direction Direction
}

type popKey struct{ routerID, pop uuid.UUID }

type ipKey struct {
	groupKey
	ip netip.Addr
}

type ipCounter struct {
	asn   uint32
	bytes uint64
}

// Aggregator adds placed flows into the open minute and the open top-IP
// window. It is not safe for concurrent use; flowd guards it with a mutex.
type Aggregator struct {
	minute        time.Time
	keys          map[MinuteKey]*counter
	keysPerRouter map[uuid.UUID]int
	droppedKeys   map[uuid.UUID]uint64

	window    time.Time
	ips       map[ipKey]*ipCounter
	ipsPerPOP map[popKey]int
}

// NewAggregator opens the minute and window holding now.
func NewAggregator(now time.Time) *Aggregator {
	a := &Aggregator{}
	a.resetMinute(now)
	a.resetWindow(now)
	return a
}

func (a *Aggregator) resetMinute(now time.Time) {
	a.minute = now.Truncate(time.Minute)
	a.keys = map[MinuteKey]*counter{}
	a.keysPerRouter = map[uuid.UUID]int{}
	a.droppedKeys = map[uuid.UUID]uint64{}
}

func (a *Aggregator) resetWindow(now time.Time) {
	a.window = now.Truncate(TopIPWindow)
	a.ips = map[ipKey]*ipCounter{}
	a.ipsPerPOP = map[popKey]int{}
}

// Add counts one placed flow whose far side is in asn.
func (a *Aggregator) Add(routerID uuid.UUID, c Classified, asn uint32) {
	a.addMinute(MinuteKey{RouterID: routerID, POP: c.POP, Upstream: c.Upstream, Direction: c.Direction, ASN: asn}, c)
	a.addIP(groupKey{RouterID: routerID, POP: c.POP, Direction: c.Direction}, c.Remote, asn, c.Bytes)
}

// addMinute folds a new key into its OtherASN key once the router holds
// MaxKeysPerRouter keys this minute: totals stay exact, only the breakdown is
// lost. OtherASN keys are always admitted; there are at most two per POP and
// upstream.
func (a *Aggregator) addMinute(key MinuteKey, c Classified) {
	cnt, ok := a.keys[key]
	if !ok && key.ASN != OtherASN && a.keysPerRouter[key.RouterID] >= MaxKeysPerRouter {
		a.droppedKeys[key.RouterID]++
		key.ASN = OtherASN
		cnt, ok = a.keys[key]
	}
	if !ok {
		cnt = &counter{}
		a.keys[key] = cnt
		a.keysPerRouter[key.RouterID]++
	}
	cnt.bytes += c.Bytes
	cnt.packets += c.Packets
}

// addIP ignores a new address once its POP holds MaxIPsPerPOP; the bytes
// are already in the minute totals.
func (a *Aggregator) addIP(g groupKey, ip netip.Addr, asn uint32, bytes uint64) {
	key := ipKey{groupKey: g, ip: ip}
	cnt, ok := a.ips[key]
	if !ok {
		pk := popKey{routerID: g.RouterID, pop: g.POP}
		if a.ipsPerPOP[pk] >= MaxIPsPerPOP {
			return
		}
		a.ipsPerPOP[pk]++
		cnt = &ipCounter{asn: asn}
		a.ips[key] = cnt
	}
	cnt.bytes += bytes
}

// CloseMinute returns the open minute's rows, keeping TopASNs per (router,
// POP, direction) and folding the rest into OtherASN, with the keys each
// router had to fold for the cap. It then opens the minute holding now.
func (a *Aggregator) CloseMinute(now time.Time) ([]MinuteRow, map[uuid.UUID]uint64) {
	keep := topASNs(a.keys)
	merged := make(map[MinuteKey]*counter, len(a.keys))
	for key, cnt := range a.keys {
		if !keep[groupKey{RouterID: key.RouterID, POP: key.POP, Direction: key.Direction}][key.ASN] {
			key.ASN = OtherASN
		}
		addCounter(merged, key, cnt)
	}
	rows := make([]MinuteRow, 0, len(merged))
	for key, cnt := range merged {
		rows = append(rows, MinuteRow{Time: a.minute, MinuteKey: key, Bytes: cnt.bytes, Packets: cnt.packets})
	}
	dropped := a.droppedKeys
	a.resetMinute(now)
	return rows, dropped
}

func addCounter(into map[MinuteKey]*counter, key MinuteKey, cnt *counter) {
	sum, ok := into[key]
	if !ok {
		sum = &counter{}
		into[key] = sum
	}
	sum.bytes += cnt.bytes
	sum.packets += cnt.packets
}

// topASNs ranks each group's ASNs by bytes summed over upstreams. OtherASN
// never takes one of the places.
func topASNs(keys map[MinuteKey]*counter) map[groupKey]map[uint32]bool {
	totals := map[groupKey]map[uint32]uint64{}
	for key, cnt := range keys {
		if key.ASN == OtherASN {
			continue
		}
		g := groupKey{RouterID: key.RouterID, POP: key.POP, Direction: key.Direction}
		if totals[g] == nil {
			totals[g] = map[uint32]uint64{}
		}
		totals[g][key.ASN] += cnt.bytes
	}
	keep := make(map[groupKey]map[uint32]bool, len(totals))
	for g, byASN := range totals {
		keep[g] = largestASNs(byASN, TopASNs)
	}
	return keep
}

func largestASNs(byASN map[uint32]uint64, n int) map[uint32]bool {
	asns := make([]uint32, 0, len(byASN))
	for asn := range byASN {
		asns = append(asns, asn)
	}
	sort.Slice(asns, func(i, j int) bool {
		if byASN[asns[i]] != byASN[asns[j]] {
			return byASN[asns[i]] > byASN[asns[j]]
		}
		return asns[i] < asns[j]
	})
	if len(asns) > n {
		asns = asns[:n]
	}
	set := make(map[uint32]bool, len(asns))
	for _, asn := range asns {
		set[asn] = true
	}
	return set
}

// WindowDue reports whether now has left the open top-IP window.
func (a *Aggregator) WindowDue(now time.Time) bool {
	return !now.Truncate(TopIPWindow).Equal(a.window)
}

// CloseWindow returns the TopIPs largest addresses per (router, POP,
// direction) of the open window and opens the window holding now.
func (a *Aggregator) CloseWindow(now time.Time) []TopIPRow {
	groups := map[groupKey][]TopIPRow{}
	for key, cnt := range a.ips {
		groups[key.groupKey] = append(groups[key.groupKey], TopIPRow{Time: a.window, RouterID: key.RouterID,
			POP: key.POP, Direction: key.Direction, IP: key.ip, ASN: cnt.asn, Bytes: cnt.bytes})
	}
	var rows []TopIPRow
	for _, group := range groups {
		sort.Slice(group, func(i, j int) bool { return group[i].Bytes > group[j].Bytes })
		if len(group) > TopIPs {
			group = group[:TopIPs]
		}
		rows = append(rows, group...)
	}
	a.resetWindow(now)
	return rows
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cd backend && go test ./internal/netflow/... -v -race`
Expected: PASS (5 new tests; the cap tests take a few seconds under `-race`).

- [ ] **Step 5: Commit**

```bash
cd backend && gofmt -s -l ./internal/netflow
git add internal/netflow
git commit -m "feat(netflow): per-minute aggregation with top-20 ASNs, top-100 IPs and key caps

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 5: Models and migration 58

**Files:**
- Create: `backend/internal/models/flow.go`
- Modify: `backend/internal/models/mikrotik.go` (`MikrotikRouter`, after `LastSeenAt`)
- Modify: `backend/internal/models/models.go` (AutoMigrate list)
- Create: `backend/migrations/58_mikrotik_traffic_flows.sql`
- Test: `backend/internal/database/migrations_flow_postgres_test.go`

**Interfaces:**
- Consumes: `insertTestRouter`, `insertTestInterface`, `freshPostgres` (existing, `internal/database` tests).
- Produces: `models.RouterFlowMinute{Time; RouterID; POPInterfaceID *uuid.UUID; UpstreamInterfaceID uuid.UUID; Direction string; ASN, Bytes, Packets int64}` (table `router_flow_minute`), `models.RouterFlowTopIP{Time; RouterID; POPInterfaceID *uuid.UUID; Direction string; IP string; ASN, Bytes int64}` (table `router_flow_top_ips`), `models.ASNName{ASN int64; Name, Country string; UpdatedAt time.Time}` (table `asn_names`), `models.RouterRoutedPrefix{ID, RouterID uuid.UUID; Prefix string; InterfaceID uuid.UUID; UpdatedAt}` (table `router_routed_prefixes`), and on `MikrotikRouter`: `FlowEnabled bool` (`flow_enabled`), `FlowSampling int` (`flow_sampling`, default 100), `FlowLastSeenAt *time.Time` (`flow_last_seen_at`). Views `router_flow_5min`, `router_flow_hourly`, `router_flow_daily` with columns `bucket, router_id, pop_interface_id, upstream_interface_id, direction, asn, bytes, packets`.

Rulings recorded here:
- `asn` is **bigint**, not the spec's "integer": four-byte AS numbers run past PostgreSQL's int4 (2^31−1).
- Compression of `router_flow_minute` starts after **4 days**, past the daily rollup's 3-day refresh window (TimescaleDB requires a refresh window of at least two buckets, so the daily policy reads 3 days back), so refreshes do not decompress.
- `router_routed_prefixes.prefix` is `cidr`, which refuses host bits: writers must store `netip.Prefix.Masked()`.

- [ ] **Step 1: Write the failing Postgres tests**

`backend/internal/database/migrations_flow_postgres_test.go`:

```go
package database

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestFlowTablesAreHypertablesWithRealTimeRollups(t *testing.T) {
	db := freshPostgres(t)

	var hypertables int64
	require.NoError(t, db.Raw(`SELECT count(*) FROM timescaledb_information.hypertables
		WHERE hypertable_schema = current_schema()
		AND hypertable_name IN ('router_flow_minute', 'router_flow_top_ips')`).Scan(&hypertables).Error)
	assert.EqualValues(t, 2, hypertables)

	var realTime int64
	require.NoError(t, db.Raw(`SELECT count(*) FROM timescaledb_information.continuous_aggregates
		WHERE view_schema = current_schema()
		AND view_name IN ('router_flow_5min', 'router_flow_hourly', 'router_flow_daily')
		AND NOT materialized_only`).Scan(&realTime).Error)
	assert.EqualValues(t, 3, realTime)
}

// flowRetention names each flow relation's retention. A rollup's policy sits
// on its materialization hypertable in _timescaledb_internal, so views are
// matched through that.
func flowRetention(t *testing.T, db *gorm.DB) map[string]string {
	t.Helper()
	var rows []struct {
		Name      string
		DropAfter string
	}
	require.NoError(t, db.Raw(`SELECT COALESCE(c.view_name, j.hypertable_name) AS name,
			j.config->>'drop_after' AS drop_after
		FROM timescaledb_information.jobs j
		LEFT JOIN timescaledb_information.continuous_aggregates c
			ON c.materialization_hypertable_schema = j.hypertable_schema
			AND c.materialization_hypertable_name = j.hypertable_name
		WHERE j.proc_name = 'policy_retention'
		AND (j.hypertable_schema = current_schema() OR c.view_schema = current_schema())
		AND COALESCE(c.view_name, j.hypertable_name) LIKE 'router_flow%'`).Scan(&rows).Error)
	out := map[string]string{}
	for _, r := range rows {
		out[r.Name] = r.DropAfter
	}
	return out
}

func TestFlowRetentionMatchesTheSpec(t *testing.T) {
	db := freshPostgres(t)

	assert.Equal(t, map[string]string{
		"router_flow_minute":  "7 days",
		"router_flow_top_ips": "7 days",
		"router_flow_5min":    "90 days",
		"router_flow_hourly":  "2 years",
	}, flowRetention(t, db), "the daily rollup has no retention: it is kept forever")
}

func TestFlowRollupsAddUpTheMinutes(t *testing.T) {
	db := freshPostgres(t)
	routerID, upstream := uuid.New(), uuid.New()
	hour := time.Now().UTC().Add(-48 * time.Hour).Truncate(time.Hour)
	for i := 0; i < 3; i++ {
		require.NoError(t, db.Exec(`INSERT INTO router_flow_minute
			(time, router_id, upstream_interface_id, direction, asn, bytes, packets)
			VALUES (?, ?, ?, 'download', 64496, 1000, 10)`,
			hour.Add(time.Duration(i)*time.Minute), routerID, upstream).Error)
	}

	buckets := map[string]time.Time{
		"router_flow_5min":   hour,
		"router_flow_hourly": hour,
		"router_flow_daily":  hour.Truncate(24 * time.Hour),
	}
	for view, bucket := range buckets {
		require.NoError(t, db.Exec("CALL refresh_continuous_aggregate('"+view+"', NULL, NULL)").Error, view)
		var bytes int64
		require.NoError(t, db.Raw("SELECT bytes FROM "+view+" WHERE router_id = ? AND bucket = ?",
			routerID, bucket).Scan(&bytes).Error, view)
		assert.EqualValues(t, 3000, bytes, view)
	}
}

func TestFlowSamplingAndDirectionAreChecked(t *testing.T) {
	db := freshPostgres(t)
	routerID := insertTestRouter(t, db)

	for _, ratio := range []int{1, 100, 10000} {
		assert.NoError(t, db.Exec(`UPDATE mikrotik_routers SET flow_sampling = ? WHERE id = ?`, ratio, routerID).Error, ratio)
	}
	for _, ratio := range []int{0, 10001} {
		assert.Error(t, db.Exec(`UPDATE mikrotik_routers SET flow_sampling = ? WHERE id = ?`, ratio, routerID).Error, ratio)
	}
	assert.Error(t, db.Exec(`INSERT INTO router_flow_minute
		(time, router_id, upstream_interface_id, direction, asn, bytes, packets)
		VALUES (now(), ?, ?, 'sideways', 64496, 1, 1)`, routerID, uuid.New()).Error)
}

// Routers added before flows existed collect nothing until an Admin turns it
// on, and then sample at the suggested 1:100.
func TestExistingRoutersDefaultToNoFlowsAtOneInAHundred(t *testing.T) {
	db := freshPostgres(t)
	routerID := insertTestRouter(t, db)

	var row struct {
		FlowEnabled  bool
		FlowSampling int
	}
	require.NoError(t, db.Raw(`SELECT flow_enabled, flow_sampling FROM mikrotik_routers WHERE id = ?`,
		routerID).Scan(&row).Error)
	assert.False(t, row.FlowEnabled)
	assert.Equal(t, 100, row.FlowSampling)
}

// A routed block belongs to the interface its gateway sits behind; with that
// interface gone the block means nothing, and it goes too.
func TestRoutedPrefixesGoWithTheirInterface(t *testing.T) {
	db := freshPostgres(t)
	routerID := insertTestRouter(t, db)
	interfaceID := insertTestInterface(t, db, routerID)
	require.NoError(t, db.Exec(`INSERT INTO router_routed_prefixes (id, router_id, prefix, interface_id, updated_at)
		VALUES (?, ?, '203.0.113.64/27', ?, now())`, uuid.New(), routerID, interfaceID).Error)

	require.NoError(t, db.Exec(`DELETE FROM router_interfaces WHERE id = ?`, interfaceID).Error)

	var left int64
	require.NoError(t, db.Raw(`SELECT count(*) FROM router_routed_prefixes WHERE router_id = ?`, routerID).Scan(&left).Error)
	assert.Zero(t, left)
}

// cidr refuses host bits, which is why discovery stores the masked prefix.
func TestRoutedPrefixMustBeANetwork(t *testing.T) {
	db := freshPostgres(t)
	routerID := insertTestRouter(t, db)
	interfaceID := insertTestInterface(t, db, routerID)

	err := db.Exec(`INSERT INTO router_routed_prefixes (id, router_id, prefix, interface_id, updated_at)
		VALUES (?, ?, '203.0.113.70/27', ?, now())`, uuid.New(), routerID, interfaceID).Error

	assert.Error(t, err)
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd backend && TEST_POSTGRES_DSN="postgres://tikman:testpw@localhost:5439/tikman_test?sslmode=disable" go test ./internal/database/ -run 'Flow|RoutedPrefix|OneInAHundred' -v`
Expected: FAIL — the schema build errors or the assertions fail: `relation "router_flow_minute" does not exist`, `column "flow_sampling" does not exist`. (If the whole package fails on `gin_trgm_ops` or a refused `DROP TABLE ont_metrics`, that is stale test-database state: `docker exec tikman-testpg psql -U tikman -d postgres -c 'DROP DATABASE tikman_test' -c 'CREATE DATABASE tikman_test'` and rerun.)

- [ ] **Step 3: Write the models**

`backend/internal/models/flow.go`:

```go
package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// RouterFlowMinute is one minute of traffic for one (router, POP, upstream,
// direction, ASN), already scaled by the sampling ratio. A NULL POP is
// traffic of addresses in no POP's blocks. Like RouterInterfaceMetric it is a
// hypertable without a primary key, and its columns change in SQL migrations
// only: the continuous aggregates read them, and an ALTER COLUMN from
// AutoMigrate would be refused and abort startup.
type RouterFlowMinute struct {
	Time                time.Time  `gorm:"not null;index:idx_router_flow_minute_router_time,priority:2"`
	RouterID            uuid.UUID  `gorm:"type:uuid;not null;index:idx_router_flow_minute_router_time,priority:1"`
	POPInterfaceID      *uuid.UUID `gorm:"column:pop_interface_id;type:uuid"`
	UpstreamInterfaceID uuid.UUID  `gorm:"type:uuid;not null"`
	Direction           string     `gorm:"type:varchar(8);not null"`
	// bigint: four-byte AS numbers run past int4.
	ASN     int64 `gorm:"column:asn;not null"`
	Bytes   int64 `gorm:"not null"`
	Packets int64 `gorm:"not null"`
}

// TableName returns the table name for RouterFlowMinute.
func (RouterFlowMinute) TableName() string { return "router_flow_minute" }

// RouterFlowTopIP is one far-side address's bytes in a five-minute window,
// kept for the window's top addresses only.
type RouterFlowTopIP struct {
	Time           time.Time  `gorm:"not null;index:idx_router_flow_top_ips_router_time,priority:2"`
	RouterID       uuid.UUID  `gorm:"type:uuid;not null;index:idx_router_flow_top_ips_router_time,priority:1"`
	POPInterfaceID *uuid.UUID `gorm:"column:pop_interface_id;type:uuid"`
	Direction      string     `gorm:"type:varchar(8);not null"`
	IP             string     `gorm:"column:ip;type:inet;not null"`
	ASN            int64      `gorm:"column:asn;not null"`
	Bytes          int64      `gorm:"not null"`
}

// TableName returns the table name for RouterFlowTopIP.
func (RouterFlowTopIP) TableName() string { return "router_flow_top_ips" }

// ASNName is an AS's name from the iptoasn dataset, kept so the API can label
// destinations without flowd. UpdatedAt is the dataset's download time.
type ASNName struct {
	ASN       int64     `gorm:"column:asn;primaryKey;autoIncrement:false" json:"asn"`
	Name      string    `gorm:"type:text;not null" json:"name"`
	Country   string    `gorm:"type:varchar(8);not null" json:"country"`
	UpdatedAt time.Time `gorm:"not null" json:"updated_at"`
}

// TableName returns the table name for ASNName.
func (ASNName) TableName() string { return "asn_names" }

// RouterRoutedPrefix is a block a static route hands to a device behind one
// of the router's interfaces, such as a POP router's /27. Discovery refills
// these; Prefix is always the masked network, as cidr requires.
type RouterRoutedPrefix struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	RouterID    uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_router_routed_prefixes_router_prefix"`
	Prefix      string    `gorm:"type:cidr;not null;uniqueIndex:idx_router_routed_prefixes_router_prefix"`
	InterfaceID uuid.UUID `gorm:"type:uuid;not null"`
	UpdatedAt   time.Time
}

func (p *RouterRoutedPrefix) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

// TableName returns the table name for RouterRoutedPrefix.
func (RouterRoutedPrefix) TableName() string { return "router_routed_prefixes" }
```

In `backend/internal/models/mikrotik.go`, add after the `LastSeenAt` field of `MikrotikRouter`:

```go
	// FlowEnabled makes flowd accept IPFIX arriving from Host.
	FlowEnabled bool `gorm:"not null;default:false" json:"flow_enabled"`
	// FlowSampling is the router's 1:N packet sampling. It must match
	// /ip traffic-flow, or every byte count is off by the difference.
	FlowSampling   int        `gorm:"not null;default:100" json:"flow_sampling"`
	FlowLastSeenAt *time.Time `json:"flow_last_seen_at"`
```

In `backend/internal/models/models.go`, append to the `db.AutoMigrate(` list after `&RouterPollJob{},`:

```go
		&RouterFlowMinute{},
		&RouterFlowTopIP{},
		&ASNName{},
		&RouterRoutedPrefix{},
```

- [ ] **Step 4: Write migration 58**

`backend/migrations/58_mikrotik_traffic_flows.sql`:

```sql
-- Traffic destinations from IPFIX (flowd). AutoMigrate builds the tables and
-- columns from internal/models/flow.go and mikrotik.go; this adds what tags
-- cannot express: checks, foreign keys, hypertables, rollups and retention.

ALTER TABLE mikrotik_routers ADD COLUMN IF NOT EXISTS flow_enabled boolean NOT NULL DEFAULT false;
ALTER TABLE mikrotik_routers ADD COLUMN IF NOT EXISTS flow_sampling integer NOT NULL DEFAULT 100;
ALTER TABLE mikrotik_routers ADD COLUMN IF NOT EXISTS flow_last_seen_at timestamptz;

ALTER TABLE mikrotik_routers DROP CONSTRAINT IF EXISTS mikrotik_routers_flow_sampling_check;
ALTER TABLE mikrotik_routers ADD CONSTRAINT mikrotik_routers_flow_sampling_check
    CHECK (flow_sampling BETWEEN 1 AND 10000);

ALTER TABLE router_routed_prefixes DROP CONSTRAINT IF EXISTS router_routed_prefixes_router_fk;
ALTER TABLE router_routed_prefixes ADD CONSTRAINT router_routed_prefixes_router_fk
    FOREIGN KEY (router_id) REFERENCES mikrotik_routers(id) ON DELETE CASCADE;

-- Without its interface a routed block belongs to no POP; the next discovery
-- stores it again under whatever interface now holds the gateway.
ALTER TABLE router_routed_prefixes DROP CONSTRAINT IF EXISTS router_routed_prefixes_interface_fk;
ALTER TABLE router_routed_prefixes ADD CONSTRAINT router_routed_prefixes_interface_fk
    FOREIGN KEY (interface_id) REFERENCES router_interfaces(id) ON DELETE CASCADE;

ALTER TABLE router_flow_minute DROP CONSTRAINT IF EXISTS router_flow_minute_direction_check;
ALTER TABLE router_flow_minute ADD CONSTRAINT router_flow_minute_direction_check
    CHECK (direction IN ('download', 'upload'));

ALTER TABLE router_flow_top_ips DROP CONSTRAINT IF EXISTS router_flow_top_ips_direction_check;
ALTER TABLE router_flow_top_ips ADD CONSTRAINT router_flow_top_ips_direction_check
    CHECK (direction IN ('download', 'upload'));

-- No foreign keys from the flow tables, as for router_interface_metrics: a
-- cascading delete into compressed chunks is what TimescaleDB handles worst,
-- and every read filters by router_id.
SELECT create_hypertable('router_flow_minute', 'time',
    chunk_time_interval => INTERVAL '1 day',
    if_not_exists => TRUE);

ALTER TABLE router_flow_minute SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'router_id',
    timescaledb.compress_orderby = 'time DESC'
);

-- Four days: past the daily rollup's three-day refresh window, so no refresh
-- reads compressed chunks.
SELECT add_compression_policy('router_flow_minute', INTERVAL '4 days', if_not_exists => TRUE);
SELECT add_retention_policy('router_flow_minute', INTERVAL '7 days', if_not_exists => TRUE);

SELECT create_hypertable('router_flow_top_ips', 'time',
    chunk_time_interval => INTERVAL '1 day',
    if_not_exists => TRUE);
SELECT add_retention_policy('router_flow_top_ips', INTERVAL '7 days', if_not_exists => TRUE);

-- Real-time aggregates, as for the interface rollups: a read unions the
-- materialized buckets with raw minutes past the last refresh.
CREATE MATERIALIZED VIEW IF NOT EXISTS router_flow_5min
WITH (timescaledb.continuous, timescaledb.materialized_only = false) AS
SELECT
    time_bucket('5 minutes', time) AS bucket,
    router_id,
    pop_interface_id,
    upstream_interface_id,
    direction,
    asn,
    SUM(bytes) AS bytes,
    SUM(packets) AS packets
FROM router_flow_minute
GROUP BY bucket, router_id, pop_interface_id, upstream_interface_id, direction, asn
WITH NO DATA;

SELECT add_continuous_aggregate_policy('router_flow_5min',
    start_offset => INTERVAL '1 hour',
    end_offset => INTERVAL '5 minutes',
    schedule_interval => INTERVAL '5 minutes',
    if_not_exists => TRUE);
SELECT add_retention_policy('router_flow_5min', INTERVAL '90 days', if_not_exists => TRUE);

CREATE MATERIALIZED VIEW IF NOT EXISTS router_flow_hourly
WITH (timescaledb.continuous, timescaledb.materialized_only = false) AS
SELECT
    time_bucket('1 hour', time) AS bucket,
    router_id,
    pop_interface_id,
    upstream_interface_id,
    direction,
    asn,
    SUM(bytes) AS bytes,
    SUM(packets) AS packets
FROM router_flow_minute
GROUP BY bucket, router_id, pop_interface_id, upstream_interface_id, direction, asn
WITH NO DATA;

SELECT add_continuous_aggregate_policy('router_flow_hourly',
    start_offset => INTERVAL '6 hours',
    end_offset => INTERVAL '1 hour',
    schedule_interval => INTERVAL '30 minutes',
    if_not_exists => TRUE);
SELECT add_retention_policy('router_flow_hourly', INTERVAL '2 years', if_not_exists => TRUE);

-- Kept forever: years of planning data at one row per day per key. A refresh
-- window must span at least two buckets, hence three days back to one.
CREATE MATERIALIZED VIEW IF NOT EXISTS router_flow_daily
WITH (timescaledb.continuous, timescaledb.materialized_only = false) AS
SELECT
    time_bucket('1 day', time) AS bucket,
    router_id,
    pop_interface_id,
    upstream_interface_id,
    direction,
    asn,
    SUM(bytes) AS bytes,
    SUM(packets) AS packets
FROM router_flow_minute
GROUP BY bucket, router_id, pop_interface_id, upstream_interface_id, direction, asn
WITH NO DATA;

SELECT add_continuous_aggregate_policy('router_flow_daily',
    start_offset => INTERVAL '3 days',
    end_offset => INTERVAL '1 day',
    schedule_interval => INTERVAL '1 hour',
    if_not_exists => TRUE);
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `cd backend && go build ./... && go test ./internal/models/ ./internal/services/ -count=1 && TEST_POSTGRES_DSN="postgres://tikman:testpw@localhost:5439/tikman_test?sslmode=disable" go test ./internal/database/ -count=1 -v -run 'Flow|RoutedPrefix|OneInAHundred|Mikrotik|Router'`
Expected: PASS — the 7 new tests and the existing MikroTik migration tests. The SQLite service suite still passes with the new models in AutoMigrate.

- [ ] **Step 6: Commit**

```bash
cd backend && gofmt -s -l ./internal
git add internal/models internal/database/migrations_flow_postgres_test.go migrations/58_mikrotik_traffic_flows.sql
git commit -m "feat(flows): flow tables, rollups and retention (migration 58)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 6: Static routes in discovery

**Files:**
- Modify: `backend/internal/mikrotik/types.go` (add `Route`)
- Modify: `backend/internal/mikrotik/client.go` (`Transport.List` gains `where ...string`; add `StaticRoutes`)
- Modify: `backend/internal/mikrotik/rest.go` (`restTransport.List`)
- Modify: `backend/internal/mikrotik/api.go` (`apiTransport.List`)
- Modify: `backend/internal/services/router_poll.go` (`RouterAPI` gains `StaticRoutes`)
- Modify: `backend/internal/services/router_discovery.go` (`routerInventory`, `readInventory`, `Discover`)
- Create: `backend/internal/services/router_routes.go`
- Modify: `backend/internal/services/router_poll_test.go` (`fakeRouterAPI`)
- Test: `backend/internal/mikrotik/client_test.go`, `backend/internal/mikrotik/api_test.go`, `backend/internal/services/router_routes_test.go`

**Interfaces:**
- Consumes: `models.RouterRoutedPrefix` (Task 5); existing `MatchInterface(remote string, ifaces []models.RouterInterface) *uuid.UUID` (`router_bgp.go`), `borderInventory`, `newPollFixture`, `fakeRouterAPI` (services tests), `openREST`, `tlsRouter`, `fakeAPIRouter`, `apiConfig` (mikrotik tests).
- Produces: `mikrotik.Route{DstAddress, Gateway, ImmediateGW, Active, Static, Connect, BGP, Disabled string}`; `Transport.List(path string, out interface{}, where ...string) error` where each `where` is `"key=value"`; `(*mikrotik.Client).StaticRoutes() ([]mikrotik.Route, error)`; `RouterAPI.StaticRoutes()`; rows in `router_routed_prefixes` after every discovery.

Infeasible as written, resolved here: the spec has discovery read `/ip/route` and keep the static ones. On a border router with full BGP tables that menu is about a million rows — past the API client's 100,000-row and 16 MiB reply caps, and a multi-hundred-MB JSON over REST within a 5-second timeout. The filter therefore runs **on the router**: REST query parameters (`/rest/ip/route?active=true&static=true`) and API query words (`?static=true`, `?active=true`, which RouterOS ANDs). The flags are checked again client-side.

- [ ] **Step 1: Write the failing mikrotik tests**

Append to `backend/internal/mikrotik/client_test.go`:

```go
// A border router's /ip/route holds the full BGP table: the filter has to
// travel to the router, not run here over a million rows.
func TestStaticRoutesAskTheRouterToFilterOverREST(t *testing.T) {
	server, host, port := tlsRouter(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/rest/ip/route", r.URL.Path)
		assert.Equal(t, "true", r.URL.Query().Get("static"))
		assert.Equal(t, "true", r.URL.Query().Get("active"))
		_, _ = w.Write([]byte(`[{"dst-address":"203.0.113.64/27","gateway":"192.0.2.34",
			"immediate-gw":"192.0.2.34%vlan-pop-a","active":"true","static":"true"}]`))
	})

	routes, err := openREST(t, host, port, "tikman", "secret", Fingerprint(server.Certificate().Raw)).StaticRoutes()

	require.NoError(t, err)
	require.Len(t, routes, 1)
	assert.Equal(t, "203.0.113.64/27", routes[0].DstAddress)
	assert.Equal(t, "192.0.2.34%vlan-pop-a", routes[0].ImmediateGW)
}
```

Append to `backend/internal/mikrotik/api_test.go`:

```go
func TestAPIStaticRoutesSendTheFilterAsQueryWords(t *testing.T) {
	router := &fakeAPIRouter{username: "tikman", password: "secret", replies: map[string][][]string{
		"/ip/route/print": {
			{"!re", "=dst-address=203.0.113.64/27", "=gateway=192.0.2.34", "=static=true", "=active=true"},
			{"!done"},
		},
	}}
	client, err := Open(apiConfig(router.start(t, nil)))
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	routes, err := client.StaticRoutes()

	require.NoError(t, err)
	require.Len(t, routes, 1)
	assert.Equal(t, "203.0.113.64/27", routes[0].DstAddress)
	assert.Equal(t, "true", routes[0].Static)
	<-router.received // login
	assert.Equal(t, []string{"/ip/route/print", "?static=true", "?active=true"}, <-router.received)
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `cd backend && go test ./internal/mikrotik/ -run 'StaticRoutes' -v`
Expected: FAIL to compile — `client.StaticRoutes undefined`.

- [ ] **Step 3: Implement the filtered read**

In `backend/internal/mikrotik/types.go`, append:

```go
// Route is one row of /ip/route. It is only ever read filtered: a border
// router's full BGP table is a million rows.
type Route struct {
	DstAddress  string `json:"dst-address"`
	Gateway     string `json:"gateway"`
	ImmediateGW string `json:"immediate-gw"`
	Active      string `json:"active"`
	Static      string `json:"static"`
	Connect     string `json:"connect"`
	BGP         string `json:"bgp"`
	Disabled    string `json:"disabled"`
}
```

In `backend/internal/mikrotik/client.go`, replace the `Transport` interface with:

```go
// Transport reads one RouterOS menu. Paths are the menu paths both REST and
// the API use, such as "/routing/bgp/session". Each where is "key=value", a
// filter the router applies before replying.
type Transport interface {
	List(path string, out interface{}, where ...string) error
	Close() error
}
```

and append:

```go
// StaticRoutes lists active static routes, filtered on the router.
func (c *Client) StaticRoutes() ([]Route, error) {
	var out []Route
	err := c.transport.List("/ip/route", &out, "static=true", "active=true")
	return out, err
}
```

In `backend/internal/mikrotik/rest.go`, add `"net/url"` to the imports, replace the `List` doc comment, signature and first line with:

```go
// List is GET /rest<path>, with each where as a query filter the router
// applies itself.
func (t *restTransport) List(path string, out interface{}, where ...string) error {
	target := t.baseURL + path
	if len(where) > 0 {
		target += "?" + restFilter(where)
	}
	req, err := http.NewRequest(http.MethodGet, target, nil)
```

(the rest of the function is unchanged), and append:

```go
func restFilter(where []string) string {
	query := url.Values{}
	for _, w := range where {
		key, value, _ := strings.Cut(w, "=")
		query.Set(key, value)
	}
	return query.Encode()
}
```

In `backend/internal/mikrotik/api.go`, replace `List` with:

```go
// List reads a menu with "<path>/print". Each where becomes a "?" query word;
// RouterOS ANDs them and returns only matching rows. The rows carry the same
// property names REST uses, so they decode into the same structs.
func (t *apiTransport) List(path string, out interface{}, where ...string) error {
	words := []string{path + "/print"}
	for _, w := range where {
		words = append(words, "?"+w)
	}
	rows, err := t.run(words...)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return decodeRows(rows, out)
}
```

- [ ] **Step 4: Run the mikrotik tests**

Run: `cd backend && go test ./internal/mikrotik/ -v -race`
Expected: PASS, including every earlier REST and API test.

- [ ] **Step 5: Write the failing discovery tests**

In `backend/internal/services/router_poll_test.go`, add `routes []mikrotik.Route` to `fakeRouterAPI` (after `sessions`) and the method:

```go
func (f *fakeRouterAPI) StaticRoutes() ([]mikrotik.Route, error) { return f.routes, f.err }
```

Create `backend/internal/services/router_routes_test.go`:

```go
package services

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/mikrotik"
	"github.com/tikman/olt-provisioning/internal/models"
)

// popRoutes is what /ip/route returns on borderInventory's router: two POP
// blocks, and every kind of route that is not one.
func popRoutes() []mikrotik.Route {
	return []mikrotik.Route{
		// A POP router's block, its interface named by immediate-gw.
		{DstAddress: "203.0.113.64/27", Gateway: "192.0.2.34", ImmediateGW: "192.0.2.34%vlan-pop-a", Static: "true", Active: "true"},
		// Only a gateway address, inside vlan-pop-a's second subnet; stored masked.
		{DstAddress: "203.0.113.130/26", Gateway: "192.0.2.58", Static: "true", Active: "true"},
		// The default route points at an upstream, not at a POP.
		{DstAddress: "0.0.0.0/0", Gateway: "198.51.100.0", Static: "true", Active: "true"},
		// A blackhole for the aggregate leaves through no interface.
		{DstAddress: "203.0.113.0/24", Static: "true", Active: "true"},
		// Flags the router-side filter should already have removed.
		{DstAddress: "198.51.100.128/25", Gateway: "198.51.100.0", Static: "true", Active: "true", BGP: "true"},
		{DstAddress: "192.0.2.32/29", Gateway: "vlan-pop-a", Static: "true", Active: "true", Connect: "true"},
		{DstAddress: "203.0.113.192/28", Gateway: "192.0.2.34", Static: "true", Active: "false"},
	}
}

// storedPrefixes maps each routed prefix of the fixture's router to the name
// of the interface it was stored under.
func storedPrefixes(t *testing.T, f *pollFixture) map[string]string {
	t.Helper()
	var prefixes []models.RouterRoutedPrefix
	require.NoError(t, f.db.Where("router_id = ?", f.router.ID).Find(&prefixes).Error)
	var ifaces []models.RouterInterface
	require.NoError(t, f.db.Where("router_id = ?", f.router.ID).Find(&ifaces).Error)
	names := map[uuid.UUID]string{}
	for _, i := range ifaces {
		names[i.ID] = i.Name
	}
	out := map[string]string{}
	for _, p := range prefixes {
		out[p.Prefix] = names[p.InterfaceID]
	}
	return out
}

func TestDiscoverStoresStaticRoutesUnderTheirGatewayInterface(t *testing.T) {
	f := newPollFixture(t)
	borderInventory(f.api)
	f.api.routes = popRoutes()

	require.NoError(t, f.service.Discover(f.router))

	assert.Equal(t, map[string]string{
		"203.0.113.64/27":  "vlan-pop-a",
		"203.0.113.128/26": "vlan-pop-a",
	}, storedPrefixes(t, f))
}

func TestDiscoverDropsRoutesGoneFromTheRouter(t *testing.T) {
	f := newPollFixture(t)
	borderInventory(f.api)
	f.api.routes = popRoutes()
	require.NoError(t, f.service.Discover(f.router))

	f.api.routes = popRoutes()[:1]
	require.NoError(t, f.service.Discover(f.router))

	assert.Equal(t, map[string]string{"203.0.113.64/27": "vlan-pop-a"}, storedPrefixes(t, f))
}
```

- [ ] **Step 6: Run them to verify they fail**

Run: `cd backend && go test ./internal/services/ -run 'Discover' -v`
Expected: FAIL — both new tests get an empty map: discovery does not read routes yet (the fake's extra method compiles; nothing calls it).

- [ ] **Step 7: Implement route discovery**

In `backend/internal/services/router_poll.go`, add to the `RouterAPI` interface after `BGPSessions`:

```go
	StaticRoutes() ([]mikrotik.Route, error)
```

In `backend/internal/services/router_discovery.go`:

- add `routes []mikrotik.Route` to `routerInventory`;
- replace the end of `readInventory` (`inventory.addresses, err = api.Addresses()` / `return inventory, err`) with:

```go
	if inventory.addresses, err = api.Addresses(); err != nil {
		return inventory, err
	}
	inventory.routes, err = api.StaticRoutes()
	return inventory, err
```

- in `Discover`, replace `return rematchSessions(tx, router.ID)` inside the transaction with:

```go
		if err := rematchSessions(tx, router.ID); err != nil {
			return err
		}
		return replaceRoutedPrefixes(tx, router.ID, inventory.routes)
```

Create `backend/internal/services/router_routes.go`:

```go
package services

import (
	"net/netip"
	"strings"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/mikrotik"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// routedPrefixBatch is rows per INSERT; a border router has tens of static
// routes, not thousands.
const routedPrefixBatch = 200

// replaceRoutedPrefixes refills the router's routed blocks from this
// discovery's routes: a route removed on the router disappears here.
func replaceRoutedPrefixes(tx *gorm.DB, routerID uuid.UUID, routes []mikrotik.Route) error {
	var ifaces []models.RouterInterface
	if err := tx.Where("router_id = ?", routerID).Find(&ifaces).Error; err != nil {
		return err
	}
	if err := tx.Where("router_id = ?", routerID).Delete(&models.RouterRoutedPrefix{}).Error; err != nil {
		return err
	}
	rows := buildRoutedPrefixes(routerID, routes, ifaces)
	if len(rows) == 0 {
		return nil
	}
	return tx.CreateInBatches(rows, routedPrefixBatch).Error
}

// buildRoutedPrefixes keeps the static routes that hand a block to a device
// behind one of the router's interfaces, keyed to that interface.
func buildRoutedPrefixes(routerID uuid.UUID, routes []mikrotik.Route, ifaces []models.RouterInterface) []models.RouterRoutedPrefix {
	byName := make(map[string]uuid.UUID, len(ifaces))
	for _, i := range ifaces {
		byName[i.Name] = i.ID
	}
	seen := map[string]bool{}
	var rows []models.RouterRoutedPrefix
	for _, r := range routes {
		prefix, ok := routedPrefix(r)
		if !ok || seen[prefix] {
			continue
		}
		interfaceID := routeInterface(r, byName, ifaces)
		if interfaceID == nil {
			continue
		}
		seen[prefix] = true
		rows = append(rows, models.RouterRoutedPrefix{RouterID: routerID, Prefix: prefix, InterfaceID: *interfaceID})
	}
	return rows
}

// routedPrefix returns the masked block of a route that can be a POP's. The
// router already filtered on static and active; the flags are checked again
// because a BGP route slipping through would hand an upstream's whole table
// to a POP.
func routedPrefix(r mikrotik.Route) (string, bool) {
	if !mikrotik.ParseBool(r.Static) || !mikrotik.ParseBool(r.Active) || mikrotik.ParseBool(r.BGP) ||
		mikrotik.ParseBool(r.Connect) || mikrotik.ParseBool(r.Disabled) {
		return "", false
	}
	p, err := netip.ParsePrefix(r.DstAddress)
	// A default route points at an upstream, never at a POP's block.
	if err != nil || !p.Addr().Is4() || p.Bits() == 0 {
		return "", false
	}
	return p.Masked().String(), true
}

// routeInterface finds the interface a route leaves through: the one
// immediate-gw names ("192.0.2.34%vlan-pop-a"), else the one whose subnet
// holds the gateway, else a gateway that is itself an interface name.
func routeInterface(r mikrotik.Route, byName map[string]uuid.UUID, ifaces []models.RouterInterface) *uuid.UUID {
	if _, name, ok := strings.Cut(r.ImmediateGW, "%"); ok {
		if id, known := byName[name]; known {
			return &id
		}
	}
	if id := MatchInterface(r.Gateway, ifaces); id != nil {
		return id
	}
	if id, known := byName[r.Gateway]; known {
		return &id
	}
	return nil
}
```

- [ ] **Step 8: Run the tests to verify they pass**

Run: `cd backend && go build ./... && go test ./internal/services/ ./internal/mikrotik/ -race -count=1`
Expected: PASS, including the existing discovery and poll tests.

- [ ] **Step 9: Commit**

```bash
cd backend && gofmt -s -l ./internal
git add internal/mikrotik internal/services/router_poll.go internal/services/router_poll_test.go \
  internal/services/router_discovery.go internal/services/router_routes.go internal/services/router_routes_test.go
git commit -m "feat(mikrotik): discover static POP routes, filtered on the router

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 7: Flow mapping loader and flow store

**Files:**
- Create: `backend/internal/services/flow_mapping.go`
- Create: `backend/internal/services/flow_store.go`
- Test: `backend/internal/services/flow_mapping_test.go`, `backend/internal/services/flow_store_test.go`

**Interfaces:**
- Consumes: `netflow.RouterMap`, `netflow.PrefixEntry`, `netflow.NewPrefixTable` (Task 3); `netflow.MinuteRow`, `netflow.TopIPRow`, `netflow.Download/Upload` (Tasks 3–4); `netflow.ASNInfo` (Task 2); models from Task 5; existing test helpers `newRouterServiceForTest`, `validRouterInput`, `routerTestKey`, `setupTestDB`.
- Produces: `func NewFlowMappingService(db *gorm.DB) *FlowMappingService`; `(*FlowMappingService).Load() (map[netip.Addr]*netflow.RouterMap, []string, error)` — maps keyed by the router's host address, plus the names of collecting routers whose host is not an IP literal; `func NewFlowStore(db *gorm.DB) *FlowStore` with `WriteMinute(ctx context.Context, rows []netflow.MinuteRow) error`, `WriteTopIPs(ctx context.Context, rows []netflow.TopIPRow) error`, `MarkSeen(ctx context.Context, routerIDs []uuid.UUID, at time.Time) error`, `SaveASNNames(ctx context.Context, names map[uint32]netflow.ASNInfo, updatedAt time.Time) error`.

Ruling recorded here: a router whose `host` is a DNS name cannot be matched — an IPFIX datagram carries only its source address. `Load` skips it and returns its name so flowd logs it; the operator guide says to enter the router by IP.

- [ ] **Step 1: Write the failing mapping tests**

`backend/internal/services/flow_mapping_test.go`:

```go
package services

import (
	"net/netip"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/netflow"
	"gorm.io/gorm"
)

type mappingFixture struct {
	db      *gorm.DB
	routers *RouterService
	router  *models.MikrotikRouter
	transit models.RouterInterface
	ix      models.RouterInterface
	popA    models.RouterInterface
}

// newMappingFixture is a collecting router at 192.0.2.1 with two upstreams
// (each carrying a BGP session) and one POP VLAN with two blocks.
func newMappingFixture(t *testing.T) *mappingFixture {
	t.Helper()
	routers, db := newRouterServiceForTest(t)
	router, err := routers.Create(validRouterInput())
	require.NoError(t, err)
	require.NoError(t, db.Model(router).Updates(map[string]interface{}{"flow_enabled": true, "flow_sampling": 512}).Error)

	f := &mappingFixture{db: db, routers: routers, router: router}
	f.transit = f.iface(t, "vlan-transit", "198.51.100.1/31", 19)
	f.ix = f.iface(t, "vlan-ix", "203.0.113.141/26", 20)
	f.popA = f.iface(t, "vlan-pop-a", "192.0.2.33/29,192.0.2.57/29", 21)
	f.session(t, "peer-transit", f.transit.ID)
	f.session(t, "peer-ix", f.ix.ID)
	return f
}

func (f *mappingFixture) iface(t *testing.T, name, addresses string, ifIndex int) models.RouterInterface {
	t.Helper()
	iface := models.RouterInterface{RouterID: f.router.ID, Name: name, Type: "vlan",
		Addresses: addresses, IfIndex: &ifIndex, OperStatus: "up"}
	require.NoError(t, f.db.Create(&iface).Error)
	return iface
}

func (f *mappingFixture) session(t *testing.T, name string, interfaceID uuid.UUID) {
	t.Helper()
	require.NoError(t, f.db.Create(&models.RouterBGPSession{RouterID: f.router.ID, Name: name,
		RemoteAddress: "198.51.100.0", RemoteAS: 64500, State: models.BGPStateEstablished,
		InterfaceID: &interfaceID}).Error)
}

func (f *mappingFixture) load(t *testing.T) *netflow.RouterMap {
	t.Helper()
	maps, skipped, err := NewFlowMappingService(f.db).Load()
	require.NoError(t, err)
	assert.Empty(t, skipped)
	return maps[netip.MustParseAddr("192.0.2.1")]
}

func TestLoadMapsUpstreamsAndPOPBlocks(t *testing.T) {
	f := newMappingFixture(t)
	require.NoError(t, f.db.Create(&models.RouterRoutedPrefix{RouterID: f.router.ID,
		Prefix: "203.0.113.64/27", InterfaceID: f.popA.ID}).Error)

	m := f.load(t)

	require.NotNil(t, m)
	assert.Equal(t, f.router.ID, m.RouterID)
	assert.Equal(t, uint64(512), m.Sampling)
	assert.Equal(t, map[uint32]uuid.UUID{19: f.transit.ID, 20: f.ix.ID}, m.Upstreams)
	cases := map[string]uuid.UUID{
		"192.0.2.38":   f.popA.ID, // interface address
		"192.0.2.60":   f.popA.ID, // its second block
		"203.0.113.70": f.popA.ID, // routed to a device behind it
		"198.51.100.0": uuid.Nil,  // an upstream's subnet is no POP
	}
	for ip, want := range cases {
		got, _ := m.POPs.Lookup(netip.MustParseAddr(ip))
		assert.Equal(t, want, got, ip)
	}
}

func TestLoadSkipsRoutersNotCollectingAndNamesHostnames(t *testing.T) {
	f := newMappingFixture(t)
	named := validRouterInput()
	named.Name, named.Host = "border-2", "border-2.example.net"
	byName, err := f.routers.Create(named)
	require.NoError(t, err)
	require.NoError(t, f.db.Model(byName).Update("flow_enabled", true).Error)
	quiet := validRouterInput()
	quiet.Name, quiet.Host = "border-3", "192.0.2.2"
	_, err = f.routers.Create(quiet)
	require.NoError(t, err)

	maps, skipped, err := NewFlowMappingService(f.db).Load()

	require.NoError(t, err)
	assert.Equal(t, []string{"border-2"}, skipped, "IPFIX carries an address, never a name")
	assert.Len(t, maps, 1, "a router without flow collection is not listened to")

	require.NoError(t, f.db.Model(f.router).Update("enabled", false).Error)
	maps, _, err = NewFlowMappingService(f.db).Load()
	require.NoError(t, err)
	assert.Empty(t, maps, "a disabled router is not listened to")
}

// Review Focus 2: RouterOS renumbers ifIndex after a reboot; the interfaces
// poll stores the new number and the next load follows it.
func TestLoadFollowsARenumberedIfIndex(t *testing.T) {
	f := newMappingFixture(t)
	require.NoError(t, f.db.Model(&f.transit).Update("if_index", 42).Error)

	m := f.load(t)

	assert.Equal(t, f.transit.ID, m.Upstreams[42])
	assert.NotContains(t, m.Upstreams, uint32(19))
}

// Review Focus 2: a disabled interface is not polled, so its stored ifIndex
// may since have been given to another; and an index two upstreams both
// claim is dropped rather than guessed.
func TestLoadNeverGuessesAnUpstreamFromAStaleIfIndex(t *testing.T) {
	f := newMappingFixture(t)
	require.NoError(t, f.db.Model(&f.ix).Update("if_index", 19).Error)
	assert.NotContains(t, f.load(t).Upstreams, uint32(19), "two upstreams claim 19")

	require.NoError(t, f.db.Model(&f.ix).Updates(map[string]interface{}{"if_index": 20, "disabled": true}).Error)
	m := f.load(t)
	assert.NotContains(t, m.Upstreams, uint32(20))
	assert.Equal(t, f.transit.ID, m.Upstreams[19])
}

// An Admin changing the ratio while flowd runs: the next load carries it.
func TestLoadReflectsTheCurrentSamplingRatio(t *testing.T) {
	f := newMappingFixture(t)
	require.NoError(t, f.db.Model(f.router).Update("flow_sampling", 1000).Error)

	assert.Equal(t, uint64(1000), f.load(t).Sampling)
}
```

- [ ] **Step 2: Write the failing store tests**

`backend/internal/services/flow_store_test.go`:

```go
package services

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/netflow"
)

var flowMinute = time.Date(2026, 10, 12, 3, 0, 0, 0, time.UTC)

func TestWriteMinuteStoresTheOtherPOPAsNull(t *testing.T) {
	db := setupTestDB(t)
	routerID, pop, upstream := uuid.New(), uuid.New(), uuid.New()
	rows := []netflow.MinuteRow{
		{Time: flowMinute, MinuteKey: netflow.MinuteKey{RouterID: routerID, POP: pop, Upstream: upstream,
			Direction: netflow.Download, ASN: 64496}, Bytes: 1500, Packets: 1},
		{Time: flowMinute, MinuteKey: netflow.MinuteKey{RouterID: routerID, Upstream: upstream,
			Direction: netflow.Upload, ASN: netflow.OtherASN}, Bytes: 80, Packets: 1},
	}

	require.NoError(t, NewFlowStore(db).WriteMinute(context.Background(), rows))

	var stored []models.RouterFlowMinute
	require.NoError(t, db.Order("bytes DESC").Find(&stored).Error)
	require.Len(t, stored, 2)
	require.NotNil(t, stored[0].POPInterfaceID)
	assert.Equal(t, pop, *stored[0].POPInterfaceID)
	assert.Equal(t, "download", stored[0].Direction)
	assert.EqualValues(t, 64496, stored[0].ASN)
	assert.Nil(t, stored[1].POPInterfaceID, "uuid.Nil is the Lainnya POP")
	assert.Equal(t, "upload", stored[1].Direction)
}

func TestWriteTopIPsStoresTheAddress(t *testing.T) {
	db := setupTestDB(t)
	routerID := uuid.New()
	rows := []netflow.TopIPRow{{Time: flowMinute, RouterID: routerID, Direction: netflow.Download,
		IP: netip.MustParseAddr("203.0.113.5"), ASN: 64498, Bytes: 9000}}

	require.NoError(t, NewFlowStore(db).WriteTopIPs(context.Background(), rows))

	var stored models.RouterFlowTopIP
	require.NoError(t, db.First(&stored, "router_id = ?", routerID).Error)
	assert.Equal(t, "203.0.113.5", stored.IP)
	assert.Nil(t, stored.POPInterfaceID)
	assert.EqualValues(t, 9000, stored.Bytes)
}

func TestMarkSeenTouchesOnlyTheGivenRouters(t *testing.T) {
	routers, db := newRouterServiceForTest(t)
	heard, err := routers.Create(validRouterInput())
	require.NoError(t, err)
	other := validRouterInput()
	other.Name, other.Host = "border-2", "192.0.2.2"
	silent, err := routers.Create(other)
	require.NoError(t, err)

	require.NoError(t, NewFlowStore(db).MarkSeen(context.Background(), []uuid.UUID{heard.ID}, flowMinute))

	stored, err := routers.Get(heard.ID)
	require.NoError(t, err)
	require.NotNil(t, stored.FlowLastSeenAt)
	assert.True(t, flowMinute.Equal(*stored.FlowLastSeenAt))
	quiet, err := routers.Get(silent.ID)
	require.NoError(t, err)
	assert.Nil(t, quiet.FlowLastSeenAt)
}

func TestSaveASNNamesReplacesNamesItAlreadyHas(t *testing.T) {
	db := setupTestDB(t)
	store := NewFlowStore(db)
	ctx := context.Background()
	require.NoError(t, store.SaveASNNames(ctx, map[uint32]netflow.ASNInfo{64496: {Name: "OLD-NAME", Country: "ZZ"}}, flowMinute))

	later := flowMinute.Add(24 * time.Hour)
	require.NoError(t, store.SaveASNNames(ctx, map[uint32]netflow.ASNInfo{
		64496: {Name: "EXAMPLE-NET-A", Country: "ZZ"}, 64497: {Name: "EXAMPLE-NET-B", Country: "ZZ"},
	}, later))

	var names []models.ASNName
	require.NoError(t, db.Order("asn").Find(&names).Error)
	require.Len(t, names, 2)
	assert.Equal(t, "EXAMPLE-NET-A", names[0].Name)
	assert.True(t, later.Equal(names[0].UpdatedAt))
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `cd backend && go test ./internal/services/ -run 'TestLoad|WriteMinute|WriteTopIPs|MarkSeen|SaveASNNames' -v`
Expected: FAIL to compile — `undefined: NewFlowMappingService`, `undefined: NewFlowStore`.

- [ ] **Step 4: Write the mapping loader**

`backend/internal/services/flow_mapping.go`:

```go
package services

import (
	"net/netip"
	"strings"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/netflow"
	"gorm.io/gorm"
)

// FlowMappingService builds flowd's view of each router from what discovery
// and the interfaces poll stored.
type FlowMappingService struct {
	db *gorm.DB
}

// NewFlowMappingService creates a mapping loader.
func NewFlowMappingService(db *gorm.DB) *FlowMappingService {
	return &FlowMappingService{db: db}
}

// Load returns a map for every enabled router collecting flows, keyed by the
// address its IPFIX arrives from, and the names of those whose host is not an
// address: IPFIX carries only the sender's address, so they cannot be matched.
func (s *FlowMappingService) Load() (map[netip.Addr]*netflow.RouterMap, []string, error) {
	var routers []models.MikrotikRouter
	if err := s.db.Where("enabled = ? AND flow_enabled = ?", true, true).Order("name").Find(&routers).Error; err != nil {
		return nil, nil, err
	}
	maps := make(map[netip.Addr]*netflow.RouterMap, len(routers))
	var skipped []string
	for i := range routers {
		addr, err := netip.ParseAddr(routers[i].Host)
		if err != nil {
			skipped = append(skipped, routers[i].Name)
			continue
		}
		m, err := s.routerMap(&routers[i])
		if err != nil {
			return nil, nil, err
		}
		maps[addr.Unmap()] = m
	}
	return maps, skipped, nil
}

func (s *FlowMappingService) routerMap(router *models.MikrotikRouter) (*netflow.RouterMap, error) {
	var ifaces []models.RouterInterface
	if err := s.db.Where("router_id = ?", router.ID).Order("name").Find(&ifaces).Error; err != nil {
		return nil, err
	}
	upstream, err := s.upstreamIDs(router.ID)
	if err != nil {
		return nil, err
	}
	var routed []models.RouterRoutedPrefix
	if err := s.db.Where("router_id = ?", router.ID).Order("prefix").Find(&routed).Error; err != nil {
		return nil, err
	}
	return &netflow.RouterMap{
		RouterID:  router.ID,
		Sampling:  uint64(max(router.FlowSampling, 1)),
		Upstreams: upstreamIndexes(ifaces, upstream),
		POPs:      netflow.NewPrefixTable(popPrefixes(ifaces, routed, upstream)),
	}, nil
}

// upstreamIDs are the interfaces discovery matched to a BGP session: where
// the router meets its upstreams.
func (s *FlowMappingService) upstreamIDs(routerID uuid.UUID) (map[uuid.UUID]bool, error) {
	var ids []uuid.UUID
	err := s.db.Model(&models.RouterBGPSession{}).
		Where("router_id = ? AND interface_id IS NOT NULL", routerID).
		Distinct("interface_id").Pluck("interface_id", &ids).Error
	set := make(map[uuid.UUID]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set, err
}

// upstreamIndexes maps each enabled upstream's ifIndex to it. A disabled
// interface is not polled, so its stored index may since have been given to
// another interface; an index two upstreams both claim is dropped rather
// than guessed.
func upstreamIndexes(ifaces []models.RouterInterface, upstream map[uuid.UUID]bool) map[uint32]uuid.UUID {
	out := map[uint32]uuid.UUID{}
	claims := map[uint32]int{}
	for _, i := range ifaces {
		if !upstream[i.ID] || i.Disabled || i.IfIndex == nil || *i.IfIndex <= 0 {
			continue
		}
		index := uint32(*i.IfIndex)
		claims[index]++
		out[index] = i.ID
	}
	for index, n := range claims {
		if n > 1 {
			delete(out, index)
		}
	}
	return out
}

// popPrefixes lists the blocks of every interface that is not an upstream:
// its addresses, then the blocks routed to devices behind it.
func popPrefixes(ifaces []models.RouterInterface, routed []models.RouterRoutedPrefix,
	upstream map[uuid.UUID]bool) []netflow.PrefixEntry {
	var entries []netflow.PrefixEntry
	for _, i := range ifaces {
		if upstream[i.ID] {
			continue
		}
		for _, cidr := range strings.Split(i.Addresses, ",") {
			if p, err := netip.ParsePrefix(strings.TrimSpace(cidr)); err == nil {
				entries = append(entries, netflow.PrefixEntry{Prefix: p, InterfaceID: i.ID})
			}
		}
	}
	for _, r := range routed {
		if p, err := netip.ParsePrefix(r.Prefix); err == nil && !upstream[r.InterfaceID] {
			entries = append(entries, netflow.PrefixEntry{Prefix: p, InterfaceID: r.InterfaceID})
		}
	}
	return entries
}
```

- [ ] **Step 5: Write the store**

`backend/internal/services/flow_store.go`:

```go
package services

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/netflow"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// flowWriteBatch is rows per INSERT. A busy minute is a few thousand rows;
// batches keep each statement well under Postgres's parameter limit.
const flowWriteBatch = 1000

// FlowStore writes what flowd aggregated.
type FlowStore struct {
	db *gorm.DB
}

// NewFlowStore creates a flow store.
func NewFlowStore(db *gorm.DB) *FlowStore {
	return &FlowStore{db: db}
}

// nullablePOP stores the Lainnya POP, uuid.Nil in memory, as NULL.
func nullablePOP(id uuid.UUID) *uuid.UUID {
	if id == uuid.Nil {
		return nil
	}
	return &id
}

// WriteMinute stores one closed minute. CreateInBatches runs in one
// transaction, so a minute is stored whole or not at all.
func (s *FlowStore) WriteMinute(ctx context.Context, rows []netflow.MinuteRow) error {
	if len(rows) == 0 {
		return nil
	}
	records := make([]models.RouterFlowMinute, len(rows))
	for i, r := range rows {
		records[i] = models.RouterFlowMinute{Time: r.Time, RouterID: r.RouterID, POPInterfaceID: nullablePOP(r.POP),
			UpstreamInterfaceID: r.Upstream, Direction: string(r.Direction), ASN: int64(r.ASN),
			Bytes: int64(r.Bytes), Packets: int64(r.Packets)}
	}
	return s.db.WithContext(ctx).CreateInBatches(records, flowWriteBatch).Error
}

// WriteTopIPs stores one closed window's top addresses.
func (s *FlowStore) WriteTopIPs(ctx context.Context, rows []netflow.TopIPRow) error {
	if len(rows) == 0 {
		return nil
	}
	records := make([]models.RouterFlowTopIP, len(rows))
	for i, r := range rows {
		records[i] = models.RouterFlowTopIP{Time: r.Time, RouterID: r.RouterID, POPInterfaceID: nullablePOP(r.POP),
			Direction: string(r.Direction), IP: r.IP.String(), ASN: int64(r.ASN), Bytes: int64(r.Bytes)}
	}
	return s.db.WithContext(ctx).CreateInBatches(records, flowWriteBatch).Error
}

// MarkSeen records when each router's IPFIX last arrived. UpdateColumn leaves
// updated_at alone: this runs every minute and is not an edit of the router.
func (s *FlowStore) MarkSeen(ctx context.Context, routerIDs []uuid.UUID, at time.Time) error {
	if len(routerIDs) == 0 {
		return nil
	}
	return s.db.WithContext(ctx).Model(&models.MikrotikRouter{}).Where("id IN ?", routerIDs).
		UpdateColumn("flow_last_seen_at", at).Error
}

// SaveASNNames upserts the dataset's names. updatedAt is the dataset's
// download time; the newest one is the version the Router tab shows.
func (s *FlowStore) SaveASNNames(ctx context.Context, names map[uint32]netflow.ASNInfo, updatedAt time.Time) error {
	if len(names) == 0 {
		return nil
	}
	rows := make([]models.ASNName, 0, len(names))
	for asn, info := range names {
		rows = append(rows, models.ASNName{ASN: int64(asn), Name: info.Name, Country: info.Country, UpdatedAt: updatedAt})
	}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "asn"}},
		DoUpdates: clause.AssignmentColumns([]string{"name", "country", "updated_at"}),
	}).CreateInBatches(rows, flowWriteBatch).Error
}
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `cd backend && go test ./internal/services/ -race -count=1`
Expected: PASS (9 new tests; the rest of the package unchanged).

- [ ] **Step 7: Commit**

```bash
cd backend && gofmt -s -l ./internal/services
git add internal/services/flow_mapping.go internal/services/flow_mapping_test.go \
  internal/services/flow_store.go internal/services/flow_store_test.go
git commit -m "feat(flows): per-router flow mapping and the flow store

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 8: `flowd` — collector, writer, ASN lifecycle, container

**Files:**
- Create: `backend/cmd/flowd/main.go`
- Create: `backend/cmd/flowd/collector.go`
- Create: `backend/cmd/flowd/writer.go`
- Create: `backend/cmd/flowd/asn.go`
- Test: `backend/cmd/flowd/collector_test.go`, `backend/cmd/flowd/writer_test.go`, `backend/cmd/flowd/asn_test.go`
- Modify: `backend/Dockerfile.worker`
- Modify: `docker-compose.yml`, `docker-compose.vps.yml`

**Interfaces:**
- Consumes: everything in `internal/netflow` (Tasks 1–4); `services.NewFlowStore`, `services.NewFlowMappingService` and their methods (Task 7); `ipfixtest` (Task 1); `config.Load`, `database.Connect` (existing).
- Produces: the `flowd` binary in the worker image and a `flowd` compose service. Package-private: `newCollector(asns *asnSource, now time.Time) *collector`, `(*collector).setRouters`, `handle(src netip.Addr, packet []byte)`, `closeMinute(now) ([]netflow.MinuteRow, []uuid.UUID, map[uuid.UUID]uint64)`, `closeWindowIfDue(now) []netflow.TopIPRow`; `retryWriter{sink flowSink; logger; pending}` with `flush(pendingBatch)`; `asnSource{fetcher, names, logger}` with `start(now)`, `refresh()`, `lookup(ip) uint32`.

- [ ] **Step 1: Write the failing collector tests**

`backend/cmd/flowd/collector_test.go`:

```go
package main

import (
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/netflow"
	"github.com/tikman/olt-provisioning/internal/netflow/ipfixtest"
	"go.uber.org/zap"
)

var (
	testRouterID = uuid.MustParse("00000000-0000-0000-0000-0000000000c1")
	testTransit  = uuid.MustParse("00000000-0000-0000-0000-0000000000b1")
	testPOP      = uuid.MustParse("00000000-0000-0000-0000-0000000000a1")
	routerAddr   = netip.MustParseAddr("192.0.2.1")
	minute0      = time.Date(2026, 10, 12, 3, 0, 0, 0, time.UTC)
)

func testCollector(t *testing.T, upstreams map[uint32]uuid.UUID) *collector {
	t.Helper()
	asns := &asnSource{logger: zap.NewNop()}
	table, err := netflow.ParseASNTable(strings.NewReader("198.51.100.0\t198.51.100.255\t64497\tZZ\tEXAMPLE-NET-B\n"))
	require.NoError(t, err)
	asns.table.Store(table)
	c := newCollector(asns, minute0)
	c.setRouters(map[netip.Addr]*netflow.RouterMap{routerAddr: {
		RouterID: testRouterID, Sampling: 100, Upstreams: upstreams,
		POPs: netflow.NewPrefixTable([]netflow.PrefixEntry{
			{Prefix: netip.MustParsePrefix("203.0.113.0/27"), InterfaceID: testPOP}}),
	}})
	return c
}

func flowPacket() []byte {
	return ipfixtest.Message(1, ipfixtest.TemplateSet(256, ipfixtest.FlowFields...),
		ipfixtest.DataSet(256, ipfixtest.FlowRecord("198.51.100.7", "203.0.113.5", 10, 20, 1500, 3)))
}

func TestHandleCountsAKnownRoutersFlows(t *testing.T) {
	c := testCollector(t, map[uint32]uuid.UUID{10: testTransit})

	c.handle(routerAddr, flowPacket())
	rows, seen, dropped := c.closeMinute(minute0.Add(time.Minute))

	require.Len(t, rows, 1)
	assert.Equal(t, netflow.MinuteRow{Time: minute0, MinuteKey: netflow.MinuteKey{RouterID: testRouterID,
		POP: testPOP, Upstream: testTransit, Direction: netflow.Download, ASN: 64497},
		Bytes: 150_000, Packets: 300}, rows[0])
	assert.Equal(t, []uuid.UUID{testRouterID}, seen)
	assert.Empty(t, dropped)
}

// A dual-stack socket reports an IPv4 sender as ::ffff:a.b.c.d.
func TestHandleRecognisesAnIPv4MappedSender(t *testing.T) {
	c := testCollector(t, map[uint32]uuid.UUID{10: testTransit})

	c.handle(netip.AddrFrom16(routerAddr.As16()), flowPacket())
	rows, _, _ := c.closeMinute(minute0.Add(time.Minute))

	assert.Len(t, rows, 1)
}

// The port is open to the internet: a datagram from anyone else is not parsed.
func TestHandleIgnoresUnknownSenders(t *testing.T) {
	c := testCollector(t, map[uint32]uuid.UUID{10: testTransit})

	c.handle(netip.MustParseAddr("198.51.100.9"), flowPacket())
	rows, seen, _ := c.closeMinute(minute0.Add(time.Minute))

	assert.Empty(t, rows)
	assert.Empty(t, seen)
	assert.EqualValues(t, 1, c.unknownSources.Load())
}

// Review Focus 3: a router whose upstreams discovery has not matched yet
// sends flows nothing can place. They are dropped, but the router is still
// marked as heard from, so the Router tab says IPFIX is arriving.
func TestHandleMarksARouterHeardEvenWhenNoFlowCanBePlaced(t *testing.T) {
	c := testCollector(t, map[uint32]uuid.UUID{})

	c.handle(routerAddr, flowPacket())
	rows, seen, _ := c.closeMinute(minute0.Add(time.Minute))

	assert.Empty(t, rows)
	assert.Equal(t, []uuid.UUID{testRouterID}, seen)
}

func TestHandleCountsGarbageWithoutFailing(t *testing.T) {
	c := testCollector(t, map[uint32]uuid.UUID{10: testTransit})

	c.handle(routerAddr, []byte{0, 10, 0})

	assert.EqualValues(t, 1, c.malformed.Load())
}

func TestTopIPsCloseOnlyAtTheWindowBoundary(t *testing.T) {
	c := testCollector(t, map[uint32]uuid.UUID{10: testTransit})
	c.handle(routerAddr, flowPacket())

	assert.Empty(t, c.closeWindowIfDue(minute0.Add(time.Minute)))
	rows := c.closeWindowIfDue(minute0.Add(netflow.TopIPWindow))

	require.Len(t, rows, 1)
	assert.Equal(t, netip.MustParseAddr("198.51.100.7"), rows[0].IP)
	assert.EqualValues(t, 64497, rows[0].ASN)
}
```

- [ ] **Step 2: Write the failing writer tests**

`backend/cmd/flowd/writer_test.go`:

```go
package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/tikman/olt-provisioning/internal/netflow"
	"go.uber.org/zap"
)

var errDown = errors.New("database unavailable")

// fakeSink fails the next calls with the errors queued for them, then stores.
type fakeSink struct {
	minuteErrs, topErrs []error
	minutes             [][]netflow.MinuteRow
	tops                [][]netflow.TopIPRow
}

func next(errs *[]error) error {
	if len(*errs) == 0 {
		return nil
	}
	err := (*errs)[0]
	*errs = (*errs)[1:]
	return err
}

func (s *fakeSink) WriteMinute(_ context.Context, rows []netflow.MinuteRow) error {
	if err := next(&s.minuteErrs); err != nil {
		return err
	}
	s.minutes = append(s.minutes, rows)
	return nil
}

func (s *fakeSink) WriteTopIPs(_ context.Context, rows []netflow.TopIPRow) error {
	if err := next(&s.topErrs); err != nil {
		return err
	}
	s.tops = append(s.tops, rows)
	return nil
}

func (s *fakeSink) MarkSeen(context.Context, []uuid.UUID, time.Time) error { return nil }

func minuteOf(bytes uint64) pendingBatch {
	return pendingBatch{minute: []netflow.MinuteRow{{Bytes: bytes}}}
}

func TestFlushRetriesAFailedBatchWithTheNext(t *testing.T) {
	sink := &fakeSink{minuteErrs: []error{errDown}}
	w := &retryWriter{sink: sink, logger: zap.NewNop()}

	w.flush(minuteOf(1))
	w.flush(minuteOf(2))

	assert.Equal(t, [][]netflow.MinuteRow{{{Bytes: 1}}, {{Bytes: 2}}}, sink.minutes)
	assert.Nil(t, w.pending)
}

// Review Focus 4: a database that stays down costs one batch per failed
// retry, never a growing backlog.
func TestFlushDropsABatchThatFailsTwice(t *testing.T) {
	sink := &fakeSink{minuteErrs: []error{errDown, errDown, errDown}}
	w := &retryWriter{sink: sink, logger: zap.NewNop()}

	w.flush(minuteOf(1)) // fails, held
	w.flush(minuteOf(2)) // the retry of 1 fails: dropped; 2 fails: held
	w.flush(minuteOf(3)) // 2 and 3 are written

	assert.Equal(t, [][]netflow.MinuteRow{{{Bytes: 2}}, {{Bytes: 3}}}, sink.minutes)
}

// Review Focus 4: the minute was stored and only the top-IP write failed;
// the retry must not store that minute a second time.
func TestRetryDoesNotStoreAMinuteTwice(t *testing.T) {
	sink := &fakeSink{topErrs: []error{errDown}}
	w := &retryWriter{sink: sink, logger: zap.NewNop()}

	w.flush(pendingBatch{minute: []netflow.MinuteRow{{Bytes: 1}}, topIPs: []netflow.TopIPRow{{Bytes: 9}}})
	w.flush(minuteOf(2))

	assert.Equal(t, [][]netflow.MinuteRow{{{Bytes: 1}}, {{Bytes: 2}}}, sink.minutes)
	assert.Equal(t, [][]netflow.TopIPRow{{{Bytes: 9}}}, sink.tops)
}
```

- [ ] **Step 3: Write the failing ASN lifecycle tests**

`backend/cmd/flowd/asn_test.go`:

```go
package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/netflow"
	"go.uber.org/zap"
)

const (
	datasetB = "198.51.100.0\t198.51.100.255\t64497\tZZ\tEXAMPLE-NET-B\n"
	datasetC = "198.51.100.0\t198.51.100.255\t64498\tZZ\tEXAMPLE-NET-C\n"
)

type namesRecorder struct{ saved int }

func (n *namesRecorder) SaveASNNames(_ context.Context, names map[uint32]netflow.ASNInfo, _ time.Time) error {
	n.saved += len(names)
	return nil
}

func gzipText(t *testing.T, text string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	_, err := w.Write([]byte(text))
	require.NoError(t, err)
	require.NoError(t, w.Close())
	return buf.Bytes()
}

func writeDataset(t *testing.T, path, text string, age time.Duration) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, gzipText(t, text), 0o644))
	modified := time.Now().Add(-age)
	require.NoError(t, os.Chtimes(path, modified, modified))
}

// datasetServer answers every request with status and, on 200, text.
func datasetServer(t *testing.T, status int, text string) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	body := gzipText(t, text)
	hits := &atomic.Int32{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)
	return server, hits
}

func testSource(server *httptest.Server, path string) (*asnSource, *namesRecorder) {
	names := &namesRecorder{}
	return &asnSource{
		fetcher: &netflow.ASNFetcher{URL: server.URL, Path: path, Client: server.Client()},
		names:   names, logger: zap.NewNop(),
	}, names
}

var probe = netip.MustParseAddr("198.51.100.7")

func TestStartUsesAFreshCopyWithoutDownloading(t *testing.T) {
	path := filepath.Join(t.TempDir(), asnFileName)
	writeDataset(t, path, datasetB, time.Hour)
	server, hits := datasetServer(t, http.StatusOK, datasetC)
	source, names := testSource(server, path)

	source.start(time.Now())

	assert.Zero(t, hits.Load())
	assert.EqualValues(t, 64497, source.lookup(probe))
	assert.Equal(t, 1, names.saved)
}

func TestStartDownloadsOverADayOldCopy(t *testing.T) {
	path := filepath.Join(t.TempDir(), asnFileName)
	writeDataset(t, path, datasetB, 25*time.Hour)
	server, _ := datasetServer(t, http.StatusOK, datasetC)
	source, _ := testSource(server, path)

	source.start(time.Now())

	assert.EqualValues(t, 64498, source.lookup(probe))
}

// An old table beats none: destinations a day stale are still mostly right.
func TestStartFallsBackToAStaleCopyWhenTheDownloadFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), asnFileName)
	writeDataset(t, path, datasetB, 25*time.Hour)
	server, _ := datasetServer(t, http.StatusBadGateway, datasetC)
	source, _ := testSource(server, path)

	source.start(time.Now())

	assert.EqualValues(t, 64497, source.lookup(probe))
}

func TestRefreshKeepsTheTableInUseWhenTheDownloadFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), asnFileName)
	writeDataset(t, path, datasetB, time.Hour)
	server, _ := datasetServer(t, http.StatusBadGateway, datasetC)
	source, _ := testSource(server, path)
	source.start(time.Now())

	source.refresh()

	assert.EqualValues(t, 64497, source.lookup(probe))
}

func TestLookupBeforeAnyDatasetIsOther(t *testing.T) {
	source := &asnSource{logger: zap.NewNop()}

	assert.EqualValues(t, netflow.OtherASN, source.lookup(probe))
}
```

- [ ] **Step 4: Run the tests to verify they fail**

Run: `cd backend && go test ./cmd/flowd/ -v`
Expected: FAIL to compile — `undefined: asnSource`, `undefined: newCollector`, `undefined: retryWriter`.

- [ ] **Step 5: Write the collector**

`backend/cmd/flowd/collector.go`:

```go
package main

import (
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/netflow"
)

// collector turns datagrams into aggregator counts. handle runs on the one
// goroutine reading the socket, which alone touches templates; the
// aggregator is shared with the minute ticker, hence the mutex.
type collector struct {
	routers   atomic.Pointer[map[netip.Addr]*netflow.RouterMap]
	asns      *asnSource
	templates map[uuid.UUID]*netflow.Templates

	mu   sync.Mutex
	agg  *netflow.Aggregator
	seen map[uuid.UUID]bool

	unknownSources atomic.Uint64
	malformed      atomic.Uint64
}

func newCollector(asns *asnSource, now time.Time) *collector {
	c := &collector{asns: asns, templates: map[uuid.UUID]*netflow.Templates{},
		agg: netflow.NewAggregator(now), seen: map[uuid.UUID]bool{}}
	c.setRouters(map[netip.Addr]*netflow.RouterMap{})
	return c
}

// setRouters swaps in a freshly loaded mapping; a datagram being handled
// keeps the one it started with.
func (c *collector) setRouters(routers map[netip.Addr]*netflow.RouterMap) {
	c.routers.Store(&routers)
}

// handle decodes one datagram from src. Anything not from a router collecting
// flows is dropped before parsing: the port is open to the internet.
func (c *collector) handle(src netip.Addr, packet []byte) {
	router, ok := (*c.routers.Load())[src.Unmap()]
	if !ok {
		c.unknownSources.Add(1)
		return
	}
	res, err := c.templatesFor(router.RouterID).Decode(packet)
	if err != nil {
		c.malformed.Add(1)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if err == nil {
		c.seen[router.RouterID] = true
	}
	for _, flow := range res.Flows {
		if placed, ok := router.Classify(flow); ok {
			c.agg.Add(router.RouterID, placed, c.asns.lookup(placed.Remote))
		}
	}
}

// templatesFor keeps each router's templates across mapping reloads, keyed by
// router rather than address, so a reload does not forget them.
func (c *collector) templatesFor(routerID uuid.UUID) *netflow.Templates {
	t, ok := c.templates[routerID]
	if !ok {
		t = netflow.NewTemplates()
		c.templates[routerID] = t
	}
	return t
}

// closeMinute returns the closed minute's rows, the routers heard from in it,
// and the keys each router had to fold for the cap.
func (c *collector) closeMinute(now time.Time) ([]netflow.MinuteRow, []uuid.UUID, map[uuid.UUID]uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	rows, dropped := c.agg.CloseMinute(now)
	seen := make([]uuid.UUID, 0, len(c.seen))
	for id := range c.seen {
		seen = append(seen, id)
	}
	c.seen = map[uuid.UUID]bool{}
	return rows, seen, dropped
}

// closeWindowIfDue returns the top-IP rows once now has left their window.
func (c *collector) closeWindowIfDue(now time.Time) []netflow.TopIPRow {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.agg.WindowDue(now) {
		return nil
	}
	return c.agg.CloseWindow(now)
}
```

- [ ] **Step 6: Write the writer**

`backend/cmd/flowd/writer.go`:

```go
package main

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/netflow"
	"go.uber.org/zap"
)

// flowWriteTimeout bounds one write. A retry and the new minute must both
// finish inside the minute, or the next close waits behind them.
const flowWriteTimeout = 20 * time.Second

// flowSink is where closed minutes go: services.FlowStore in production.
type flowSink interface {
	WriteMinute(ctx context.Context, rows []netflow.MinuteRow) error
	WriteTopIPs(ctx context.Context, rows []netflow.TopIPRow) error
	MarkSeen(ctx context.Context, routerIDs []uuid.UUID, at time.Time) error
}

// pendingBatch is what one minute close produced.
type pendingBatch struct {
	minute []netflow.MinuteRow
	topIPs []netflow.TopIPRow
}

// retryWriter writes each batch and keeps the unwritten part of one that
// failed for a single retry with the next. A batch failing twice is dropped:
// holding batches for a database that stays slow is how a collector runs
// out of memory.
type retryWriter struct {
	sink    flowSink
	logger  *zap.Logger
	pending *pendingBatch
}

func (w *retryWriter) flush(batch pendingBatch) {
	if w.pending != nil {
		if _, err := w.write(*w.pending); err != nil {
			w.logger.Error("Dropping a flow batch that failed twice", zap.Error(err),
				zap.Int("minute_rows", len(w.pending.minute)), zap.Int("top_ip_rows", len(w.pending.topIPs)))
		}
		w.pending = nil
	}
	if rest, err := w.write(batch); err != nil {
		w.logger.Warn("Flow write failed; retrying with the next minute", zap.Error(err))
		w.pending = &rest
	}
}

// write stores what it can and returns what is still unwritten, so a retry
// never stores a minute that already went in.
func (w *retryWriter) write(b pendingBatch) (pendingBatch, error) {
	ctx, cancel := context.WithTimeout(context.Background(), flowWriteTimeout)
	defer cancel()
	if len(b.minute) > 0 {
		if err := w.sink.WriteMinute(ctx, b.minute); err != nil {
			return b, err
		}
		b.minute = nil
	}
	if len(b.topIPs) > 0 {
		if err := w.sink.WriteTopIPs(ctx, b.topIPs); err != nil {
			return b, err
		}
	}
	return pendingBatch{}, nil
}
```

- [ ] **Step 7: Write the ASN lifecycle**

`backend/cmd/flowd/asn.go`:

```go
package main

import (
	"context"
	"net/netip"
	"sync/atomic"
	"time"

	"github.com/tikman/olt-provisioning/internal/netflow"
	"go.uber.org/zap"
)

// asnSaveTimeout covers upserting the dataset's names, tens of thousands of
// rows in batches.
const asnSaveTimeout = 2 * time.Minute

type asnNameSaver interface {
	SaveASNNames(ctx context.Context, names map[uint32]netflow.ASNInfo, updatedAt time.Time) error
}

// asnSource holds the table lookups read. A new dataset replaces it whole, so
// a lookup never sees half of one.
type asnSource struct {
	table   atomic.Pointer[netflow.ASNTable]
	fetcher *netflow.ASNFetcher
	names   asnNameSaver
	logger  *zap.Logger
}

// lookup names the AS of ip; until a dataset loads every address is OtherASN.
func (s *asnSource) lookup(ip netip.Addr) uint32 {
	table := s.table.Load()
	if table == nil {
		return netflow.OtherASN
	}
	return table.Lookup(ip)
}

// start loads the copy on disk, downloading first when it is missing or a
// day old.
func (s *asnSource) start(now time.Time) {
	if !s.fetcher.Stale(now) && s.installLocal() {
		return
	}
	s.refresh()
}

// refresh downloads a new dataset. On failure the table in use stays; with
// none in use yet, a stale copy on disk is better than none.
func (s *asnSource) refresh() {
	table, at, err := s.fetcher.Download()
	if err != nil {
		s.logger.Warn("ASN dataset download failed; keeping the table in use", zap.Error(err))
		if s.table.Load() == nil {
			s.installLocal()
		}
		return
	}
	s.install(table, at)
}

func (s *asnSource) installLocal() bool {
	table, at, err := s.fetcher.LoadLocal()
	if err != nil {
		s.logger.Warn("No usable ASN dataset on disk", zap.Error(err))
		return false
	}
	s.install(table, at)
	return true
}

func (s *asnSource) install(table *netflow.ASNTable, at time.Time) {
	s.table.Store(table)
	ctx, cancel := context.WithTimeout(context.Background(), asnSaveTimeout)
	defer cancel()
	if err := s.names.SaveASNNames(ctx, table.Names(), at); err != nil {
		s.logger.Error("Failed to save ASN names", zap.Error(err))
	}
	s.logger.Info("ASN dataset loaded", zap.Int("asns", len(table.Names())), zap.Time("downloaded_at", at))
}
```

- [ ] **Step 8: Write `main.go`**

`backend/cmd/flowd/main.go`:

```go
// Command flowd collects IPFIX from the MikroTik border routers and stores,
// per minute, how much each POP exchanged with each destination AS over each
// upstream.
//
// It is its own container, not in api's network namespace like trapd: the
// routers send to the VPS's public address, and restarting api — or the
// pollers, or WhatsApp — must not stop collection, nor collection them.
//
// It keeps summaries, never flows. A border router holds hundreds of
// thousands of connections; their raw records would be millions of rows a
// day on a VPS that also runs everything else.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/tikman/olt-provisioning/internal/config"
	"github.com/tikman/olt-provisioning/internal/database"
	"github.com/tikman/olt-provisioning/internal/netflow"
	"github.com/tikman/olt-provisioning/internal/services"
	"go.uber.org/zap"
)

const (
	listenAddress = ":2055"
	// mappingRefresh is how soon a block discovery found, a renumbered
	// ifIndex or a changed sampling ratio takes effect.
	mappingRefresh = 5 * time.Minute
	defaultASNDir  = "/data/asn"
	asnFileName    = "ip2asn-v4.tsv.gz"
	// socketBuffer gives bursts room while a minute is being closed.
	socketBuffer    = 4 << 20
	markSeenTimeout = 10 * time.Second
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer func() { _ = logger.Sync() }()
	db, err := database.Connect(cfg)
	if err != nil {
		logger.Fatal(fmt.Sprintf("Failed to connect to database: %v", err))
	}

	store := services.NewFlowStore(db)
	asns := &asnSource{
		fetcher: &netflow.ASNFetcher{URL: netflow.ASNDatasetURL,
			Path: filepath.Join(asnDir(), asnFileName), Client: &http.Client{}},
		names: store, logger: logger,
	}
	// Collection does not wait for the dataset: until it loads, every
	// destination counts as ASN 0.
	go func() {
		asns.start(time.Now())
		every(netflow.ASNMaxAge, asns.refresh)
	}()

	coll := newCollector(asns, time.Now())
	mappings := services.NewFlowMappingService(db)
	refreshRouters(coll, mappings, logger)
	go every(mappingRefresh, func() { refreshRouters(coll, mappings, logger) })

	conn := listen(logger)
	go runMinutes(coll, &retryWriter{sink: store, logger: logger}, store, logger)
	go closeOnSignal(conn, logger)
	logger.Info("Listening for IPFIX", zap.String("address", listenAddress))
	readLoop(conn, coll, logger)
}

// asnDir is the compose volume, or FLOWD_ASN_DIR for a run outside it.
func asnDir() string {
	if dir := os.Getenv("FLOWD_ASN_DIR"); dir != "" {
		return dir
	}
	return defaultASNDir
}

func listen(logger *zap.Logger) *net.UDPConn {
	addr, err := net.ResolveUDPAddr("udp", listenAddress)
	if err != nil {
		logger.Fatal(fmt.Sprintf("Bad listen address: %v", err))
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		logger.Fatal(fmt.Sprintf("Failed to listen on %s: %v", listenAddress, err))
	}
	if err := conn.SetReadBuffer(socketBuffer); err != nil {
		logger.Warn("Could not enlarge the socket buffer", zap.Error(err))
	}
	return conn
}

func every(interval time.Duration, run func()) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		run()
	}
}

// refreshRouters reloads the mapping. A failure keeps the previous one: an
// unreachable database should not turn every router into an unknown sender.
func refreshRouters(c *collector, mappings *services.FlowMappingService, logger *zap.Logger) {
	routers, skipped, err := mappings.Load()
	if err != nil {
		logger.Error("Failed to load router mappings; keeping the previous ones", zap.Error(err))
		return
	}
	for _, name := range skipped {
		logger.Warn("Router host is not an IP address, so its IPFIX cannot be recognised", zap.String("router", name))
	}
	c.setRouters(routers)
	logger.Info("Router mappings loaded", zap.Int("routers", len(routers)),
		zap.Uint64("unknown_source_packets", c.unknownSources.Swap(0)),
		zap.Uint64("malformed_packets", c.malformed.Swap(0)))
}

// runMinutes closes each minute on the clock's boundary and writes it. The
// minute open at shutdown is lost; a restart costs at most that minute.
func runMinutes(c *collector, w *retryWriter, sink flowSink, logger *zap.Logger) {
	time.Sleep(time.Until(time.Now().Truncate(time.Minute).Add(time.Minute)))
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for now := time.Now(); ; now = <-ticker.C {
		finishMinute(c, w, sink, logger, now)
	}
}

func finishMinute(c *collector, w *retryWriter, sink flowSink, logger *zap.Logger, now time.Time) {
	rows, seen, dropped := c.closeMinute(now)
	for routerID, n := range dropped {
		logger.Warn("Keys over the per-router cap folded into ASN 0",
			zap.String("router_id", routerID.String()), zap.Uint64("dropped_keys", n))
	}
	w.flush(pendingBatch{minute: rows, topIPs: c.closeWindowIfDue(now)})
	ctx, cancel := context.WithTimeout(context.Background(), markSeenTimeout)
	defer cancel()
	if err := sink.MarkSeen(ctx, seen, now); err != nil {
		logger.Warn("Failed to record when flows last arrived", zap.Error(err))
	}
}

func readLoop(conn *net.UDPConn, c *collector, logger *zap.Logger) {
	buf := make([]byte, netflow.MaxPacketBytes)
	for {
		n, src, err := conn.ReadFromUDPAddrPort(buf)
		if errors.Is(err, net.ErrClosed) {
			return
		}
		if err != nil {
			logger.Warn("UDP read failed", zap.Error(err))
			continue
		}
		c.handle(src.Addr(), buf[:n])
	}
}

func closeOnSignal(conn *net.UDPConn, logger *zap.Logger) {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
	logger.Info("Received shutdown signal")
	_ = conn.Close()
}
```

- [ ] **Step 9: Run the tests and the build**

Run: `cd backend && go vet ./cmd/flowd/ && go test ./cmd/flowd/ -v -race && go build -o /dev/null ./cmd/flowd`
Expected: PASS (14 tests); vet and build clean.

- [ ] **Step 10: Ship it in the worker image and compose**

In `backend/Dockerfile.worker`, after the `wgstat` build:

```dockerfile
# flowd ships here too, but runs as its own container: IPFIX arrives on the
# VPS's public address, not through the tunnel trapd listens on.
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o flowd ./cmd/flowd
```

and after `COPY --from=builder /app/wgstat .`:

```dockerfile
COPY --from=builder /app/flowd .
```

In `docker-compose.yml`, add after the `trapd` service:

```yaml
  flowd:
    build:
      context: ./backend
      dockerfile: Dockerfile.worker
    container_name: tikman-flowd
    # Its own network namespace, unlike trapd: the border routers send IPFIX
    # to the VPS's public address, and recreating api must not stop
    # collection. Docker's DNAT keeps the routers' source addresses, which is
    # what flowd recognises them by.
    entrypoint: ["./flowd"]
    ports:
      - "2055:2055/udp"
    environment:
      - DB_HOST=postgres
      - DB_PORT=5432
      - DB_USER=tikman
      - DB_PASSWORD=${POSTGRES_PASSWORD}
      - DB_NAME=tikman
      - ENCRYPTION_KEY=${ENCRYPTION_KEY}
      - SESSION_SECRET=${SESSION_SECRET}
      - LOG_LEVEL=info
    volumes:
      # The ASN dataset survives a restart, so flowd does not download it
      # again unless it is a day old.
      - asn_data:/data/asn
    depends_on:
      postgres:
        condition: service_healthy
      # api applies migration 58, which creates the tables flowd writes.
      api:
        condition: service_healthy
    networks:
      - tikman-network
    restart: unless-stopped
```

and add `asn_data:` to the top-level `volumes:` list.

In `docker-compose.vps.yml`, add under `services:`:

```yaml
  flowd:
    # IPFIX from the border routers arrives on the public address. Restrict
    # 2055/udp to the routers' IPs in the VPS provider's firewall.
    ports: !override
      - "2055:2055/udp"
```

Run: `docker compose -f docker-compose.yml -f docker-compose.vps.yml config --services | grep flowd && docker compose -f docker-compose.yml -f docker-compose.vps.yml config | grep -A3 'flowd:' | head -20`
Expected: `flowd` is listed; the merged config shows its `2055/udp` port and `asn_data` volume. (Missing `.env` variables only produce warnings here.)

- [ ] **Step 11: Commit**

```bash
cd backend && gofmt -s -l ./cmd/flowd
cd .. && git add backend/cmd/flowd backend/Dockerfile.worker docker-compose.yml docker-compose.vps.yml
git commit -m "feat(flowd): IPFIX collector service on UDP 2055

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 9: Flow query service

**Files:**
- Create: `backend/internal/services/flow_resolution.go`
- Create: `backend/internal/services/flow_query.go`
- Create: `backend/internal/services/flow_summary.go`
- Create: `backend/internal/services/flow_top_ips.go`
- Test: `backend/internal/services/flow_resolution_test.go`, `backend/internal/services/flow_query_test.go` (SQLite), `backend/internal/services/flow_query_postgres_test.go`

**Interfaces:**
- Consumes: models from Task 5; `netflow.OtherASN` (Task 4); existing `ErrValidation`, `setupTestDB`, `setupPostgresTestDB` (runs AutoMigrate and every SQL migration in a private schema).
- Produces:
  - `func NewFlowQueryService(db *gorm.DB) *FlowQueryService`
  - `type FlowFilter struct { From, To time.Time; POP *uuid.UUID; OtherPOP bool; Upstream *uuid.UUID; Direction string }`
  - `(*FlowQueryService).Destinations(routerID uuid.UUID, f FlowFilter, limit int) (*FlowDestinations, error)` → `FlowDestinations{StepSeconds int64 "step_seconds"; Series []FlowSeries "series"}`, `FlowSeries{ASN int64 "asn"; Name string "name"; Points []FlowPoint "points"}`, `FlowPoint{Time time.Time "time"; Bps float64 "bps"}`
  - `(*FlowQueryService).Summary(routerID uuid.UUID, f FlowFilter) (*FlowSummary, error)` → `FlowSummary{TotalBytes int64 "total_bytes"; PeakWindowSeconds int64 "peak_window_seconds"; Rows []FlowSummaryRow "rows"}`, `FlowSummaryRow{ASN, Name, Bytes, Percent "percent", AvgBps "avg_bps", PeakBps "peak_bps", Upstreams []FlowUpstreamShare "upstreams"}`, `FlowUpstreamShare{UpstreamInterfaceID uuid.UUID "upstream_interface_id"; Bytes int64 "bytes"}`
  - `(*FlowQueryService).POPs(routerID uuid.UUID, f FlowFilter) ([]FlowPOPRow, error)` → `FlowPOPRow{POPInterfaceID *uuid.UUID "pop_interface_id"; Bytes int64 "bytes"; TopASNs []FlowASNBytes "top_asns"}`, `FlowASNBytes{ASN, Name, Bytes}`
  - `(*FlowQueryService).TopIPs(routerID uuid.UUID, f FlowFilter, now time.Time) ([]FlowTopIP, error)` → `FlowTopIP{IP "ip"; ASN "asn"; Name "name"; Bytes "bytes"}`; `var ErrTopIPsRange` (wraps `ErrValidation`)
  - `(*FlowQueryService).Status() (*FlowStatus, error)` → `FlowStatus{ASNDatasetUpdatedAt *time.Time "asn_dataset_updated_at"; ASNCount int64 "asn_count"}`
  - Names: ASN 0 is always `"Lainnya"`; an AS the dataset does not name has `""` (the UI shows `AS<n>`).

Rulings recorded here:
- The top-IP range check allows **5 minutes of slack** past 7 days: the **7 hari** preset's start is computed in the browser a moment before the server checks it, and must not be refused. Retention drops whole chunks only once they are entirely older than 7 days, so the data is there.
- `Summary.PeakBps` is the busiest single step **of the source** (1 min, 5 min, 1 h or 1 day), not of the widened chart step; `peak_window_seconds` says which, so the UI can label it.

- [ ] **Step 1: Write the failing resolution and SQLite tests**

`backend/internal/services/flow_resolution_test.go`:

```go
package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestResolveFlowRangePicksTheFinestSourceStillHoldingTheStart(t *testing.T) {
	now := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	day := 24 * time.Hour
	cases := []struct {
		name     string
		from, to time.Time
		table    string
		step     time.Duration
	}{
		{"15 minutes", now.Add(-15 * time.Minute), now, "router_flow_minute", time.Minute},
		{"24 hours", now.Add(-day), now, "router_flow_minute", time.Minute},
		{"7 days", now.Add(-7 * day), now, "router_flow_minute", 7 * time.Minute},
		{"an hour 30 days back", now.Add(-30 * day), now.Add(-30*day + time.Hour), "router_flow_5min", 5 * time.Minute},
		{"30 days", now.Add(-30 * day), now, "router_flow_5min", 30 * time.Minute},
		{"90 days", now.Add(-90 * day), now, "router_flow_5min", 90 * time.Minute},
		{"1 year", now.Add(-365 * day), now, "router_flow_hourly", 6 * time.Hour},
		{"a day 3 years back", now.Add(-3 * 365 * day), now.Add(-3*365*day + day), "router_flow_daily", day},
		{"5 years", now.Add(-5 * 365 * day), now, "router_flow_daily", 2 * day},
	}
	for _, c := range cases {
		res := resolveFlowRange(c.from, c.to, now)
		assert.Equal(t, c.table, res.source.table, c.name)
		assert.Equal(t, c.step, res.step, c.name)
		assert.Zero(t, res.step%res.source.step, "%s: a whole multiple of the source's step", c.name)
		assert.LessOrEqual(t, int64(c.to.Sub(c.from)/res.step)+1, int64(maxFlowPoints), c.name)
	}
}
```

`backend/internal/services/flow_query_test.go`:

```go
package services

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

func TestTopIPsRefusesARangeOlderThanItsRetention(t *testing.T) {
	now := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	week := 7 * 24 * time.Hour

	assert.ErrorIs(t, checkTopIPsRange(now.Add(-8*24*time.Hour), now), ErrTopIPsRange)
	assert.ErrorIs(t, checkTopIPsRange(now.Add(-8*24*time.Hour), now), ErrValidation)
	assert.NoError(t, checkTopIPsRange(now.Add(-week-2*time.Second), now),
		"the 7-day preset's start was computed a moment before the request")

	_, err := NewFlowQueryService(setupTestDB(t)).TopIPs(uuid.New(),
		FlowFilter{From: now.Add(-30 * 24 * time.Hour), To: now}, now)
	assert.ErrorIs(t, err, ErrTopIPsRange)
}

func TestStatusReportsTheNewestDataset(t *testing.T) {
	db := setupTestDB(t)
	flows := NewFlowQueryService(db)

	empty, err := flows.Status()
	require.NoError(t, err)
	assert.Nil(t, empty.ASNDatasetUpdatedAt)
	assert.Zero(t, empty.ASNCount)

	older := time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC)
	newer := older.Add(24 * time.Hour)
	require.NoError(t, db.Create(&models.ASNName{ASN: 64496, Name: "EXAMPLE-NET-A", Country: "ZZ", UpdatedAt: older}).Error)
	require.NoError(t, db.Create(&models.ASNName{ASN: 64497, Name: "EXAMPLE-NET-B", Country: "ZZ", UpdatedAt: newer}).Error)

	status, err := flows.Status()
	require.NoError(t, err)
	require.NotNil(t, status.ASNDatasetUpdatedAt)
	assert.True(t, newer.Equal(*status.ASNDatasetUpdatedAt))
	assert.EqualValues(t, 2, status.ASNCount)
}
```

- [ ] **Step 2: Write the failing Postgres tests**

`backend/internal/services/flow_query_postgres_test.go`:

```go
package services

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

func seedFlowMinute(t *testing.T, db *gorm.DB, row models.RouterFlowMinute) {
	t.Helper()
	if row.Packets == 0 {
		row.Packets = 1
	}
	require.NoError(t, db.Create(&row).Error)
}

// bytesIn adds up every series of a destinations read back into bytes.
func bytesIn(t *testing.T, flows *FlowQueryService, routerID uuid.UUID, f FlowFilter) float64 {
	t.Helper()
	out, err := flows.Destinations(routerID, f, 10)
	require.NoError(t, err)
	var total float64
	for _, series := range out.Series {
		for _, p := range series.Points {
			total += p.Bps * float64(out.StepSeconds) / 8
		}
	}
	return total
}

func TestDestinationsStackTheTopASNsAndFoldTheRest(t *testing.T) {
	db := setupPostgresTestDB(t)
	flows := NewFlowQueryService(db)
	routerID, transit := uuid.New(), uuid.New()
	end := time.Now().UTC().Truncate(time.Minute)
	start := end.Add(-10 * time.Minute)
	for i := 0; i < 3; i++ {
		at := start.Add(time.Duration(i) * time.Minute)
		for asn, bytes := range map[int64]int64{64496: 6_000_000, 64497: 3_000_000, 64498: 600} {
			seedFlowMinute(t, db, models.RouterFlowMinute{Time: at, RouterID: routerID,
				UpstreamInterfaceID: transit, Direction: "download", ASN: asn, Bytes: bytes})
		}
	}
	require.NoError(t, db.Create(&models.ASNName{ASN: 64496, Name: "EXAMPLE-NET-A", Country: "ZZ", UpdatedAt: end}).Error)

	out, err := flows.Destinations(routerID, FlowFilter{From: start, To: end, Direction: "download"}, 2)

	require.NoError(t, err)
	assert.EqualValues(t, 60, out.StepSeconds)
	require.Len(t, out.Series, 3)
	assert.Equal(t, []int64{64496, 64497, otherASN}, []int64{out.Series[0].ASN, out.Series[1].ASN, out.Series[2].ASN})
	assert.Equal(t, "EXAMPLE-NET-A", out.Series[0].Name)
	assert.Empty(t, out.Series[1].Name, "an AS the dataset does not name")
	assert.Equal(t, otherASNName, out.Series[2].Name)
	require.Len(t, out.Series[0].Points, 3)
	assert.InDelta(t, 6_000_000*8/60.0, out.Series[0].Points[0].Bps, 0.001)
	assert.InDelta(t, 600*8/60.0, out.Series[2].Points[0].Bps, 0.001)
}

func TestDestinationsFilterByPOPOtherPOPAndUpstream(t *testing.T) {
	db := setupPostgresTestDB(t)
	flows := NewFlowQueryService(db)
	routerID, pop, transit, ix := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	at := time.Now().UTC().Truncate(time.Minute).Add(-5 * time.Minute)
	seedFlowMinute(t, db, models.RouterFlowMinute{Time: at, RouterID: routerID, POPInterfaceID: &pop,
		UpstreamInterfaceID: transit, Direction: "download", ASN: 64496, Bytes: 1000})
	seedFlowMinute(t, db, models.RouterFlowMinute{Time: at, RouterID: routerID,
		UpstreamInterfaceID: ix, Direction: "download", ASN: 64496, Bytes: 50})
	seedFlowMinute(t, db, models.RouterFlowMinute{Time: at, RouterID: uuid.New(),
		UpstreamInterfaceID: transit, Direction: "download", ASN: 64496, Bytes: 7})

	window := FlowFilter{From: at.Add(-time.Minute), To: at.Add(time.Minute), Direction: "download"}
	byPOP, other, viaIX := window, window, window
	byPOP.POP = &pop
	other.OtherPOP = true
	viaIX.Upstream = &ix

	assert.InDelta(t, 1050, bytesIn(t, flows, routerID, window), 0.001, "another router's bytes stay out")
	assert.InDelta(t, 1000, bytesIn(t, flows, routerID, byPOP), 0.001)
	assert.InDelta(t, 50, bytesIn(t, flows, routerID, other), 0.001)
	assert.InDelta(t, 50, bytesIn(t, flows, routerID, viaIX), 0.001)
}

func TestDestinationsReadTheFiveMinuteRollupForAMonthOldRange(t *testing.T) {
	db := setupPostgresTestDB(t)
	flows := NewFlowQueryService(db)
	routerID, transit := uuid.New(), uuid.New()
	old := time.Now().UTC().Add(-30 * 24 * time.Hour).Truncate(5 * time.Minute)
	for i := 0; i < 5; i++ {
		seedFlowMinute(t, db, models.RouterFlowMinute{Time: old.Add(time.Duration(i) * time.Minute), RouterID: routerID,
			UpstreamInterfaceID: transit, Direction: "download", ASN: 64496, Bytes: 1000})
	}
	// Materialized at once: these raw rows are past their 7-day retention,
	// and the policy job may drop them at any moment.
	require.NoError(t, db.Exec(fmt.Sprintf("CALL refresh_continuous_aggregate('router_flow_5min', '%s', '%s')",
		old.Add(-time.Hour).Format(time.RFC3339), old.Add(time.Hour).Format(time.RFC3339))).Error)

	out, err := flows.Destinations(routerID, FlowFilter{From: old.Add(-24 * time.Hour), To: old.Add(24 * time.Hour)}, 10)

	require.NoError(t, err)
	assert.EqualValues(t, 300, out.StepSeconds)
	require.Len(t, out.Series[0].Points, 1)
	assert.True(t, old.Equal(out.Series[0].Points[0].Time))
	assert.InDelta(t, 5000*8/300.0, out.Series[0].Points[0].Bps, 0.001)
}

func TestSummaryGivesShareAverageAndPeak(t *testing.T) {
	db := setupPostgresTestDB(t)
	flows := NewFlowQueryService(db)
	routerID, transit, ix := uuid.New(), uuid.New(), uuid.New()
	m0 := time.Now().UTC().Truncate(time.Minute).Add(-8 * time.Minute)
	seeds := []models.RouterFlowMinute{
		{Time: m0, UpstreamInterfaceID: transit, ASN: 64496, Bytes: 6000},
		{Time: m0, UpstreamInterfaceID: ix, ASN: 64496, Bytes: 3000},
		{Time: m0.Add(time.Minute), UpstreamInterfaceID: transit, ASN: 64496, Bytes: 1000},
		{Time: m0, UpstreamInterfaceID: transit, ASN: 64497, Bytes: 2000},
	}
	for _, s := range seeds {
		s.RouterID, s.Direction = routerID, "download"
		seedFlowMinute(t, db, s)
	}

	out, err := flows.Summary(routerID, FlowFilter{From: m0, To: m0.Add(10 * time.Minute), Direction: "download"})

	require.NoError(t, err)
	assert.EqualValues(t, 12000, out.TotalBytes)
	assert.EqualValues(t, 60, out.PeakWindowSeconds)
	require.Len(t, out.Rows, 2)
	top := out.Rows[0]
	assert.EqualValues(t, 64496, top.ASN)
	assert.EqualValues(t, 10000, top.Bytes)
	assert.InDelta(t, 83.333, top.Percent, 0.001)
	assert.InDelta(t, 10000*8/600.0, top.AvgBps, 0.001)
	assert.InDelta(t, 9000*8/60.0, top.PeakBps, 0.001, "the busiest minute, both upstreams together")
	assert.Equal(t, []FlowUpstreamShare{{UpstreamInterfaceID: transit, Bytes: 7000},
		{UpstreamInterfaceID: ix, Bytes: 3000}}, top.Upstreams)
}

func TestPOPsListEachPOPsTotalAndTopThree(t *testing.T) {
	db := setupPostgresTestDB(t)
	flows := NewFlowQueryService(db)
	routerID, pop, transit := uuid.New(), uuid.New(), uuid.New()
	at := time.Now().UTC().Truncate(time.Minute).Add(-3 * time.Minute)
	for asn, bytes := range map[int64]int64{64496: 400, 64497: 300, 64498: 200, 64499: 100} {
		seedFlowMinute(t, db, models.RouterFlowMinute{Time: at, RouterID: routerID, POPInterfaceID: &pop,
			UpstreamInterfaceID: transit, Direction: "download", ASN: asn, Bytes: bytes})
	}
	seedFlowMinute(t, db, models.RouterFlowMinute{Time: at, RouterID: routerID,
		UpstreamInterfaceID: transit, Direction: "download", ASN: 64496, Bytes: 50})

	rows, err := flows.POPs(routerID, FlowFilter{From: at.Add(-time.Minute), To: at.Add(time.Minute), Direction: "download"})

	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.NotNil(t, rows[0].POPInterfaceID)
	assert.Equal(t, pop, *rows[0].POPInterfaceID)
	assert.EqualValues(t, 1000, rows[0].Bytes)
	var asns []int64
	for _, a := range rows[0].TopASNs {
		asns = append(asns, a.ASN)
	}
	assert.Equal(t, []int64{64496, 64497, 64498}, asns)
	assert.Nil(t, rows[1].POPInterfaceID, "addresses in no POP's blocks")
	assert.EqualValues(t, 50, rows[1].Bytes)
}

func TestTopIPsSumAnAddressOverItsWindows(t *testing.T) {
	db := setupPostgresTestDB(t)
	flows := NewFlowQueryService(db)
	routerID := uuid.New()
	w0 := time.Now().UTC().Truncate(5 * time.Minute).Add(-10 * time.Minute)
	for _, r := range []models.RouterFlowTopIP{
		{Time: w0, IP: "203.0.113.5", ASN: 64498, Bytes: 100},
		{Time: w0.Add(5 * time.Minute), IP: "203.0.113.5", ASN: 64498, Bytes: 50},
		{Time: w0, IP: "203.0.113.6", ASN: 64498, Bytes: 120},
	} {
		r.RouterID, r.Direction = routerID, "download"
		require.NoError(t, db.Create(&r).Error)
	}
	require.NoError(t, db.Create(&models.ASNName{ASN: 64498, Name: "EXAMPLE-NET-C", Country: "ZZ", UpdatedAt: w0}).Error)

	rows, err := flows.TopIPs(routerID, FlowFilter{From: w0, To: w0.Add(15 * time.Minute)}, time.Now())

	require.NoError(t, err)
	assert.Equal(t, []FlowTopIP{
		{IP: "203.0.113.5", ASN: 64498, Name: "EXAMPLE-NET-C", Bytes: 150},
		{IP: "203.0.113.6", ASN: 64498, Name: "EXAMPLE-NET-C", Bytes: 120},
	}, rows)
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `cd backend && go test ./internal/services/ -run 'ResolveFlowRange|TopIPs|StatusReports|Destinations|Summary|POPs' -v`
Expected: FAIL to compile — `undefined: resolveFlowRange`, `undefined: NewFlowQueryService`.

- [ ] **Step 4: Write the resolution**

`backend/internal/services/flow_resolution.go`:

```go
package services

import "time"

// maxFlowPoints bounds one series in a response: more points than a chart
// has pixels only costs the query and the browser.
const maxFlowPoints = 1500

// flowSource is a relation the flow API reads: the raw minutes or a rollup,
// with the step its rows are spaced at and how long it keeps them.
type flowSource struct {
	table      string
	timeColumn string
	step       time.Duration
	retention  time.Duration // 0 keeps rows forever
}

// flowSources, finest first. Retention matches migration 58's policies.
var flowSources = []flowSource{
	{table: "router_flow_minute", timeColumn: "time", step: time.Minute, retention: 7 * 24 * time.Hour},
	{table: "router_flow_5min", timeColumn: "bucket", step: 5 * time.Minute, retention: 90 * 24 * time.Hour},
	{table: "router_flow_hourly", timeColumn: "bucket", step: time.Hour, retention: 2 * 365 * 24 * time.Hour},
	{table: "router_flow_daily", timeColumn: "bucket", step: 24 * time.Hour},
}

// flowResolution is where a query reads and the step it reports at.
type flowResolution struct {
	source flowSource
	step   time.Duration
}

// resolveFlowRange picks the finest source whose retention still holds from
// — span alone is not enough, a short range a month back has no raw minutes
// left — then widens the step, in whole multiples of the source's, until the
// range fits maxFlowPoints buckets.
func resolveFlowRange(from, to, now time.Time) flowResolution {
	source := flowSources[len(flowSources)-1]
	for _, s := range flowSources {
		if s.retention == 0 || now.Sub(from) <= s.retention {
			source = s
			break
		}
	}
	// maxFlowPoints-1 intervals: a range not aligned to the step touches one
	// bucket more than it spans.
	need := to.Sub(from) / (maxFlowPoints - 1)
	multiple := int64((need + source.step - 1) / source.step)
	if multiple < 1 {
		multiple = 1
	}
	return flowResolution{source: source, step: source.step * time.Duration(multiple)}
}
```

- [ ] **Step 5: Write the destinations query**

`backend/internal/services/flow_query.go`:

```go
package services

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/netflow"
	"gorm.io/gorm"
)

// otherASN is netflow.OtherASN as stored.
const otherASN int64 = netflow.OtherASN

// otherASNName labels ASN 0: destinations outside the stored top, and those
// no dataset range covers.
const otherASNName = "Lainnya"

// FlowQueryService reads the summaries flowd stores. Its range queries use
// time_bucket, so they run on TimescaleDB only.
type FlowQueryService struct {
	db *gorm.DB
}

// NewFlowQueryService creates a flow query service.
func NewFlowQueryService(db *gorm.DB) *FlowQueryService {
	return &FlowQueryService{db: db}
}

// FlowFilter narrows a flow read. OtherPOP selects the traffic of addresses in
// no POP's blocks; with neither it nor POP set, every POP counts.
type FlowFilter struct {
	From, To  time.Time
	POP       *uuid.UUID
	OtherPOP  bool
	Upstream  *uuid.UUID
	Direction string
}

// where builds the WHERE clause every flow query shares. Column names are
// fixed here; every value is bound.
func (f FlowFilter) where(routerID uuid.UUID, timeColumn string) (string, []interface{}) {
	clauses := []string{"router_id = ?", timeColumn + " >= ?", timeColumn + " < ?"}
	args := []interface{}{routerID, f.From, f.To}
	switch {
	case f.OtherPOP:
		clauses = append(clauses, "pop_interface_id IS NULL")
	case f.POP != nil:
		clauses = append(clauses, "pop_interface_id = ?")
		args = append(args, *f.POP)
	}
	if f.Upstream != nil {
		clauses = append(clauses, "upstream_interface_id = ?")
		args = append(args, *f.Upstream)
	}
	if f.Direction != "" {
		clauses = append(clauses, "direction = ?")
		args = append(args, f.Direction)
	}
	return strings.Join(clauses, " AND "), args
}

// withArgs copies before appending, so one filter's arguments can feed
// several queries without one overwriting another's.
func withArgs(args []interface{}, extra ...interface{}) []interface{} {
	out := make([]interface{}, 0, len(args)+len(extra))
	return append(append(out, args...), extra...)
}

func bitsPerSecond(bytes int64, over time.Duration) float64 {
	return float64(bytes) * 8 / over.Seconds()
}

// FlowPoint is one step of a destination's traffic.
type FlowPoint struct {
	Time time.Time `json:"time"`
	Bps  float64   `json:"bps"`
}

// FlowSeries is one destination AS over the range.
type FlowSeries struct {
	ASN    int64       `json:"asn"`
	Name   string      `json:"name"`
	Points []FlowPoint `json:"points"`
}

// FlowDestinations is a stacked chart: the top destinations and Lainnya.
type FlowDestinations struct {
	StepSeconds int64        `json:"step_seconds"`
	Series      []FlowSeries `json:"series"`
}

type asnBucket struct {
	Time  time.Time
	ASN   int64 `gorm:"column:asn"`
	Bytes int64
}

// Destinations returns the limit largest destination ASNs of the range, each
// as bit/s per step, and everything else as one OtherASN series.
func (s *FlowQueryService) Destinations(routerID uuid.UUID, f FlowFilter, limit int) (*FlowDestinations, error) {
	res := resolveFlowRange(f.From, f.To, time.Now())
	where, args := f.where(routerID, res.source.timeColumn)
	top, err := s.topASNs(res.source.table, where, args, limit)
	if err != nil {
		return nil, err
	}
	rows, err := s.bucketedASNs(res, where, args, top)
	if err != nil {
		return nil, err
	}
	names, err := s.asnNames(top)
	if err != nil {
		return nil, err
	}
	return &FlowDestinations{StepSeconds: int64(res.step.Seconds()),
		Series: buildSeries(rows, top, names, res.step)}, nil
}

func (s *FlowQueryService) topASNs(table, where string, args []interface{}, limit int) ([]int64, error) {
	// table comes from flowSources, never from the request.
	query := fmt.Sprintf(`SELECT asn FROM %s WHERE %s AND asn <> 0
		GROUP BY asn ORDER BY SUM(bytes) DESC LIMIT ?`, table, where)
	var asns []int64
	return asns, s.db.Raw(query, withArgs(args, limit)...).Scan(&asns).Error
}

// bucketedASNs folds everything outside top into OtherASN in SQL, so the
// response carries limit+1 series however many ASNs the range holds.
func (s *FlowQueryService) bucketedASNs(res flowResolution, where string, args []interface{}, top []int64) ([]asnBucket, error) {
	asnExpr := "0"
	params := []interface{}{fmt.Sprintf("%d seconds", int64(res.step.Seconds()))}
	if len(top) > 0 {
		asnExpr = "CASE WHEN asn IN ? THEN asn ELSE 0 END"
		params = append(params, top)
	}
	query := fmt.Sprintf(`SELECT time_bucket(?::interval, %s) AS time, %s AS asn, SUM(bytes)::bigint AS bytes
		FROM %s WHERE %s GROUP BY 1, 2 ORDER BY 1`, res.source.timeColumn, asnExpr, res.source.table, where)
	var rows []asnBucket
	return rows, s.db.Raw(query, withArgs(params, args...)...).Scan(&rows).Error
}

func buildSeries(rows []asnBucket, top []int64, names map[int64]string, step time.Duration) []FlowSeries {
	order := append(append([]int64{}, top...), otherASN)
	index := make(map[int64]int, len(order))
	series := make([]FlowSeries, len(order))
	for i, asn := range order {
		index[asn] = i
		series[i] = FlowSeries{ASN: asn, Name: asnLabel(asn, names), Points: []FlowPoint{}}
	}
	for _, r := range rows {
		i := index[r.ASN]
		series[i].Points = append(series[i].Points, FlowPoint{Time: r.Time, Bps: bitsPerSecond(r.Bytes, step)})
	}
	return series
}

// asnNames looks the given ASNs up in asn_names.
func (s *FlowQueryService) asnNames(asns []int64) (map[int64]string, error) {
	names := map[int64]string{}
	if len(asns) == 0 {
		return names, nil
	}
	var rows []models.ASNName
	if err := s.db.Where("asn IN ?", asns).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		names[r.ASN] = r.Name
	}
	return names, nil
}

// asnLabel is Lainnya for ASN 0 and the dataset's name otherwise, or "" for
// an AS the dataset does not name; the UI then shows the number.
func asnLabel(asn int64, names map[int64]string) string {
	if asn == otherASN {
		return otherASNName
	}
	return names[asn]
}
```

- [ ] **Step 6: Write the summary and POP queries**

`backend/internal/services/flow_summary.go`:

```go
package services

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

const (
	// summaryLimit is how many destinations the summary table lists.
	summaryLimit = 50
	// popTopASNs is how many destinations the POP table names per POP.
	popTopASNs = 3
)

// FlowUpstreamShare is one upstream's part of a destination's bytes.
type FlowUpstreamShare struct {
	UpstreamInterfaceID uuid.UUID `json:"upstream_interface_id" gorm:"column:upstream_interface_id"`
	Bytes               int64     `json:"bytes"`
}

// FlowSummaryRow is one destination over the whole range.
type FlowSummaryRow struct {
	ASN       int64               `json:"asn"`
	Name      string              `json:"name"`
	Bytes     int64               `json:"bytes"`
	Percent   float64             `json:"percent"`
	AvgBps    float64             `json:"avg_bps"`
	PeakBps   float64             `json:"peak_bps"`
	Upstreams []FlowUpstreamShare `json:"upstreams"`
}

// FlowSummary is the destination table. PeakWindowSeconds is the step the
// peak was measured over.
type FlowSummary struct {
	TotalBytes        int64            `json:"total_bytes"`
	PeakWindowSeconds int64            `json:"peak_window_seconds"`
	Rows              []FlowSummaryRow `json:"rows"`
}

type asnTotal struct {
	ASN   int64 `gorm:"column:asn"`
	Bytes int64
	Peak  int64
}

// Summary ranks the range's destinations with their share, average, peak and
// split over upstreams.
func (s *FlowQueryService) Summary(routerID uuid.UUID, f FlowFilter) (*FlowSummary, error) {
	src := resolveFlowRange(f.From, f.To, time.Now()).source
	where, args := f.where(routerID, src.timeColumn)
	total, err := s.totalBytes(src.table, where, args)
	if err != nil {
		return nil, err
	}
	totals, err := s.asnTotals(src, where, args)
	if err != nil {
		return nil, err
	}
	asns := make([]int64, len(totals))
	for i, t := range totals {
		asns[i] = t.ASN
	}
	shares, err := s.upstreamShares(src.table, where, args, asns)
	if err != nil {
		return nil, err
	}
	names, err := s.asnNames(asns)
	if err != nil {
		return nil, err
	}
	return &FlowSummary{TotalBytes: total, PeakWindowSeconds: int64(src.step.Seconds()),
		Rows: summaryRows(totals, shares, names, total, f.To.Sub(f.From), src.step)}, nil
}

func summaryRows(totals []asnTotal, shares map[int64][]FlowUpstreamShare, names map[int64]string,
	total int64, span, step time.Duration) []FlowSummaryRow {
	rows := make([]FlowSummaryRow, 0, len(totals))
	for _, t := range totals {
		upstreams := shares[t.ASN]
		if upstreams == nil {
			upstreams = []FlowUpstreamShare{}
		}
		rows = append(rows, FlowSummaryRow{ASN: t.ASN, Name: asnLabel(t.ASN, names), Bytes: t.Bytes,
			Percent: percentOf(t.Bytes, total), AvgBps: bitsPerSecond(t.Bytes, span),
			PeakBps: bitsPerSecond(t.Peak, step), Upstreams: upstreams})
	}
	return rows
}

func percentOf(part, total int64) float64 {
	if total == 0 {
		return 0
	}
	return float64(part) * 100 / float64(total)
}

func (s *FlowQueryService) totalBytes(table, where string, args []interface{}) (int64, error) {
	var total int64
	query := fmt.Sprintf(`SELECT COALESCE(SUM(bytes), 0)::bigint FROM %s WHERE %s`, table, where)
	return total, s.db.Raw(query, args...).Scan(&total).Error
}

// asnTotals ranks destinations by bytes. Peak is the busiest single source
// step, summed over the upstreams and POPs the filter keeps.
func (s *FlowQueryService) asnTotals(src flowSource, where string, args []interface{}) ([]asnTotal, error) {
	query := fmt.Sprintf(`SELECT asn, SUM(step_bytes)::bigint AS bytes, MAX(step_bytes)::bigint AS peak
		FROM (SELECT %s AS step, asn, SUM(bytes) AS step_bytes FROM %s WHERE %s GROUP BY 1, 2) per_step
		GROUP BY asn ORDER BY bytes DESC LIMIT ?`, src.timeColumn, src.table, where)
	var rows []asnTotal
	return rows, s.db.Raw(query, withArgs(args, summaryLimit)...).Scan(&rows).Error
}

func (s *FlowQueryService) upstreamShares(table, where string, args []interface{}, asns []int64) (map[int64][]FlowUpstreamShare, error) {
	out := map[int64][]FlowUpstreamShare{}
	if len(asns) == 0 {
		return out, nil
	}
	var rows []struct {
		ASN int64 `gorm:"column:asn"`
		FlowUpstreamShare
	}
	query := fmt.Sprintf(`SELECT asn, upstream_interface_id, SUM(bytes)::bigint AS bytes FROM %s
		WHERE %s AND asn IN ? GROUP BY 1, 2 ORDER BY 3 DESC`, table, where)
	if err := s.db.Raw(query, withArgs(args, asns)...).Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.ASN] = append(out[r.ASN], r.FlowUpstreamShare)
	}
	return out, nil
}

// FlowASNBytes is one destination's bytes within a POP.
type FlowASNBytes struct {
	ASN   int64  `json:"asn"`
	Name  string `json:"name"`
	Bytes int64  `json:"bytes"`
}

// FlowPOPRow is one POP's total and its largest destinations. A nil POP is
// the traffic of addresses in no POP's blocks.
type FlowPOPRow struct {
	POPInterfaceID *uuid.UUID     `json:"pop_interface_id"`
	Bytes          int64          `json:"bytes"`
	TopASNs        []FlowASNBytes `json:"top_asns"`
}

type popASN struct {
	POPInterfaceID *uuid.UUID `gorm:"column:pop_interface_id"`
	ASN            int64      `gorm:"column:asn"`
	Bytes          int64
}

// POPs lists each POP's total, largest first, with its popTopASNs largest
// destinations.
func (s *FlowQueryService) POPs(routerID uuid.UUID, f FlowFilter) ([]FlowPOPRow, error) {
	src := resolveFlowRange(f.From, f.To, time.Now()).source
	where, args := f.where(routerID, src.timeColumn)
	var totals []popASN
	query := fmt.Sprintf(`SELECT pop_interface_id, SUM(bytes)::bigint AS bytes FROM %s WHERE %s
		GROUP BY 1 ORDER BY 2 DESC`, src.table, where)
	if err := s.db.Raw(query, args...).Scan(&totals).Error; err != nil {
		return nil, err
	}
	tops, err := s.popTopASNs(src.table, where, args)
	if err != nil {
		return nil, err
	}
	asns := make([]int64, len(tops))
	for i, t := range tops {
		asns[i] = t.ASN
	}
	names, err := s.asnNames(asns)
	if err != nil {
		return nil, err
	}
	return buildPOPRows(totals, tops, names), nil
}

func (s *FlowQueryService) popTopASNs(table, where string, args []interface{}) ([]popASN, error) {
	query := fmt.Sprintf(`SELECT pop_interface_id, asn, bytes FROM (
			SELECT pop_interface_id, asn, SUM(bytes)::bigint AS bytes,
				ROW_NUMBER() OVER (PARTITION BY pop_interface_id ORDER BY SUM(bytes) DESC, asn) AS position
			FROM %s WHERE %s AND asn <> 0 GROUP BY 1, 2) ranked
		WHERE position <= ? ORDER BY bytes DESC`, table, where)
	var rows []popASN
	return rows, s.db.Raw(query, withArgs(args, popTopASNs)...).Scan(&rows).Error
}

func popOf(id *uuid.UUID) uuid.UUID {
	if id == nil {
		return uuid.Nil
	}
	return *id
}

func buildPOPRows(totals, tops []popASN, names map[int64]string) []FlowPOPRow {
	byPOP := map[uuid.UUID][]FlowASNBytes{}
	for _, t := range tops {
		key := popOf(t.POPInterfaceID)
		byPOP[key] = append(byPOP[key], FlowASNBytes{ASN: t.ASN, Name: asnLabel(t.ASN, names), Bytes: t.Bytes})
	}
	rows := make([]FlowPOPRow, 0, len(totals))
	for _, t := range totals {
		top := byPOP[popOf(t.POPInterfaceID)]
		if top == nil {
			top = []FlowASNBytes{}
		}
		rows = append(rows, FlowPOPRow{POPInterfaceID: t.POPInterfaceID, Bytes: t.Bytes, TopASNs: top})
	}
	return rows
}
```

- [ ] **Step 7: Write the top-IP and status reads**

`backend/internal/services/flow_top_ips.go`:

```go
package services

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
)

const (
	// topIPRetention matches router_flow_top_ips' retention policy.
	topIPRetention = 7 * 24 * time.Hour
	// topIPRangeSlack lets the 7-day preset through: its start is computed in
	// the browser a moment before this check runs.
	topIPRangeSlack = 5 * time.Minute
	topIPLimit      = 100
)

// ErrTopIPsRange refuses a top-IP read reaching before what the table keeps.
var ErrTopIPsRange = fmt.Errorf("%w: top IPs are kept for 7 days; the range starts before that", ErrValidation)

// FlowTopIP is one destination address over the range.
type FlowTopIP struct {
	IP    string `json:"ip" gorm:"column:ip"`
	ASN   int64  `json:"asn" gorm:"column:asn"`
	Name  string `json:"name" gorm:"-"`
	Bytes int64  `json:"bytes"`
}

// FlowStatus describes the ASN dataset flowd last loaded.
type FlowStatus struct {
	ASNDatasetUpdatedAt *time.Time `json:"asn_dataset_updated_at"`
	ASNCount            int64      `json:"asn_count"`
}

func checkTopIPsRange(from, now time.Time) error {
	if now.Sub(from) > topIPRetention+topIPRangeSlack {
		return ErrTopIPsRange
	}
	return nil
}

// TopIPs sums each address's windows over the range. A window kept only its
// top addresses, so one just under the cut in some windows is undercounted
// there: this ranks heavy hitters, it does not total them.
func (s *FlowQueryService) TopIPs(routerID uuid.UUID, f FlowFilter, now time.Time) ([]FlowTopIP, error) {
	if err := checkTopIPsRange(f.From, now); err != nil {
		return nil, err
	}
	f.Upstream = nil // the table has no upstream column
	where, args := f.where(routerID, "time")
	query := fmt.Sprintf(`SELECT host(ip) AS ip, asn, SUM(bytes)::bigint AS bytes FROM router_flow_top_ips
		WHERE %s GROUP BY ip, asn ORDER BY 3 DESC LIMIT ?`, where)
	rows := []FlowTopIP{}
	if err := s.db.Raw(query, withArgs(args, topIPLimit)...).Scan(&rows).Error; err != nil {
		return nil, err
	}
	asns := make([]int64, len(rows))
	for i, r := range rows {
		asns[i] = r.ASN
	}
	names, err := s.asnNames(asns)
	if err != nil {
		return nil, err
	}
	for i := range rows {
		rows[i].Name = asnLabel(rows[i].ASN, names)
	}
	return rows, nil
}

// Status reports the newest dataset's download time and how many ASes it
// names. flowd writes every name with its dataset's time, so the newest row
// dates the dataset in use.
func (s *FlowQueryService) Status() (*FlowStatus, error) {
	var count int64
	if err := s.db.Model(&models.ASNName{}).Count(&count).Error; err != nil {
		return nil, err
	}
	if count == 0 {
		return &FlowStatus{}, nil
	}
	var newest models.ASNName
	if err := s.db.Order("updated_at DESC").First(&newest).Error; err != nil {
		return nil, err
	}
	return &FlowStatus{ASNDatasetUpdatedAt: &newest.UpdatedAt, ASNCount: count}, nil
}
```

- [ ] **Step 8: Run the tests to verify they pass**

Run: `cd backend && go test ./internal/services/ -run 'ResolveFlowRange|TopIPsRefuses|StatusReports' -v -race && TEST_POSTGRES_DSN="postgres://tikman:testpw@localhost:5439/tikman_test?sslmode=disable" go test ./internal/services/ -run 'Destinations|Summary|POPs|TopIPsSum' -v -count=1`
Expected: PASS — 3 SQLite tests, 6 Postgres tests. (A Postgres failure on `gin_trgm_ops` or stale continuous-aggregate state is the known stale test database: recreate `tikman_test` as in Task 5.)

- [ ] **Step 9: Commit**

```bash
cd backend && gofmt -s -l ./internal/services
git add internal/services/flow_resolution.go internal/services/flow_resolution_test.go \
  internal/services/flow_query.go internal/services/flow_summary.go internal/services/flow_top_ips.go \
  internal/services/flow_query_test.go internal/services/flow_query_postgres_test.go
git commit -m "feat(flows): destination, summary, POP and top-IP queries with automatic resolution

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 10: API — flow reads and flow settings

**Files:**
- Create: `backend/internal/api/router_flow_handler.go`
- Modify: `backend/internal/api/router_handler_reads.go` (`routerTimeRange` → shared `timeRangeWithin`)
- Modify: `backend/internal/api/router_dto.go` (`UpdateRouterRequest`, `toUpdate`)
- Modify: `backend/internal/api/router_handler.go` (`Update` audit values)
- Modify: `backend/internal/api/router_handlers.go` (handler field and wiring)
- Modify: `backend/internal/api/router_routes_mikrotik.go` (routes)
- Modify: `backend/internal/services/router_service.go` (`RouterUpdate`, `updateColumns`)
- Test: `backend/internal/api/router_flow_handler_test.go` (new), `backend/internal/api/router_handler_test.go`, `backend/internal/api/router_rbac_test.go`, `backend/internal/services/router_service_test.go`

**Interfaces:**
- Consumes: `services.FlowQueryService` and everything it produces (Task 9); `netflow.Download`, `netflow.Upload` (Task 3); existing `routerParam`, `respondRouterError`, `ErrorResponse`, `setupRouterHandlerTest`, `createRouterVia`, `rbacTestRouter`, `requestAs`, `SetupTestContext`.
- Produces (HTTP, all under the authenticated `/api/v1/routers` group, every role):
  - `GET /:id/flows/destinations?from&to&pop&upstream&direction&limit` → `FlowDestinations` (limit 1–20, default 10)
  - `GET /:id/flows/summary?from&to&pop&upstream&direction` → `FlowSummary`
  - `GET /:id/flows/pops?from&to&direction` → `[]FlowPOPRow`
  - `GET /:id/flows/top-ips?from&to&pop&direction` → `[]FlowTopIP`; a range starting more than 7 days back → 400 `TOP_IPS_RANGE`
  - `GET /:id/flows/status` → `FlowStatus`
  - `pop` is an interface UUID or `other` (addresses in no POP's blocks); `direction` is `download` or `upload`; bad values → 400 `INVALID_FILTER`; `from`/`to` RFC3339 at most 5 years apart → else 400 `INVALID_RANGE`.
  - `PUT /:id` (Admin) also takes `flow_enabled` (bool) and `flow_sampling` (1–10000). The router JSON carries `flow_enabled`, `flow_sampling`, `flow_last_seen_at`.

Rulings recorded here:
- **`GET /:id/flows/status`** is added beyond the spec's four reads: the Router tab must show the ASN dataset's version, which only `asn_names` knows.
- Flow reads accept ranges up to **5 years** (the existing graphs stop at 400 days): the daily rollup is kept forever and planning wants multi-year views, but one request must still be bounded.
- The flow settings are offered on **edit only**: the spec has `PUT` carry them, and the add flow is about reaching the router at all.

- [ ] **Step 1: Write the failing service tests**

Append to `backend/internal/services/router_service_test.go`:

```go
func TestUpdateRouterSetsFlowCollection(t *testing.T) {
	service, _ := newRouterServiceForTest(t)
	router, err := service.Create(validRouterInput())
	require.NoError(t, err)
	enabled, sampling := true, 512

	updated, err := service.Update(router.ID, RouterUpdate{FlowEnabled: &enabled, FlowSampling: &sampling})

	require.NoError(t, err)
	assert.True(t, updated.FlowEnabled)
	assert.Equal(t, 512, updated.FlowSampling)
}

func TestUpdateRouterRejectsASamplingRatioOutOfRange(t *testing.T) {
	service, _ := newRouterServiceForTest(t)
	router, err := service.Create(validRouterInput())
	require.NoError(t, err)

	for _, ratio := range []int{0, 10001} {
		_, err := service.Update(router.ID, RouterUpdate{FlowSampling: &ratio})
		assert.ErrorIs(t, err, ErrValidation, ratio)
	}
}
```

- [ ] **Step 2: Write the failing handler tests**

Append to `backend/internal/api/router_handler_test.go`:

```go
func TestUpdateRouterSetsFlowCollection(t *testing.T) {
	handler, _ := setupRouterHandlerTest(t)
	created := createRouterVia(t, handler)
	w, c := SetupTestContext(http.MethodPut, "/api/v1/routers/"+created.ID.String(),
		map[string]interface{}{"flow_enabled": true, "flow_sampling": 512})
	c.Params = gin.Params{{Key: "id", Value: created.ID.String()}}

	handler.Update(c)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"flow_enabled":true`)
	assert.Contains(t, w.Body.String(), `"flow_sampling":512`)
}

func TestUpdateRouterRefusesASamplingRatioOutOfRange(t *testing.T) {
	handler, _ := setupRouterHandlerTest(t)
	created := createRouterVia(t, handler)
	for _, ratio := range []int{0, 10001} {
		w, c := SetupTestContext(http.MethodPut, "/api/v1/routers/"+created.ID.String(),
			map[string]interface{}{"flow_sampling": ratio})
		c.Params = gin.Params{{Key: "id", Value: created.ID.String()}}

		handler.Update(c)

		assert.Equal(t, http.StatusBadRequest, w.Code, ratio)
	}
}
```

Create `backend/internal/api/router_flow_handler_test.go`:

```go
package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"gorm.io/gorm"
)

func setupRouterFlowHandlerTest(t *testing.T) (*RouterFlowHandler, RouterResponse, *gorm.DB) {
	t.Helper()
	routers, db := setupRouterHandlerTest(t)
	created := createRouterVia(t, routers)
	flows := NewRouterFlowHandler(services.NewRouterService(db, testEncryptionKey), services.NewFlowQueryService(db))
	return flows, created, db
}

func flowContext(routerID, query string) (*httptest.ResponseRecorder, *gin.Context) {
	w, c := SetupTestContext(http.MethodGet, "/api/v1/routers/"+routerID+"/flows/read?"+query, nil)
	c.Params = gin.Params{{Key: "id", Value: routerID}}
	return w, c
}

func rangeQuery(span time.Duration) string {
	now := time.Now().UTC()
	return "from=" + now.Add(-span).Format(time.RFC3339) + "&to=" + now.Format(time.RFC3339)
}

func TestTopIPsRefusesARangeBeyondSevenDays(t *testing.T) {
	flows, created, _ := setupRouterFlowHandlerTest(t)
	w, c := flowContext(created.ID.String(), rangeQuery(30*24*time.Hour))

	flows.TopIPs(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "TOP_IPS_RANGE")
}

func TestFlowReadsRefuseMalformedRequests(t *testing.T) {
	flows, created, _ := setupRouterFlowHandlerTest(t)
	hour := rangeQuery(time.Hour)
	cases := map[string]string{
		"direction": hour + "&direction=sideways",
		"pop":       hour + "&pop=vlan-pop-a",
		"upstream":  hour + "&upstream=transit",
		"limit":     hour + "&limit=50",
		"six years": rangeQuery(6 * 366 * 24 * time.Hour),
		"no range":  "",
	}
	for name, query := range cases {
		w, c := flowContext(created.ID.String(), query)

		flows.Destinations(c)

		assert.Equal(t, http.StatusBadRequest, w.Code, name)
	}
}

func TestFlowReadsOfAnUnknownRouterAre404(t *testing.T) {
	flows, _, _ := setupRouterFlowHandlerTest(t)
	w, c := flowContext(uuid.NewString(), rangeQuery(time.Hour))

	flows.Summary(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestFlowStatusReportsTheASNDataset(t *testing.T) {
	flows, created, db := setupRouterFlowHandlerTest(t)
	require.NoError(t, db.Create(&models.ASNName{ASN: 64496, Name: "EXAMPLE-NET-A", Country: "ZZ",
		UpdatedAt: time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)}).Error)
	w, c := flowContext(created.ID.String(), "")

	flows.Status(c)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"asn_count":1`)
	assert.Contains(t, w.Body.String(), `"asn_dataset_updated_at":"2026-10-12T00:00:00`)
}
```

Append to `backend/internal/api/router_rbac_test.go`:

```go
// Every role reads traffic destinations. A missing route would answer 404; a
// present one refuses a request without a range with 400, whatever the role.
func TestEveryRoleReadsFlows(t *testing.T) {
	router, store := rbacTestRouter(t)
	id := uuid.NewString()

	for _, role := range []models.UserRole{models.UserRoleViewer, models.UserRoleTechnician, models.UserRoleAdmin} {
		for _, read := range []string{"destinations", "summary", "pops", "top-ips"} {
			code := requestAs(t, router, store, role, http.MethodGet, "/api/v1/routers/"+id+"/flows/"+read)
			assert.Equal(t, http.StatusBadRequest, code, "%s %s", role, read)
		}
		code := requestAs(t, router, store, role, http.MethodGet, "/api/v1/routers/"+id+"/flows/status")
		assert.NotEqual(t, http.StatusForbidden, code, role)
	}
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `cd backend && go test ./internal/services/ -run 'UpdateRouter' -v; go test ./internal/api/ -run 'Router|Flow' -v 2>&1 | tail -20`
Expected: FAIL to compile — `unknown field FlowEnabled in struct literal of type RouterUpdate`, `undefined: NewRouterFlowHandler`.

- [ ] **Step 4: Accept the flow settings on update**

In `backend/internal/services/router_service.go`:

- add to `RouterUpdate` after `Enabled *bool`:

```go
	FlowEnabled   *bool
	FlowSampling  *int
```

- add next to `fingerprintLength`:

```go
// The sampling ratio's bounds, as migration 58's CHECK enforces them.
const (
	minFlowSampling = 1
	maxFlowSampling = 10000
)
```

- in `updateColumns`, after `setIfGiven(updates, "enabled", upd.Enabled)`:

```go
	if upd.FlowSampling != nil && (*upd.FlowSampling < minFlowSampling || *upd.FlowSampling > maxFlowSampling) {
		return nil, fmt.Errorf("%w: flow_sampling must be between %d and %d", ErrValidation, minFlowSampling, maxFlowSampling)
	}
	setIfGiven(updates, "flow_enabled", upd.FlowEnabled)
	setIfGiven(updates, "flow_sampling", upd.FlowSampling)
```

In `backend/internal/api/router_dto.go`, add to `UpdateRouterRequest` after `Enabled`:

```go
	FlowEnabled   *bool      `json:"flow_enabled"`
	FlowSampling  *int       `json:"flow_sampling" binding:"omitempty,min=1,max=10000"`
```

and in `toUpdate` add `FlowEnabled: r.FlowEnabled, FlowSampling: r.FlowSampling,` to the literal.

In `backend/internal/api/router_handler.go` `Update`, add `"flow_enabled": router.FlowEnabled, "flow_sampling": router.FlowSampling,` to the audit values map.

- [ ] **Step 5: Share the range parsing**

In `backend/internal/api/router_handler_reads.go`, add `"fmt"` to the imports and replace `routerTimeRange` with:

```go
func routerTimeRange(c *gin.Context) (time.Time, time.Time, bool) {
	return timeRangeWithin(c, maxRouterRange)
}

// timeRangeWithin reads from and to, refusing a range longer than maxSpan.
func timeRangeWithin(c *gin.Context, maxSpan time.Duration) (time.Time, time.Time, bool) {
	from, errFrom := time.Parse(time.RFC3339, c.Query("from"))
	to, errTo := time.Parse(time.RFC3339, c.Query("to"))
	if errFrom != nil || errTo != nil || !to.After(from) || to.Sub(from) > maxSpan {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: fmt.Sprintf("from and to must be RFC3339, with to after from and at most %d days apart",
				int(maxSpan.Hours()/24)),
			Code: "INVALID_RANGE",
		})
		return time.Time{}, time.Time{}, false
	}
	return from, to, true
}
```

- [ ] **Step 6: Write the flow handler**

`backend/internal/api/router_flow_handler.go`:

```go
package api

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/netflow"
	"github.com/tikman/olt-provisioning/internal/services"
)

const (
	// maxFlowRange bounds one flow read. The daily rollup is kept forever;
	// five years covers planning and still bounds what a request scans.
	maxFlowRange     = 5 * 366 * 24 * time.Hour
	defaultFlowLimit = 10
	maxFlowLimit     = 20
	// flowPOPOther selects the traffic of addresses in no POP's blocks.
	flowPOPOther = "other"
)

// RouterFlowHandler serves a router's traffic destinations.
type RouterFlowHandler struct {
	routers *services.RouterService
	flows   *services.FlowQueryService
}

// NewRouterFlowHandler creates a flow handler.
func NewRouterFlowHandler(routers *services.RouterService, flows *services.FlowQueryService) *RouterFlowHandler {
	return &RouterFlowHandler{routers: routers, flows: flows}
}

// Destinations is the stacked chart of the largest destinations.
func (h *RouterFlowHandler) Destinations(c *gin.Context) {
	id, filter, ok := h.flowRequest(c)
	if !ok {
		return
	}
	limit, ok := flowLimit(c)
	if !ok {
		return
	}
	out, err := h.flows.Destinations(id, filter, limit)
	respondFlow(c, out, err)
}

// Summary is the destination table.
func (h *RouterFlowHandler) Summary(c *gin.Context) {
	id, filter, ok := h.flowRequest(c)
	if !ok {
		return
	}
	out, err := h.flows.Summary(id, filter)
	respondFlow(c, out, err)
}

// POPs is the per-POP table; it is filtered by direction only, since its
// rows are what the POP filter is chosen from.
func (h *RouterFlowHandler) POPs(c *gin.Context) {
	id, filter, ok := h.flowRequest(c)
	if !ok {
		return
	}
	out, err := h.flows.POPs(id, services.FlowFilter{From: filter.From, To: filter.To, Direction: filter.Direction})
	respondFlow(c, out, err)
}

// TopIPs lists the largest destination addresses of the last 7 days.
func (h *RouterFlowHandler) TopIPs(c *gin.Context) {
	id, filter, ok := h.flowRequest(c)
	if !ok {
		return
	}
	out, err := h.flows.TopIPs(id, filter, time.Now())
	respondFlow(c, out, err)
}

// Status describes the ASN dataset in use.
func (h *RouterFlowHandler) Status(c *gin.Context) {
	if _, ok := h.existingRouter(c); !ok {
		return
	}
	out, err := h.flows.Status()
	respondFlow(c, out, err)
}

// flowRequest reads what every range read shares. Range and filters are
// checked before the router is looked up, so a malformed request is a 400
// whichever router it names.
func (h *RouterFlowHandler) flowRequest(c *gin.Context) (uuid.UUID, services.FlowFilter, bool) {
	from, to, ok := timeRangeWithin(c, maxFlowRange)
	if !ok {
		return uuid.Nil, services.FlowFilter{}, false
	}
	filter, err := parseFlowFilter(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error(), Code: "INVALID_FILTER"})
		return uuid.Nil, services.FlowFilter{}, false
	}
	filter.From, filter.To = from, to
	id, ok := h.existingRouter(c)
	return id, filter, ok
}

func (h *RouterFlowHandler) existingRouter(c *gin.Context) (uuid.UUID, bool) {
	id, ok := routerParam(c, "id")
	if !ok {
		return uuid.Nil, false
	}
	if _, err := h.routers.Get(id); err != nil {
		respondRouterError(c, err)
		return uuid.Nil, false
	}
	return id, true
}

func parseFlowFilter(c *gin.Context) (services.FlowFilter, error) {
	var f services.FlowFilter
	if err := parsePOP(c.Query("pop"), &f); err != nil {
		return f, err
	}
	if upstream := c.Query("upstream"); upstream != "" {
		id, err := uuid.Parse(upstream)
		if err != nil {
			return f, errors.New("upstream must be an interface ID")
		}
		f.Upstream = &id
	}
	switch direction := c.Query("direction"); direction {
	case "", string(netflow.Download), string(netflow.Upload):
		f.Direction = direction
	default:
		return f, errors.New("direction must be download or upload")
	}
	return f, nil
}

func parsePOP(pop string, f *services.FlowFilter) error {
	switch pop {
	case "":
		return nil
	case flowPOPOther:
		f.OtherPOP = true
		return nil
	}
	id, err := uuid.Parse(pop)
	if err != nil {
		return errors.New(`pop must be an interface ID or "other"`)
	}
	f.POP = &id
	return nil
}

func flowLimit(c *gin.Context) (int, bool) {
	text := c.Query("limit")
	if text == "" {
		return defaultFlowLimit, true
	}
	limit, err := strconv.Atoi(text)
	if err != nil || limit < 1 || limit > maxFlowLimit {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "limit must be between 1 and 20", Code: "INVALID_FILTER"})
		return 0, false
	}
	return limit, true
}

func respondFlow(c *gin.Context, out interface{}, err error) {
	switch {
	case errors.Is(err, services.ErrTopIPsRange):
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error(), Code: "TOP_IPS_RANGE"})
	case err != nil:
		respondRouterError(c, err)
	default:
		c.JSON(http.StatusOK, out)
	}
}
```

- [ ] **Step 7: Wire and route it**

In `backend/internal/api/router_handlers.go`, add the field `routerFlowHandler *RouterFlowHandler` after `routerHandler` in `handlers`, and in `newHandlers` after the `routerHandler:` entry:

```go
		routerFlowHandler: NewRouterFlowHandler(services.NewRouterService(db, cfg.EncryptionKey),
			services.NewFlowQueryService(db)),
```

In `backend/internal/api/router_routes_mikrotik.go`, add inside the `routers` block after the `bgp/events` line:

```go
		routers.GET("/:id/flows/destinations", h.routerFlowHandler.Destinations)
		routers.GET("/:id/flows/summary", h.routerFlowHandler.Summary)
		routers.GET("/:id/flows/pops", h.routerFlowHandler.POPs)
		routers.GET("/:id/flows/top-ips", h.routerFlowHandler.TopIPs)
		routers.GET("/:id/flows/status", h.routerFlowHandler.Status)
```

- [ ] **Step 8: Run the tests to verify they pass**

Run: `cd backend && go build ./... && go test ./internal/services/ -run 'UpdateRouter' -race -count=1 && go test ./internal/api/ -run 'Router|Flow' -race -count=1`
Expected: PASS. (`internal/api` only with `-run`: its WireGuard tests answer 400 on the dev Mac while `ppp0` holds 10.0.0.0/8, which is environmental.)

- [ ] **Step 9: Commit**

```bash
cd backend && gofmt -s -l ./internal
git add internal/api internal/services/router_service.go internal/services/router_service_test.go
git commit -m "feat(api): traffic destination reads and per-router flow settings

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 11: Frontend data layer

**Files:**
- Create: `frontend/src/domain/entities/RouterFlow.ts`
- Modify: `frontend/src/domain/entities/index.ts`
- Modify: `frontend/src/domain/entities/MikrotikRouter.ts` (`MikrotikRouter`, `UpdateRouterDto`)
- Modify: `frontend/src/infrastructure/http/endpoints.ts`
- Create: `frontend/src/infrastructure/repositories/RouterFlowRepository.ts`
- Modify: `frontend/src/infrastructure/repositories/index.ts`
- Create: `frontend/src/application/hooks/useRouterFlows.ts`
- Create: `frontend/src/presentation/components/routers/routerFlows.ts`
- Test: `frontend/src/presentation/components/routers/routerFlows.test.ts`

**Interfaces:**
- Consumes: the HTTP API of Task 10 (camelized by the axios client); existing `TrafficRange`, `TimeRange`, `resolveTrafficRange`, `trafficRangeKey`, `liveRefetchInterval`, `ChartRow`.
- Produces:
  - Types `FlowDirection`, `FlowFilter { pop?; upstream?; direction? }`, `FlowPoint`, `FlowSeries`, `FlowDestinations { stepSeconds; series }`, `FlowUpstreamShare { upstreamInterfaceId; bytes }`, `FlowSummaryRow { asn; name; bytes; percent; avgBps; peakBps; upstreams }`, `FlowSummary { totalBytes; peakWindowSeconds; rows }`, `FlowAsnBytes`, `FlowPopRow { popInterfaceId: string | null; bytes; topAsns }`, `FlowTopIp { ip; asn; name; bytes }`, `FlowStatus { asnDatasetUpdatedAt: string | null; asnCount }`; `MikrotikRouter.flowEnabled/flowSampling/flowLastSeenAt`; `UpdateRouterDto.flowEnabled?/flowSampling?`.
  - Hooks `useFlowDestinations(routerId, range, filter)`, `useFlowSummary(routerId, range, filter)`, `useFlowPops(routerId, range, direction?)`, `useFlowTopIps(routerId, range, filter, enabled)`, `useFlowStatus(routerId)`.
  - Helpers in `routerFlows.ts`: `OTHER_POP = "other"`, `FLOW_RANGE_PRESETS`, `DEFAULT_FLOW_RANGE`, `TOP_IPS_RETENTION_MS`, `FLOW_STALE_MS`, `asnLabel(asn, name)`, `popLabel(id, labels)`, `flowChartRows(data)`, `topIpsAvailable(range, now?)`, `flowStale(router, nowMs)`.

- [ ] **Step 1: Write the failing helper tests**

`frontend/src/presentation/components/routers/routerFlows.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import type { FlowDestinations, MikrotikRouter } from "@/domain/entities";
import {
  asnLabel,
  flowChartRows,
  flowStale,
  popLabel,
  topIpsAvailable,
} from "./routerFlows";

const NOW = new Date("2026-10-12T12:00:00Z");
const DAY_HOURS = 24;

describe("asnLabel", () => {
  it("names ASN 0 Lainnya and shows the number beside a name", () => {
    expect(asnLabel(0, "Lainnya")).toBe("Lainnya");
    expect(asnLabel(64496, "EXAMPLE-NET-A")).toBe("EXAMPLE-NET-A (AS64496)");
    expect(asnLabel(64497, "")).toBe("AS64497");
  });
});

describe("popLabel", () => {
  it("names the POP by its interface, and null as outside every POP", () => {
    const labels = new Map([["pop-a", "POP A"]]);
    expect(popLabel("pop-a", labels)).toBe("POP A");
    expect(popLabel(null, labels)).toBe("Lainnya (di luar blok POP)");
  });
});

describe("flowChartRows", () => {
  const data: FlowDestinations = {
    stepSeconds: 60,
    series: [
      {
        asn: 64496,
        name: "EXAMPLE-NET-A",
        points: [
          { time: "2026-10-12T03:01:00Z", bps: 200 },
          { time: "2026-10-12T03:00:00Z", bps: 100 },
        ],
      },
      { asn: 64497, name: "", points: [] },
      {
        asn: 0,
        name: "Lainnya",
        points: [{ time: "2026-10-12T03:01:00Z", bps: 5 }],
      },
    ],
  };

  it("pivots series into time-ordered rows, zero where a series has no point", () => {
    const { rows, series } = flowChartRows(data);

    expect(series).toEqual(["EXAMPLE-NET-A (AS64496)", "Lainnya"]);
    expect(rows).toEqual([
      {
        time: "2026-10-12T03:00:00Z",
        "EXAMPLE-NET-A (AS64496)": 100,
        Lainnya: 0,
      },
      {
        time: "2026-10-12T03:01:00Z",
        "EXAMPLE-NET-A (AS64496)": 200,
        Lainnya: 5,
      },
    ]);
  });

  it("draws nothing before data arrives", () => {
    expect(flowChartRows(undefined)).toEqual({ rows: [], series: [] });
  });
});

describe("topIpsAvailable", () => {
  it("allows the 7-day preset and nothing reaching further back", () => {
    expect(
      topIpsAvailable({ kind: "relative", hours: 7 * DAY_HOURS }, NOW),
    ).toBe(true);
    expect(
      topIpsAvailable({ kind: "relative", hours: 30 * DAY_HOURS }, NOW),
    ).toBe(false);
    expect(
      topIpsAvailable(
        {
          kind: "absolute",
          from: "2026-10-04T12:00:00Z",
          to: "2026-10-05T12:00:00Z",
        },
        NOW,
      ),
    ).toBe(false);
  });
});

describe("flowStale", () => {
  const router = (over: Partial<MikrotikRouter>) =>
    ({
      flowEnabled: true,
      flowLastSeenAt: null,
      ...over,
    }) as MikrotikRouter;
  const minutesAgo = (m: number) =>
    new Date(NOW.getTime() - m * 60_000).toISOString();

  it("warns only when collection is on and nothing arrived for 5 minutes", () => {
    expect(flowStale(router({ flowEnabled: false }), NOW.getTime())).toBe(
      false,
    );
    expect(flowStale(router({}), NOW.getTime())).toBe(true);
    expect(
      flowStale(router({ flowLastSeenAt: minutesAgo(2) }), NOW.getTime()),
    ).toBe(false);
    expect(
      flowStale(router({ flowLastSeenAt: minutesAgo(6) }), NOW.getTime()),
    ).toBe(true);
  });
});
```

- [ ] **Step 2: Run them to verify they fail**

Run: `cd frontend && npm test -- --run src/presentation/components/routers/routerFlows.test.ts`
Expected: FAIL — `Failed to resolve import "./routerFlows"` (and the `Flow*` types do not exist yet).

- [ ] **Step 3: Write the types**

`frontend/src/domain/entities/RouterFlow.ts`:

```ts
export type FlowDirection = "download" | "upload";

/** Narrows a flow read; pop "other" is traffic of addresses in no POP's blocks. */
export interface FlowFilter {
  pop?: string;
  upstream?: string;
  direction?: FlowDirection;
}

export interface FlowPoint {
  time: string;
  bps: number;
}

/** One destination AS; ASN 0 is "Lainnya", an empty name is an unnamed AS. */
export interface FlowSeries {
  asn: number;
  name: string;
  points: FlowPoint[];
}

export interface FlowDestinations {
  stepSeconds: number;
  series: FlowSeries[];
}

export interface FlowUpstreamShare {
  upstreamInterfaceId: string;
  bytes: number;
}

export interface FlowSummaryRow {
  asn: number;
  name: string;
  bytes: number;
  percent: number;
  avgBps: number;
  peakBps: number;
  upstreams: FlowUpstreamShare[];
}

export interface FlowSummary {
  totalBytes: number;
  peakWindowSeconds: number;
  rows: FlowSummaryRow[];
}

export interface FlowAsnBytes {
  asn: number;
  name: string;
  bytes: number;
}

/** A null POP is the traffic of addresses in no POP's blocks. */
export interface FlowPopRow {
  popInterfaceId: string | null;
  bytes: number;
  topAsns: FlowAsnBytes[];
}

export interface FlowTopIp {
  ip: string;
  asn: number;
  name: string;
  bytes: number;
}

export interface FlowStatus {
  asnDatasetUpdatedAt: string | null;
  asnCount: number;
}
```

In `frontend/src/domain/entities/index.ts`, after `export * from "./MikrotikRouter";` add:

```ts
export * from "./RouterFlow";
```

In `frontend/src/domain/entities/MikrotikRouter.ts`, add to `MikrotikRouter` after `lastSeenAt`:

```ts
  flowEnabled: boolean;
  flowSampling: number;
  flowLastSeenAt: string | null;
```

and to `UpdateRouterDto` after `enabled?: boolean;`:

```ts
  flowEnabled?: boolean;
  flowSampling?: number;
```

- [ ] **Step 4: Write the repository and hooks**

In `frontend/src/infrastructure/http/endpoints.ts`, after `MIKROTIK_ROUTER_EVENTS`:

```ts
  MIKROTIK_ROUTER_FLOWS: (id: string, read: string) =>
    `/api/v1/routers/${id}/flows/${read}`,
```

`frontend/src/infrastructure/repositories/RouterFlowRepository.ts`:

```ts
import { apiClient } from "../http/apiClient";
import { API_ENDPOINTS } from "../http/endpoints";
import type {
  FlowDestinations,
  FlowFilter,
  FlowPopRow,
  FlowStatus,
  FlowSummary,
  FlowTopIp,
  TimeRange,
} from "@/domain/entities";

export class RouterFlowRepository {
  private async read<T>(id: string, read: string, params: object = {}) {
    const response = await apiClient.get<T>(
      API_ENDPOINTS.MIKROTIK_ROUTER_FLOWS(id, read),
      { params },
    );
    return response.data;
  }

  destinations(
    id: string,
    range: TimeRange,
    filter: FlowFilter,
  ): Promise<FlowDestinations> {
    return this.read(id, "destinations", { ...range, ...filter });
  }

  summary(
    id: string,
    range: TimeRange,
    filter: FlowFilter,
  ): Promise<FlowSummary> {
    return this.read(id, "summary", { ...range, ...filter });
  }

  pops(
    id: string,
    range: TimeRange,
    filter: FlowFilter,
  ): Promise<FlowPopRow[]> {
    return this.read(id, "pops", { ...range, ...filter });
  }

  topIps(
    id: string,
    range: TimeRange,
    filter: FlowFilter,
  ): Promise<FlowTopIp[]> {
    return this.read(id, "top-ips", { ...range, ...filter });
  }

  status(id: string): Promise<FlowStatus> {
    return this.read(id, "status");
  }
}
```

In `frontend/src/infrastructure/repositories/index.ts`, add:

```ts
export * from "./RouterFlowRepository";
```

`frontend/src/application/hooks/useRouterFlows.ts`:

```ts
import { useQuery } from "@tanstack/react-query";
import { RouterFlowRepository } from "@/infrastructure/repositories";
import type {
  FlowDirection,
  FlowFilter,
  TrafficRange,
} from "@/domain/entities";
import {
  liveRefetchInterval,
  resolveTrafficRange,
  trafficRangeKey,
} from "./trafficRange";

const repository = new RouterFlowRepository();

// flowd writes once a minute; reading faster returns the same rows.
const FLOW_POLL_MS = 60_000;

function flowKey(
  routerId: string,
  read: string,
  range: TrafficRange,
  filter: FlowFilter,
) {
  return [
    "mikrotik",
    routerId,
    "flows",
    read,
    ...trafficRangeKey(range),
    filter.pop ?? "",
    filter.upstream ?? "",
    filter.direction ?? "",
  ];
}

export function useFlowDestinations(
  routerId: string,
  range: TrafficRange,
  filter: FlowFilter,
) {
  return useQuery({
    queryKey: flowKey(routerId, "destinations", range, filter),
    queryFn: () =>
      repository.destinations(routerId, resolveTrafficRange(range), filter),
    refetchInterval: liveRefetchInterval(range, FLOW_POLL_MS),
  });
}

export function useFlowSummary(
  routerId: string,
  range: TrafficRange,
  filter: FlowFilter,
) {
  return useQuery({
    queryKey: flowKey(routerId, "summary", range, filter),
    queryFn: () =>
      repository.summary(routerId, resolveTrafficRange(range), filter),
    refetchInterval: liveRefetchInterval(range, FLOW_POLL_MS),
  });
}

export function useFlowPops(
  routerId: string,
  range: TrafficRange,
  direction?: FlowDirection,
) {
  const filter = { direction };
  return useQuery({
    queryKey: flowKey(routerId, "pops", range, filter),
    queryFn: () =>
      repository.pops(routerId, resolveTrafficRange(range), filter),
    refetchInterval: liveRefetchInterval(range, FLOW_POLL_MS),
  });
}

/** Disabled past seven days: the server keeps top addresses no longer. */
export function useFlowTopIps(
  routerId: string,
  range: TrafficRange,
  filter: FlowFilter,
  enabled: boolean,
) {
  return useQuery({
    queryKey: flowKey(routerId, "top-ips", range, filter),
    queryFn: () =>
      repository.topIps(routerId, resolveTrafficRange(range), filter),
    refetchInterval: liveRefetchInterval(range, FLOW_POLL_MS),
    enabled,
  });
}

export function useFlowStatus(routerId: string) {
  return useQuery({
    queryKey: ["mikrotik", routerId, "flows", "status"],
    queryFn: () => repository.status(routerId),
    refetchInterval: FLOW_POLL_MS,
  });
}
```

- [ ] **Step 5: Write the helpers**

`frontend/src/presentation/components/routers/routerFlows.ts`:

```ts
import type {
  FlowDestinations,
  MikrotikRouter,
  TrafficRange,
} from "@/domain/entities";
import { resolveTrafficRange } from "@/application/hooks/trafficRange";
import type { ChartRow } from "./routerTraffic";

const HOURS_PER_DAY = 24;
const MINUTE_MS = 60_000;

/** The POP filter value for addresses in no POP's blocks. */
export const OTHER_POP = "other";

/** Must match the backend's topIPRetention. */
export const TOP_IPS_RETENTION_MS = 7 * HOURS_PER_DAY * 60 * MINUTE_MS;

/** flowd marks a router heard every minute; five silent minutes is a stop. */
export const FLOW_STALE_MS = 5 * MINUTE_MS;

export const FLOW_RANGE_PRESETS = [
  { label: "15 menit", value: 0.25 },
  { label: "1 jam", value: 1 },
  { label: "24 jam", value: HOURS_PER_DAY },
  { label: "7 hari", value: 7 * HOURS_PER_DAY },
  { label: "30 hari", value: 30 * HOURS_PER_DAY },
  { label: "90 hari", value: 90 * HOURS_PER_DAY },
  { label: "1 tahun", value: 365 * HOURS_PER_DAY },
];

export const DEFAULT_FLOW_RANGE: TrafficRange = { kind: "relative", hours: 1 };

/** The organisation with its number, since two ASes can share a name. */
export function asnLabel(asn: number, name: string): string {
  if (asn === 0) return name || "Lainnya";
  return name ? `${name} (AS${asn})` : `AS${asn}`;
}

export function popLabel(
  id: string | null,
  labels: Map<string, string>,
): string {
  if (id === null) return "Lainnya (di luar blok POP)";
  return labels.get(id) ?? id;
}

/** One row per time with a column per drawn series, zero where it had none. */
export function flowChartRows(data: FlowDestinations | undefined): {
  rows: ChartRow[];
  series: string[];
} {
  if (!data) return { rows: [], series: [] };
  const drawn = data.series.filter((s) => s.points.length > 0);
  const names = drawn.map((s) => asnLabel(s.asn, s.name));
  const emptyRow = (time: string): ChartRow => {
    const row: ChartRow = { time };
    for (const name of names) row[name] = 0;
    return row;
  };
  const byTime = new Map<string, ChartRow>();
  drawn.forEach((s, i) => {
    for (const point of s.points) {
      const row = byTime.get(point.time) ?? emptyRow(point.time);
      row[names[i]] = point.bps;
      byTime.set(point.time, row);
    }
  });
  const rows = [...byTime.values()].sort(
    (a, b) => Date.parse(String(a.time)) - Date.parse(String(b.time)),
  );
  return { rows, series: names };
}

export function topIpsAvailable(
  range: TrafficRange,
  now: Date = new Date(),
): boolean {
  const { from } = resolveTrafficRange(range, now);
  return now.getTime() - Date.parse(from) <= TOP_IPS_RETENTION_MS;
}

export function flowStale(router: MikrotikRouter, nowMs: number): boolean {
  if (!router.flowEnabled) return false;
  if (router.flowLastSeenAt === null) return true;
  return nowMs - Date.parse(router.flowLastSeenAt) > FLOW_STALE_MS;
}
```

- [ ] **Step 6: Run the tests and the type check**

Run: `cd frontend && npm test -- --run src/presentation/components/routers/routerFlows.test.ts && npx tsc --noEmit`
Expected: PASS (5 tests); `tsc` reports no errors.

- [ ] **Step 7: Commit**

```bash
cd frontend && npx prettier --write src/domain/entities src/infrastructure src/application/hooks/useRouterFlows.ts src/presentation/components/routers/routerFlows.ts src/presentation/components/routers/routerFlows.test.ts
git add src
git commit -m "feat(frontend): flow types, repository, hooks and chart helpers

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 12: Frontend UI — Tujuan Trafik tab, flow status, NetFlow settings

**Files:**
- Modify: `frontend/src/presentation/components/routers/TrafficRangePicker.tsx` (optional `presets`)
- Modify: `frontend/src/presentation/components/routers/TrafficChart.tsx` (palette)
- Modify: `frontend/src/presentation/components/routers/routerFlows.ts` (+ `peakWindowLabel`) and `routerFlows.test.ts`
- Create: `frontend/src/presentation/components/routers/FlowDestinationsTab.tsx`
- Create: `frontend/src/presentation/components/routers/FlowFilters.tsx`
- Create: `frontend/src/presentation/components/routers/FlowSummaryTable.tsx`
- Create: `frontend/src/presentation/components/routers/FlowPopTable.tsx`
- Create: `frontend/src/presentation/components/routers/FlowTopIpsPanel.tsx`
- Create: `frontend/src/presentation/components/routers/RouterFlowStatus.tsx`
- Create: `frontend/src/test/mikrotikRouterFixture.ts`
- Modify: `frontend/src/presentation/components/routers/RouterInfoTab.tsx`
- Modify: `frontend/src/presentation/components/routers/RouterFormFields.tsx`
- Modify: `frontend/src/presentation/components/routers/RouterFormModal.tsx` (`initialValues`)
- Modify: `frontend/src/presentation/components/routers/useRouterTestSteps.ts` (`RouterFormValues`)
- Modify: `frontend/src/presentation/pages/RouterDetailPage.tsx`
- Test: `FlowDestinationsTab.test.tsx`, `RouterFlowStatus.test.tsx` (new), `RouterFormModal.test.tsx`

**Interfaces:**
- Consumes: hooks and helpers of Task 11; existing `useRouterInterfaces`, `useRouterBgpSessions`, `interfaceLabel`, `isUpstream`, `formatBps`, `formatBytes`, `TrafficChart`, `TrafficRangePicker`.
- Produces: `<FlowDestinationsTab routerId />`, `<RouterFlowStatus router />`, `TrafficRangePicker` prop `presets?: RangePreset[]` (`RangePreset { label: string; value: number }` in hours), `peakWindowLabel(seconds)`, test helper `mikrotikRouter(over?: Partial<MikrotikRouter>): MikrotikRouter`.

Ruling recorded here: the spec calls the presets 15m/1j/24j/7h/30h/90h/1t those of "the existing `TrafficRangePicker`", but that picker offers 6 jam/24 jam/7 hari/30 hari. The picker gains an optional `presets` prop; the new tab passes `FLOW_RANGE_PRESETS`, and the interface graphs keep theirs unchanged.

- [ ] **Step 1: Write the test fixture and the failing component tests**

`frontend/src/test/mikrotikRouterFixture.ts`:

```ts
import type { MikrotikRouter } from "@/domain/entities";

/** A complete router for component tests; override what the test is about. */
export function mikrotikRouter(over: Partial<MikrotikRouter> = {}): MikrotikRouter {
  return {
    id: "r1",
    name: "Border",
    host: "192.0.2.1",
    siteId: null,
    snmpPort: 161,
    apiUsername: "tikman",
    apiPort: 8443,
    apiProtocol: "rest",
    enabled: true,
    tlsFingerprint: "ab".repeat(32),
    presentedFingerprint: null,
    status: "up",
    apiStatus: "ok",
    apiError: null,
    cpuLoad: null,
    memoryTotal: null,
    memoryFree: null,
    uptimeSeconds: null,
    rosVersion: null,
    boardName: null,
    lastSeenAt: null,
    flowEnabled: false,
    flowSampling: 100,
    flowLastSeenAt: null,
    bgpEstablished: 0,
    bgpTotal: 0,
    ...over,
  };
}
```

`frontend/src/presentation/components/routers/FlowDestinationsTab.test.tsx`:

```tsx
import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import type { RouterInterface } from "@/domain/entities";
import { FlowDestinationsTab } from "./FlowDestinationsTab";

const destinations = vi.fn();
const topIps = vi.fn();

function iface(id: string, comment: string): RouterInterface {
  return {
    id,
    routerId: "r1",
    name: id,
    comment,
    type: "vlan",
    vlanId: null,
    parentName: null,
    ifIndex: null,
    speedMbps: null,
    disabled: false,
    operStatus: "up",
    addresses: "",
    lastInBps: null,
    lastOutBps: null,
    lastPolledAt: null,
  };
}

vi.mock("@/application/hooks/useMikrotikRouters", () => ({
  useRouterInterfaces: () => ({
    data: [iface("pop-a", "POP A"), iface("transit", "Transit")],
  }),
  useRouterBgpSessions: () => ({ data: [{ interfaceId: "transit" }] }),
}));

vi.mock("@/application/hooks/useRouterFlows", () => ({
  useFlowDestinations: (...args: unknown[]) => {
    destinations(...args);
    return { data: undefined };
  },
  useFlowSummary: () => ({ data: undefined, isLoading: false }),
  useFlowPops: () => ({
    data: [{ popInterfaceId: "pop-a", bytes: 1000, topAsns: [] }],
    isLoading: false,
  }),
  useFlowTopIps: (...args: unknown[]) => {
    topIps(...args);
    return { data: [], isLoading: false };
  },
}));

describe("FlowDestinationsTab", () => {
  beforeEach(() => {
    destinations.mockReset();
    topIps.mockReset();
  });

  it("filters by the POP whose row is clicked", async () => {
    render(<FlowDestinationsTab routerId="r1" />);

    fireEvent.click(screen.getByText("POP A"));

    await waitFor(() =>
      expect(destinations).toHaveBeenLastCalledWith(
        "r1",
        expect.anything(),
        expect.objectContaining({ pop: "pop-a", direction: "download" }),
      ),
    );
  });

  it("turns the top-IP panel off for a range past seven days", async () => {
    render(<FlowDestinationsTab routerId="r1" />);
    expect(screen.queryByText(/hanya disimpan 7 hari/)).toBeNull();

    fireEvent.click(screen.getByText("30 hari"));

    expect(await screen.findByText(/hanya disimpan 7 hari/)).toBeTruthy();
    expect(topIps).toHaveBeenLastCalledWith(
      "r1",
      expect.anything(),
      expect.anything(),
      false,
    );
  });
});
```

`frontend/src/presentation/components/routers/RouterFlowStatus.test.tsx`:

```tsx
import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { mikrotikRouter } from "@/test/mikrotikRouterFixture";
import { RouterFlowStatus } from "./RouterFlowStatus";

vi.mock("@/application/hooks/useRouterFlows", () => ({
  useFlowStatus: () => ({
    data: { asnDatasetUpdatedAt: "2026-10-12T00:00:00Z", asnCount: 3 },
  }),
}));

const STALE = /tidak ada flow masuk lebih dari 5 menit/;
const minutesAgo = (m: number) =>
  new Date(Date.now() - m * 60_000).toISOString();

describe("RouterFlowStatus", () => {
  it("warns when collection is on but nothing arrived for five minutes", () => {
    render(
      <RouterFlowStatus
        router={mikrotikRouter({
          flowEnabled: true,
          flowLastSeenAt: minutesAgo(10),
        })}
      />,
    );

    expect(screen.getByText(STALE)).toBeTruthy();
  });

  it("stays quiet while flows arrive, and while collection is off", () => {
    const { unmount } = render(
      <RouterFlowStatus
        router={mikrotikRouter({
          flowEnabled: true,
          flowLastSeenAt: minutesAgo(1),
        })}
      />,
    );
    expect(screen.queryByText(STALE)).toBeNull();
    unmount();

    render(<RouterFlowStatus router={mikrotikRouter({ flowEnabled: false })} />);

    expect(screen.queryByText(STALE)).toBeNull();
    expect(screen.getByText("Nonaktif")).toBeTruthy();
  });

  it("shows the sampling ratio and the ASN dataset in use", () => {
    render(
      <RouterFlowStatus
        router={mikrotikRouter({ flowEnabled: true, flowSampling: 512 })}
      />,
    );

    expect(screen.getByText("1:512")).toBeTruthy();
    expect(screen.getByText(/3 ASN/)).toBeTruthy();
  });
});
```

In `frontend/src/presentation/components/routers/RouterFormModal.test.tsx`:

- add `const updateMutate = vi.fn();` next to `createMutate`, change the mock entry to `useUpdateMikrotikRouter: () => ({ mutateAsync: updateMutate, isPending: false }),`, add `updateMutate.mockReset();` to `beforeEach`, and `import { mikrotikRouter } from "@/test/mikrotikRouterFixture";`;
- append inside the `describe` block:

```tsx
  it("lets an admin turn on NetFlow with its sampling ratio when editing", async () => {
    updateMutate.mockResolvedValue({});
    render(<RouterFormModal router={mikrotikRouter()} onClose={vi.fn()} />);

    fireEvent.click(screen.getByLabelText("Kumpulkan NetFlow"));
    fireEvent.change(screen.getByLabelText("Rasio sampling (1:N)"), {
      target: { value: "512" },
    });
    fireEvent.click(screen.getByText("Simpan"));

    await waitFor(() => expect(updateMutate).toHaveBeenCalled());
    expect(updateMutate.mock.calls[0][0].data).toMatchObject({
      flowEnabled: true,
      flowSampling: 512,
    });
  });

  it("does not offer NetFlow while adding a router", () => {
    render(<RouterFormModal onClose={vi.fn()} />);

    expect(screen.queryByLabelText("Kumpulkan NetFlow")).toBeNull();
  });
```

Append to `frontend/src/presentation/components/routers/routerFlows.test.ts` (add `peakWindowLabel` to its import):

```ts
describe("peakWindowLabel", () => {
  it("names the step the peak was measured over", () => {
    expect(peakWindowLabel(60)).toBe("1 menit");
    expect(peakWindowLabel(300)).toBe("5 menit");
    expect(peakWindowLabel(3_600)).toBe("1 jam");
    expect(peakWindowLabel(86_400)).toBe("1 hari");
  });
});
```

- [ ] **Step 2: Run them to verify they fail**

Run: `cd frontend && npm test -- --run src/presentation/components/routers/`
Expected: FAIL — `Failed to resolve import "./FlowDestinationsTab"` and `"./RouterFlowStatus"`; the modal's NetFlow test cannot find the label "Kumpulkan NetFlow"; `peakWindowLabel` is not exported.

- [ ] **Step 3: Extend the picker, the chart palette and the helpers**

Replace `frontend/src/presentation/components/routers/TrafficRangePicker.tsx` with:

```tsx
import { DatePicker, Radio, Space } from "antd";
import dayjs from "dayjs";
import type { TrafficRange } from "@/domain/entities";
import { resolveTrafficRange } from "@/application/hooks/trafficRange";

const { RangePicker } = DatePicker;

/** A preset range, in hours back from now. */
export interface RangePreset {
  label: string;
  value: number;
}

// Presets live outside the RangePicker because its own presets arrive through
// onChange exactly like a manual pick, and only a preset should follow the clock.
const DEFAULT_PRESETS: RangePreset[] = [
  { label: "6 jam", value: 6 },
  { label: "24 jam", value: 24 },
  { label: "7 hari", value: 7 * 24 },
  { label: "30 hari", value: 30 * 24 },
];

interface Props {
  value: TrafficRange;
  onChange: (range: TrafficRange) => void;
  presets?: RangePreset[];
}

export function TrafficRangePicker({
  value,
  onChange,
  presets = DEFAULT_PRESETS,
}: Props) {
  const shown = resolveTrafficRange(value);
  return (
    <Space wrap>
      <Radio.Group
        optionType="button"
        buttonStyle="solid"
        options={presets}
        value={value.kind === "relative" ? value.hours : null}
        onChange={(e) =>
          onChange({ kind: "relative", hours: Number(e.target.value) })
        }
      />
      <RangePicker
        showTime
        value={[dayjs(shown.from), dayjs(shown.to)]}
        onChange={(dates) => {
          if (dates?.[0] && dates[1]) {
            onChange({
              kind: "absolute",
              from: dates[0].toISOString(),
              to: dates[1].toISOString(),
            });
          }
        }}
      />
    </Space>
  );
}
```

In `frontend/src/presentation/components/routers/TrafficChart.tsx`, replace `COLORS` with eleven colours (ten destinations and Lainnya stay distinguishable):

```ts
const COLORS = [
  "#1677ff",
  "#52c41a",
  "#fa8c16",
  "#eb2f96",
  "#722ed1",
  "#13c2c2",
  "#2f54eb",
  "#a0d911",
  "#faad14",
  "#f5222d",
  "#8c8c8c",
];
```

Append to `frontend/src/presentation/components/routers/routerFlows.ts`:

```ts
const SECONDS_PER_MINUTE = 60;
const SECONDS_PER_HOUR = 3_600;
const SECONDS_PER_DAY = 86_400;

/** The step a summary's peak was measured over, in words. */
export function peakWindowLabel(seconds: number): string {
  if (seconds >= SECONDS_PER_DAY) return `${seconds / SECONDS_PER_DAY} hari`;
  if (seconds >= SECONDS_PER_HOUR) return `${seconds / SECONDS_PER_HOUR} jam`;
  return `${seconds / SECONDS_PER_MINUTE} menit`;
}
```

- [ ] **Step 4: Write the tab parts**

`frontend/src/presentation/components/routers/FlowFilters.tsx`:

```tsx
import { Radio, Select, Space } from "antd";
import type {
  FlowDirection,
  FlowFilter,
  RouterInterface,
} from "@/domain/entities";
import { OTHER_POP } from "./routerFlows";
import { interfaceLabel } from "./routerTraffic";

const DIRECTIONS = [
  { label: "Download", value: "download" },
  { label: "Upload", value: "upload" },
];

interface Props {
  filter: FlowFilter;
  onChange: (filter: FlowFilter) => void;
  pops: RouterInterface[];
  upstreams: RouterInterface[];
}

export function FlowFilters({ filter, onChange, pops, upstreams }: Props) {
  const popOptions = [
    ...pops.map((i) => ({ label: interfaceLabel(i), value: i.id })),
    { label: "Lainnya (di luar blok POP)", value: OTHER_POP },
  ];
  return (
    <Space wrap>
      <Radio.Group
        optionType="button"
        options={DIRECTIONS}
        value={filter.direction}
        onChange={(e) =>
          onChange({ ...filter, direction: e.target.value as FlowDirection })
        }
      />
      <Select
        allowClear
        placeholder="Semua POP"
        style={{ minWidth: 220 }}
        value={filter.pop}
        options={popOptions}
        onChange={(pop: string | undefined) => onChange({ ...filter, pop })}
      />
      <Select
        allowClear
        placeholder="Semua upstream"
        style={{ minWidth: 220 }}
        value={filter.upstream}
        options={upstreams.map((i) => ({ label: interfaceLabel(i), value: i.id }))}
        onChange={(upstream: string | undefined) =>
          onChange({ ...filter, upstream })
        }
      />
    </Space>
  );
}
```

`frontend/src/presentation/components/routers/FlowSummaryTable.tsx`:

```tsx
import { Card, Table, Tooltip } from "antd";
import type {
  FlowSummary,
  FlowSummaryRow,
  FlowUpstreamShare,
} from "@/domain/entities";
import { formatBytes } from "../trafficFormat";
import { asnLabel, peakWindowLabel } from "./routerFlows";
import { formatBps } from "./routerTraffic";

const SHARE_COLORS = ["#1677ff", "#52c41a", "#fa8c16", "#eb2f96", "#722ed1"];
const BAR_WIDTH = 160;
const BAR_HEIGHT = 10;

function UpstreamShareBar({
  shares,
  labels,
}: {
  shares: FlowUpstreamShare[];
  labels: Map<string, string>;
}) {
  const total = shares.reduce((sum, s) => sum + s.bytes, 0);
  if (total === 0) return null;
  return (
    <div style={{ display: "flex", width: BAR_WIDTH, height: BAR_HEIGHT }}>
      {shares.map((s, i) => (
        <Tooltip
          key={s.upstreamInterfaceId}
          title={`${labels.get(s.upstreamInterfaceId) ?? "?"}: ${formatBytes(s.bytes)}`}
        >
          <div
            style={{
              width: `${(s.bytes / total) * 100}%`,
              background: SHARE_COLORS[i % SHARE_COLORS.length],
            }}
          />
        </Tooltip>
      ))}
    </div>
  );
}

interface Props {
  data?: FlowSummary;
  labels: Map<string, string>;
  loading: boolean;
}

export function FlowSummaryTable({ data, labels, loading }: Props) {
  const peakTitle = data
    ? `Puncak per ${peakWindowLabel(data.peakWindowSeconds)}`
    : "Puncak";
  return (
    <Card size="small" title="Tujuan">
      <Table<FlowSummaryRow>
        size="small"
        rowKey="asn"
        loading={loading}
        pagination={false}
        dataSource={data?.rows ?? []}
        columns={[
          { title: "Organisasi", render: (_, r) => asnLabel(r.asn, r.name) },
          { title: "ASN", dataIndex: "asn", render: (asn: number) => asn || "—" },
          { title: "Total", dataIndex: "bytes", render: (b: number) => formatBytes(b) },
          { title: "%", dataIndex: "percent", render: (p: number) => `${p.toFixed(1)}%` },
          { title: "Rata-rata", dataIndex: "avgBps", render: (v: number) => formatBps(v) },
          { title: peakTitle, dataIndex: "peakBps", render: (v: number) => formatBps(v) },
          {
            title: "Per upstream",
            render: (_, r) => <UpstreamShareBar shares={r.upstreams} labels={labels} />,
          },
        ]}
      />
    </Card>
  );
}
```

`frontend/src/presentation/components/routers/FlowPopTable.tsx`:

```tsx
import { Card, Table } from "antd";
import type { FlowPopRow } from "@/domain/entities";
import { formatBytes } from "../trafficFormat";
import { OTHER_POP, asnLabel, popLabel } from "./routerFlows";

interface Props {
  rows: FlowPopRow[];
  labels: Map<string, string>;
  loading: boolean;
  onSelect: (pop: string) => void;
}

export function FlowPopTable({ rows, labels, loading, onSelect }: Props) {
  return (
    <Card size="small" title="Per POP (klik baris untuk memfilter)">
      <Table<FlowPopRow>
        size="small"
        loading={loading}
        pagination={false}
        dataSource={rows}
        rowKey={(r) => r.popInterfaceId ?? OTHER_POP}
        onRow={(r) => ({
          onClick: () => onSelect(r.popInterfaceId ?? OTHER_POP),
          style: { cursor: "pointer" },
        })}
        columns={[
          { title: "POP", render: (_, r) => popLabel(r.popInterfaceId, labels) },
          { title: "Total", dataIndex: "bytes", render: (b: number) => formatBytes(b) },
          {
            title: "3 tujuan teratas",
            render: (_, r) =>
              r.topAsns.map((a) => asnLabel(a.asn, a.name)).join(", ") || "—",
          },
        ]}
      />
    </Card>
  );
}
```

`frontend/src/presentation/components/routers/FlowTopIpsPanel.tsx`:

```tsx
import { Alert, Card, Table } from "antd";
import type { FlowTopIp } from "@/domain/entities";
import { formatBytes } from "../trafficFormat";
import { asnLabel } from "./routerFlows";

const PAGE_SIZE = 20;

interface Props {
  available: boolean;
  rows: FlowTopIp[];
  loading: boolean;
}

export function FlowTopIpsPanel({ available, rows, loading }: Props) {
  return (
    <Card size="small" title="IP tujuan teratas">
      {available ? (
        <Table<FlowTopIp>
          size="small"
          rowKey="ip"
          loading={loading}
          pagination={{ pageSize: PAGE_SIZE }}
          dataSource={rows}
          columns={[
            { title: "IP", dataIndex: "ip" },
            { title: "Organisasi", render: (_, r) => asnLabel(r.asn, r.name) },
            { title: "Total", dataIndex: "bytes", render: (b: number) => formatBytes(b) },
          ]}
        />
      ) : (
        <Alert
          type="info"
          showIcon
          message="IP tujuan hanya disimpan 7 hari. Pilih rentang di dalam 7 hari terakhir untuk melihatnya."
        />
      )}
    </Card>
  );
}
```

`frontend/src/presentation/components/routers/FlowDestinationsTab.tsx`:

```tsx
import { useMemo, useState } from "react";
import { Space } from "antd";
import type { FlowFilter } from "@/domain/entities";
import {
  useRouterBgpSessions,
  useRouterInterfaces,
} from "@/application/hooks/useMikrotikRouters";
import {
  useFlowDestinations,
  useFlowPops,
  useFlowSummary,
  useFlowTopIps,
} from "@/application/hooks/useRouterFlows";
import { FlowFilters } from "./FlowFilters";
import { FlowPopTable } from "./FlowPopTable";
import { FlowSummaryTable } from "./FlowSummaryTable";
import { FlowTopIpsPanel } from "./FlowTopIpsPanel";
import { TrafficChart } from "./TrafficChart";
import { TrafficRangePicker } from "./TrafficRangePicker";
import {
  DEFAULT_FLOW_RANGE,
  FLOW_RANGE_PRESETS,
  flowChartRows,
  topIpsAvailable,
} from "./routerFlows";
import { interfaceLabel, isUpstream } from "./routerTraffic";

export function FlowDestinationsTab({ routerId }: { routerId: string }) {
  const [range, setRange] = useState(DEFAULT_FLOW_RANGE);
  const [filter, setFilter] = useState<FlowFilter>({ direction: "download" });
  const { data: interfaces = [] } = useRouterInterfaces(routerId);
  const { data: sessions = [] } = useRouterBgpSessions(routerId);
  const labels = useMemo(
    () => new Map(interfaces.map((i) => [i.id, interfaceLabel(i)])),
    [interfaces],
  );
  const upstreams = interfaces.filter((i) => isUpstream(i, sessions));
  const pops = interfaces.filter((i) => !isUpstream(i, sessions));

  const destinations = useFlowDestinations(routerId, range, filter);
  const summary = useFlowSummary(routerId, range, filter);
  const popRows = useFlowPops(routerId, range, filter.direction);
  const ipsAvailable = topIpsAvailable(range);
  const topIps = useFlowTopIps(routerId, range, filter, ipsAvailable);
  const chart = flowChartRows(destinations.data);

  return (
    <Space direction="vertical" size="large" style={{ width: "100%" }}>
      <TrafficRangePicker
        value={range}
        onChange={setRange}
        presets={FLOW_RANGE_PRESETS}
      />
      <FlowFilters
        filter={filter}
        onChange={setFilter}
        pops={pops}
        upstreams={upstreams}
      />
      <TrafficChart
        stacked
        title="Tujuan trafik (bit/s)"
        series={chart.series}
        data={chart.rows}
      />
      <FlowSummaryTable
        data={summary.data}
        labels={labels}
        loading={summary.isLoading}
      />
      <FlowPopTable
        rows={popRows.data ?? []}
        labels={labels}
        loading={popRows.isLoading}
        onSelect={(pop) => setFilter({ ...filter, pop })}
      />
      <FlowTopIpsPanel
        available={ipsAvailable}
        rows={topIps.data ?? []}
        loading={topIps.isLoading}
      />
    </Space>
  );
}
```

- [ ] **Step 5: Write the Router-tab status and the form fields**

`frontend/src/presentation/components/routers/RouterFlowStatus.tsx`:

```tsx
import { Alert, Descriptions, Space } from "antd";
import dayjs from "dayjs";
import type { MikrotikRouter } from "@/domain/entities";
import { useFlowStatus } from "@/application/hooks/useRouterFlows";
import { flowStale } from "./routerFlows";

const DATE_FORMAT = "DD MMM YYYY HH:mm:ss";

export function RouterFlowStatus({ router }: { router: MikrotikRouter }) {
  const { data: status } = useFlowStatus(router.id);
  const dataset = status?.asnDatasetUpdatedAt
    ? `${dayjs(status.asnDatasetUpdatedAt).format(DATE_FORMAT)} · ${status.asnCount} ASN`
    : "Belum dimuat";
  return (
    <Space direction="vertical" style={{ width: "100%" }}>
      {flowStale(router, Date.now()) && (
        <Alert
          type="warning"
          showIcon
          message="NetFlow aktif tetapi tidak ada flow masuk lebih dari 5 menit"
          description="Periksa /ip traffic-flow dan target-nya di router, serta firewall UDP 2055 di VPS."
        />
      )}
      <Descriptions bordered size="small" column={1} title="NetFlow">
        <Descriptions.Item label="Status">
          {router.flowEnabled ? "Aktif" : "Nonaktif"}
        </Descriptions.Item>
        <Descriptions.Item label="Rasio sampling">
          {`1:${router.flowSampling}`}
        </Descriptions.Item>
        <Descriptions.Item label="Flow terakhir diterima">
          {router.flowLastSeenAt
            ? dayjs(router.flowLastSeenAt).format(DATE_FORMAT)
            : "—"}
        </Descriptions.Item>
        <Descriptions.Item label="Dataset ASN">{dataset}</Descriptions.Item>
      </Descriptions>
    </Space>
  );
}
```

In `frontend/src/presentation/components/routers/RouterInfoTab.tsx`, `import { RouterFlowStatus } from "./RouterFlowStatus";` and render `<RouterFlowStatus router={router} />` right after the closing `</Descriptions>`.

In `frontend/src/presentation/components/routers/useRouterTestSteps.ts`, add to `RouterFormValues`:

```ts
  flowEnabled?: boolean;
  flowSampling?: number;
```

In `frontend/src/presentation/components/routers/RouterFormModal.tsx` `initialValues`, add to the returned object:

```ts
    flowEnabled: router?.flowEnabled ?? false,
    flowSampling: router?.flowSampling ?? DEFAULT_FLOW_SAMPLING,
```

with `const DEFAULT_FLOW_SAMPLING = 100;` next to `DEFAULT_SNMP_PORT`.

In `frontend/src/presentation/components/routers/RouterFormFields.tsx`, add `Switch` to the antd import, add the component:

```tsx
// Only an existing router is offered NetFlow: the add flow is about reaching
// the router at all, and flow collection is switched on once it is in place.
function FlowFields() {
  return (
    <>
      <Form.Item
        name="flowEnabled"
        label="Kumpulkan NetFlow"
        valuePropName="checked"
      >
        <Switch />
      </Form.Item>
      <Form.Item
        name="flowSampling"
        label="Rasio sampling (1:N)"
        rules={[{ required: true, type: "number", min: 1, max: 10000 }]}
        extra="Harus sama dengan sampling di /ip traffic-flow router: N = (sampling-interval + sampling-space) / sampling-interval."
      >
        <InputNumber min={1} max={10000} />
      </Form.Item>
    </>
  );
}
```

and render `{editing && <FlowFields />}` after the `apiPort` `Form.Item` inside `RouterFormFields`.

In `frontend/src/presentation/pages/RouterDetailPage.tsx`, `import { FlowDestinationsTab } from "@/presentation/components/routers/FlowDestinationsTab";` and insert after the `upstream` tab item:

```tsx
          {
            key: "flows",
            label: "Tujuan Trafik",
            children: <FlowDestinationsTab routerId={router.id} />,
          },
```

- [ ] **Step 6: Run the component tests**

Run: `cd frontend && npm test -- --run src/presentation/components/routers/`
Expected: PASS — the new tab, status and modal tests, `routerFlows.test.ts`, and the existing router tests.

- [ ] **Step 7: Run the full frontend gates**

Run: `cd frontend && npx prettier --write src/presentation/components/routers src/presentation/pages/RouterDetailPage.tsx src/test/mikrotikRouterFixture.ts && npm test -- --run && npm run lint && npm run format:check && npm run build`
Expected: all tests pass, 0 lint errors (pre-existing warnings in `components/cs` may remain), format clean, build succeeds.

- [ ] **Step 8: Commit**

```bash
cd frontend && git add src
git commit -m "feat(frontend): Tujuan Trafik tab, flow status and NetFlow settings

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 13: Operator guide, CLAUDE.md and whole-branch verification

**Files:**
- Modify: `docs/operator_guide.md` (section "Memantau router MikroTik border", after "Menambah router")
- Modify: `CLAUDE.md` (entry points list; Deployment)

**Interfaces:**
- Consumes: everything above.
- Produces: operator documentation; a verified branch.

- [ ] **Step 1: Document NetFlow for operators**

Append to the end of the "Memantau router MikroTik border" section of `docs/operator_guide.md` (after the "Menambah router" subsection):

````markdown
### Tujuan trafik (NetFlow)

Tab **Tujuan Trafik** menunjukkan ke mana trafik pelanggan pergi —
organisasi/AS tujuan, dari POP mana, lewat upstream mana, ke arah mana — dari
15 menit terakhir sampai setahun, atau rentang bebas. Datanya IPFIX yang
dikirim router border ke service `flowd` di VPS (UDP 2055).

Setiap POP me-NAT pelanggannya sendiri, jadi di border yang terlihat adalah
IP publik POP, bukan IP pelanggan: data ini per POP, tidak per pelanggan.

Di router, mulai dengan sampling 1:100 (ganti `IP_VPS` dan `IP_ROUTER`):

```
/ip traffic-flow set enabled=yes interfaces=all active-flow-timeout=1m inactive-flow-timeout=15s packet-sampling=yes sampling-interval=1 sampling-space=99
/ip traffic-flow target add dst-address=IP_VPS port=2055 version=ipfix src-address=IP_ROUTER
```

- `src-address` harus sama dengan IP router di TikMan. `flowd` mengenali
  router dari alamat pengirim dan membuang paket dari alamat lain; karena itu
  router harus didaftarkan dengan IP, bukan hostname.
- `active-flow-timeout=1m`: tanpa ini RouterOS mengirim flow yang panjang
  setelah 30 menit sekaligus, dan grafik menampilkannya sebagai lonjakan.
- Rasio sampling = (sampling-interval + sampling-space) / sampling-interval;
  contoh di atas 1:100. Pantau CPU router setelah mengaktifkannya, dan naikkan
  rasionya bila CPU tinggi.

Di TikMan (Admin): buka router → **Ubah** → nyalakan **Kumpulkan NetFlow** dan
isi **Rasio sampling (1:N)** dengan N yang sama persis dengan router. Rasio
yang berbeda membuat setiap angka meleset sebesar selisihnya.

Di VPS: buka `2055/udp` di firewall provider, dibatasi ke IP router bila
memungkinkan.

Memastikan data masuk: tab **Router** → bagian **NetFlow** menampilkan
**Flow terakhir diterima** (diperbarui tiap menit) dan **Dataset ASN** yang
dipakai. Bila NetFlow aktif tetapi lebih dari 5 menit tidak ada flow, muncul
peringatan. Cek kewajaran: total download di Tujuan Trafik selama satu jam
kira-kira sama dengan grafik download upstream di tab **Upstream & BGP**;
selisih besar hampir selalu berarti rasio sampling di TikMan tidak sama dengan
di router.

Yang perlu diketahui saat membaca datanya:

- POP dikenali dari alamat di interface POP dan dari rute statis ke router POP,
  keduanya diambil saat discovery (tiap 6 jam, atau segera lewat **Refresh
  discovery**). Blok baru dikenali paling lama 5 menit setelah discovery;
  sebelum itu trafiknya masuk **Lainnya (di luar blok POP)**.
- Upstream adalah interface yang membawa sesi BGP.
- **Lainnya** di daftar tujuan berarti AS di luar 20 teratas per POP per
  menit, atau alamat yang tidak ada di dataset ASN.
- IP tujuan hanya disimpan 7 hari. Ringkasan per AS disimpan per menit selama
  7 hari, per 5 menit 90 hari, per jam 2 tahun, dan per hari selamanya.
- Dataset ASN dari iptoasn.com diunduh ulang tiap 24 jam; bila gagal, dataset
  sebelumnya tetap dipakai.
````

- [ ] **Step 2: Update CLAUDE.md**

In `CLAUDE.md`, add to the backend entry points list after the `cmd/snmpbench/main.go` line:

```markdown
- `cmd/flowd/main.go` — IPFIX collector for the border routers' traffic
  destinations (per POP, upstream and destination AS). Its own container on
  UDP 2055, not in `api`'s namespace; built from `Dockerfile.worker`; keeps the
  iptoasn dataset in the `asn_data` volume
```

In the Deployment section, after the paragraph ending "recreating `api` under them fails.", add:

```markdown
`flowd` comes along with `up -d --build`. It does not share `api`'s namespace,
so it can be recreated alone with `up -d --no-deps flowd`; its `2055/udp` must
be open in the VPS provider's firewall, ideally only to the border routers.
```

and extend the "Check after" route list sentence to:

```markdown
Routes should answer **401** — present and auth-gated — not 404. Check
`/api/v1/mapping/nodes`, `/api/v1/wireguard/server`, `/api/v1/cs/conversations`
and `/api/v1/routers/x/flows/status` at least.
```

- [ ] **Step 3: Public-repo check**

Run: `git diff main -- . | grep -oE '\b([0-9]{1,3}\.){3}[0-9]{1,3}\b' | grep -vE '^(192\.0\.2|198\.51\.100|203\.0\.113|127\.0\.0)\.|^0\.0\.0\.0$' | sort -u`
Expected: no output — every IPv4 address in the change is documentation, loopback, or `0.0.0.0` as the default route. Then: `git diff main -- . | grep -oE '\bAS ?[0-9]{4,10}\b|\b6[45][0-9]{3}\b' | sort -u` lists only 64496–64511 and 65536–65551 (port numbers such as 8443 do not match). Read the diff for provider, POP or router names that are not obviously fictional; the controller knows the real ones and checks them outside the repository.

- [ ] **Step 4: Full verification**

```bash
cd backend && gofmt -s -l . && go vet ./... && go mod verify && \
  go test $(go list ./... | grep -v /internal/api) -race -count=1 && \
  go test ./internal/api/ -run 'Router|Flow' -race -count=1 && \
  TEST_POSTGRES_DSN="postgres://tikman:testpw@localhost:5439/tikman_test?sslmode=disable" \
  go test ./internal/database/ ./internal/services/ -race -count=1
go build -o /dev/null ./cmd/api && go build -o /dev/null ./cmd/worker && go build -o /dev/null ./cmd/flowd
cd ../frontend && npm test -- --run && npm run lint && npm run format:check && npm run build
cd .. && graphify update .
```

Expected: `gofmt` prints nothing; every Go package `ok`; the three binaries build; frontend all green. (Two Postgres tests are known to flake only under concurrent load, and a stale `tikman_test` fails metrics tests deterministically — recreate the database as in Task 5 before calling anything a regression.)

- [ ] **Step 5: Commit**

```bash
git add docs/operator_guide.md CLAUDE.md
git commit -m "docs: NetFlow traffic destinations — router setup, flowd deploy

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

- [ ] **Step 6: Post-deploy check (operator, not automated)**

After deploy, on the real border router:

1. **Refresh discovery**, then confirm `router_routed_prefixes` holds the POP routers' blocks and no BGP prefixes (a few dozen rows at most). This proves RouterOS honours the `static=true`/`active=true` filter over the protocol in use (REST query parameters or API `?` words); if discovery fails or returns thousands of rows, the filter is not applied and Task 6 must change before NetFlow is enabled.
2. Enable NetFlow at 1:100. Within two minutes **Flow terakhir diterima** updates; if it does not and `flowd` logs `unknown_source_packets`, the router's `src-address` differs from its host in TikMan.
3. After an hour, compare the Tujuan Trafik download total with the upstream download graph. They should agree within a few percent; a constant factor means the sampling semantics (`sampling-interval`/`sampling-space`) differ from what the guide assumes, and the guide's formula must be corrected.
