# Router Health LEDs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Make the faceplate's PWR and FAULT LEDs real. Read the router's health sensors over SNMP and show PSU state, faults, temperatures, fans and power on the rack and the router detail.

**Architecture:** The 1-minute router interface job (`RouterPollService.PollInterfaces`) also walks MIKROTIK-MIB `mtxrGaugeTable`. That table holds the same items as `/system health print`, by name. The latest reading is stored on the router row as jsonb. The frontend decodes it into LEDs, a status line and a readings strip.

**Spec (short, operator-approved 2026-11-19):** The first faceplate dropped PWR1/PWR2/FAULT because TikMan read no power data, so they would have been decoration. The operator asked to have them back, with real data. SNMP was chosen over the RouterOS API so health keeps updating when the API is failing. It uses the same channel as the router's reachability.
- Every LED shows real data or is off.
- An unreachable router shows every LED off. It never shows the last known state as live.

## Facts (verified, do not re-derive)

- `mtxrGaugeTable` = `.1.3.6.1.4.1.14988.1.1.3.100.1`. Columns: `.2` name (DisplayString, e.g. `cpu-temperature`, `fan1-speed`, `psu1-state`, `psu2-voltage`, `power-consumption`), `.3` value (Integer32), `.4` unit (enum).
- Unit enum: 1 celsius, 2 rpm, 3 dV, 4 dA, 5 dW, 6 status.
  - dV, dA and dW are tenths: value 121 = 12.1 V.
  - For status: 0 = ok, 1 = fail. This is LibreNMS's production mapping for RouterOS (`resources/definitions/os_discovery/routeros.yaml`, `mtxrGaugeStatus`).
- Not every model has every item. An RB5009 has no PSU state. A CCR2116 has `psu1-state` and `psu2-state`. Never assume a fixed set.

## Global Constraints

- CLAUDE.md:
  - Files ≤ 350 lines and functions ≤ 50 lines. `snmp_*.go` files that construct gosnmp clients are exempt.
  - Comments explain why. No new dependencies.
  - The repo is PUBLIC: tests use generic hosts (192.0.2.x) and names.
- UI copy is Indonesian:
  - LED labels: "STATUS", "PWR1", "PWR2", …, "FAULT".
  - Rack issue: "PSU{n} fail", plus "{name} fail" for any other failed status item.
  - Readings strip heading: "Kesehatan".
  - No data: "Belum ada data kesehatan".
- Units on screen: °C, RPM, V, A, W, with one decimal for V/A/W.
- `faceplate.css` is at 350 lines. New rules go in a new `health.css`, or in `rack.css` for rack-only rules.

## Review Focus

1. A router whose gauge walk fails, or whose model has no gauge table: the interface poll still succeeds, the router stays "up", and the UI shows no PWR LEDs and "Belum ada data kesehatan". It never shows a fake "ok".
2. A board with only `psu2-state` failing: PWR2 red, FAULT red, rack line "PSU2 fail".
3. Offline router: STATUS, PWR and FAULT all off, and the readings strip is marked as last read (with its time).
4. Stale health (`health_at` older than 5 minutes while the router is up): treat the reading as missing for the LEDs, so a stuck reading cannot look healthy.
5. Unknown unit codes or unexpected names are listed raw in the strip, never dropped or crashed on.

---

### Task 1: Backend — walk, store, serve

**Files:**
- Create: `backend/internal/connectivity/snmp_gauges.go`, modelled on `snmp_ifxtable.go`: `type Gauge struct { Name string; Value int64; Unit int }`, `func WalkGauges(ctx context.Context, host string, port int, community string) ([]Gauge, error)`, ordered by index.
- Create: `backend/migrations/80_router_health.sql`, adding `health jsonb` and `health_at timestamptz` (nullable) to `mikrotik_routers`, with a why-comment.
- Modify: `backend/internal/models/mikrotik.go`:
  - `Health datatypes.JSON \`gorm:"type:jsonb" json:"health"\``, serialised as `[{"name":…,"value":…,"unit":…}]`
  - `HealthAt *time.Time \`json:"health_at"\``
- Modify: `backend/internal/services/router_poll.go`:
  - new field `readGauges GaugeReader` (type `func(ctx, host, port, community) ([]connectivity.Gauge, error)`), wired to `connectivity.WalkGauges` in `NewRouterPollService`
  - after the counters succeed, `PollInterfaces` reads gauges within the same `snmpWalkBudget`
  - on success it writes `health` and `health_at` in the same transaction
  - on failure, or with zero rows, it leaves `health` untouched, logs at debug, and does NOT fail the poll or mark the router down
- Tests: `backend/internal/services/router_poll_test.go`, using the existing fake reader pattern:
  - `TestPollInterfacesStoresHealth`: 3 gauges are stored, `health_at` == now
  - `TestPollInterfacesKeepsPollingWhenHealthFails`: the gauge reader errors, the poll returns nil, status stays up, health is unchanged
  - `TestPollInterfacesIgnoresEmptyHealth`: zero gauges leave the previous health in place
- [ ] TDD: write the tests, see them fail, implement, see them pass. Run `go test ./internal/...`, `go vet ./...` and `gofmt -s -l .` (empty). Commit `feat(routers): read health sensors over SNMP`.

### Task 2: Frontend — decode and draw

**Files:**
- Modify: `frontend/src/domain/entities/MikrotikRouter.ts`: `export interface RouterHealthItem { name: string; value: number; unit: number }`, plus `health: RouterHealthItem[] | null; healthAt: string | null` on `MikrotikRouter`.
- Create: `frontend/src/presentation/components/routers/faceplate/healthModel.ts` (pure) + test:
  ```ts
  export interface HealthReading { name: string; label: string; text: string; failed: boolean }
  export interface HealthView { fresh: boolean; psus: { n: number; ok: boolean }[]; failures: string[]; readings: HealthReading[] }
  export function healthView(r: MikrotikRouter, now: Date): HealthView
  ```
  - `fresh` is `healthAt` within 5 minutes and `routerReachable(r)` (from `portModel`).
  - `psus` comes from items matching `/^psu(\d+)-state$/` with unit 6, sorted by n.
  - `failures` holds every status item with value 1: "PSU{n}" for psu states, otherwise the item name.
  - `readings` covers every item:
    - Labels: `cpu-temperature` → "Suhu CPU", `board-temperature{n}` → "Suhu board {n}", `fan{n}-speed` → "Kipas {n}", `psu{n}-voltage` → "PSU{n} tegangan", `power-consumption` → "Daya". Anything else keeps its raw name.
    - Text by unit: 1 "43 °C", 2 "5654 RPM", 3/4/5 value/10 with one decimal plus V/A/W, 6 "ok"/"fail". An unknown unit shows the raw value.
  - Tests:
    - CCR-like set with psu2 fail: psus `[{1,true},{2,false}]`, failures `["PSU2"]`, and "12.1 V" for 121 dV.
    - RB5009-like set with no psu: psus `[]`.
    - Stale `healthAt` makes `fresh` false.
    - Unreachable router makes `fresh` false.
    - Unknown unit 9 is kept raw.
- Modify: `Chassis.tsx`. Replace the single status LED with a row:
  - STATUS: green when reachable.
  - One PWR{n} per `psus` entry: green ok, red fail.
  - FAULT: red when `failures.length > 0`, otherwise dark.
  - Everything is off when not fresh. With no PSU items, no PWR LEDs are drawn.
  - The row's `title` lists the readings ("Suhu CPU 43 °C · Kipas 1 5654 RPM · …").
  - New prop `health: HealthView`. Callers pass `healthView(router, new Date())`.
  - Keep the function and file limits. LED styles go in `health.css`.
  - Tests:
    - PWR2 red and FAULT red for psu2 fail.
    - No PWR LEDs for an RB5009-like set.
    - All off when not fresh.
- Modify: `RackUnit.tsx`. `unitStatus` appends "PSU2 fail" (and other failures) to the issues when fresh. Test.
- Modify: `RouterFaceplate.tsx` (the "Panel depan" tab). Under the stage, add a "Kesehatan" strip listing every reading, with failed ones in red. Without health it shows "Belum ada data kesehatan". When not fresh, it shows "Pembacaan terakhir {DD MMM HH:mm}" and is dimmed. Test.
- [ ] TDD per file. Run `npm test -- --run && npm run lint && npm run format:check && npm run build`. Commit `feat(routers): power and fault LEDs from the router's health sensors`.
