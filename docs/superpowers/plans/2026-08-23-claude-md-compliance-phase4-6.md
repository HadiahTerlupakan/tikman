# CLAUDE.md Compliance (Phases 4-6) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete full compliance with `CLAUDE.md` by adding test coverage for pure logic, refactoring oversized files/functions, removing DB access from handlers, and reconciling targets.

**Architecture:** Three independent phases, each producing working software. Phase 4 builds test jaring pengaman before any refactor. Phase 5 does structural changes behind that net. Phase 6 decides final targets based on evidence.

**Tech Stack:** Go 1.25 (GORM, Gin), React 18 + TypeScript (Vitest), GitHub Actions, PostgreSQL/TimescaleDB, Redis.

**Spec:** `docs/superpowers/specs/2026-08-23-claude-md-compliance-phase4-6.md` (read first)

## Global Constraints

- Work on branch `main` — user explicitly approved this. Do NOT push until all phases complete.
- Backend gates, all must pass before each commit: `gofmt -s -l .` (empty output), `go vet ./...`, `go build ./...`, `go test ./... -race`.
- Frontend gates, all must pass before each commit: `npm run lint`, `npm run format:check`, `npm test -- --run`, `npm run build`.
- No new `.md` files. No new dependencies.
- No TODO/FIXME, debug printing, commented-out code.
- Comments explain *why*, never *what*.
- Commit messages: conventional-commit prefix, imperative mood, no emoji, no "Generated with" trailer.
- Network-bound SNMP/Telnet functions excluded from line limits (documented exemption in CLAUDE.md).

---

## Phase 4: Test for Pure Logic (Jaring Pengaman)

### Task 1: Write tests for snmp_encoding.go decodeOnuIDIfIndex

**Files:**
- Create: `internal/connectivity/snmp_encoding_test.go` (append existing tests)
- Modify: N/A (existing functions, new tests only)

**Interfaces:**
- Consumes: nothing.
- Produces: test coverage for `decodeOnuIDIfIndex` function.

- [ ] **Step 1: Write failing test**

Create table-driven test for `decodeOnuIDIfIndex`:

```go
func TestDecodeOnuIDIfIndex(t *testing.T) {
	tests := []struct {
		ifIndex uint32
		slot    int
		port    int
		ok      bool
	}{
		// Valid cases from encode example
		{ifIndex: 0x10030100, slot: 3, port: 1, ok: true}, // frame=1, slot=3, port=1
		{ifIndex: 0x10010100, slot: 1, port: 1, ok: true}, // frame=1, slot=1, port=1
		{ifIndex: 0x10ff0100, slot: 255, port: 1, ok: true}, // max slot
		{ifIndex: 0x1003ff00, slot: 3, port: 255, ok: true}, // max port
		
		// Edge cases that should fail
		{ifIndex: 0x10000100, slot: 0, port: 1, ok: false}, // slot=0 invalid
		{ifIndex: 0x10010000, slot: 1, port: 0, ok: false}, // port=0 invalid
		{ifIndex: 0x00000000, slot: 0, port: 0, ok: false}, // zero value
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("ifIndex=%08x", tt.ifIndex), func(t *testing.T) {
			slot, port, ok := decodeOnuIDIfIndex(tt.ifIndex)
			if ok != tt.ok {
				t.Errorf("ok mismatch: got %v, want %v", ok, tt.ok)
			}
			if ok && (slot != tt.slot || port != tt.port) {
				t.Errorf("decode result: got slot=%d port=%d, want slot=%d port=%d",
					slot, port, tt.slot, tt.port)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/connectivity/ -run TestDecodeOnuIDIfIndex -v`
Expected: FAIL with "no such function" or similar error if function doesn't exist, or actual test failures.

- [ ] **Step 3: Verify the function exists (it does)**

The function `decodeOnuIDIfIndex` exists at `snmp_encoding.go:24`. The test should compile. If it runs but fails, note which cases fail.

- [ ] **Step 4: Ensure implementation matches specification**

The function extracts slot/port from bytes 2 and 1 respectively, rejects zero values. If tests pass immediately, great — if they reveal bugs, fix them.

- [ ] **Step 5: Commit**

```bash
git add internal/connectivity/snmp_encoding_test.go
git commit -m "test(connectivity): add TDD tests for decodeOnuIDIfIndex"
```

- [ ] **Step 6: Verify all connectivity tests pass**

Run: `cd backend && go test ./internal/connectivity/ -race`
Expected: all packages ok.

Repeat similar pattern for remaining functions:
- `decodeOnuTypeIfIndex` (lines 36-59)
- `encodeOnuIDIfIndex` (lines 60-66)
- `parseZteHexTimestamp` (lines 67-90)
- `decodeZxGponPower` edge cases (line 91, already 90%)

Each gets its own commit after verification.

---

### Task 2: Write tests for snmp_discovery_structure.go buildTopologyStructure

**Files:**
- Create: `internal/connectivity/snmp_discovery_structure_test.go`
- Read: `snmp_discovery_structure.go` for function signature and behavior

**Interfaces:**
- Consumes: `ONTLocation` struct, topology input maps.
- Produces: test coverage for `buildTopologyStructure`.

This function maps slot/port ONT counts into TopologySlotResponse structures. Needs table-driven tests covering:
- Empty input → empty output
- Single slot, single port
- Multiple slots, multiple ports
- Boundary conditions (max slot/port numbers)

Follow same TDD red-green pattern as Task 1. Each function/test combination gets one commit.

---

### Task 3: Write handler tests for DB query logic

**Files:**
- Create: `internal/api/dto_test.go` (extract ToSiteResponse/ToOLTResponse logic)
- Modify: potentially move query logic to service layer first

**Interfaces:**
- Consumes: model structs (Site, OLT, etc.).
- Produces: tests that verify query correctness without real DB.

Strategy: Don't test DB calls directly — extract the query logic into a service method first (Phase 5), then test that service method.

Alternative: Mock GORM.DB in tests using testify/mock or gomock. Document mock setup clearly.

---

### Task 4: Run frontend axios replacement planning

**Files:**
- Read: `frontend/src/application/hooks/useOntListLogic.ts` line 183
- Create: repository file for topology fetches if needed

**Action:** Replace `axios.get()` at line 183 with either:
- Existing OntRepository (if it has topology method)
- New method in OntRepository
- Dedicated OnusRepository for unconfigured ONU fetches

Verify: `apiClient.ts` interceptor handles camelCase/snakCage conversion correctly for topology response.

---

## Phase 5: Structural Refactor

### Task 5: Split metrics_service.go into operation-specific files

**Files:**
- Delete: `internal/services/metrics_service.go` (525 lines)
- Create: 
  - `internal/services/metrics_get_realtime.go`
  - `internal/services/metrics_get_history.go`
  - `internal/services/metrics_get_latest.go`
  - `internal/services/metrics_store.go`

**Strategy:** Split by business capability. Each file defines one public function (GetRealtime, GetHistory, GetLatest, StoreMetrics). Keep shared helpers together or inline if simple.

**Constraint:** One unit per commit. After each split, run full test suite. If anything breaks, fix immediately.

---

### Task 6: Extract DTO queries to service layer

**Files:**
- Delete: `internal/api/dto.go` query lines (not entire file)
- Create/Add to: `internal/services/site_service.go` (LineCountOLTBySite), `internal/services/olt_service.go` (CountONTByOLT)

**Target lines:** `dto.go:87` (site→OLT count), `dto.go:171` (OLT→site lookup)

Move these queries into dedicated service methods. Update all callers to use service instead of passing DB.

---

### Task 7: Remove direct DB access from handlers

**Files:**
- `internal/api/seed_handler.go:138,160` → event_service owns event creation
- `internal/api/ont_handler.go:119` → ont_service owns ONT queries

**Pattern:** Handlers call services; services own database. Never cross that boundary.

---

### Task 8: Frontend axios → repository

**Files:**
- Modify: `frontend/src/application/hooks/useOntListLogic.ts:183`
- Modify: possibly `frontend/src/infrastructure/repositories/OntRepository.ts`

Replace raw axios call with repository method. Ensure humps conversion works for nested topology response.

---

## Phase 6: Reconcile Targets

No implementation steps — this is purely decision-making based on final metrics.

After Phase 5 completes:
1. Run `go test ./... -coverprofile=/tmp/cov.out && go tool cover -func /tmp/cov.out`
2. Run `find . -name '*.go' -not -path './node_modules/*' | xargs wc -l | sort -rn`
3. Report: coverage %, remaining files >350 lines, remaining functions >50 lines
4. User decides: keep targets, adjust targets, accept state

No automatic decisions. This is your choice.

---

## Notes for executor

- Tasks are designed to be independent where possible.
- If a task reveals design issues, stop and report — don't paper over.
- Every commit must have green gates before proceeding.
- Network-bound SNMP/Telnet functions exempt from line limits (documented in CLAUDE.md).
- Phase 4 tests are critical — they're the safety net for Phase 5. Don't skip them.
- If you find yourself needing to understand code beyond what was provided, escalate rather than guess.

## Execution handoff

After saving the plan, offer execution choice:

**Plan complete and saved to `docs/superpowers/plans/2026-08-23-claude-md-compliance-phase4-6.md`. Two execution options:**

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints

**Which approach?**
