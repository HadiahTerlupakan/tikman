# Router Rack and Faceplate Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Show `/routers` as a rack of MikroTik front panels with live port state, and put a labelled faceplate above the router detail tabs.

**Architecture:** One new nullable column (`rack_position`) and its form field. Everything else is frontend. Pure modules decide the port layout, the per-port view model and the rack order. React components draw the chassis, the hanging labels and the rack from the existing per-router reads.

**Tech Stack:** Go/GORM/Gin, React 18 + TypeScript, Ant Design, React Query, Vitest + Testing Library.

**Spec:** `docs/superpowers/specs/2026-11-19-router-rack-faceplate-design.md`. Approved mockup (gitignored, invented data): `.superpowers/brainstorm/39339-1795087692/content/router-faceplate-v4.html`. Copy its geometry, colours and CSS. Do not copy its data.

## Global Constraints

- Files ≤ 350 lines and functions ≤ 50 lines, with no nesting deeper than 3 levels (CLAUDE.md).
- No new dependencies.
- UI copy is Indonesian, matching the mockup: "Posisi di rak", "+N VLAN lain", "Tidak ada sesi BGP lewat port ini.", "Tidak ada VLAN di port ini.", "Semua normal", "N port mati", "N port diam", "N sesi BGP mati", "Tidak terjangkau". The detail stays its own route (`/routers/:id`), so the mockup's in-page back button is not built.
- Colours (CSS custom properties): tape upstream `#f2d23a`, distribution `#f7f8f9` (plug `#e2e6ea`), management `#2f69b3`, trunk label `#3a4047`, LEDs up `#34d27b`, warn `#f3b437`, down `#ef4b4f`, off `#454b52`.
- The faceplate CSS needs pseudo-elements and a keyframe, so it lives in one `faceplate.css` imported by the components. That is the one exception to the inline-style pattern, and the reason belongs in a one-line comment at the import.
- The blinking LED respects `prefers-reduced-motion`.
- The repo is public: tests use generic names (`peer-transit`, `border-1`), never real peer names, IPs or AS numbers.

## Review Focus

1. **A router that was never discovered** (no interfaces): its unit still renders with an empty chassis and the line "Belum ada data interface". It never throws.
2. **`boardName` null or an unknown model**: falls back to the automatic layout.
3. **Wide boards** (a 48-port CRS): the rack scales every unit to one width, and the detail stage scrolls horizontally instead of overflowing the page.
4. **Non-port interfaces** (`bridge`, `vlan`, `wireguard`, `lte1`, `wlan1`): never drawn as jacks. Only `type === "ether"` interfaces without a `parentName` are physical.
5. **The 60 s refetch** must not reset the port selected in the detail panel.

---

### Task 1: Backend `rack_position`

**Files:**
- Modify: `backend/internal/models/mikrotik.go` (field after `FlowLastSeenAt`)
- Create: `backend/migrations/79_router_rack_position.sql`
- Modify: `backend/internal/services/router_service.go` (`RouterUpdate`, `updateColumns`)
- Modify: `backend/internal/api/router_dto.go` (`UpdateRouterRequest`, `toUpdate`)
- Test: `backend/internal/services/router_service_test.go`, `backend/internal/api/router_handler_test.go`

**Interfaces:**
- Produces: JSON `rack_position: number | null` on every router. PUT accepts `rack_position` (int, 1–99) and `clear_rack_position` (bool).

- [ ] **Step 1: Write failing tests**
  - `TestRouterUpdateSetsRackPosition`: update with `RackPosition: ptr(3)` gives `Get(id).RackPosition == 3`.
  - `TestRouterUpdateClearsRackPosition`: set 3, then `ClearRackPosition: true` gives nil.
  - `TestRouterUpdateRejectsRackPositionOutOfRange`: 0 and 100 give `errors.Is(err, ErrValidation)`.
  - Handler: PUT `{"rack_position": 2}` as admin answers 200 with `"rack_position":2` in the body.
- [ ] **Step 2:** `cd backend && go test ./internal/services/ ./internal/api/ -run RackPosition -v`. Expect FAIL (unknown field).
- [ ] **Step 3: Implement**
  - Model: `RackPosition *int \`json:"rack_position"\``, with a doc comment: the slot counted from the top of the rack, so the screen can match the physical rack.
  - Migration: `ALTER TABLE mikrotik_routers ADD COLUMN IF NOT EXISTS rack_position integer;`, with one line on why.
  - Service: `RackPosition *int` and `ClearRackPosition bool` on `RouterUpdate`, validated against consts `minRackPosition = 1` and `maxRackPosition = 99`. Follow the `SiteID`/`ClearSite` pattern.
  - DTO: `RackPosition *int \`json:"rack_position" binding:"omitempty,min=1,max=99"\``, `ClearRackPosition bool \`json:"clear_rack_position"\``.
- [ ] **Step 4:** Rerun the step 2 command (PASS), then `go test ./internal/... ` and `gofmt -s -l .` (empty).
- [ ] **Step 5:** Commit `feat(routers): store each router's rack position`.

### Task 2: Rack position in the router form

**Files:**
- Modify: `frontend/src/domain/entities/MikrotikRouter.ts` (`rackPosition: number | null` on `MikrotikRouter`; `rackPosition?: number; clearRackPosition?: boolean` on `UpdateRouterDto`)
- Modify: `frontend/src/presentation/components/routers/RouterFormFields.tsx` (InputNumber `name="rackPosition"`, label "Posisi di rak", min 1, max 99, `extra="Urutan dari atas di halaman MikroTik; kosongkan untuk mengurut menurut nama"`)
- Modify: `frontend/src/presentation/components/routers/RouterFormModal.tsx` (`initialValues` carries `rackPosition`; on edit, an empty field sends `clearRackPosition: true`)
- Test: `RouterFormModal.test.tsx`

- [ ] **Step 1: Failing tests**
  - Editing a router with `rackPosition: 2` shows 2 in "Posisi di rak".
  - Clearing that field and saving calls update with `clearRackPosition: true` and without `rackPosition`.
- [ ] **Step 2:** `cd frontend && npx vitest run src/presentation/components/routers/RouterFormModal.test.tsx`. Expect FAIL.
- [ ] **Step 3:** Implement as listed under Files. Strip `rackPosition: null` from the payload, because the backend binding rejects null against `min=1`.
- [ ] **Step 4:** Rerun the step 2 command (PASS).
- [ ] **Step 5:** Commit `feat(routers): set rack position in the router form`.

### Task 3: Port layout (pure)

**Files:**
- Create: `frontend/src/presentation/components/routers/faceplate/portLayout.ts`
- Test: `.../faceplate/portLayout.test.ts`

**Interfaces:**
- Produces:
  ```ts
  export type JackKind = "rj" | "sfp" | "qsfp";
  export interface PlacedJack { name: string; kind: JackKind; row: "top" | "bottom"; x: number; y: number; w: number; h: number; mgmt: boolean }
  export interface FaceplateLayout { templated: boolean; model: string; extras: boolean; jacks: PlacedJack[]; width: number }
  export const CHASSIS_HEIGHT = 132;
  export function physicalPortNames(interfaces: RouterInterface[]): string[]  // type "ether", no parentName, natural sort (ether2 < ether10)
  export function layoutFaceplate(boardName: string | null, portNames: string[]): FaceplateLayout
  ```
- Templates `CCR2116-12G-4S+` and `CCR2004-16G-2S+`, plus the automatic layout: geometry, pitches, row rule (index 0 top, 1 bottom), 44 px group gap, start at 166 with extras or 40 without, width `+230` templated or `+330` automatic. All copied from the mockup's `TEMPLATES`, `GEO` and `autoLayout`/`buildChassis`.
- A templated board draws its template's ports even when discovery has not yet reported them all, because empty jacks are real jacks.

- [ ] **Step 1: Failing tests**
  - CCR2116: 17 jacks, `templated` true, `ether1` has `mgmt` and row "bottom", `ether2` top and `ether3` bottom in the same column (same x), `sfp-sfpplus1` kind "sfp".
  - Unknown board `["ether1", "sfp28-1", "sfp28-2", "qsfp28-1-1"]`: `templated` false, 3 groups in order rj → sfp → qsfp, `qsfp28-1-1` kind "qsfp" (not "sfp").
  - `boardName` null behaves like an unknown board.
  - Empty port list: `jacks` empty, `width` > 0.
  - `physicalPortNames` keeps `ether1`, `sfp-sfpplus1` and `ether10` in natural order, and drops `vlan201` (has a parent), `bridge1`, `wg0` and `lte1`.
- [ ] **Step 2:** Run `npx vitest run src/presentation/components/routers/faceplate/portLayout.test.ts`. Expect FAIL.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** Rerun the step 2 command (PASS).
- [ ] **Step 5:** Commit `feat(routers): lay out a router's front panel from its board`.

### Task 4: Port view model (pure)

**Files:**
- Create: `frontend/src/presentation/components/routers/faceplate/portModel.ts`
- Test: `.../faceplate/portModel.test.ts`

**Interfaces:**
- Consumes: `interfaceLabel` from `../routerTraffic`; entities `MikrotikRouter`, `RouterInterface`, `RouterBgpSession`.
- Produces:
  ```ts
  export type PortRole = "upstream" | "dist" | "mgmt";
  export type PortState = "up" | "down" | "silent" | "disabled";
  export interface VlanView { iface: RouterInterface; label: string; role: PortRole; inBps: number; sessions: RouterBgpSession[] }
  export interface PortView {
    iface: RouterInterface; label: string; state: PortState;
    role: PortRole | "trunk" | null;      // null = unused, drawn dark
    roles: PortRole[];                    // plug stripes, one per distinct role
    vlans: VlanView[]; sessions: RouterBgpSession[];  // sessions = own + every VLAN's
    deadSessions: number; inBps: number; outBps: number;
  }
  export function routerReachable(r: MikrotikRouter): boolean  // enabled && status === "up"
  export function bgpStale(r: MikrotikRouter): boolean          // reachable && apiStatus !== "ok"
  export function buildPortViews(r: MikrotikRouter, interfaces: RouterInterface[], sessions: RouterBgpSession[]): Map<string, PortView>  // keyed by port name
  export function pollHost(r: MikrotikRouter): string            // host, for the management match
  ```
- Rules (spec "Decisions" plus the builder-check calls):
  - upstream: a session's `interfaceId` is this port or one of its VLANs.
  - mgmt: a port or VLAN `addresses` entry (comma-separated CIDRs) whose IP equals `r.host`.
  - dist: anything else that is not disabled and has a comment or is oper-up.
  - Port with VLANs: role "trunk" when its VLANs hold more than one distinct role, else that role. `roles` lists the distinct roles.
  - state: `disabled` if disabled, `down` if operStatus is "down", `silent` if `silentSince`, else `up`.

- [ ] **Step 1: Failing tests**, each a fixture with generic names:
  - Session on `sfp-sfpplus1`: role upstream, `sessions.length` 1.
  - Sessions on `vlan201` (parent `sfp-sfpplus2`) plus `vlan101` with nothing: the port is "trunk", `roles` is `["upstream", "dist"]`, and the VLAN views carry their own roles.
  - A non-established session behind a VLAN counts in the port's `deadSessions`.
  - Port address `192.0.2.1/24` with `host` "192.0.2.1" gives mgmt. With `host` "border.example" nothing is mgmt.
  - Disabled port with no comment: role null. A port with `silentSince`: state "silent".
  - `routerReachable` is false for `status` "unknown", for "down" and for `enabled: false`. `bgpStale` is true for up with `apiStatus` "error".
  - A session whose `interfaceId` matches no port (bridge or loopback) adds no role.
- [ ] **Step 2:** Run the test. Expect FAIL.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** Rerun the test (PASS).
- [ ] **Step 5:** Commit `feat(routers): derive each port's role and state`.

### Task 5: Rack order (pure)

**Files:**
- Create: `.../faceplate/rackOrder.ts`, test `.../faceplate/rackOrder.test.ts`

**Interfaces:**
- Produces: `export type RackSlot = { kind: "router"; router: MikrotikRouter; u: number } | { kind: "blank"; u: number }; export function rackSlots(routers: MikrotikRouter[]): RackSlot[]`
- Positioned routers sit at their own U, with gaps filled by blanks. Unpositioned routers follow the highest occupied U, sorted by name. Two routers on one U: the first by name keeps it, the other moves to the end of the unpositioned list.

- [ ] **Step 1: Failing tests**
  - Positions 1 and 3 plus one unpositioned router give `[router u1, blank u2, router u3, router u4]`.
  - With nothing positioned, routers are in name order from u1.
  - A clash on U 2: the second router lands after everything else.
- [ ] **Steps 2–5:** Run (FAIL), implement, rerun (PASS), commit `feat(routers): order routers into rack slots`.

### Task 6: Chassis component

**Files:**
- Create: `.../faceplate/Chassis.tsx`, `.../faceplate/faceplate.css`, test `.../faceplate/Chassis.test.tsx`

**Interfaces:**
- Consumes: `FaceplateLayout`, `PortView`.
- Produces: `export function Chassis(props: { layout: FaceplateLayout; ports: Map<string, PortView>; offline: boolean; selected?: string; onSelect?: (name: string) => void }): JSX.Element`. The root is `<div className="fp-chassis">`, sized `layout.width` × `CHASSIS_HEIGHT`, with ears, console and USB when `layout.extras`, silkscreen numbers, the model (plus " · susunan otomatis" when not templated), and one status LED that is lit when `!offline`.
- A used port is a `<button>` with `aria-label="{name}, {label}, {state text}"`, plus `title` when `onSelect` is absent (rack hover). An unused port is a `<div aria-hidden>`. LEDs and the activity blink follow the mockup: blink period `max(0.12, 1.2 - log10(inBps)/9)` s, only when `inBps > 1e6`. `offline` turns every LED off.
- CSS: port the mockup's chassis, jack, plug, LED and num rules under an `fp-` prefix.

- [ ] **Step 1: Failing tests**
  - Renders a button for each used port and none for unused ones.
  - `offline` renders no element with class `fp-led-up`.
  - Clicking a port button calls `onSelect("sfp-sfpplus1")`.
  - The automatic layout shows "susunan otomatis".
- [ ] **Steps 2–5:** Run (FAIL), implement, rerun (PASS), commit `feat(routers): draw a router's front panel`.

### Task 7: Detail faceplate with labels and port panel

**Files:**
- Create: `.../faceplate/PortLabels.tsx` (label lanes and the SVG cable layer), `.../faceplate/PortDetailPanel.tsx`, `.../faceplate/RouterFaceplate.tsx`, test `.../faceplate/RouterFaceplate.test.tsx`
- Modify: `frontend/src/presentation/pages/RouterDetailPage.tsx` (render `<RouterFaceplate router={router} />` between the Card title and `<Tabs>`)

**Interfaces:**
- Consumes: `useRouterInterfaces`, `useRouterBgpSessions`, `useInterfaceTraffic(routerId, interfaceId, { kind: "relative", hours: 24 })`, and `TrafficChart`, all existing.
- `RouterFaceplate` keeps the selected port name in state. It defaults to the first trunk, else the first used port, and is not reset by a refetch (Review Focus 5).
- Labels go in two lanes (top-row ports above, the rest below), left to right with a 10 px gap, joined to their port by an elbow cable. Layout is measured with `useLayoutEffect` exactly as the mockup's `render()` does. A trunk label lists its first three VLANs, then "+N VLAN lain", with a red dot on a VLAN that has a dead session.
- The panel shows the label, the port name, comment, state, speed, in, out, utilisation, the 24 h inbound chart, a BGP table (Sesi, Lewat, Prefix, Status) and a VLAN table (VLAN, Nama, Masuk), with the empty copy from Global Constraints. When `bgpStale`, show an Alert "Data BGP usang: API router sedang gagal".
- The stage wrapper has `overflow-x: auto` (Review Focus 3). With no physical ports, show "Belum ada data interface" (Review Focus 1).

- [ ] **Step 1: Failing tests** (mock the hooks the way existing router tests do)
  - A trunk with 5 VLANs shows 3 VLAN rows and "+2 VLAN lain".
  - Clicking the label "peer-transit" shows its sessions in the panel.
  - After the query data changes (rerender with new data), the selected port stays selected.
  - With no interfaces, the text "Belum ada data interface" is shown.
- [ ] **Steps 2–5:** Run (FAIL), implement, rerun (PASS), commit `feat(routers): faceplate with port labels on the router page`.

### Task 8: Rack on `/routers`

**Files:**
- Create: `.../faceplate/RouterRack.tsx` (cabinet and rows), `.../faceplate/RackUnit.tsx` (one unit, which fetches its own interfaces and sessions)
- Modify: `frontend/src/presentation/pages/RoutersPage.tsx` (Card title "MikroTik", the summary line, and `<RouterRack>` in place of the Table, keeping "Tambah router" and the edit modal)
- Test: update `frontend/src/presentation/pages/__tests__/RoutersPage.test.tsx`, create `.../faceplate/RouterRack.test.tsx`

**Interfaces:**
- Consumes: `rackSlots`, `Chassis`, `buildPortViews`, `routerReachable`, `bgpStale`.
- `RouterRack` props: `{ routers: MikrotikRouter[]; isAdmin: boolean; onEdit: (r: MikrotikRouter) => void }`. Clicking a unit navigates to `/routers/{id}`.
- Every unit scales to one shared width: the widest layout among the units, set on every chassis, with `scale = unitWidth / sharedWidth` from a `ResizeObserver`. The name tape sits bottom-right.
- The info column shows the name, then the status line, then "model · CPU n% · BGP x/y" (BGP reads "BGP usang" when `bgpStale`), then "Ubah" for admins only. Status line:
  - unreachable: "Tidak terjangkau" plus the `RouterStatusTag` text;
  - otherwise the joined issues ("N port mati", "N port diam", "N sesi BGP mati"), or "Semua normal".
- The summary above reads: "{n} router · {m} terjangkau[, {k} tidak terjangkau] · BGP {est}/{total} established", using `bgpEstablished`/`bgpTotal` from the list.

- [ ] **Step 1: Failing tests**
  - Admin sees "Ubah" for each router, and clicking it opens the form for that router (the existing assertion, kept).
  - A technician sees no "Ubah".
  - Routers at positions 1 and 3 render a blank slot between them (`data-testid="rack-blank"`).
  - An unreachable router shows "Tidak terjangkau" and no lit LEDs.
  - Clicking a unit navigates to `/routers/r-1`.
- [ ] **Steps 2–5:** Run (FAIL), implement, rerun (PASS), commit `feat(routers): show routers as a rack`.

### Task 9: Whole-branch verification

- [ ] `cd backend && go test ./... -race && go vet ./... && gofmt -s -l .` (all clean). The ppp0 WireGuard test failures are environmental; see memory.
- [ ] `cd frontend && npm test -- --run && npm run lint && npm run format:check && npm run build`
- [ ] `graphify update .`
- [ ] Visually compare `/routers` and one router detail in the dev server against the mockup.
