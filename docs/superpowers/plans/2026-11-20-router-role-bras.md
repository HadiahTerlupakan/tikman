# Router Role and BRAS Monitoring Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give each MikroTik a role (border / bras / other) that decides
its polling and tabs, and give a BRAS its own per-minute monitoring and a
Ringkasan page.

**Architecture:** A `role` column drives which `router_poll_jobs` rows
exist. A new `bras` job reads six RouterOS menus over the existing
REST/API client into a snapshot column and a per-minute sample
hypertable. A BRAS's interfaces poll GETs only the ports discovery has
mapped, never the 2000 PPPoE rows. The frontend picks its tabs by role
and adds a Ringkasan tab fed by one new endpoint.

**Tech Stack:** Go 1.26, Gin, GORM, gosnmp, TimescaleDB; React + TS,
AntD, React Query, recharts.

**Spec:** `docs/superpowers/specs/2026-11-20-router-role-bras-design.md`

## Global Constraints

- The repo is public. Fixtures use 192.0.2.x / 198.51.100.x, AS64500,
  and invented names (`bras-1`, `pel-0001`). Never real ones.
- Migrations are single files: `81_router_role.sql`,
  `82_router_uplink.sql`, `83_router_bras.sql`. Use
  `ADD COLUMN IF NOT EXISTS`. GORM tags must match the SQL, because
  AutoMigrate runs first.
- Role values are exactly `border`, `bras`, `other`. The UI labels are
  Border, BRAS, Lainnya, and "Otomatis" for "let TikMan guess".
- The Go type is `models.RouterRole`. The TS type is `RouterRole`. Both
  are distinct from the faceplate's `PortRole`.
- Every new RouterOS read works over REST, api and api-ssl.
- CLAUDE.md applies:
  - ≤350 lines per file, ≤50 lines per function
  - comments explain why
  - TDD, and every bug fix gets a test
  - frontend copy is Indonesian, sentence case
  - `router_poll.go` is at 311 lines, so new poll code goes in new
    files
- Border routers must behave exactly as today: same jobs, same walk,
  same tabs, same colours.
- Thresholds, copied from the spec:
  - Drop: sessions fall by ≥ max(100, 5% of the previous sample) against
    a sample at most 2 min older.
  - `sessions15mAgo`: the sample nearest to latest−15 min, within
    ±2 min.
  - CPU LED: amber ≥70%, red ≥90%.
  - Pool LED: amber <10% free, red <2% free.
  - Snapshot stale after 5 min.
  - Sample retention: 30 days.
  - Top users: 10, by download.
  - BRAS ifName walk: 50 repetitions per GETBULK, 30 s budget.

## Review Focus

1. **A BRAS that uses local PPP secrets and has no RADIUS entry.** The
   RADIUS signal reads "Tidak dipakai", grey. It is never red.
   (Task 5 test, Task 8 test.)
2. **count-only replies differ.** REST returns `[{"ret":"2041"}]` or
   `{"ret":"2041"}`; the API returns `!done =ret=2041`. Every shape
   parses to 2041. Anything else is an error, never 0. (Task 2 tests.)
3. **A pool whose total is unknown** is left out of `pool_min_free`. If
   no pool has a known total, the pool signal shows "—", grey. (Task 5,
   Task 8.)
4. **A role change while a job is locked.** The deleted job row is
   later settled by the worker. Complete/Fail on a missing row must not
   error or recreate the row. (Task 1 test.)
5. **Gaps in samples.** A missing 15-minute-ago sample gives null,
   shown as "—", never 0. No drop is detected across a gap of more than
   2 min. Counters that went down give null rejects, not a negative
   number. (Task 6 tests.)

---

### Task 1: Role column, poll kinds per role, edit and reset

**Files:**
- Create: `backend/migrations/81_router_role.sql`
- Create: `backend/internal/services/router_role.go`, `router_role_test.go`
- Modify: `backend/internal/models/mikrotik.go`
- Modify: `backend/internal/services/router_poll_jobs.go`
  (`AllRouterPollKinds` → `RouterPollKindsFor`, `RouterIntervalFor` +
  bras)
- Modify: `backend/internal/services/router_service.go`
  (create seeds by role, update applies role)
- Modify: `backend/internal/api/router_dto.go`, `router_handler.go`,
  `router_handler_test.go`
- Not modified here: `backend/cmd/worker/router_jobs.go`. The `bras`
  case comes in Task 5. Until then, a `bras` job fails as an unknown
  kind. That is acceptable because no deploy happens before Task 5.

**Interfaces:**
- Produces:
  - `models.RouterRole` (string) with `RouterRoleBorder="border"`,
    `RouterRoleBRAS="bras"`, `RouterRoleOther="other"`
  - `models.RouterPollKindBRAS = "bras"`
  - Model fields:
    - `Role RouterRole gorm:"type:varchar(8);not null;default:border" json:"role"`
    - `RoleSetByAdmin bool gorm:"not null;default:false" json:"role_set_by_admin"`
    - `RoleGuess *RouterRole gorm:"type:varchar(8)" json:"role_guess"`,
      which discovery writes so the form can say what TikMan guessed
  - `services.RouterPollKindsFor(role models.RouterRole) []models.RouterPollKind`
  - `services.SetRouterRole(tx *gorm.DB, routerID uuid.UUID, role models.RouterRole, byAdmin bool) error`:
    writes the role, syncs the job rows, and deletes
    `router_bgp_sessions` when `bgp` leaves the set
  - `RouterIntervalFor(bras) = 1 * time.Minute`
  - `UpdateRouterRequest` gains `Role *string binding:"omitempty,oneof=border bras other"`
    and `ResetRole bool`. `CreateRouterRequest` gains
    `Role *string binding:"omitempty,oneof=border bras other"`.

Migration 81 adds `role`, `role_set_by_admin` and `role_guess` with the
defaults above. It adds no job backfill: every existing row becomes
border.

- [ ] **Step 1: Write failing tests** in `router_role_test.go`:
  - `TestRouterPollKindsFor`, table-driven:
    - border → `[discovery interfaces bgp routes]`
    - other → `[discovery interfaces bgp]`
    - bras → `[discovery interfaces bras]`
  - `TestSetRouterRoleSyncsJobs`:
    - Start with a border router that has 4 job rows and 2 BGP session
      rows. `SetRouterRole(bras, true)`.
    - Job kinds = `{discovery, interfaces, bras}`. The new `bras` row is
      due at or before now. The kept rows' `due_at` is unchanged. BGP
      sessions = 0. `role_set_by_admin` = true.
  - `TestSetRouterRoleBackToBorderRestoresJobs`: bras → border gives the
    4 border kinds back.
  - `TestCompleteOnDeletedJobIsNoop`: claim a `routes` job, change the
    role to bras (the row is deleted), then `Complete(job)` returns nil,
    and no `routes` row exists afterwards. (Review Focus 4.)
- [ ] **Step 2:** Run `cd backend && go test ./internal/services/ -run 'RouterPollKindsFor|SetRouterRole|CompleteOnDeleted' -v`.
  Expected: FAIL (undefined).
- [ ] **Step 3: Implement.**
  - `release` must use an UPDATE that touches 0 rows without error. It
    already does: verify, and fix only if the test shows otherwise.
  - `createRouterJobs(tx, routerID, role)` seeds
    `RouterPollKindsFor(role)`.
  - Create with `Role` set stores it with `role_set_by_admin=true`.
    Without it: border, false.
- [ ] **Step 4: Handler tests** in `router_handler_test.go`, following
  the rack position test at :103-127:
  - `TestUpdateRouterRole`: PATCH `{"role":"bras"}` returns 200, the
    stored role is bras, `role_set_by_admin` is true, and the audit
    entry includes role.
  - `TestUpdateRouterRoleRejectsUnknown`: `{"role":"core"}` returns 400.
  - `TestUpdateRouterResetRole`: `{"reset_role":true}` sets
    `role_set_by_admin` false and makes the discovery job due now.
  - The router JSON includes `role`, `role_set_by_admin` and
    `role_guess`.
- [ ] **Step 5:** Run `go test ./internal/services/ ./internal/api/ -v`
  → PASS. Then run `gofmt -s -l . && go vet ./...` → clean.
- [ ] **Step 6: Commit** `feat(routers): give each router a role that picks its poll jobs`.

### Task 2: RouterOS reads for count, monitor, proplist and the BRAS menus

**Files:**
- Modify: `backend/internal/mikrotik/client.go`, `rest.go`, `api.go`,
  `types.go`
- Create: `backend/internal/mikrotik/client_bras.go`, `client_bras_test.go`
- Modify: `backend/internal/services/router_poll.go`: add the new
  methods to `RouterAPI`, and to the fake in `router_poll_test.go`

**Interfaces:**
- Produces, as `Transport` additions with names left to the builder:
  - a count: REST `POST /rest/<path>/print {"count-only":"", ".query":[...]}`,
    API `<path>/print =count-only= ?k=v`
  - a one-shot command with args: REST `POST /rest/<path> {args}`,
    API `<path> =k=v`
  - a list with a property list: REST GET `?.proplist=a,b`,
    API `=.proplist=a,b`
- `Client` methods. Each works over all three protocols, and each is
  added to `services.RouterAPI`:
  - `PPPoEServers() ([]PPPoEServer, error)` reads `/interface/pppoe-server/server`
  - `PPPActiveCount() (int, error)`
  - `IPPools() ([]IPPool, error)`, where `IPPool{Name, Ranges, Total, Used, Available string}`
  - `PoolUsedCount(pool string) (int, error)`
  - `RadiusServers() ([]RadiusServer, error)`, where `RadiusServer{ID ".id", Address, Service, Disabled string}`
  - `RadiusMonitor(id string) (RadiusStats, error)`, where
    `RadiusStats{Pending, Requests, Accepts, Rejects, Resends, Timeouts, BadReplies, LastRequestRTT string}`
    and the RouterOS keys are kebab-case
  - `CPULoads() ([]CPULoad, error)` reads `/system/resource/cpu`, where `CPULoad{CPU, Load string}`
  - `DynamicQueues() ([]SimpleQueue, error)` reads `/queue/simple`
    with `dynamic=true` and proplist `name,target,rate`, where
    `SimpleQueue{Name, Target, Rate string}`
  - `DefaultRoute() ([]Route, error)` reads `/ip/route` with
    `dst-address=0.0.0.0/0` and `active=true`, any origin. `Route` gains
    `Gateway` if it is missing.
  - `Interfaces()` now filters `dynamic=false` on the router.

- [ ] **Step 1: Write failing tests** in `client_bras_test.go`, using
  the existing REST httptest and API fake-wire helpers in
  `client_test.go` / `api_test.go`:
  - `TestCountParsesEveryShape`, table-driven over REST
    `[{"ret":"2041"}]`, REST `{"ret":"2041"}`, and API `!done =ret=2041`,
    all → 2041. REST `[]` returns an error, not 0. (Review Focus 2.)
  - `TestCountSendsQueryAndCountOnly`: the REST request body has
    `"count-only":""` and `".query":["pool=pool-a"]`. The API words
    include `=count-only=` and `?pool=pool-a`.
  - `TestRadiusMonitorSendsOnce`: the REST body is
    `{"numbers":"*1","once":""}`; the API sends
    `/radius/monitor =numbers=*1 =once=`. The reply decodes into
    `RadiusStats`.
  - `TestDynamicQueuesAsksForProplist`: REST query `.proplist=name,target,rate`
    plus `dynamic=true`; API `=.proplist=name,target,rate`.
  - `TestInterfacesFiltersDynamic`: REST query `dynamic=false`; API
    `?dynamic=false`.
  - `TestDefaultRouteFilter`: `dst-address=0.0.0.0/0` and `active=true`,
    with no `static` filter.
- [ ] **Step 2:** Run `go test ./internal/mikrotik/ -v` → FAIL.
- [ ] **Step 3: Implement.** Keep `api.go` ≤350 lines. Put the command
  helpers in a new file if needed.
- [ ] **Step 4:** Run `go test ./internal/mikrotik/ ./internal/services/ -v`
  → PASS, then `gofmt`/`vet` → clean.
- [ ] **Step 5: Commit** `feat(mikrotik): read PPP, pools, RADIUS, CPU cores and queues`.

### Task 3: Discovery guesses the role, finds the uplink, and maps BRAS ports

**Files:**
- Create: `backend/migrations/82_router_uplink.sql`
  (`uplink_interface_id uuid NULL REFERENCES router_interfaces(id) ON DELETE SET NULL`)
- Create: `backend/internal/services/router_discovery_role.go`, `_test.go`
- Create: `backend/internal/connectivity/snmp_ifnames.go`
- Modify: `backend/internal/services/router_discovery.go`,
  `router_poll.go` (a new `IfNameReader` field on `RouterPollService`,
  wired in the constructor and the worker)
- Modify: `backend/internal/models/mikrotik.go`
  (`UplinkInterfaceID *uuid.UUID json:"uplink_interface_id"`)

**Interfaces:**
- Consumes: `PPPoEServers`, `BGPConnections`, `DefaultRoute` (Task 2),
  and `SetRouterRole` (Task 1).
- Produces:
  - `guessRole(pppoe []mikrotik.PPPoEServer, bgp []mikrotik.BGPConnection) models.RouterRole`
    - any PPPoE server → bras
    - else any connection with `disabled` false → border
    - else other
  - `uplinkFor(routes []mikrotik.Route, ifaces []models.RouterInterface) *uuid.UUID`
  - `connectivity.WalkIfNames(ctx, host, port, community string) (map[string]int, error)`:
    walks ifName only, with 50 repetitions per GETBULK, and returns
    name → ifIndex
  - `IfNameReader` with the same signature

Behaviour, all inside discovery's existing transaction:
- `role_guess` is always written.
- `role` follows the guess through `SetRouterRole(..., byAdmin=false)`
  only while `role_set_by_admin` is false, and only when both reads
  succeeded.
- For a BRAS only:
  - `uplink_interface_id` is written. A failed `DefaultRoute` keeps the
    old value.
  - After the upsert, `WalkIfNames` runs under a 30 s context and
    writes `if_index` for every stored interface it names. A failed walk
    keeps the stored indexes and does not fail discovery.

- [ ] **Step 1: Write failing tests.**
  - `TestGuessRole`, table-driven:
    - PPPoE + BGP → bras
    - BGP only → border
    - disabled BGP only → other
    - nothing → other
  - `TestUplinkFor`:
    - gateway `192.0.2.1` with interface addresses `192.0.2.2/30` on
      `sfp-sfpplus1` → that interface's id
    - gateway `sfp-sfpplus2` (an interface name) → that id
    - gateway `192.0.2.1%vlan-10` → the vlan-10 id
    - no route → nil
  - `TestDiscoverWritesGuessUnlessAdminSet`:
    - With a fake API returning a PPPoE server, a border router that
      the admin has not set becomes bras, and its jobs become the bras
      set.
    - With `role_set_by_admin=true`, the role stays border and
      `role_guess=bras`.
  - `TestDiscoverKeepsRoleWhenPPPoEReadFails`: the read errors, the role
    is unchanged, and discovery returns nil.
  - `TestDiscoverMapsBRASIfIndexes`: the fake IfNameReader returns
    `{"sfp-sfpplus1":3}`, so the stored row gets `if_index=3`. With a
    reader error, discovery still succeeds and the index is unchanged.
- [ ] **Step 2:** Run `go test ./internal/services/ -run 'GuessRole|UplinkFor|Discover' -v`
  → FAIL.
- [ ] **Step 3: Implement.** Border discovery must not call
  `DefaultRoute` or `WalkIfNames`; assert this with the fake's call log
  in `TestDiscoverBorderSkipsBRASReads`.
- [ ] **Step 4:** Run `go test ./... ` in backend → PASS, then
  gofmt/vet.
- [ ] **Step 5: Commit** `feat(routers): guess the role and find a BRAS's uplink at discovery`.

### Task 4: BRAS interfaces poll by targeted GET

**Files:**
- Create: `backend/internal/connectivity/snmp_ifget.go`
- Create: `backend/internal/services/router_poll_bras_ifaces.go`, `_test.go`
- Modify: `backend/internal/services/router_poll.go` (`PollInterfaces`
  branches on role; the branch is the only line added there)

**Interfaces:**
- Produces:
  - `connectivity.GetIfCounters(ctx, host string, port int, community string, indexes []int) ([]IfCounters, error)`:
    GETs ifName, HCIn, HCOut, HighSpeed and OperStatus for each index,
    packed many varbinds per PDU. An index the agent reports as
    noSuchInstance is omitted.
  - `IfCounterGetter` with that signature, as a field on
    `RouterPollService`.
- Consumes: `if_index` written by Task 3.

Behaviour for role bras:
- Only enabled stored interfaces with `if_index` are asked for.
- A returned name different from the stored name, or an interface
  without `if_index`, gets no sample and no update. The `discovery` job
  is made due now: one UPDATE on `router_poll_jobs`, not a dependency
  on `RouterPollJobService`.
- Only a failed GET marks the router down.
- Health runs after the GETs, as today.

- [ ] **Step 1: Write failing tests** in `router_poll_bras_ifaces_test.go`:
  - `TestBRASPollAsksOnlyMappedPorts`: 2 stored interfaces with indexes
    3 and 7, plus 1 without an index. The getter receives `[3 7]`. Both
    are sampled on the second poll. The walker is never called.
  - `TestBRASPollSkipsRenamedIndex`: the getter returns name `<pppoe-x>`
    at 7. Interface 7 gets no sample, the router stays up, and the
    discovery job's `due_at` is ≤ now.
  - `TestBRASPollGetFailureMarksDown`: the getter errors, so the status
    is down and the job errors.
  - `TestBorderPollStillWalks`: a border router uses `readCounters`, and
    the getter is not called.
- [ ] **Step 2:** Run `go test ./internal/services/ -run 'BRASPoll|BorderPollStillWalks' -v`
  → FAIL.
- [ ] **Step 3: Implement.** Reuse `recordCounters` unchanged.
- [ ] **Step 4:** Run `go test ./... ` → PASS, then gofmt/vet.
- [ ] **Step 5: Commit** `feat(routers): poll a BRAS's known ports by GET instead of walking every session`.

### Task 5: The `bras` job: snapshot and samples

**Files:**
- Create: `backend/migrations/83_router_bras.sql`
- Create: `backend/internal/services/router_poll_bras.go`, `router_bras_snapshot.go`, `_test.go` files
- Create: `backend/internal/models/router_bras.go`
- Modify: `backend/internal/services/router_poll.go`: factor the
  resource write (`resourceUpdates` + `api_status` ok) into one shared
  function. PollBGP keeps calling it.
- Modify: `backend/cmd/worker/router_jobs.go` (`case bras → PollBRAS`)

**Interfaces:**
- Produces:
  - `(*RouterPollService).PollBRAS(router *models.MikrotikRouter, now time.Time) error`
  - `models.RouterBRASSample{Time, RouterID, Sessions int, CPUMax, CPUAvg int16, RadiusAccepts, RadiusRejects, RadiusTimeouts int64, PoolMinFree *int}`,
    table `router_bras_samples`, PK (router_id, time)
  - `models.MikrotikRouter`: `BRAS datatypes.JSON gorm:"type:jsonb" json:"bras"`,
    `BRASAt *time.Time json:"bras_at"`
  - Snapshot JSON (`services.BRASSnapshot`), camel-cased by humps on the
    frontend:
    ```
    {sessions, cores:[{cpu,load}], pools:[{name,used,total|null,free|null}],
     radius:[{id,address,requests,accepts,rejects,timeouts,pending,rttMs|null,answering|null}],
     topUsers:[{name,downBps,upBps}], queuesFound: bool}
    ```
    In Go the struct fields carry snake_case json tags, like the rest of
    the API.

Migration 83 creates `router_bras_samples` (FK to `mikrotik_routers` ON
DELETE CASCADE), then runs
`create_hypertable('router_bras_samples','time', if_not_exists => TRUE)`
and `add_retention_policy(... INTERVAL '30 days')`, following
`56_mikrotik_monitoring.sql`. It also adds the `bras` and `bras_at`
columns.

Behaviour:
- Reads run in the spec's order. The resource read and its write come
  first, so `api_status` behaves as in PollBGP. Any read error goes
  through `recordAPIError` and fails the job.
- Pools: when `total` is empty, fall back to `PoolUsedCount` per pool,
  with the total parsed from IPv4 `ranges`
  (`a.b.c.d-a.b.c.e`, comma-separated, single addresses allowed).
  Anything unparseable makes the total null.
- RADIUS: no entries means an empty list. Each server's `answering` is
  computed against the previous snapshot's counters for the same id:
  - requests grew while accepts and rejects did not, and timeouts grew
    → false
  - requests grew and accepts or rejects grew → true
  - no new requests → the previous verdict
  - no previous snapshot → null

  `rttMs` is parsed from RouterOS durations (`12ms`, `1s200ms`) with
  `mikrotik.ParseDuration`.
- Top users: `rate` is "up/down" in bps. Keep the 10 highest by down.
  `queuesFound` is false when there are no dynamic queues.
- One sample row a minute: RADIUS totals summed over servers,
  `pool_min_free` over pools with a known total (null if none), and
  CPU max and average over cores.

- [ ] **Step 1: Write failing tests:**
  - `TestPollBRASWritesSnapshotAndSample`: a fake API with 2 cores
    (40, 90), 2 pools (one with a known total, one without), 1 RADIUS
    server, and 12 queues. Asserts:
    - the sample: cpu_max 90, cpu_avg 65, pool_min_free from the
      known pool
    - the snapshot: 10 top users sorted by down, `api_status` ok, and
      `board_name` written
  - `TestPollBRASNoRadius`: no entries gives `radius: []` and zero
    totals. (Review Focus 1.)
  - `TestPoolTotalFromRanges`, table-driven:
    - `10.0.0.1-10.0.0.254` → 254
    - `"10.0.0.1-10.0.0.10,10.0.1.1"` → 11
    - `garbage` → nil
  - `TestRadiusAnsweringVerdict`, table-driven over the four rules
    above.
  - `TestPollBRASReadErrorFailsJob`: `PPPActiveCount` errors, so the job
    errors and no sample is written.
  - In `cmd/worker` (or the services dispatch test, if there is one):
    the `bras` kind dispatches to PollBRAS.
- [ ] **Step 2:** Run `go test ./internal/services/ -run 'PollBRAS|PoolTotal|RadiusAnswering' -v`
  → FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4:** Run `go test ./... ` and `go test -race ./internal/services/`
  → PASS, then gofmt/vet.
- [ ] **Step 5: Commit** `feat(routers): poll a BRAS's sessions, pools, RADIUS, CPU cores and top users`.

### Task 6: `GET /api/v1/routers/:id/bras`

**Files:**
- Create: `backend/internal/services/router_bras_query.go`, `_test.go`
- Create: `backend/internal/api/router_bras_handler.go`, `_test.go`
- Modify: `backend/internal/api/router.go`: register next to the other
  router reads, with the same auth middleware

**Interfaces:**
- Produces:
  - `services.BRASView{Snapshot json.RawMessage, At *time.Time, Samples []models.RouterBRASSample, Sessions15mAgo *int, RadiusRejects1h, RadiusTimeouts1h *int64, Drops []BRASDrop}`
  - `BRASDrop{Time time.Time, Lost int}`
  - `(*RouterService).BRASView(id uuid.UUID, from, to time.Time) (*BRASView, error)`:
    `ErrNotFound` for a missing router or a non-bras one
  - The query takes `from`/`to` (RFC3339). It defaults to the last 24 h
    and clamps the range at 31 days.

- [ ] **Step 1: Write failing tests** (services):
  - `TestBRASDrops`:
    - samples 2000 → 1880 (−120 ≥ max(100, 100)) → drop of 120
    - 2000 → 1950 → no drop
    - 3000 → 2860 (−140 < 150, which is 5%) → no drop
    - a gap of 3 min → no drop (Review Focus 5)
  - `TestSessions15mAgo`: the nearest sample within ±2 min, else nil.
  - `TestRadius1h`: the latest total minus the total about 60 min
    earlier. A negative difference gives nil (Review Focus 5). No
    sample about 60 min earlier gives nil.
  - `TestBRASViewRejectsBorder`: a border router gives ErrNotFound.
  - Handler: 200 with the fields for a bras router, 404 for a border
    one, 401 without a session.
- [ ] **Step 2:** Run the tests → FAIL.
- [ ] **Step 3: Implement.** "About 60 min earlier" means the sample
  nearest to latest−60 min, within ±2 min. It is computed from samples
  read for that purpose, not from the requested range.
- [ ] **Step 4:** Run `go test ./... ` → PASS, then gofmt/vet.
- [ ] **Step 5: Commit** `feat(routers): serve a BRAS's samples, drops and hourly RADIUS counts`.

### Task 7: Frontend role: entity, form, rack tape, tabs by role, uplink colour

**Files:**
- Modify: `frontend/src/domain/entities/MikrotikRouter.ts`,
  `test/mikrotikRouterFixture.ts`
- Modify: `RouterFormFields.tsx`, `RouterFormModal.tsx`,
  `useRouterTestSteps.ts`, `RouterFormModal.test.tsx`
- Modify: `faceplate/RackUnit.tsx`, `rack.css`, `RouterRack.test.tsx`
- Modify: `pages/RoutersPage.tsx` (summary), `pages/RouterDetailPage.tsx`,
  and both page tests
- Modify: `RouterInfoTab.tsx`, `faceplate/portModel.ts` (uplink → upstream)
- Create: `frontend/src/presentation/components/routers/routerRole.ts`
  (labels, tab keys per role)

**Interfaces:**
- Produces:
  - `type RouterRole = "border" | "bras" | "other"`
  - On `MikrotikRouter`: `role`, `roleSetByAdmin`,
    `roleGuess: RouterRole | null`, `bras: BrasSnapshot | null`,
    `brasAt: string | null`, `uplinkInterfaceId: string | null`
  - `BrasSnapshot` mirrors Task 5's JSON in camelCase
  - `ROLE_LABEL: Record<RouterRole,string>`: Border, BRAS, Lainnya
  - `tabKeysFor(role): string[]`
  - UpdateRouterDto: `role?`, `resetRole?`; CreateRouterDto: `role?`
- Consumes: Task 1 and Task 3 JSON.
- In this task, `tabKeysFor("bras")` returns
  `["panel","interfaces","router"]`. Task 8 adds `ringkasan` and
  `trafik`.

Behaviour:
- **Form.** "Peran" Radio.Group: Otomatis / Border / BRAS / Lainnya.
  - The initial value is "Otomatis" when `!roleSetByAdmin`, otherwise
    the role.
  - Saving "Otomatis" sends `resetRole: true`. Saving any other value
    sends `role`.
  - Help text: "Tebakan TikMan: <label>", or "Belum ditebak" when
    `roleGuess` is null.
  - On create the field shows with Otomatis selected. Otomatis sends no
    role.
- **Rack.** The role tape goes next to the name tape, with class
  `fp-roletape` plus a role modifier: yellow `--fp-tape-upstream`
  BORDER, blue `--fp-tape-mgmt` BRAS, white `--fp-tape-dist` LAINNYA.
  On a BRAS unit:
  - the detail line shows `<n> online` (from `bras.sessions`) and CPU
  - `useRouterBgpSessions` is not called (pass `enabled:false` or skip
    by role)
  - BGP-dead issues are left out
- **Summary.** `· <n> pelanggan online`, summed over BRAS snapshots, is
  added only when at least one BRAS exists. The BGP part counts only
  non-bras routers.
- **Router tab.** Shows "Peran" with "(tebakan TikMan)" when
  `!roleSetByAdmin`.
- **Faceplate.** For a BRAS, the port of `uplinkInterfaceId` (or that
  VLAN's parent) is role upstream. Border colouring is unchanged.

- [ ] **Step 1: Write failing tests:**
  - RouterFormModal: Otomatis sends `{resetRole:true}`; choosing BRAS
    sends `{role:"bras"}`; the help text shows the guess.
  - RouterRack: a BRAS unit shows the "BRAS" tape and "1.842 online",
    and does not request BGP sessions.
  - RoutersPage: the summary with one border and one BRAS reads
    `"2 router · 2 terjangkau · BGP 3/4 established · 1.842 pelanggan online"`.
  - RouterDetailPage: a bras router has no "Upstream & BGP" tab; a
    border router keeps the same six tabs as today.
  - portModel: the BRAS uplink port gets role upstream; a border
    router's ports are unchanged against the existing fixtures.
- [ ] **Step 2:** Run `cd frontend && npm test -- --run` → the new tests
  FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4:** Run `npm test -- --run && npm run lint && npm run format:check && npm run build`
  → all pass.
- [ ] **Step 5: Commit** `feat(routers): show and edit each router's role`.

### Task 8: Frontend BRAS Ringkasan and Trafik tabs

**Files:**
- Create in `frontend/src/presentation/components/routers/bras/`:
  - `BrasSummaryTab.tsx`
  - `BrasSignals.tsx`
  - `BrasSessionsChart.tsx`
  - `BrasPools.tsx`
  - `BrasRadius.tsx`
  - `BrasTopUsers.tsx`
  - `BrasUplink.tsx`
  - `BrasTrafficTab.tsx`
  - `brasSignals.ts`
  - `bras.css`
  - tests
- Modify: `MikrotikRouterRepository.ts`, `endpoints.ts`,
  `useMikrotikRouters.ts` (or a new `useRouterBras.ts`),
  `routerRole.ts` (bras tabs: ringkasan, panel, trafik, interfaces,
  router), and `RouterDetailPage.tsx`

**Interfaces:**
- Consumes: Task 6's endpoint and Task 7's `BrasSnapshot` and
  `uplinkInterfaceId`.
- Produces:
  - `useRouterBras(routerId, range)`, following the `trafficQuery`
    pattern, refetching every 60 s
  - `brasSignals(view, now): {sessions, radius, cpu, pool}`, each
    `{level: "ok"|"warn"|"bad"|"none", value: string, context: string}`

Behaviour: the spec's Frontend section, with these rules from it:
- **Pelanggan.** bad if any drop is within 15 min of `brasAt`. Context
  "15 menit lalu N", or "—" when null.
- **RADIUS.**
  - none (grey), with value "Tidak dipakai", when the list is empty
  - bad when every server is answering=false
  - warn when some are false, or timeouts1h > 0, or
    rejects1h > 10% of the accepts delta over the same hour
  - context "N login ditolak dalam 1 jam"
- **CPU.** Value = max core; warn ≥70, bad ≥90. Context "rata-rata N
  core X%".
- **Pool.** Value = the fewest free addresses; warn <10% free, bad <2%.
  "—" with level none when no total is known.
- **Stale.** When `brasAt` is older than 5 min, every level becomes none
  and the row shows "Pembacaan terakhir <relative time>".
- **Chart.** Recharts LineChart over the samples' `sessions`, with
  `connectNulls={false}`. Insert a null point where two samples are
  more than 2 min apart. Each drop gets a ReferenceDot and the label
  `"HH:mm turun N sesi"`.
- **Top users.** Username with AntD `Typography.Text copyable`,
  download, upload. When `!queuesFound`: "Tidak ada antrean dinamis:
  batas kecepatan tidak diberikan oleh RADIUS atau profil."
- **Uplink.** The interface's current `lastInBps`/`lastOutBps`/`speedMbps`
  from `useRouterInterfaces`. Hidden when `uplinkInterfaceId` is null.
- **Trafik tab.** The UpstreamTab pattern over the single uplink
  interface. When there is no uplink: "Jalur ke border belum ditemukan.
  TikMan mencarinya dari default route saat discovery."

- [ ] **Step 1: Write failing tests:**
  - `brasSignals.test.ts`: one case per rule above, including Review
    Focus 1, 3 and 5, and stale.
  - `BrasSessionsChart.test.tsx`: a 3-minute gap produces a null point;
    a drop label renders.
  - `BrasTopUsers.test.tsx`: the copy button exists, and the
    no-queues text shows.
  - `RouterDetailPage.test.tsx`: a bras router opens on "Ringkasan".
- [ ] **Step 2:** Run `npm test -- --run` → FAIL.
- [ ] **Step 3: Implement.** Match the approved mockup's order and copy
  (scratchpad `bras-mock/index.html`, summarised in the spec). Use AntD
  and the faceplate LED colours from `faceplate.css`. Add no new
  dependency.
- [ ] **Step 4:** Run `npm test -- --run && npm run lint && npm run format:check && npm run build`
  → pass. Then look at the tab in the dev server with mocked API data
  through the Chrome extension, at 1440 px and 390 px.
- [ ] **Step 5: Commit** `feat(routers): BRAS summary and uplink traffic tabs`.

---

After Task 8: run the whole-branch review, then ask the user before
merging and deploying. Migrations 81–83 mean the full restart order.
After deploy, the user registers the BRAS, and the spec's "verify on the
real BRAS" list is checked against its data.
