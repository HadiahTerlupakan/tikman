# CLAUDE.md Compliance — Design

**Date:** 2026-08-23
**Goal:** bring the codebase into consistent compliance with `CLAUDE.md`,
and correct `CLAUDE.md` where it contradicts itself or the code.

## Problem

An audit of the repo against `CLAUDE.md` found every automated CI gate green
(`gofmt -s`, `go vet`, `go build`, `golangci-lint`, `go test -race`, frontend
lint/format/test/build) while eleven of its written rules were violated. The
gates do not encode most of the document.

One violation is a live defect, verified against the running database:

```
$ docker exec tikman-postgres psql -U tikman -d tikman -c 'SELECT count(*) FROM audit_logs;'
ERROR:  relation "audit_logs" does not exist
```

`models.AuditLog` declares `TableName() → "audit_logs"` and 11 call sites across
5 handlers write to it, but the table was in neither `AutoMigrate()` nor
`backend/migrations/`. The existing test passed because it created the table
itself, so it exercised a path that never ran in production.

## Findings

Grouped by cost and risk, because they are not one project.

| Group | Contents | Risk | Verifiable |
|---|---|---|---|
| A. Behaviour bugs | `audit_logs` missing; 7 auth/session tests skip everywhere | Low | Yes |
| B. Deletions | `internal/worker/` (1005 lines), 8 `console.log`, `cmd/test_metrics/`, banner comments, 7 report `.md` files | None | Yes |
| C. Doc accuracy | 4 stale claims in `CLAUDE.md` | None | Yes |
| D. Structural | 10 files >350 lines, 37 functions >50 lines, DB access in handlers, raw `axios` in frontend | High | Partly |
| E. Coverage | 38.4% against a documented >80% target | Medium | Yes |

### A — Behaviour

`audit_logs` is fixed (see Phase 1). The second item: `internal/auth` sits at
0.0% coverage not for lack of tests but because `session_test.go:18` and
`middleware/auth_test.go:22` hardcode `localhost:6379` and `t.Skip` when it is
unreachable. `docker port tikman-redis` is empty and `ci.yml`'s `backend-test`
job has no `services:` block, so those 7 tests skip **locally and in CI alike**.
Session management and auth middleware are untested everywhere.

### D — Structural, and why SNMP files are excluded

Oversized functions split cleanly by whether they touch the network:

- **25 are pure logic** — handlers, services, `router.go:Setup`, `collectMetrics`
  (429-line `cmd/worker/main.go` has zero `gosnmp`/`net.Dial` references; it
  reaches SNMP only through `connectivity` calls, which are injectable at the
  call boundary).
- **12 are network-bound** — they construct `&gosnmp.GoSNMP{}` or dial directly.
  `snmp_client.go:31` builds its client inside the function body: no interface,
  no injection point. `zteWalkMetrics` (286 lines, 0% coverage) is the largest.

Testing the second group requires refactoring production code that serves live
OLTs, standing up a fake SNMP responder, or real hardware. Splitting those
functions would therefore be verified by `go build` and manual review only.

**Decision: exempt network-bound SNMP/Telnet code from the 350-line and 50-line
limits, with the reason recorded in `CLAUDE.md`.** Restructuring untestable code
to satisfy a line count is risk without payoff, and `CLAUDE.md` already forbids
speculative abstraction. Excluding `internal/worker/` (deleted in Phase 2) and
`cmd/test_metrics/` (deleted in Phase 2), the in-scope structural work is
**10 oversized files and 37 oversized functions**.

### E — Why a single 80% target is the wrong shape

Per-file coverage shows the uncovered code is three kinds with very different
value:

1. **Pure logic at 0%** — `snmp_encoding.go` (ifIndex encode/decode, ZTE power
   and timestamp decoding, 6 functions) and `snmp_discovery_structure.go`
   (`buildTopologyStructure`, map in → struct out). Deterministic, no network.
   This is where ZTE decoding bugs would hide, and it is the highest-value
   testing available.
2. **Network-bound** — `snmp_client.go`, `snmp_walks.go`, `snmp_metrics_walk.go`,
   `telnet.go`. Expensive to test, brittle once tested.
3. **Entry points** — `cmd/*/main.go`, `config.go`. Wiring, not logic.

Chasing 80% globally pushes toward either testing group 2 or padding with
trivial tests — which `CLAUDE.md` explicitly forbids ("Do not pad coverage with
trivial getter/setter tests to hit the 80% number"). The document contradicts
itself here. Phase 6 proposes per-layer targets in its place; the number is the
user's decision, not an assumption made here.

## Approach

Test before refactor. `collectMetrics` (293 lines) sits at 14.8% coverage; if it
is split first, nothing proves the split preserved behaviour. Raise coverage on
what is cheaply testable, then restructure behind that net.

Rejected: refactor-first (reaches the line limits sooner, but every split is an
unverified change to code monitoring live OLTs).

## Phases

Each phase stands alone and is independently revertable. One commit per unit of
work; full gates green between commits.

**Phase 1 — Behaviour bugs.** `&AuditLog{}` into `AutoMigrate()`; audit test
setup switched from `db.AutoMigrate(&models.AuditLog{}, ...)` to
`models.AutoMigrate(db)` so it can fail when registration is missing. Redis
`services:` block (`redis:7-alpine`, matching `docker-compose.yml`) added to
`ci.yml`'s `backend-test` job, unskipping the 7 auth/session tests. `t.Skip` is
kept so `go test ./...` still runs without Redis locally. These 7 tests have
never executed; if any are genuinely red, that is reported rather than fixed
quietly.

**Phase 2 — Deletions.** `internal/worker/` (371 production + 634 test lines):
nothing imports it, `Dockerfile.worker:12` builds `./cmd/worker/main.go`, and
`cmd/worker` is a functional superset (it has `PruneMissingFromDiscovery` and
traffic-rate collection that `internal/worker` lacks). Its 19 tests cover code
nobody runs. Note: total coverage will *drop* when its 43.5% leaves the report —
a metric artifact, not a regression. Also: 8 `console.log`. Two `useEffect`
hooks in `useOntListLogic.ts` are logging-only (lines 103-107 and 147-168) and go
entirely; the hook at 110-118 keeps its `refetch()` and loses only its
`console.log`. Then `cmd/test_metrics/` (hardcoded production
OLT IP, `public` community, `fmt.Println("✅ ... verified!")`), the `// =====`
banners in `snmp_constants.go` (the OID content stays — it is exactly the "why"
commenting the document asks for), and 7 report/summary `.md` files.
`cmd/probe_hsgq/` and `cmd/seed-events/` keep their `fmt.Println`: that is
intended CLI output.

Deleting `cmd/test_metrics/` does not remove the OLT credentials from git
history; that needs a separate step, and the `public` community on a reachable
OLT warrants rotation regardless. Flagged for the user, not actioned here.

**Phase 3 — `CLAUDE.md` accuracy.** Four stale claims: AutoMigrate now covers 6
models, not 3; `DATABASE_URL`/`REDIS_URL` do not exist (the code reads
`DB_HOST`/`DB_PORT`/`DB_USER`/`DB_PASSWORD`/`DB_NAME`, `REDIS_HOST`/`REDIS_PORT`);
`golangci-lint` is a local gate, absent from `ci.yml`; `npm test` is watch mode,
CI runs `npm test -- --run --coverage`. Plus the SNMP exemption from group D.

**Phase 4 — Tests for pure logic.** `snmp_encoding.go`,
`snmp_discovery_structure.go`, and reachable handler paths. Strict TDD: every
test run red against existing code first, with the production change that would
break it named before it is written. Table-driven with hand-derived literals —
no expectations computed by the code under test. This is the safety net Phase 5
depends on.

**Phase 5 — Structural refactor.** Split the 10 oversized files and 37
oversized functions; move DB access out of `api/` into services; replace the raw
`axios` call at `useOntListLogic.ts:183` with the repository, which also removes
the `port_id ?? portId` dual-key fallbacks it forced. One unit per commit, tests
green between each.

Note: moving the queries in `dto.go:87,171` into a service changes
`ToOLTResponse(db, olt)` and `ToSiteResponse(db, site)` to drop their `db`
parameter, touching every caller. Mechanical, but it spreads.

**Phase 6 — Reconcile the targets.** Report final coverage, propose per-layer
targets with the SNMP rationale, and let the user set the numbers. No target is
lowered unilaterally: moving the goalposts to match reality is not compliance.

## Verification

Per commit — backend: `gofmt -s -l .`, `go vet ./...`, `go build ./...`,
`go test ./... -race`. Frontend: `npm run lint`, `npm run format:check`,
`npm test -- --run`, `npm run build`.

Phase 1's Redis change cannot be proven green without a push; it is verified
locally against Redis on a temporary published port, and the CI result reported
honestly once pushed.

All work on a branch. Pushing to `main` triggers `docker-build`, which publishes
images to GHCR — an outward-facing side effect requiring explicit approval.

## Out of scope

- Rewriting git history to purge the OLT credentials (flagged above).
- Rotating the `public` SNMP community.
- Refactoring network-bound SNMP/Telnet functions (rationale above).
- Changing any documented target without the user setting the number.
