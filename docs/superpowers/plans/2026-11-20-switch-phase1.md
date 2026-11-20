# Switch Module Phase 1 (Read-Only) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A "Switch" menu in TikMan that reads MikroTik CRS, Huawei CE
and FiberHome switches without changing them. It covers:
- inventory;
- port status, traffic, errors, VLANs and STP edge;
- MAC and LLDP neighbours;
- config backups, with history and a diff;
- switches detected from the neighbour tables.

**Architecture:**
- A `switches` module mirrors the MikroTik router module: model,
  migration, poll-job queue, service, handlers and routes.
- Per-vendor read drivers sit behind one registry:
  - CRS goes over the existing RouterOS transport.
  - CE and FiberHome go over a new interactive shell with a pinned SSH
    host key.
- Counters come from SNMP IF-MIB for every vendor.
- The frontend reuses the router faceplate, with vendor-aware port
  kinds.

**Tech Stack:** Go 1.26, Gin, GORM, gosnmp, `golang.org/x/crypto/ssh`
(already a dependency), TimescaleDB, React + TS, AntD, React Query.

**Spec:** `docs/superpowers/specs/2026-11-20-switch-module-design.md`.
Phase 1 only. Nothing in this plan writes to a switch. `switch_changes`
and every Validate/Apply path belong to phases 2–3 and are NOT built
here.

## Global Constraints

- **No write to a switch, of any kind.** Every command sent is a read:
  - CRS: `print` and `/export`.
  - CE: `display`, `screen-length 0 temporary`.
  - FiberHome: `show`, `terminal length 0`.

  Each driver has a test asserting that the commands it sends come
  from that allow-list.
- Vendor values are exactly `crs`, `huawei_ce`, `fiberhome`. Access
  protocol per vendor:
  - CRS: `rest` | `api-ssl` | `api`
  - CE: `ssh`
  - FiberHome: `ssh` | `telnet`
- **CRS requires RouterOS v7.** Identify refuses SwOS or v6 with
  `ErrUnsupportedFirmware`, and the add form shows
  "CRS harus RouterOS v7 (SwOS/v6 tidak didukung)".
- **Secrets.** The SNMP community and password are stored encrypted
  with `utils.Encrypt` and serialised as `json:"-"`. Raw config text is
  never stored or logged. Backups store masked text plus the SHA-256 of
  the raw text.
- **Masking is deny-by-default.** Any line containing (case-insensitive)
  `password`, `cipher`, `key`, `secret`, `community`, `usm-user`,
  `%^%#` or `authentication` is masked from its first argument onward
  unless a vendor rule keeps it. The CRS `/snmp community` lines are
  masked too.
- **SSH host key.**
  - The first connection records the SHA-256 fingerprint (base64, as
    `ssh-keygen -l` prints it) and refuses to log in until an admin
    pins it.
  - A later mismatch sets `host_key_changed` and refuses.
  - `InsecureIgnoreHostKey` is never used.
- **Roles:**
  - Reads: every role.
  - Backup-now: admin and technician.
  - Add, edit, delete, trust key: admin.
- Migrations are numbered `84_…` and up, as single files. GORM tags must
  match the SQL.
- **Retention:** interface metrics 7 days raw, as for routers. Backups
  90 days. (Change-linked backups do not exist yet.)
- **Poll kinds and intervals:** `interfaces` 1 min, `inventory` 30 min,
  `backup` 24 h. Leases are 2 min, and 5 min for `inventory` and
  `backup`.
- **UI copy** is Indonesian and sentence case. The menu label is
  "Switch", at path `/switches`.
- CLAUDE.md applies: ≤350 lines per file, ≤50 lines per function, TDD,
  comments say why. Fixtures use 192.0.2.x and invented names. The files
  already near the limit (`router_poll.go`, `router_service.go`,
  `Chassis.tsx`, `api/router.go`) get at most one added line each.

## Review Focus

1. **A command outside the read allow-list** must never be sent.
   - Pin it per driver with a fake transport or shell that records
     every command.
   - (T3, T4, T5)
2. **The pager or a wrapped line in long output.**
   - `display current-configuration` or `show running-config` longer
     than one screen, with `--More--` / `---- More ----` in the middle,
     must come back complete, with no pager text and no line broken at
     the terminal width.
   - (T2)
3. **The host key changing after it was pinned.**
   - Connect must fail with `ErrHostKeyChanged`, store the presented
     fingerprint and set `host_key_changed`.
   - It must never log in.
   - (T2, T8)
4. **A secret in a vendor line no rule knows.**
   - For example CE `ntp authentication-keyid 1 authentication-mode
     md5 cipher %^%#abc`.
   - It must come out masked.
   - (T6)
5. **A switch that stops answering mid-inventory.**
   - The earlier interfaces, VLANs and neighbours stay.
   - Status goes `down` only when the SNMP counters fail, as for
     routers.
   - (T6)

---

### Task 1: Switch model, migration, CRUD API

**Files:**
- Create: `backend/internal/models/switch.go`
- Create: `backend/migrations/84_switch_module.sql`
- Create: `backend/internal/services/switch_service.go`, `switch_service_test.go`
- Create: `backend/internal/api/switch_dto.go`, `switch_handler.go`,
  `switch_handler_test.go`, `router_routes_switch.go`
- Modify: `backend/internal/models/models.go` (AutoMigrate list)
- Modify: `backend/internal/api/router.go` (one line, registering the
  switch routes)
- Modify: `backend/internal/api/router_handlers.go` (wiring)

**Interfaces (produces):**
- **`models.Switch`:**
  - ID, SiteID, Name, Host, Vendor (`SwitchVendor`), Model, Version
  - SNMPPort, SNMPCommunity (enc), Protocol, Port, Username, Password (enc)
  - HostKeyFingerprint, PresentedFingerprint, HostKeyChanged
  - TLSFingerprint (CRS)
  - Enabled, Status, LastSeenAt
  - WritesEnabled (bool, always false in phase 1)
  - MgmtPorts (text[] via `pq.StringArray`, or jsonb, matched to the SQL)
  - RackPosition
- **`models.SwitchInterface`:**
  - ID, SwitchID, Name, Alias, Kind (`rj|sfp|qsfp|virtual`), IfIndex
  - AdminUp, OperStatus, SpeedMbps
  - Mode (`access|trunk|hybrid|""`), PVID, AllowedVLANs (text), STPEdge
    (*bool)
  - LastInBps, LastOutBps, InErrors, OutErrors, InDiscards
  - PrevInOctets, PrevOutOctets, PrevSampleAt, LastPolledAt
- **`models.SwitchInterfaceMetric`:** a hypertable with the router metric
  shape and 7-day retention.
- **`models.SwitchVLAN`:** SwitchID, VID, Name.
- **`models.SwitchNeighbor`:** SwitchID, Port, Kind (`lldp|mac`), MAC,
  VID, RemoteName, RemotePort, RemotePlatform.
- **`models.SwitchConfigBackup`:** ID, SwitchID, TakenAt, Reason
  (`daily|manual`), RawSHA256, Masked (text), Size.
- **`models.SwitchPollJob`:** the same columns as `RouterPollJob`, with
  `switch_id`.
- **`services.SwitchService`:**
  - `List()`, `Get(id)`, `Create(SwitchInput)`,
    `Update(id, SwitchUpdate)`, `Delete(id)`, `Secrets(*Switch)
    (community, password string, err error)`
  - Create seeds the poll jobs for all three kinds.
- **Routes** under `/api/v1/switches`:

  | Method | Path | Role |
  |---|---|---|
  | GET | `` | all |
  | GET | `/:id` | all |
  | POST | `` | admin |
  | PUT | `/:id` | admin |
  | DELETE | `/:id` | admin |

- [ ] **Tests first.** Service:
  - create stores the community and password encrypted (the raw DB
    value ≠ plaintext);
  - JSON never contains them;
  - an invalid vendor/protocol pair (`huawei_ce` + `telnet`) is
    rejected with `ErrValidation`;
  - Create seeds 3 jobs;
  - delete cascades its jobs.

  Handler and RBAC: a technician POST gets 403, a viewer GET gets 200,
  and an admin POST gets 201 with the body not containing `password`.
- [ ] **Implement.** Migration 84:
  - creates the FKs with ON DELETE CASCADE;
  - makes `switch_interface_metrics` a hypertable with a 7-day retention
    policy, following `56_mikrotik_monitoring.sql`.
- [ ] Run `cd backend && go test ./... && gofmt -s -l . && go vet ./...`.
  Then run the migration test against TimescaleDB with
  `TEST_POSTGRES_DSN` (the controller can provide it). Everything must
  pass.
- [ ] Commit `feat(switches): switch model, migration and CRUD API`.

### Task 2: Interactive shell with pinned SSH host keys

**Files:**
- Create: `backend/internal/connectivity/shell/` containing `shell.go`
  (the `Shell` interface and session), `ssh.go` (PTY session and
  host-key callback), `telnet.go` (wrapping the existing telnet
  negotiation), `prompt.go` (vendor prompt regexes), `pager.go`, and
  tests
- Do not modify `command_executor.go` or `ssh_command_executor.go`. The
  OLT paths stay as they are.

**Interfaces (produces):**
- **`shell.Config`:**
  - Protocol (`ssh|telnet`), Host, Port, Username, Password
  - PinnedFingerprint
  - Dialect (`shell.DialectVRP | shell.DialectFiberHome`)
- **`shell.Open(ctx, Config) (*Session, error)`:**
  - Returns `ErrHostKeyUnpinned{Presented}` or
    `ErrHostKeyChanged{Presented}` without sending credentials.
  - On success, the session has learned the prompt and disabled paging:
    VRP `screen-length 0 temporary`, FiberHome `terminal length 0`.
- **`(*Session) Run(ctx, cmd string) (string, error)`:**
  - Discards pending output, sends the command, and reads until the
    prompt.
  - Answers `--More--`, `---- More ----` and `  ---- More ----` with a
    space, then strips them.
  - Unwraps lines at the session's terminal width. Request a 512-column
    PTY so wrapping rarely happens at all.
- **`(*Session) Close() error`**
- **Prompt regexes:**
  - VRP: `^<name>$`, `^\[[~*]?name(-[^\]]+)?\]$`
  - FiberHome: `^name(\([^)]*\))?[>#]$`

- [ ] **Tests first.** Use an in-process SSH server built with
  `golang.org/x/crypto/ssh` that serves canned transcripts:
  - unpinned → `ErrHostKeyUnpinned` carrying the server's fingerprint,
    and the server sees no password attempt;
  - pinned and matching → login, then `Run` returns the output without
    the echoed command or the prompt;
  - mismatch → `ErrHostKeyChanged`;
  - pager: output with two `---- More ----` breaks comes back complete,
    with the pager text stripped (Review Focus 2);
  - a prompt that never comes back → the context deadline error, not a
    hang;
  - a telnet transcript → the same `Run` results.
- [ ] Implement, lifting the read loop from `TelnetCommander`
  (`readUntil`, the hostname-anchored prompt, `discardPending`).
- [ ] Run `go test ./internal/connectivity/... -race`, gofmt and vet.
- [ ] Commit `feat(switches): interactive CLI shell with pinned SSH host keys`.

### Task 3: Driver registry and the CRS read driver

**Files:**
- Create: `backend/internal/switchdriver/` containing `driver.go` (the
  interface, registry and types), `crs.go`, `crs_test.go`
- Create: `backend/internal/mikrotik/client_switch.go`, with the CRS
  reads on the existing transport

**Interfaces (produces):**
```
type Identity struct{ Model, Version string }
type Port struct{ Name, Alias, Kind, Mode string; AdminUp bool; PVID int; AllowedVLANs string; STPEdge *bool }
type VLAN struct{ VID int; Name string }
type Neighbor struct{ Port, Kind, MAC string; VID int; RemoteName, RemotePort, RemotePlatform string }
type Reader interface {
  Identify(ctx) (Identity, error)
  ReadPorts(ctx) ([]Port, error)
  ReadVLANs(ctx) ([]VLAN, error)
  ReadNeighbors(ctx) ([]Neighbor, error)
  Backup(ctx) (string, error)   // raw text; the caller masks it
  Close() error
}
func Open(ctx, sw *models.Switch, community, password string) (Reader, error)  // picks by vendor
var ErrUnsupportedFirmware error
```
`client_switch.go` adds these read methods on `*mikrotik.Client`:
- `Ethernet()`, `BridgePorts()`, `BridgeVLANs()`, `Neighbors()` (from
  `/ip/neighbor`), `BridgeHosts()`
- `Export()`: `Command("/execute", out, "script=/export terse
  show-sensitive=no", "as-string=")`
- `Routerboard()`

CRS reads:
- **Identify:** `/system/resource` for version, `/system/routerboard`
  for model. A version < 7 → `ErrUnsupportedFirmware`.
- **Ports:** ethernet joined with bridge ports.
  - mode = trunk when frame-types is admit-only-vlan-tagged, or the
    port is tagged in any bridge VLAN;
  - otherwise access with the PVID;
  - STPEdge comes from `edge` (yes* → true, no* → false, auto → nil).

- [ ] **Tests first**, with a fake transport recording every
  path and argument:
  - parsing of each read from recorded REST JSON;
  - v6 → `ErrUnsupportedFirmware`;
  - **every recorded call is a `List` or `ListProps`, or a `Command` to
    `/execute` whose script begins with `/export` (Review Focus 1)**;
  - trunk/access derivation table.
- [ ] Implement, gofmt, vet, test.
- [ ] Commit `feat(switches): driver registry and CRS read driver`.

### Task 4: Huawei CE read driver

**Files:**
- Create: `backend/internal/switchdriver/huawei.go`,
  `huawei_parse.go`, tests, and fixtures in `testdata/huawei/`

**Behaviour:**
- Opens `shell.Open` with DialectVRP.
- Reads:
  - `display version` (model and version)
  - `display interface brief` (names, admin and oper state)
  - `display port vlan` (mode, PVID, trunk allowed list)
  - `display vlan` (VIDs and names)
  - `display lldp neighbor brief`
  - `display mac-address`
  - `display stp brief` (edge)
  - `display current-configuration` for Backup
- Interface names map to kinds:
  - `GE` → rj, unless the media is fiber in `display interface brief`
  - `10GE`/`25GE` → sfp
  - `40GE`/`100GE` → qsfp
  - `Vlanif`, `Eth-Trunk`, `NULL`, `LoopBack`, `MEth` → virtual, except
    `MEth` which is rj

**Tests first:**
- [ ] Parsers, using fixtures typed from Huawei's documented sample
  outputs. Each fixture file has a source comment.
- [ ] A fake shell recording commands. **Every command starts with
  `display ` or is `screen-length 0 temporary` (Review Focus 1).**
- [ ] Implement, gofmt, vet, test.
- [ ] Commit `feat(switches): Huawei CloudEngine read driver`.

### Task 5: FiberHome read driver and generic SNMP neighbour walks

**Files:**
- Create: `backend/internal/switchdriver/fiberhome.go`,
  `fiberhome_parse.go`, tests, and `testdata/fiberhome/`
- Create: `backend/internal/connectivity/snmp_lldp.go` (LLDP-MIB
  lldpRemTable) and `snmp_fdb.go` (Q-BRIDGE dot1qTpFdb, falling back to
  BRIDGE dot1dTpFdb)

**Behaviour:**
- FiberHome tries the SNMP walks for neighbours first, then falls back
  to the CLI:
  - `show lldp remote`
  - `show mac-address-table`
- Ports and VLANs come from `show interface brief`, `show vlan` and
  `show running-config` (the `port link-type`, `port default vlan`,
  `port trunk allow-pass vlan` and `stp edge-port` stanzas).
- Model and version come from `show version`.
- Kinds: `gigaethernet` → rj, `xgigaethernet` → sfp,
  `40gigaethernet` → qsfp, `eth-trunk`/`vlan` → virtual.
- The parsers must tolerate unknown lines, because FiberHome docs are
  thin. A line they cannot parse is skipped, never an error.

**Tests first:**
- [ ] Parser fixtures from the S6800 command reference.
- [ ] SNMP decoders fed with PDU fixtures.
- [ ] Recorded commands are `show …` or `terminal length 0` only
  (Review Focus 1).
- [ ] Implement, gofmt, vet, test.
- [ ] Commit `feat(switches): FiberHome read driver and SNMP LLDP/FDB walks`.

### Task 6: Poll jobs, worker loop, masking, backups

**Files:**
- Create: `backend/internal/services/switch_poll_jobs.go` (a copy of the
  router job service with the switch table and kinds)
- Create: `switch_poll.go` (PollInterfaces, Inventory, Backup)
- Create: `switch_mask.go` and tests
- Create: `backend/cmd/worker/switch_jobs.go`
- Modify: `backend/cmd/worker/main.go` (one call in the loop, and
  wiring)
- Create: `backend/internal/connectivity/snmp_ifstate.go`, which walks
  ifAlias, ifAdminStatus, ifInErrors, ifOutErrors and ifInDiscards.
  `WalkIfCounters` is reused for the rest.

**Interfaces (produces):**
- `MaskConfig(vendor models.SwitchVendor, raw string) string`
- `(*SwitchPollService).PollInterfaces / Inventory / Backup(sw, now) error`

**Behaviour:**
- **interfaces:** the router PollInterfaces pattern. A failed walk sets
  status down; success sets up and `last_seen_at`. Rates use
  `InterfaceRate`.
- **inventory:**
  - Open the driver, Identify, then write model and version.
  - Upsert ports: matched by name, merged onto the SNMP rows. Replace
    VLANs and neighbours in one transaction.
  - A driver error records it and keeps the earlier rows (Review
    Focus 5).
  - `ErrHostKeyUnpinned`/`ErrHostKeyChanged` store the presented
    fingerprint, set `host_key_changed` when the key changed, and fail
    the job.
- **backup:**
  - `Backup()` → `MaskConfig` → store masked text and the SHA-256 of
    the raw text.
  - Skip storing when the hash equals the latest backup's.
  - Prune `daily` backups older than 90 days.

**Tests first:**
- [ ] The masking table:
  - CE `snmp-agent community read cipher %^%#x`
  - `local-user a password irreversible-cipher …`
  - `radius-server shared-key cipher …`
  - the NTP line from Review Focus 4
  - FiberHome `snmp-server community x ro`
  - FiberHome `username a password 0 x`
  - CRS `/snmp community add name=x`
  - a harmless line (`interface 10GE1/0/1`) left unchanged
- [ ] Inventory keeps old rows on driver error.
- [ ] A backup with the same hash is not stored twice.
- [ ] A host-key mismatch sets the flag.
- [ ] The worker dispatches the three kinds.
- [ ] Implement, gofmt, vet, test with `-race` on services.
- [ ] Commit `feat(switches): polling, inventory, masked config backups`.

### Task 7: Read and trust endpoints, plus detection

**Files:**
- Create: `backend/internal/api/switch_reads_handler.go` and tests
- Create: `backend/internal/services/switch_query.go` and
  `switch_detect.go`, with tests

**Endpoints:**

| Endpoint | Role | Notes |
|---|---|---|
| `GET /switches/:id/interfaces` | all | |
| `GET /switches/:id/vlans` | all | |
| `GET /switches/:id/neighbors` | all | |
| `GET /switches/:id/backups` | all | list without the text |
| `GET /switches/:id/backups/:bid` | all | masked text |
| `GET /switches/:id/backups/diff?from=&to=` | all | unified diff of the masked texts, computed in Go with a small LCS, no new dependency |
| `POST /switches/:id/backup` | admin, technician | makes the `backup` job due now |
| `POST /switches/:id/trust-host-key` | admin | pins the presented fingerprint and clears `host_key_changed` |
| `POST /switches/:id/refresh` | admin, technician | |
| `POST /switches/test-connection` | admin | opens the driver read-only, returns the identity or `ErrHostKeyUnpinned` with the fingerprint for the form to show |
| `GET /switches/detected` | all | |

**Detection:**
- Sources:
  - a new read of `/ip/neighbor` on routers TikMan polls (a new
    `mikrotik.Client.Neighbors()` reused from T3, called from router
    discovery and stored in `router_neighbors`, migration 85);
  - plus `switch_neighbors` of kind lldp.
- Keep entries whose platform or identity matches
  `(?i)CRS|CloudEngine|\bCE\d|FiberHome|S\d{4}`. Drop hosts already in
  `switches`.

**Tests first:**
- [ ] RBAC per endpoint.
- [ ] A diff of two fixtures.
- [ ] Detection filtering and de-dup.
- [ ] Trust pins and clears the flag.
- [ ] Implement, gofmt, vet, test.
- [ ] Commit `feat(switches): read endpoints, host-key trust, detection`.

### Task 8: Frontend list, add/edit form, detection, menu

**Files:**
- Create: `frontend/src/domain/entities/Switch.ts`
- Create: `infrastructure/repositories/SwitchRepository.ts`
- Modify: `endpoints.ts`
- Create: `application/hooks/useSwitches.ts`
- Create: `presentation/pages/SwitchesPage.tsx`
- Create: `components/switches/SwitchFormModal.tsx`,
  `SwitchFormFields.tsx`, `DetectedSwitches.tsx`
- Modify: `navigationRoutes.tsx` (adds "Switch" after "MikroTik")
- Modify: `routes/index.tsx`

**Behaviour:**
- **List:** the rack view. Generalise `faceplate/portLayout.ts` port
  kinds by vendor:
  - Huawei `GE|10GE|25GE|40GE|100GE` `\d+/\d+/\d+`
  - FiberHome `gigaethernet|xgigaethernet|40gigaethernet`

  Pass the kind patterns in, and keep the RouterOS defaults so the
  router views do not change. Reuse `RouterRack`'s unit drawing by
  passing a generic device. If that needs more than a thin adapter,
  create `SwitchRack.tsx` with the same CSS instead.
- **Form:**
  - vendor first. The protocol options follow the vendor.
  - "Test koneksi":
    - an unpinned key shows the fingerprint with "Percayai kunci ini"
      before save;
    - CRS v6/SwOS shows the firmware message.
- **Detected panel:** each row has "Tambah", which prefills the form.

**Tests first:**
- [ ] The protocol options per vendor.
- [ ] The fingerprint confirm flow.
- [ ] The firmware message.
- [ ] Detected → prefill.
- [ ] A Huawei name layout test. The router layout tests stay
  unchanged.
- [ ] Run `npm test -- --run && npm run lint && npm run format:check && npm run build`.
- [ ] Commit `feat(switches): switch list, form and detection`.

### Task 9: Frontend switch detail

**Files:**
- Create: `presentation/pages/SwitchDetailPage.tsx`
- Create, in `components/switches/`: `SwitchPortsTab.tsx`,
  `SwitchVlanTab.tsx`, `SwitchNeighborsTab.tsx`,
  `SwitchConfigTab.tsx`, `ConfigDiff.tsx`, and tests

**Tabs:**
- **Panel depan:** the faceplate, with LEDs following the router rules.
- **Port:** a table with filters:
  - columns: status, speed, mode, PVID or allowed VLANs, STP edge,
    traffic, errors, alias;
  - a row click opens the port detail.
- **VLAN:** a list plus a matrix: VLAN rows × port columns, showing T
  for tagged and U for untagged.
- **Tetangga:** an LLDP table and a MAC table, both searchable.
- **Config:**
  - a backup list and a viewer of the masked text;
  - pick two backups to see the diff (added lines green, removed red);
  - "Backup sekarang" for admin and technician.

A host-key-changed banner shows the presented fingerprint. Admins also
get a "Percayai" button.

**Tests first:**
- [ ] The matrix from fixture data.
- [ ] Diff rendering.
- [ ] Role gating of the buttons.
- [ ] The banner.
- [ ] Run `npm test -- --run && npm run lint && npm run format:check && npm run build`.
- [ ] Commit `feat(switches): switch detail tabs`.

---

After Task 9:
1. The whole-branch review.
2. Merge and deploy, with approval. Migrations 84–85 mean the full
   restart order.
3. The user registers one switch per vendor. The controller then checks
   ports, VLANs, neighbours and a backup against the user's own reading
   of each switch.
