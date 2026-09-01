# CLAUDE.md Compliance (Phases 1-3) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix the two behaviour bugs, delete all dead code and debug output, and correct the four stale claims in `CLAUDE.md`.

**Architecture:** Three independent groups of change, one commit per task, full gates green between commits. No production logic is restructured — that is Phases 4-5 in a separate plan. Every deletion here is verified to have zero importers first.

**Tech Stack:** Go 1.25 (GORM, Gin, zap, gosnmp, testify), React 18 + TypeScript (Vitest, Ant Design), GitHub Actions, Docker Compose, PostgreSQL/TimescaleDB, Redis.

**Spec:** `docs/superpowers/specs/2026-08-23-claude-md-compliance-design.md`

## Global Constraints

- Work on a branch, never `main`. Pushing to `main` triggers `docker-build`, which publishes images to GHCR.
- Backend gates, all must pass before each commit: `gofmt -s -l .` (empty output), `go vet ./...`, `go build ./...`, `go test ./... -race`.
- Frontend gates, all must pass before each commit: `npm run lint`, `npm run format:check`, `npm test -- --run`, `npm run build`.
- `golangci-lint run` is a local gate only; it is not in `ci.yml`. Run it anyway — it is currently at 0 issues.
- No new `.md` files. No new dependencies.
- Do not add `TODO`/`FIXME`, debug printing, or commented-out code.
- Comments explain *why*, never *what*.
- Commit messages: conventional-commit prefix, imperative mood, no emoji, no "Generated with" trailer unless the user asks.

---

### Task 1: Register AuditLog in AutoMigrate

Already implemented in the working tree; this task commits it. The `audit_logs` table exists in no database: `models.AuditLog` declares `TableName() → "audit_logs"` and 11 call sites write to it, but it was absent from `AutoMigrate()` and from `backend/migrations/`. Verified against the live database:
`docker exec tikman-postgres psql -U tikman -d tikman -c 'SELECT count(*) FROM audit_logs;'` → `ERROR: relation "audit_logs" does not exist`.

`AutoMigrate` is the right home rather than a new `.sql` file: `docker-compose.yml:9` mounts `backend/migrations` at `/docker-entrypoint-initdb.d`, which Postgres runs only when the data directory is empty. The running database has 3 days of data, so a new SQL file would never execute. `AutoMigrate` runs on every startup (`cmd/api/main.go:46`) and so repairs existing databases too.

**Files:**
- Modify: `backend/internal/models/models.go:6-13`
- Test: `backend/internal/services/audit_service_test.go:14-22`

**Interfaces:**
- Consumes: nothing.
- Produces: `models.AutoMigrate(db *gorm.DB) error` now creates `audit_logs`. Later tasks rely on nothing from this.

- [ ] **Step 1: Confirm the test setup change is present**

The test must migrate via `models.AutoMigrate(db)`, not by naming the model locally. A local `db.AutoMigrate(&models.AuditLog{}, ...)` passes whether or not the model is registered, which is what let the table ship missing while tests stayed green.

`backend/internal/services/audit_service_test.go:14-22` should read:

```go
func setupTestAuditDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	// models.AutoMigrate rather than a local AutoMigrate(&models.AuditLog{}):
	// migrating the model here passes whether or not AuditLog is registered in
	// models.AutoMigrate, which is what let audit_logs ship missing from every
	// real database while these tests stayed green.
	err = models.AutoMigrate(db)
	assert.NoError(t, err)

	return db
}
```

- [ ] **Step 2: Verify the test fails without the fix**

Temporarily remove `&AuditLog{}` from `backend/internal/models/models.go`, then run:

`cd backend && go test ./internal/services/ -run TestAuditService -v`

Expected: FAIL with `no such table: audit_logs` — the same error string Postgres produced. This proves the test can catch the bug.

- [ ] **Step 3: Restore the fix**

`backend/internal/models/models.go` must read:

```go
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&User{},
		&Site{},
		&OLT{},
		&ONT{},
		&ONTEvent{},
		&AuditLog{},
	)
}
```

- [ ] **Step 4: Verify the tests pass**

Run: `cd backend && go test ./internal/services/ -run TestAuditService -v`
Expected: PASS for `TestAuditService_Log` and `TestAuditService_LogUpdate`.

- [ ] **Step 5: Verify AutoMigrate invented no reverse foreign key**

Adding a model to `AutoMigrate` is exactly what caused the reversed-FK bug in commit `c98d5e8`, where GORM emitted `onts.ont_id -> ont_events.id`. `AuditLog` has no struct relation field (only `UserID *uuid.UUID`), so GORM has nothing to infer from — but verify rather than assume.

Run: `cd backend && go test ./internal/models/ -race -v`
Expected: PASS, including `TestAutoMigrateDoesNotPointONTsAtEvents`.

- [ ] **Step 6: Run the full gates**

```bash
cd backend
gofmt -s -l .          # expect empty output
go vet ./...
go build ./...
go test ./... -race
golangci-lint run      # expect "0 issues."
```

- [ ] **Step 7: Commit**

```bash
git add backend/internal/models/models.go backend/internal/services/audit_service_test.go
git commit -m "fix(models): create the audit_logs table AuditService writes to"
```

---

### Task 2: Run the auth and session tests against Redis in CI

`internal/auth` is at 0.0% coverage not for lack of tests but because `internal/auth/session_test.go:16-27` and `internal/middleware/auth_test.go:20-31` dial a hardcoded `localhost:6379` and `t.Skip` when it is unreachable. `docker port tikman-redis` returns nothing (the compose service publishes no host port) and `ci.yml`'s `backend-test` job has no `services:` block, so 7 tests covering session management and auth middleware skip **locally and in CI alike**.

These 7 tests have never executed. Some may be genuinely red. If so, report the failures rather than fixing them here — a fix belongs in its own task with its own root-cause investigation.

`t.Skip` stays: it is what lets `go test ./...` run on a laptop without Redis.

The service container is reached at `localhost:6379` from the job, so no test code changes. Do not add an env var — it would touch two test files for no gain.

**Files:**
- Modify: `.github/workflows/ci.yml:35-58` (the `backend-test` job)

**Interfaces:**
- Consumes: nothing.
- Produces: nothing consumed by later tasks.

- [ ] **Step 1: Confirm the tests currently skip**

Run: `cd backend && go test ./internal/auth/ ./internal/middleware/ -v 2>&1 | grep -E 'SKIP|PASS|FAIL'`
Expected: 4 `SKIP` in `internal/auth`, 3 `SKIP` in `internal/middleware`.

- [ ] **Step 2: Prove the tests pass against a real Redis before changing CI**

CI cannot be verified without a push, so verify the tests themselves locally. Start a throwaway Redis on the port the tests expect (the compose instance requires a password and publishes no port, so use a separate container):

```bash
docker run -d --rm --name tikman-redis-test -p 6379:6379 redis:7-alpine
cd backend && go test ./internal/auth/ ./internal/middleware/ -v -race 2>&1 | grep -E 'SKIP|PASS|FAIL|ok '
docker stop tikman-redis-test
```

Expected: zero `SKIP`; all 7 previously-skipped tests run. Record the real result. If any FAIL, stop and report it — do not proceed to Step 3 pretending it is green.

- [ ] **Step 3: Add the Redis service to the backend-test job**

In `.github/workflows/ci.yml`, insert a `services:` block into the `backend-test` job between `if: needs.changes.outputs.backend == 'true'` (line 39) and `steps:` (line 41). Use `redis:7-alpine` to match `docker-compose.yml`. No password: the tests connect without one, and the compose password only applies to the compose instance.

```yaml
  backend-test:
    name: Backend Tests
    runs-on: ubuntu-latest
    needs: changes
    if: needs.changes.outputs.backend == 'true'

    # internal/auth and internal/middleware tests t.Skip without a reachable
    # Redis, which silently left session and auth-middleware code untested.
    services:
      redis:
        image: redis:7-alpine
        ports:
          - 6379:6379
        options: >-
          --health-cmd "redis-cli ping"
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5

    steps:
```

- [ ] **Step 4: Validate the workflow YAML parses**

Run: `cd /Users/rohadimraja/Documents/tikman && python3 -c "import yaml,sys; yaml.safe_load(open('.github/workflows/ci.yml')); print('ci.yml parses')"`
Expected: `ci.yml parses`

- [ ] **Step 5: Confirm the rest of the suite is unaffected**

Run: `cd backend && go test ./... -race`
Expected: all packages `ok`. The tests skip again locally now that the throwaway Redis is stopped, which is intended.

- [ ] **Step 6: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci(backend): run the auth and session tests against Redis"
```

---

### Task 3: Delete the replaced internal/worker package

`internal/worker/` is 371 production lines plus 634 test lines that nothing imports. Verified: `grep -rn 'olt-provisioning/internal/worker' --include='*.go' .` returns nothing, and `Dockerfile.worker:12` builds `./cmd/worker/main.go`.

`cmd/worker` is the replacement and a functional superset — it has `PruneMissingFromDiscovery` and traffic-rate collection that `internal/worker` lacks, plus everything `internal/worker` does. `lookupByPortAndONT` is duplicated character-for-character between `cmd/worker/main.go:386` and `internal/worker/monitoring_tasks.go:124`.

Its 19 tests cover code nobody runs, so they protect nothing. `cmd/worker` keeps its own 4 tests (`olt_status_test.go`, `ont_events_test.go`).

Expect total coverage to **drop** when this package's 43.5% leaves the report. That is a metric artifact — no real test is lost.

**Files:**
- Delete: `backend/internal/worker/monitoring_worker.go` (121 lines)
- Delete: `backend/internal/worker/monitoring_tasks.go` (250 lines)
- Delete: `backend/internal/worker/monitoring_worker_test.go` (634 lines)

**Interfaces:**
- Consumes: nothing.
- Produces: nothing. The package had no importers.

- [ ] **Step 1: Re-verify nothing imports the package**

```bash
cd backend
grep -rn 'olt-provisioning/internal/worker' --include='*.go' . ; echo "exit=$?"
grep -n 'worker' Dockerfile.worker
```

Expected: no grep matches (`exit=1`), and `Dockerfile.worker` referencing `./cmd/worker/main.go`. If anything imports it, STOP and report — the deletion premise is wrong.

- [ ] **Step 2: Delete the package**

```bash
cd /Users/rohadimraja/Documents/tikman
git rm -r backend/internal/worker/
```

- [ ] **Step 3: Verify the build and suite still pass**

```bash
cd backend
go build ./...
go build -o /tmp/worker-check cmd/worker/main.go && rm -f /tmp/worker-check
go test ./... -race
```

Expected: build succeeds (both `./...` and the worker binary that Docker actually builds), all remaining packages `ok`, and `internal/worker` no longer listed.

- [ ] **Step 4: Run the remaining gates**

```bash
cd backend
gofmt -s -l .
go vet ./...
golangci-lint run
```

- [ ] **Step 5: Commit**

```bash
git commit -m "refactor(worker): delete the unused monitoring worker package"
```

---

### Task 4: Delete the scratch metrics tool

`backend/cmd/test_metrics/` is a scratch tool with a hardcoded production OLT address, SNMP community, and port at `main.go:20` — `driver.WalkMetrics("192.0.2.10", "public", 23161)` — closing with `fmt.Println("\n✅ RX Power decoder fix verified!")`. It is shipped debug output, magic values, and a real device address plus community string in git history.

Deleting the directory does **not** remove the credentials from git history; that needs a separate history rewrite, and the `public` community on a reachable OLT warrants rotation regardless. Both are out of scope here — flag them to the user, do not action them.

`cmd/probe_hsgq/` and `cmd/seed-events/` are **not** touched: their `fmt.Println` calls are intended CLI output, not debug leftovers.

**Files:**
- Delete: `backend/cmd/test_metrics/main.go`

**Interfaces:**
- Consumes: nothing.
- Produces: nothing.

- [ ] **Step 1: Verify it is not built or referenced**

```bash
cd /Users/rohadimraja/Documents/tikman
grep -rn 'test_metrics' --include='*.yml' --include='*.yaml' --include='Dockerfile*' --include='*.sh' --include='*.go' . | grep -v node_modules | grep -v graphify-out
```

Expected: no matches outside `backend/cmd/test_metrics/` itself. `ci.yml` builds only `cmd/api/main.go` and `cmd/worker/main.go`.

- [ ] **Step 2: Delete it**

```bash
git rm -r backend/cmd/test_metrics/
```

- [ ] **Step 3: Verify the build and suite**

```bash
cd backend
go build ./...
go test ./... -race
gofmt -s -l .
go vet ./...
```

- [ ] **Step 4: Commit**

```bash
git commit -m "chore(backend): delete the scratch metrics verification tool"
```

---

### Task 5: Remove the frontend debug logging

Eight `console.log` calls ship to production. Two `useEffect` hooks in `useOntListLogic.ts` exist only to log and go entirely; a third keeps its `refetch()` and loses only the log line.

Do not delete the hook at lines 110-118 — `refetch()` drives status filtering. Removing it breaks the filter.

The `useEffect` import stays: it is used 5 times in the file and survives both deletions.

**Files:**
- Modify: `frontend/src/application/hooks/useOntListLogic.ts` — delete lines 102-107 and 147-168, and the log call inside the hook at 110-118
- Modify: `frontend/src/presentation/components/ONTEventTimeline.tsx:75-79`
- Modify: `frontend/src/presentation/components/OntFilters.tsx:125`

**Interfaces:**
- Consumes: nothing.
- Produces: no signature changes. `useOntListLogic` keeps its return shape.

- [ ] **Step 1: Delete the logging-only "Update total after data loads" hook**

Remove these lines from `useOntListLogic.ts` (currently 102-107) entirely, comment included:

```typescript
  // Update total after data loads
  useEffect(() => {
    if (ontsData && ontsData.total) {
      console.log(`[Data Loaded] Total ONTs: ${ontsData.total}`);
    }
  }, [ontsData]);
```

- [ ] **Step 2: Strip the log from the refetch hook, keeping the hook**

Replace:

```typescript
  // Only refetch when filter changes
  useEffect(() => {
    if (statusFilter) {
      console.log(
        "[Status Filter Changed] Refetching with status:",
        statusFilter,
      );
      refetch();
    }
  }, [statusFilter, refetch]);
```

with:

```typescript
  // Only refetch when filter changes
  useEffect(() => {
    if (statusFilter) {
      refetch();
    }
  }, [statusFilter, refetch]);
```

- [ ] **Step 3: Delete the "[ONT DEBUG]" hook**

Remove the whole block (currently 147-168), comment included — the entire `useEffect` whose only body is `console.log("[ONT DEBUG] ontsData:", {...})` with its 6-entry dependency array.

- [ ] **Step 4: Delete the ONTEventTimeline debug logs**

In `ONTEventTimeline.tsx`, remove lines 75-79 so that `const events = eventsData?.events || [];` is followed directly by a blank line and then `const getEventIcon = ...`:

```typescript
  console.log("[ONTEventTimeline DEBUG] Raw eventsData:", eventsData);
  console.log("[ONTEventTimeline DEBUG] Parsed events:", events);
  if (events.length > 0) {
    console.log("[ONTEventTimeline DEBUG] First event:", events[0]);
  }
```

- [ ] **Step 5: Delete the OntFilters log**

In `OntFilters.tsx`, change:

```typescript
          onChange={(value) => {
            console.log("[Port Selected]", value);
            setSelectedPortId(value);
          }}
```

to:

```typescript
          onChange={(value) => {
            setSelectedPortId(value);
          }}
```

- [ ] **Step 6: Verify no console.log remains in src**

Run: `cd frontend && grep -rn 'console\.log' src/ ; echo "exit=$?"`
Expected: `exit=1` (no matches). Grep for `console.log` only — `console.error` calls in catch blocks (`useOntListLogic.ts:218,239,252` and `apiClient.ts`) are intentional error reporting and stay.

- [ ] **Step 7: Run the frontend gates**

```bash
cd frontend
npm run format:check
npm run lint
npm test -- --run
npm run build
```

Expected: all pass. 89 tests in 24 files. If `format:check` fails, run `npm run format` and re-check.

- [ ] **Step 8: Commit**

```bash
git add frontend/src
git commit -m "chore(frontend): remove debug logging from the ONT list and timeline"
```

---

### Task 6: Remove the banner comments from snmp_constants.go

`backend/internal/connectivity/snmp_constants.go` uses `// ====...` separator banners at lines 3, 6, and 26, which `CLAUDE.md` bans as section banners.

Keep every line of the content between them. The OID references, the two index-space formulas, and the verified-hardware note are exactly the "why" commenting the document asks for — only the separator lines go.

**Files:**
- Modify: `backend/internal/connectivity/snmp_constants.go:1-26`

**Interfaces:**
- Consumes: nothing.
- Produces: no code changes; comments only. All exported constants keep their names and values.

- [ ] **Step 1: Delete the three banner lines**

Remove the three lines consisting of `// ` followed by `=` characters (currently 3, 6, and 26). The result begins:

```go
package connectivity

// ZTE C300/C320 SNMP OIDs - VERIFIED AGAINST https://github.com/Cepat-Kilat-Teknologi/snmp-olt-zte
// Tested against ZTE C300 V2.1.0 and C320 V2.1.0 production hardware
//
// TWO INDEX SPACES:
```

and continues unchanged through the OID list to `const (`.

- [ ] **Step 2: Verify no constant value changed**

```bash
cd backend
git diff -- internal/connectivity/snmp_constants.go | grep -E '^[-+]' | grep -v '^[-+][-+]' | grep -vE '^[-+]// ?=*$'
```

Expected: no output. Any line here means something other than the banners changed — revert and redo.

- [ ] **Step 3: Verify no banner comments remain anywhere**

Run: `cd /Users/rohadimraja/Documents/tikman && grep -rn -E '// ?={3,}|// ?-{3,}' backend --include='*.go' frontend/src ; echo "exit=$?"`
Expected: `exit=1`.

- [ ] **Step 4: Run the gates**

```bash
cd backend
gofmt -s -l .
go vet ./...
go build ./...
go test ./... -race
```

- [ ] **Step 5: Commit**

```bash
git add backend/internal/connectivity/snmp_constants.go
git commit -m "style(connectivity): drop the banner separators from the OID notes"
```

---

### Task 7: Delete the generated report and summary documents

Seven files are progress reports and fix summaries that `CLAUDE.md` bans ("Do not generate progress-report or hand-off documents", "no emoji headers, no celebration"). They were created on 2026-08-15 in `7e77c25`, five days before the rule was written, so this is old debt rather than a fresh violation.

Delete:
- `backend/CODE_QUALITY_REPORT.md` (128 lines, "**Status:** ✅ Completed")
- `docs/SECURITY_IMPROVEMENTS_SUMMARY.md` (278, "🔒 ... 8.5/10 → **9.5/10** 🎉")
- `docs/ONT_LIST_PERFORMANCE_FIX.md` (126)
- `docs/archive/IMPLEMENTATION_COMPLETE.md` (418)
- `docs/archive/FIX_SUMMARY.md` (236)
- `docs/archive/TOPOLOGY_METRICS_FIX.md` (193)
- `docs/archive/METRICS_FIX_DOCUMENTATION.md` (237)

Keep, because they are reference documentation rather than progress reports: `docs/SECURITY.md`, `docs/SECURITY_AUDIT.md`, `docs/MONITORING_MODULE_DESIGN.md`, `backend/TESTING.md`, `backend/TEST_TRAFFIC_STATS.md`, `docs/archive/TROUBLESHOOTING_DUPLICATE_SERIALS.md`, `docs/archive/ONT_MONITORING_QUICK_START.md`, `docs/archive/INSTRUCTIONS_TO_FIX_YOUR_OLT.md`, `docs/archive/SOT.md`, and everything in `docs/superpowers/`.

`docs/README.md` indexes four of the deleted files and must be updated, not left pointing at missing paths.

**Files:**
- Delete: the 7 files listed above
- Modify: `docs/README.md`

**Interfaces:**
- Consumes: nothing.
- Produces: nothing.

- [ ] **Step 1: Delete the seven files**

```bash
cd /Users/rohadimraja/Documents/tikman
git rm backend/CODE_QUALITY_REPORT.md \
       docs/SECURITY_IMPROVEMENTS_SUMMARY.md \
       docs/ONT_LIST_PERFORMANCE_FIX.md \
       docs/archive/IMPLEMENTATION_COMPLETE.md \
       docs/archive/FIX_SUMMARY.md \
       docs/archive/TOPOLOGY_METRICS_FIX.md \
       docs/archive/METRICS_FIX_DOCUMENTATION.md
```

- [ ] **Step 2: Update docs/README.md to drop the deleted entries**

Remove the bullets for `FIX_SUMMARY.md`, `METRICS_FIX_DOCUMENTATION.md`, `IMPLEMENTATION_COMPLETE.md`, and `TOPOLOGY_METRICS_FIX.md` from the `/archive` list, and `SECURITY_IMPROVEMENTS_SUMMARY.md` plus `ONT_LIST_PERFORMANCE_FIX.md` from the root list. The `/archive` list keeps `TROUBLESHOOTING_DUPLICATE_SERIALS.md`, `ONT_MONITORING_QUICK_START.md`, `INSTRUCTIONS_TO_FIX_YOUR_OLT.md`, and `SOT.md`; the root list keeps `SECURITY.md`, `SECURITY_AUDIT.md`, and `MONITORING_MODULE_DESIGN.md`.

- [ ] **Step 3: Verify no dangling links to the deleted files**

```bash
cd /Users/rohadimraja/Documents/tikman
grep -rn -E 'CODE_QUALITY_REPORT|SECURITY_IMPROVEMENTS_SUMMARY|ONT_LIST_PERFORMANCE_FIX|IMPLEMENTATION_COMPLETE|FIX_SUMMARY|TOPOLOGY_METRICS_FIX|METRICS_FIX_DOCUMENTATION' \
  --include='*.md' . | grep -v node_modules | grep -v graphify-out | grep -v docs/superpowers
```

Expected: no output. Matches inside `docs/superpowers/specs/` are this work's own spec quoting the filenames and are fine.

- [ ] **Step 4: Commit**

```bash
git add docs/README.md
git commit -m "docs: delete the generated progress reports and fix summaries"
```

---

### Task 8: Correct the stale claims in CLAUDE.md

Four documented facts no longer match the code, plus the SNMP exemption the spec decided. A stale map is worse than no map: the next session works from it.

**Files:**
- Modify: `CLAUDE.md` lines 66, 137, 253, 261, 275-276, 426-427, and the Code Quality Standards section

**Interfaces:**
- Consumes: the deletions in Tasks 3-7 (so the file describes the tree as it now is).
- Produces: nothing.

- [ ] **Step 1: Fix the AutoMigrate claim (line 137)**

Replace:

```
- `models.go` defines AutoMigrate - currently only migrates `User`, `Site`, `OLT`
```

with:

```
- `models.go` defines AutoMigrate - migrates `User`, `Site`, `OLT`, `ONT`, `ONTEvent`, `AuditLog`
```

- [ ] **Step 2: Fix the environment variable names (lines 426-427)**

`DATABASE_URL` and `REDIS_URL` do not exist. `internal/config/config.go` reads discrete keys via Viper, matching `.env.example`. Replace:

```
- `DATABASE_URL` - PostgreSQL connection string
- `REDIS_URL` - Redis connection string  
```

with:

```
- `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME` - PostgreSQL connection
- `REDIS_HOST`, `REDIS_PORT`, `REDIS_PASSWORD` - Redis connection
```

- [ ] **Step 3: Mark golangci-lint as a local-only gate**

`ci.yml`'s `backend-lint` job runs `go vet`, `gofmt -s`, and `go mod verify` — not `golangci-lint`. In the backend testing block around line 253, change the `# Run linter (must have zero issues)` comment to note it is a local gate absent from CI, and in the checklist at line 276 mark `golangci-lint run` as local-only. Also add the two CI gates the checklist omits: `gofmt -s -l .` and `go mod verify`.

- [ ] **Step 4: Correct the npm test invocation (lines 66, 261, 275)**

`npm test` is `vitest` in watch mode and never exits, so it cannot be a gate as written. CI runs `npm test -- --run --coverage`. Update the three places to show `npm test -- --run` (or `-- --run --coverage`) and note that bare `npm test` is watch mode.

- [ ] **Step 5: Record the SNMP exemption in Code Quality Standards**

Under the file-size and function-length limits, add the exemption the spec decided, with its reason:

```markdown
**Exemption: network-bound SNMP/Telnet code.** Functions that construct
`&gosnmp.GoSNMP{}` or dial directly (`snmp_client.go`, `snmp_walks.go`,
`snmp_metrics_walk.go`, `telnet.go`) are exempt from the line limits. They
cannot be unit tested without an interface refactor, a fake SNMP responder, or
real hardware, so splitting them would be verified by `go build` and review
alone. Restructuring untestable code to satisfy a line count is risk without
payoff. Code that only *calls* `connectivity` is not exempt.
```

- [ ] **Step 6: Verify no stale references to deleted paths remain**

```bash
cd /Users/rohadimraja/Documents/tikman
grep -n -E 'internal/worker|test_metrics|DATABASE_URL|REDIS_URL|only migrates' CLAUDE.md ; echo "exit=$?"
```

Expected: `exit=1`.

- [ ] **Step 7: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: correct the stale AutoMigrate, env var, and gate claims"
```

---

### Task 9: Commit the spec and report the outcome

**Files:**
- Add: `docs/superpowers/specs/2026-08-23-claude-md-compliance-design.md`
- Add: `docs/superpowers/plans/2026-08-23-claude-md-compliance-phase1-3.md`

**Interfaces:**
- Consumes: all prior tasks.
- Produces: the baseline Phases 4-5 are planned against.

- [ ] **Step 1: Re-run every gate on the final tree**

```bash
cd backend
gofmt -s -l . && go vet ./... && go build ./... && go test ./... -race && golangci-lint run
cd ../frontend
npm run lint && npm run format:check && npm test -- --run && npm run build
```

- [ ] **Step 2: Record the post-deletion coverage number**

```bash
cd backend
go test ./... -coverprofile=/tmp/cov.out >/dev/null 2>&1
go tool cover -func=/tmp/cov.out | tail -1
rm -f /tmp/cov.out
```

Expect a figure at or below the 38.4% baseline: deleting `internal/worker` removes its 43.5% from the average. Report it as the metric artifact it is, not as a regression. This number is the input to Phase 6.

- [ ] **Step 3: Commit the spec and plan**

```bash
cd /Users/rohadimraja/Documents/tikman
git add docs/superpowers/specs/2026-08-23-claude-md-compliance-design.md \
        docs/superpowers/plans/2026-08-23-claude-md-compliance-phase1-3.md
git commit -m "docs(superpowers): add the CLAUDE.md compliance spec and phase 1-3 plan"
```

- [ ] **Step 4: Report to the user, do not push**

Summarise: lines deleted, the Redis test result from Task 2 Step 2 (including any genuine failures), and the new coverage figure. Then flag, without acting:
- The OLT credentials (`192.0.2.10`, community `public`) remain in git history; purging needs a history rewrite.
- That community warrants rotation regardless.
- Phases 4-5 (37 functions, 10 files, handler DB access, the raw `axios` call) need their own plan.
- Phase 6 needs the user to set the coverage and line-limit numbers.

Pushing to `main` publishes images to GHCR. Ask before pushing anything.

---

## Notes for the executor

- Tasks 1-8 are independent and land as 8 commits. If a task fails its gates, stop and report; do not roll it into the next task.
- Task 2 Step 2 is the one place a genuine unknown may surface. Those 7 tests have never run. Report red honestly.
- No task here restructures production logic. If you find yourself refactoring a function, you are in Phase 5's territory — stop.
