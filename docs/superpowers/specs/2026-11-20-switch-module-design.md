# Switch module

## Intent

Operators manage the access and aggregation switches by SSH or telnet
from a terminal. The switches are MikroTik CRS, Huawei CloudEngine (CE)
and FiberHome. The user wants a "Switch" menu in TikMan that does the
day-to-day work there instead:

- see port status, traffic, errors and what is plugged in;
- manage VLANs and access/trunk tagging;
- set ports: enable/disable, description, speed, STP per port;
- back up configs and compare them.

The switches are production, and there is no test unit. The user's rule
for changes: before a change is applied, the switch itself checks that
every command is right (with `?`/help or the vendor's own check), and
nothing is applied by that check.

## Goals

The work is delivered in three phases, each usable on its own.

1. **Read-only, all three vendors.**
   - Switch list and detail.
   - A front panel like the router's.
   - Per-port status, speed, traffic, errors and discards, VLAN mode
     and membership, and STP edge.
   - Connected devices: MAC table and LLDP neighbours.
   - Daily config backup, backup history and a diff between any two
     backups.
   - Candidate switches suggested from the neighbour tables of routers
     and switches TikMan already polls.
2. **Changes on CRS and Huawei CE.**
   - Supported changes:
     - create or delete a VLAN;
     - set a port to access (VLAN N) or trunk (allowed VLANs, PVID);
     - enable or disable a port;
     - description;
     - speed/auto-negotiation;
     - STP edge on or off, and STP off per port where the vendor has it.
   - Each change is rendered into vendor commands and shown in full.
     **Cek perintah** has the switch validate them without applying
     them. **Terapkan** then applies them with a backup before and
     after, a diff, and an audit entry.
3. **The same changes on FiberHome.** These come only after the `?`
   check has been confirmed against the real switch's output (see
   Phase 3).

## Non-goals

- Firmware upgrades, reboots, and saving or loading config files to or
  from the switch.
- ACL, QoS, routing, LACP/eth-trunk creation (trunks are shown, not
  built), global STP settings, and stacking.
- One change applied to many switches at once.
- Restoring a whole backup onto a switch. Backups are for reading and
  diffing; undoing a change uses the change's own inverse lines (see
  "Failure and undo").

## Anti-goals

The module has failed, even if it technically works, if:

- **A change cuts TikMan off from the switch,** or cuts a POP off,
  without the operator having been told plainly beforehand.
- **Cek perintah alters anything** on a switch.
- **A stored backup leaks secrets.** Configs hold SNMP communities,
  local-user password hashes and RADIUS keys.
- **A command the switch rejected is reported as success,** or one it
  accepted is reported as failed.
- **SSH trusts any host key.** TikMan's OLT SSH does
  (`InsecureIgnoreHostKey`), and this module must not.

## Constraints

- The repository is public. Fixtures use 192.0.2.x and invented names.
- CLAUDE.md applies: ≤350 lines per file, ≤50 lines per function, TDD,
  and comments say why.
- `router_poll.go`, `router_service.go`, `Chassis.tsx` and `api/router.go`
  are close to the line limit. New code goes in new files.
- Migrations are single files, numbered 84 and up.

## Approaches considered

1. **CLI everywhere (SSH/telnet for all three vendors).** One mechanism,
   but it ignores the RouterOS API TikMan already speaks to CRS, and it
   loses `:parse`. Rejected for CRS.
2. **A per-vendor driver behind one interface (chosen).**
   - CRS uses the existing RouterOS client (API/REST) for reads and for
     `/execute` (validate with `:parse`, apply the same text).
   - CE and FiberHome use a new interactive SSH shell (with telnet
     fallback for FiberHome).
   - All three vendors use SNMP for counters.

   This matches how OLT drivers are registered, and uses each vendor's
   strongest validation.
3. **NETCONF for Huawei CE.** It has true candidate validation, but it
   must be enabled on every production CE, the YANG models differ by
   release, and the repo has no NETCONF library. Rejected for now. The
   CLI two-stage commit gives per-line validation, and `commit trial`
   gives an automatic revert.

## Design

### Data

New models, following the router module (`models/mikrotik.go`). Secrets
are encrypted with `utils.Encrypt` and serialised as `json:"-"`.

- **`switches`:**
  - name, site_id, host
  - vendor: `crs` | `huawei_ce` | `fiberhome`
  - model, software version
  - SNMP port and community (encrypted)
  - access protocol:
    - CRS: `rest` | `api-ssl` | `api`
    - CE: `ssh`
    - FiberHome: `ssh` | `telnet`
  - port, username, password (encrypted)
  - pinned key:
    - TLS fingerprint (CRS)
    - SSH host-key fingerprint (CE, FiberHome)
    - presented fingerprint
  - enabled, status, last_seen_at
  - `writes_enabled` (bool, default false; below)
  - `mgmt_ports` (text[]): ports and VLANs that changes must never
    touch. TikMan fills it from the LLDP neighbour toward a known
    router, or from the SVI holding the switch's management IP. An
    admin can edit it.
- **`switch_interfaces`:**
  - name, alias, kind (rj/sfp/qsfp/virtual), ifIndex
  - admin and oper status, speed
  - mode: access | trunk | hybrid
  - PVID, allowed VLANs
  - STP edge/disabled
  - last in/out bps, errors, discards
  - previous counters, used for rates
- **`switch_interface_metrics`:** a hypertable, the same shape and
  retention as the router's.
- **`switch_vlans`:** switch, VLAN id, name.
- **`switch_neighbors`:** the latest snapshot, replaced on every read.
  Each row is port, kind (`lldp` | `mac`), MAC, VLAN, and the remote
  system name and port.
- **`switch_config_backups`:**
  - switch, taken_at, reason (`daily` | `before_change` |
    `after_change` | `manual`)
  - SHA-256 of the raw text
  - **the masked text:** every secret value is replaced by `***`,
    using a per-vendor rule list:
    - CE: `snmp-agent community`, `local-user ... password`, radius
      `shared-key`
    - CRS: `/export show-sensitive=no` hides passwords, but SNMP community
    names still appear, so the `/snmp community` lines are masked too
    - FiberHome: `snmp-server community`, `username ... password`
  - The masking is deny-by-default. The rules are a list of what is
    known to be safe to keep. Any line holding `password`, `cipher`,
    `key`, `secret`, `community`, `usm-user`, `%^%#` or `authentication`
    that no rule recognises is masked from its first argument onward.
  - The raw text is never stored or logged.
  - Kept for 90 days, except backups linked to a change, which are
    kept for good.
- **`switch_changes`:**
  - switch and the author's user id
  - the structured request (JSON) and the rendered commands
  - state: `draft` → `validated` | `rejected` → `applied` | `failed` |
    `reverted` | `partial`
  - the inverse lines
  - per-line validation output
  - before and after backup ids
  - applied_by, applied_at
- **`switch_poll_jobs`:** a copy of `router_poll_jobs` with these kinds:

  | Kind | Interval | What it reads |
  |---|---|---|
  | `interfaces` | 1 min | SNMP IF-MIB + ifAlias, ifAdminStatus and error/discard columns |
  | `inventory` | 30 min | the driver's view of ports, VLANs and neighbours |
  | `backup` | daily | the running config |

  The worker gains a third claim loop next to the OLT and router loops.

### Drivers

`connectivity/switchdriver` defines one interface and a registry keyed
by vendor, in the same style as the OLT `DriverFor`:

```
Identify() (model, version)
ReadPorts() / ReadVLANs() / ReadNeighbors()
Backup() (raw text)
Render(change) (commands []string, err)    // refuses guarded targets
Validate(commands) []LineResult            // must not change the switch
Apply(commands) ApplyResult
```

Each vendor's driver:

- **CRS** uses the existing `internal/mikrotik` transport. Its generic
  `Command` (`rest.go:88`, `api.go:188`) carries the new reads and
  `/execute`. Only the router getters exist today, so the CRS methods are
  new, in their own file. It requires RouterOS v7. A CRS running SwOS or
  RouterOS v6 is refused when it is added, with that reason shown.
  - Reads:
    - `/interface/ethernet`, `/interface/bridge/port`,
      `/interface/bridge/vlan`
    - neighbours from `/ip/neighbor`, MACs from `/interface/bridge/host`
    - `/system/routerboard`
  - Backup: `/export terse show-sensitive=no` through `/execute`.
  - Validate: every line goes through
    `:do { :local f [:parse "<line>"] ; :put ok } on-error={ :put err }`
    via `/execute` (`as-string`). The parsed code is never called.
    - User text (descriptions, VLAN names) is restricted to
      `[A-Za-z0-9 ._/-]`, at most 64 characters, before it is rendered.
      Nothing typed can then close the quoted string or reach `$` or `[`.
      The quoting is escaped anyway, and the escaping is tested.
  - Apply: run the same validated lines through `/execute`.
  - Render refuses:
    - toggling `vlan-filtering` or `protocol-mode`;
    - removing a VLAN from the bridge's own tagged list;
    - touching `mgmt_ports`.

    The RouterOS docs warn that these lock the operator out or
    disable every port.
  - The speed syntax branches on RouterOS < 7.12 vs ≥ 7.12.
- **Huawei CE** uses an interactive SSH shell (below).
  - Session setup: `screen-length 0 temporary`.
  - Reads: `display interface brief`, `display port vlan`, `display vlan`,
    `display lldp neighbor brief`, `display mac-address`,
    `display current-configuration`.
  - Validate:
    1. `system-view`. The prompt must then read `[~name]`, which shows
       two-stage commit is in force. Anything else (immediate mode, or an
       uncommitted candidate left by someone else) refuses the check
       before a single line is sent.
    2. Send each line. A line fails if the device answers with a
       `^` marker and/or a line starting with `Error:`.
    3. `display configuration candidate changes`, which becomes the
       preview diff.
    4. `clear configuration candidate`, then `return`. Any Y/N prompt
       on leaving is answered `N`.

    The prompt must read `[~name]` (nothing uncommitted) at the end, or
    the validation itself is reported as failed.
  - Apply:
    1. `system-view`.
    2. The lines.
    3. `commit trial 120`.
    4. TikMan opens a **new** session and reads the switch.
    5. If that succeeds, it sends `commit` in the original session to
       confirm.
    6. If TikMan cannot reach the switch, it does nothing, and the
       switch reverts by itself after 120 s. The change is marked
       `reverted`.

    Commit-time `Error:` lines mark the change `failed`. That is the
    only class of error the per-line check cannot catch, and the UI says
    so.
- **FiberHome** uses an interactive SSH shell, or telnet when the
  switch has no SSH.
  - Session setup: `terminal length 0`. A `---- More ----` / `--More--`
    responder handles paging in case it is ignored.
  - Reads: `show interface brief`, `show vlan`, `show lldp remote`,
    `show mac-address-table`, `show running-config`, with SNMP first
    where the MIBs answer.
  - Validate: in `configure` mode, each line is either a **navigation
    line** or a **content line**, and Render tags each one.
    - **Navigation lines** may be sent with Enter during a check: `configure`,
      `end`, `exit`, and `interface <port>` for a physical port that the
      latest inventory lists. Entering an existing physical interface
      creates nothing.
    - Any line that creates something (`vlan N`, `interface vlan N`) is a
      content line, and so is everything else.
    - Content lines are sent followed by ` ?` and never with Enter.
    - A line passes only when the help lists `<cr>`.
    - Any `^`, `%`, `Error`, `Unknown`, `Invalid` or `Incomplete`
      output fails it.
    - The echoed input is then cleared with Ctrl-U. If the prompt is
      not clean afterwards, Ctrl-C and `end`, and the check reports
      itself as failed.
    - A content line is never followed by Enter.
    - A content line that can only be checked inside a context it would
      itself create, such as a VLAN name inside `vlan N`, cannot be
      checked. Render refuses such changes on FiberHome. VLAN creation
      there is the bare `vlan N` line only.
  - Apply: `configure`, the lines, `end`. `write file` runs only when
    the operator ticks "Simpan permanen". FiberHome has no auto-revert.

### Interactive shell (new)

`connectivity/shell` provides a PTY-backed SSH session, plus the existing
telnet reader, behind one `Shell` type:

- The prompt is learned at login and is matched by a per-vendor regex:
  - CE: `<name>`, `[~name]`, `[*name]`
  - FiberHome: `name>`, `name#`, `name(config…)#`
- The pager responder handles `--More--`.
- Pending output is discarded before every command.
- Each wait has a deadline.
- Output is unwrapped at the terminal width.

The read loop is lifted from `TelnetCommander` (`readUntil`, the
hostname-anchored prompt, `discardPending`). The shell is new because
`SSHCommander` opens one exec per command and cannot hold
`system-view` or `configure` across lines.

### Host-key pinning

This is new work. Routers pin a TLS fingerprint at creation, from the
connection test, and `TrustCertificate` only re-pins after a mismatch.
Switches do the same at creation, and add an explicit step:

1. The first connection records the presented SHA-256 host-key
   fingerprint and refuses to log in.
2. An admin confirms the fingerprint on the switch page.
3. A later mismatch sets `host_key_changed` and stops all access until
   an admin re-confirms.

### Guards and failure

**Guarded targets.**
- `mgmt_ports` and their VLANs: Render refuses any change to them.
- **Uplink ports:** a port whose LLDP neighbour is a router or switch in
  TikMan, or a trunk carrying more than one VLAN. A change to one needs
  the operator to type the port name to confirm. The warning names every
  VLAN that would be affected.

**What the check covers.** The UI states it per vendor. The device check
on CRS and FiberHome is a syntax check. Render adds a meaning check
against the latest inventory: the port exists, the VLAN exists (or is
being created), and the id is in range 1–4094. CE checks meaning in the
candidate too, except errors that only appear at commit.

**Failure and undo.** Render produces two lists for every change: the
apply lines, and their inverse, worked out from the current state.
- **CE:** a rejected line stops the apply before `commit`, and the
  candidate is cleared, so nothing changes. The change is marked
  `failed`.
- **CRS:**
  - The lines run as one script that stops at the first error.
  - Before running it, TikMan adds a one-shot scheduler entry that
    runs the inverse lines in 120 s.
  - After the apply, a fresh session reads the switch and removes the
    scheduler entry.
  - If TikMan cannot reach the switch, the inverse runs by itself and
    the change is marked `reverted`.
  - If a line failed partway, TikMan runs the inverse of the lines that
    did apply, then marks the change `failed`, listing which lines ran.
- **FiberHome:** there is no automatic revert. The apply stops at the
  first rejected line, and the change is marked `partial` with the lines
  that ran. The page offers their inverse as a new change. Changes to
  uplink ports on FiberHome also need the typed confirmation, because
  nothing will undo them by itself.

### Writes are opt-in per switch

`writes_enabled` is false on every new switch. An admin turns it on
after phase 1 has read the switch correctly (ports, VLANs and
`mgmt_ports` all look right). Turning it on records an audit entry.

For FiberHome, the toggle also stays locked until the switch passes a
**probe**: TikMan sends, in `configure` mode and each followed by ` ?`,
after the navigation line `interface <an unused physical port>`:
one known-good line (`description tikman-probe`) and one known-bad
line (`descriptionx`). It stores both raw help outputs on
the switch and enables writes only if the rule above classifies them
correctly. Enter is never sent. If the probe fails, the switch page
shows the two outputs, so the parser can be fixed from real data.

### Roles

| Action | Admin | Technician | Viewer |
|---|---|---|---|
| Read everything | yes | yes | yes |
| Run a backup now | yes | yes | no |
| Draft a change | yes | yes | no |
| Cek perintah | yes | yes | no |
| Terapkan | yes | no | no |
| Add or edit a switch, trust keys, enable writes | yes | no | no |

### Detection

Phase 1 adds a "Switch terdeteksi" list. It gathers the LLDP/MNDP
neighbours seen by the routers (`/ip/neighbor`) and by switches already
in TikMan, keeps the ones whose platform or system description names
CRS, CloudEngine/CE or FiberHome/S-series, and drops the ones already
added. Each entry has a "Tambah" button that prefills host, vendor and
model. The VLAN comments seen on the routers today ("MGMT CRS309",
"SW APJII Huawei") are only hints. They are not parsed.

### UI

- **Menu "Switch":**
  - A list in the rack style: one unit per switch with its front panel.
  - Summary: switches, reachable, ports down.
  - The "Switch terdeteksi" panel.
- **Detail, tabs:**
  - **Panel depan:** the faceplate. Port LEDs follow the rules the
    routers use.
  - **Port:** a table with filters. Clicking a port opens its panel:
    status, VLANs, neighbour, "Ubah port".
  - **VLAN:** a list, and a VLAN × port matrix showing tagged and
    untagged.
  - **Tetangga:** the LLDP and MAC tables.
  - **Config:** the backup list, a viewer and the diff of any two.
  - **Perubahan:** the history of changes, and the change editor.
- **Change editor:**
  1. Pick a change type from a form, never free text.
  2. Commands appear as the switch will receive them.
  3. **Cek perintah** shows each line with the device's verdict and its
     raw reply.
  4. **Terapkan** is enabled only after a passing check of the exact
     same commands. Editing the change invalidates the check.
  5. Afterwards the page shows the before/after diff.
- **Front panel:** `portLayout` grows vendor-aware port kinds:
  - CE names: `GE`, `10GE`, `25GE`, `40GE`, `100GE` `x/y/z`
  - FiberHome names: `gigaethernet`, `xgigaethernet`, `40gigaethernet`

  Templates are added only for models the user actually has. Others use
  the automatic layout.

## Phasing and verification

Each phase has its own plan, review and deploy.

- **Phase 1** is deployable once one switch of each vendor the user
  registers is read correctly. "Correctly" is checked against the
  user's own reading of the switch.
- **Phase 2:**
  - Cek perintah is exercised on the real CRS and CE switches with
    harmless changes (a description on an unused port) and with
    deliberately wrong ones.
  - The first Terapkan on each vendor is a description on an unused
    port, with the user watching.
- **Phase 3** starts with the probe on the user's FiberHome. The parser
  is fixed from its real output before writes can be enabled.

## Left to the builder

- Table and column names beyond those named here.
- File split.
- Parsers for each vendor's `display`/`show` output. Each needs fixture
  tests built from documented sample output, then checked against the
  real switch in phase verification.
- The faceplate generalisation approach.

## Unknowns the user can settle later (not blocking)

- **The switch inventory:** models, sites, and how each is reached. TikMan
  reaches a switch over the VPN when the site router routes its
  management IP. That needs the switch's /32 in the site tunnel's
  allowed IPs, or the switch's own VPN peer.
- **The FiberHome model.** The probe makes the parser self-checking
  either way.
