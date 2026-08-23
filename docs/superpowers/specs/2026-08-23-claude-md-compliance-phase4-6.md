# CLAUDE.md Compliance — Phases 4-6 Design

**Date:** 2026-08-23
**Goal:** Bring the codebase into full compliance with `CLAUDE.md`, including structural refactor and test coverage targets.

## Problem Summary

Phases 1-3 (bug fixes, deletions, doc corrections) are complete and pushed. Four remaining issues violate CLAUDE.md's written rules:

| Issue | Count | Status | Risk |
|---|---|---|---|
| Files >350 lines | 10 files | Needs split | High |
| Functions >50 lines | 37 functions | Needs split | High |
| DB access in handlers | ~10 calls | Needs refactor | Medium |
| Raw axios in frontend | 1 call | Needs repo | Medium |
| Coverage <80% | 38.6% vs 80% target | Needs tests | N/A |

Plus one exemption already documented: **network-bound SNMP/Telnet functions** cannot be tested without fake hardware or SNR spoofing, so they're exempt from line limits and testing requirements.

## Phase Breakdown

### Phase 4: Test for Pure Logic (Jaring Pengaman)

Before any refactor, we need test coverage that can catch mistakes. This phase adds TDD tests for:

1. **`snmp_encoding.go`**: All encode/decode functions except the ones already at 100%.
   - `decodeOnuIDIfIndex` (lines 24-35)
   - `decodeOnuTypeIfIndex` (lines 36-59)
   - `encodeOnuIDIfIndex` (lines 60-66)
   - `parseZteHexTimestamp` (lines 67-90)
   - `decodeZxGponPower` has 90% but not perfect; verify edge cases.

2. **`snmp_discovery_structure.go`**: `buildTopologyStructure` function (lines 10-320+).
   - Input: slotMap → statuses → structure mapping
   - Output: TopologySlotResponse with validated ONT counts

3. **Handler paths with DB access**: Isolated handler tests that don't touch real DB.
   - `internal/api/dto.go` query logic extracted into service layer first (Phase 5), then tested there.

**TDD pattern:** Write failing test → minimal implementation → pass. No mocks for pure logic; use hand-derived literals. Table-driven tests with verified outputs.

**Why this order:** `collectMetrics` is 293 lines with no test coverage. Splitting it without a safety net means we could break the worker serving live OLTs. Tests first = jaring pengaman.

### Phase 5: Structural Refactor

After Phase 4 passes, Phase 5 splits large files/functions and removes DB access from handlers.

**Files to split (10 total):**
- `internal/services/metrics_service.go` (525 lines) → split by operation (GetRealtime, GetHistory, GetLatest, StoreMetrics)
- `internal/connectivity/driver_hsgq.go` (578 lines) → separate HSGQ parsing logic from main driver interface
- `internal/api/dto.go` (373 lines) → extract ToSiteResponse and ToOLTResponse into `internal/services/site_service.go` and `olt_service.go`
- `internal/connectivity/snmp_metrics_walk.go` (337 lines) → separate walking logic from aggregation
- `internal/connectivity/snmp_discovery_structure.go` (317 lines) → already pure, will have tests from Phase 4; keep as one file if needed
- Plus 5 more files that aggregate multiple concerns

**Functions to split (25 pure + 12 network-bound excluded):**
- `cmd/worker/main.go:76` → `collectMetrics` (293 lines)
- `internal/api/router.go:7145` → `Setup` (115 lines) — already well-structured, consider keeping intact
- Handler methods: List, Create, Update, Delete across all handlers

**DB access removal:**
- `internal/api/dto.go:87,171` — queries in ToSiteResponse/ToOLTResponse → move to `site_service.go` and `olt_service.go`
- `internal/api/seed_handler.go:138,160` — DB.Create calls → seed service or event service owns this
- `internal/api/ont_handler.go:119` — `.Where().Select().Find()` → ont_service owns ONT queries

**Frontend axios fix:**
- `frontend/src/application/hooks/useOntListLogic.ts:183` — raw axios.get() → use OntRepository or create one specifically for topology fetches

**Constraint:** One unit per commit. Every commit must have green gates. If a split reveals a bug, stop and document it — don't paper over with more changes.

### Phase 6: Reconcile Targets

After Phase 5 completes, report the actual numbers:

1. New total coverage % (expect some increase from Phase 4)
2. Remaining files >350 lines (expect reduction from 10 to maybe 5-6 after strategic splits only)
3. Remaining functions >50 lines (expect reduction from 37 to 20-25, minus network-bound exemptions)

Then user decides:
- Keep current targets (350 lines, 80% coverage)?
- Raise them higher?
- Add new exemptions for known hard-to-test areas?
- Accept current state and move on?

No lowering targets without user explicit decision. Moving goalposts isn't compliance.

## Implementation Strategy

**Order matters:**
1. Phase 4 first (tests)
2. Phase 5 second (refactor behind tests)
3. Phase 6 third (decide based on evidence)

**Risk control:**
- Each Phase 4 test runs red-green before any Phase 5 change
- Phase 5 commits are small and isolated
- Full CI run after every commit
- If any gate fails, stop immediately; revert or fix before proceeding

**Scope boundaries:**
- No new dependencies
- No breaking API changes
- Preserve existing external contracts (routes, response formats, frontend hooks)
- Worker binary behavior unchanged (verified via tests or manual verification)

**What's NOT included:**
- Network-bound SNMP/Telnet function refactoring (cannot test safely)
- History rewrite for credentials in git history (separate scope)
- Adding new features beyond what's needed to satisfy existing rules
- Speculative abstraction (interfaces "just in case")

## Deliverables

1. Phase 4 spec file (this document + detailed test specs)
2. Phase 5 plan file (task-by-task split instructions)
3. Phase 6 recommendation (based on final metrics)

Once you approve the plan, I'll dispatch subagents like Phase 1-3 did, maintaining the same quality standards.
