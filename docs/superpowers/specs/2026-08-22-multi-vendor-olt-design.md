# Multi-Vendor OLT Support

**Date:** 2026-08-22
**Status:** Proposed

## Problem

TikMan speaks SNMP to exactly one vendor. The ZTE C300/C320 assumptions are
not confined to a table of OID strings — they are spread across seven files in
`internal/connectivity/` and cover four distinct concerns:

1. **OID strings** — 30 prefix constants across two enterprise trees
   (`.1.3.6.1.4.1.3902.1082` and `.3902.1012`), plus 6 index-arithmetic
   constants
2. **ifIndex arithmetic** — `encodeZxGponIfIndex` packs `0xFFSSPP00`;
   `decodeOnuTypeIfIndex` uses a base plus slot/port strides
3. **Value scaling** — `decodeZxGponPower` applies `(raw * 0.002) - 30` and
   treats `65535` plus the range `30000..32767` as no-signal sentinels
4. **Timestamp format** — `parseZteHexTimestamp` decodes a ZTE-specific hex
   layout

An OLT row has no model field, so there is nothing to dispatch on. Adding a
second vendor today means editing the walk logic itself.

Affected files: `snmp_constants.go`, `snmp_encoding.go`, `snmp_parsing.go`,
`snmp_client.go`, `snmp_walks.go`, `snmp_walk_uncfg.go`,
`snmp_discovery_topology.go`, `snmp_traffic_rates.go`.

## Goals

- Support ZTE C300, ZTE C320, and HSGQ as selectable OLT models
- Adding a further vendor requires one new file and one registry entry, with no
  change to walk logic or call sites
- Preserve current ZTE behaviour exactly; the existing test suite is the
  regression guard
- Never present unverified vendor data as if it were confirmed

## Non-Goals

- Vendor-specific *provisioning* (CLI/telnet command differences). This spec
  covers read paths — monitoring and discovery — only.
- Auto-detecting the model via `sysObjectID`. Operators select it explicitly.
  Detection can build on this abstraction later.
- Migrating ZTE OID values. They move behind an interface unchanged.

## Vendor Research: HSGQ

OIDs were taken from `github.com/fajriyandi/Go-SNMP-HSGQ`. That repository ships
no Go source, only a compiled binary, so the OID strings were extracted from the
binary's string table and cross-checked against the suffixes documented in its
README. Two independent sources agreeing raises confidence, but neither is the
vendor's own MIB.

**Confirmed — enterprise `50224`, base `.1.3.6.1.4.1.50224`:**

| Metric | OID | Scaling |
|---|---|---|
| ONU name | `.3.12.2.1.2.{ifIndex}` | — |
| Status | `.3.12.2.1.3.{ifIndex}` | binary 0/1 |
| Vendor | `.3.12.2.1.8.{ifIndex}` | — |
| Model | `.3.12.2.1.9.{ifIndex}` | — |
| Firmware | `.3.12.2.1.13.{ifIndex}` | — |
| Serial | `.3.12.2.1.15.{ifIndex}` | hex, 4B vendor + 4B UID |
| Distance | `.3.12.2.1.19.{ifIndex}` | metres |
| Last up time | `.3.12.2.1.20.{ifIndex}` | datetime |
| Uptime | `.3.12.2.1.21.{ifIndex}` | timeticks |
| ONU rx power | `.3.12.3.1.4.{ifIndex}.0.0` | ÷100 → dBm |
| OLT rx power | `.3.12.3.1.4.{ifIndex}.65535.65535` | ÷100 → dBm |
| Tx power | `.3.12.3.1.5.{ifIndex}.0.0` | ÷100 → dBm |
| Bias current | `.3.12.3.1.6.{ifIndex}.0.0` | ÷100 → mA |
| Voltage | `.3.12.3.1.7.{ifIndex}.0.0` | ÷100 → V |

Device-level values live under `.3.1.1.x`; they are out of scope here.

**Unconfirmed, and deliberately not guessed:**

| Unknown | Consequence while unresolved |
|---|---|
| ifIndex → PON/ONU derivation | ONUs are labelled by raw ifIndex, not "PON 1 / ONU 3" |
| Which of 0/1 means online | Status maps to `unknown` rather than a guess |
| Serial byte order | Rendered via the existing autofind decoder; may need reversing |
| No-signal sentinel | Out-of-range readings surface as `nil`, not a fabricated dBm |

Guessing any of these produces output that looks correct and is wrong. Status
polarity is the worst case: inverted, the dashboard reports a healthy network
during an outage.

## Design

### Two structural differences drive the interface

HSGQ's optical OIDs carry a trailing `.0.0` (and `.65535.65535` for the OLT
side). ZTE has no equivalent, so a driver must **build** a full OID rather than
supply a prefix that shared code concatenates.

More importantly, the two vendors discover ONUs in opposite directions. ZTE
*computes* ifIndex from slot and port. HSGQ *walks* a table and derives PON and
ONU from the ifIndex it finds. An interface built around `EncodeIfIndex(slot,
port)` would encode the ZTE direction into the contract. Enumeration is
therefore the primitive; computed indices are a ZTE implementation detail.

### Driver interface

```go
// Driver adapts one OLT model's SNMP dialect. Implementations are read-only
// and hold no connection state.
type Driver interface {
    Model() models.OLTModel

    // Enumerate lists the ONUs the OLT currently knows about. ZTE computes
    // ifIndexes from its slot/port arithmetic; HSGQ walks a table.
    Enumerate(client *gosnmp.GoSNMP) ([]ONURef, error)

    // MetricOID returns the fully-formed OID for one metric on one ONU,
    // including any vendor-specific suffix.
    MetricOID(metric Metric, ref ONURef) (string, bool)

    DecodePower(raw int64) *float64          // nil = no signal
    DecodeStatus(raw any) models.ONTStatus   // UNKNOWN when unmappable
    DecodeSerial(raw any) string
    DecodeTimestamp(raw any) (time.Time, error)
}

// ONURef identifies one ONU. Slot and Port are zero when the driver cannot
// derive them, in which case IfIndex is the only stable identifier.
type ONURef struct {
    IfIndex uint32
    Slot    int
    Port    int
    ONUID   int
}
```

`Metric` is a closed enum (`MetricSerial`, `MetricRxPower`, …). `MetricOID`
returns `false` for metrics a vendor does not expose, so callers skip rather
than query a fabricated OID.

### Registry

```go
func Register(d Driver)                       // called from each driver's init
func For(model models.OLTModel) (Driver, error)
```

`For` returns an error for an unregistered model. It must never fall back to
ZTE: a silent default is how one vendor's decoding gets applied to another's
raw values, producing readings that are plausible and wrong.

### Threading the driver through

Thirteen exported functions share the shape
`(ipAddress, community string, snmpPort int, ...)` and are vendor-specific:

`DiscoverOLTTopology`, `DiscoverONTs`, `PollOntStatus`,
`QueryONTMetricsWithDynamicPort`, `QueryONUTrafficRates`,
`QuerySingleONTMetrics`, `WalkONTHardwareVersions`, `WalkONTIPAddresses`,
`WalkONTMACAddresses`, `WalkONTMetrics`, `WalkONTStatuses`,
`WalkONUTrafficRates`, `WalkUnconfiguredONUs`.

`PingTest`, `SSHTest`, `TelnetTest`, and `SNMPTest` are protocol-level and stay
vendor-agnostic.

Each of the thirteen gains a leading `driver Driver` parameter. Services resolve
the driver from `olt.Model` before calling. This is a compile-time break at every
call site, which is the point: the compiler enumerates the work rather than
leaving a silent ZTE default behind.

Only the six metrics HSGQ actually exposes need driver coverage in phase 1. The
remaining entry points (MAC address, hardware version, IP address, traffic
rates) return "unsupported" for HSGQ via `MetricOID`'s `false` result until the
vendor's equivalents are known.

### Data model

```go
type OLTModel string

const (
    OLTModelZTEC300 OLTModel = "zte_c300"
    OLTModelZTEC320 OLTModel = "zte_c320"
    OLTModelHSGQ    OLTModel = "hsgq"
)
```

Column: `model varchar(30) not null default 'zte_c300'`. Existing rows adopt
`zte_c300`, matching what the code already assumed. GORM `AutoMigrate` applies
this; the non-null default means no backfill step.

C300 and C320 are separate models even though their OIDs are currently
identical, so a future firmware divergence needs no migration.

### API and frontend

`CreateOLTRequest` gains `model` with
`binding:"required,oneof=zte_c300 zte_c320 hsgq"`; `UpdateOLTRequest` gains an
optional pointer with the same `oneof`. `OLTResponse` exposes `model`.

Frontend: a required `Select` in `OltModal.tsx` and a column in `OltTable.tsx`,
both fed from one exported list of `{value, label}` so adding a vendor touches a
single place. HSGQ's label carries an "OID belum diverifikasi" hint, since the
operator deserves to know which readings rest on a third-party reference.

### Error handling

- Unknown model in the database → explicit error naming the model; no fallback
- Metric unsupported by a vendor → omitted from results, not zero-filled
- Undecodable status → `ONTStatusUnknown`; the dashboard already renders this
- Out-of-range optical reading → `nil`, which existing code renders as "no
  signal"

## Testing

The in-process SNMP agent from `snmp_uncfg_agent_test.go` generalises: it serves
a controlled table over UDP through gosnmp's own encode/decode path, so driver
tests exercise real GetNext traffic rather than a stub.

Per driver: `Enumerate` against a served table; `MetricOID` including HSGQ's
`.0.0` and `.65535.65535` suffixes; `DecodePower` across normal values, both ZTE
sentinel ranges, and HSGQ's ÷100; `DecodeStatus` for unmapped input →
`UNKNOWN`.

Registry: every registered model resolves; an unknown model errors rather than
returning a driver.

Regression: the existing ZTE tests must pass unmodified. Any change needed to
those tests means behaviour shifted, and the refactor is wrong.

The HSGQ driver's unverified decoders get tests pinning the *defensive* result —
that unmappable status yields `UNKNOWN` rather than a coin-flip guess. Those
tests change in phase 2 when real values are known.

## Phasing

**Phase 1 (this spec).** Driver interface, registry, ZTE driver extracted
behind it, HSGQ driver with the confirmed OIDs, model column, API validation,
frontend select.

HSGQ supports these in phase 1: serial, name, status, rx power, tx power,
distance, firmware, vendor, model. Not supported (no known OID): MAC address,
hardware version, management IP, traffic rate counters, last-offline reason.

ONUs are labelled by ifIndex rather than PON/ONU; status resolves to `unknown`
until polarity is confirmed.

**Phase 2 (separate task).** A probe run against the operator's HSGQ resolves
the four unknowns; PON/ONU labelling and status mapping become exact. This is
refinement of a working module, not activation of a dormant one.

## Risks

The HSGQ OIDs come from a third-party implementation, not HSGQ's MIB. If the
operator's firmware differs, some OIDs may return nothing — and that will only
surface against real hardware. The defensive decoders ensure the failure mode is
visibly missing data rather than confidently wrong numbers.

Threading a parameter through thirteen entry points touches roughly a dozen
files across `connectivity/`, `services/`, and `worker/`. The risk is altering
ZTE behaviour mid-refactor; the unmodified ZTE test suite is what holds that
line.

This is large enough that it is worth asking whether it should land as one
change or as two — the interface plus ZTE extraction first, then HSGQ on top of
a green build. The plan should split it that way.

`BaseOID3` (`.3902.1015`) is declared in `snmp_constants.go` and referenced
nowhere. It should be deleted as part of moving the constants, not carried into
the ZTE driver.
