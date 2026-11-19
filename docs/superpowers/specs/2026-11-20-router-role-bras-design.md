# Router role and BRAS monitoring

## Intent

TikMan treats every MikroTik as a border router. Every router gets the
BGP, route-advice and flow tabs, and every router is polled for BGP. The
operator wants to add BRAS routers, the PPPoE concentrators that
customers dial into. A BRAS page should show the state of the BRAS as a
network device, not just a customer count.

The operator left the order of the BRAS page to us and will adjust it
after seeing it. The approved mockup (2026-11-20) put the Ringkasan tab
first, ordered by one question: "is there a mass outage right now?".

## Goals

- Each router has a role: `border`, `bras` or `other`.
  - The role is shown as a label-maker tape on its rack unit and on the
    detail page.
  - The role decides which poll jobs run and which detail tabs show.
- TikMan guesses the role from what discovery finds on the router. Once
  an admin sets the role in the edit form, it stays as set.
- A BRAS is polled every minute for:
  - PPP session count
  - IP pool use
  - RADIUS client counters
  - CPU load per core
  - the heaviest current users
  - system resource: version, board, CPU, memory, uptime and API
    status, which the BGP job writes today
- The BRAS detail page opens on **Ringkasan**:
  1. Four current signals:
     - customers online, compared with 15 minutes ago
     - whether RADIUS is answering, and how many logins it rejected in
       the last hour
     - the busiest CPU core
     - the fewest addresses left in any pool
  2. A 24-hour chart of sessions online, with mass drops marked.
  3. Pools, RADIUS servers, top users, and the uplink to the border.
- A BRAS keeps the Panel depan, Interface & VLAN and Router tabs. It
  loses Upstream & BGP, Jalur BGP and Tujuan Trafik.

## Non-goals

- Per-customer actions: disconnect, edit a secret, view a customer's
  history. netmanager (admin.radpro.id) owns these.
- Alerts or notifications on a mass drop. The page only marks drops.
  Alerting can come later on the same samples.
- Per-customer history. Only the latest top-users list is kept.
- Hotspot, L2TP, SSTP or IPv6 pool monitoring.
- A deep link from a top user into netmanager. netmanager's PPP list
  (`app/admin/pelanggan/ppp/PppList.tsx`) does not read a search term
  from the URL, so no link can land on one customer without changing
  netmanager first. The username is shown with a copy button instead.

## Anti-goals

The work has failed, even if it technically works, if any of these
happens:

- **A BRAS reads as down because it is busy.** On a BRAS, the current
  interfaces poll walks about 2000 dynamic `<pppoe-…>` rows. That is
  about 1030 sequential GETBULKs against a 10 s budget. When the walk
  runs out, `PollInterfaces` sets `status = down` (router_poll.go:96-99).
- **Polling loads the BRAS CPU noticeably.** Forum reports show large
  PPPoE servers lagging under SNMP walks. Prefer REST calls that the
  router answers from one table.
- **An admin's role choice is overwritten** by the next discovery.
- **The border router changes behaviour.** Its jobs, tabs and colouring
  stay exactly as they are today.

## Constraints

- The repository is public. Tests and fixtures use 192.0.2.x, AS64500
  and invented names, never real ones.
- Migrations are single files `NN_name.sql`. This work uses 81 and up.
  The GORM tags must match the migration, because AutoMigrate runs first
  (models/mikrotik.go:56-57).
- CLAUDE.md limits apply: ≤350 lines per file, ≤50 lines per function,
  comments explain why, TDD.
- The RouterOS client (`backend/internal/mikrotik`) supports REST, api
  and api-ssl. Every new read must work over all three.
- Workers are scaled and share the job queue. Any state carried from one
  poll to the next lives in the database, never in a worker's memory.

## Approaches considered

1. **Role as a label only.** The tape shows, tabs are filtered by role,
   polling is unchanged. Cheap, but a BRAS would still be walked into
   "down", and the operator asked to see the BRAS's own data. Rejected.
2. **Role plus a REST-based BRAS job, with SNMP limited to known ports
   (chosen).** The role picks the job set. A new `bras` job reads
   everything the page needs through five or six REST calls. On a BRAS,
   the interfaces poll asks only for the ports TikMan already knows.
3. **Collect everything over SNMP.** That means a CISCO-AAA-SESSION-MIB
   session count, ifTable counters per PPPoE user, and
   mtxrQueueSimpleTable. Rejected: it means walking thousands of rows
   every minute, which is the load we must avoid. RADIUS counters also
   have no SNMP equivalent, and rates would need stored deltas per user.

## Design

### Role

- Migration 81 adds `role varchar(8) not null default 'border'` and
  `role_set_by_admin boolean not null default false` to
  `mikrotik_routers`.
  - Existing routers become `border`, which matches what they are today.
  - The Go type is `models.RouterRole`, with constants `border`, `bras`
    and `other`. It is not `PortRole`, which already exists in the
    faceplate.
- **Guess.** `Discover` gains two reads: `/interface/pppoe-server/server`
  and `/routing/bgp/connection`. The second uses the existing
  `BGPConnections`; discovery does not read it today. It then computes
  the guess in its existing transaction:
  - any PPPoE server entry → `bras`
  - otherwise, any enabled BGP connection → `border`
  - otherwise → `other`

  The PPPoE check comes first because a BRAS often runs iBGP to the
  border. The guess is written only while `role_set_by_admin` is false.
  If either read fails, the role stays unchanged and discovery does not
  fail, the same way `ErrStaticRoutesUnread` is handled.
- **Edit.** `UpdateRouterRequest` gains
  `Role *string binding:"omitempty,oneof=border bras other"`. Setting it
  writes the role and sets `role_set_by_admin = true`. The handler
  audits the change like the other fields.
  - The form shows Peran as a Radio.Group: Otomatis / Border / BRAS /
    Lainnya.
  - Choosing **Otomatis** sends `ResetRole: true`. That clears
    `role_set_by_admin` and calls `RunNow(discovery)`, so the guess
    comes back within a minute.
  - The help line under the field says what TikMan guessed and whether
    the current value came from that guess or from an admin.
  - Create takes the same optional field. Leaving it unset means "let
    discovery guess".
- **Jobs follow the role.** `RouterPollKindsFor(role)` returns:

  | Role | Jobs |
  |---|---|
  | border | discovery, interfaces, bgp, routes |
  | other | discovery, interfaces, bgp |
  | bras | discovery, interfaces, bras |

  `other` keeps `bgp` because that job writes the system resource
  fields. A core router with no BGP simply polls an empty session list.

  Create seeds the kinds for the given role, or for `border` when
  discovery will guess. Each role change, whether guessed or set by an
  admin, syncs the jobs in the same transaction: missing rows are
  inserted with due_at now, and rows that no longer apply are deleted.
  The migration does not need to backfill jobs, because every existing
  router stays `border`. The worker switch (`cmd/worker/router_jobs.go`)
  and `RouterIntervalFor` each gain a `bras` case. `AllRouterPollKinds`
  is replaced by `RouterPollKindsFor`.

  **What a role change does to old data.** It hides the data and keeps
  it, so the change can be undone. There is one exception. When the
  `bgp` job is removed, the router's `router_bgp_sessions` rows are
  deleted in the same transaction. Nothing would update them any more,
  and they would keep counting in `bgpEstablished` / `bgpTotal`. Routed
  prefixes, route advice and `flow_enabled` are left alone. flowd keeps
  whatever it is sent, and the admin turns flow off in the form if they
  want it off.
- **Tabs.** `RouterDetailPage` builds its tabs from the role:

  | Role | Tabs |
  |---|---|
  | border | today's six tabs, unchanged |
  | bras | Ringkasan, Panel depan, Trafik, Interface & VLAN, Router |
  | other | Panel depan, Interface & VLAN, Router |

  The BRAS Trafik tab is the uplink chart, which reuses the UpstreamTab
  pattern for the uplink interface.

### Interfaces poll on a BRAS

When the role is `bras`, `PollInterfaces` no longer walks the whole
ifXTable. It reads only the stored, enabled interfaces that have an
`if_index`:

- It sends GETs for ifName, ifHCInOctets, ifHCOutOctets, ifHighSpeed and
  ifOperStatus at those indexes, packed many varbinds per PDU.
- The returned ifName is checked against the stored name.
  - A mismatched interface, or one with no `if_index`, gets no sample
    this minute, and the job calls `RunNow(discovery)`.
  - The router is marked down only when the GETs themselves fail. That
    keeps today's meaning of down: unreachable.
- **Indexes come from discovery.** For a BRAS, discovery maps ifName to
  ifIndex once per run. It walks the ifName column alone with 50
  repetitions per GETBULK, which is about 40 requests for 2000 rows,
  under a 30 s budget. Discovery's lease is already 5 minutes. A failed
  walk keeps the indexes stored earlier.
- The per-minute job never walks. Physical ports keep their ifIndex on
  RouterOS, so a re-map should be rare; PPP churn does not renumber
  them.

Border and other routers keep the current walk and the current way of
storing `if_index`.

This work also adds a `dynamic=false` filter to `Client.Interfaces` for
all three transports. Today it lists everything
(`mikrotik/client.go:72-76`). With the filter, a BRAS stops returning
its 2000 dynamic rows to discovery, which discards them anyway.

The health gauge walk runs after the counters, as today.

### The `bras` job (every minute)

**Transport change.** The client transport today can only `List`
(GET / `print`). The job also needs:

- `count-only` on a print:
  - REST: `POST /rest/<path>/print` with `{"count-only":"", ".query":[…]}`
  - API: `=count-only=`
- `monitor … once`:
  - REST: `POST /rest/radius/monitor` with `{"numbers":"…","once":""}`
  - API: `/radius/monitor =numbers= =once=`
- `.proplist` on a list, to keep the queue payload small.

These are added to the transport for all three protocols, with tests
against recorded replies.

**Reads, in order.** A failure in one read stops the job and records it
as `last_error`. The system resource read updates `api_status` exactly
as `PollBGP` does today; that write is factored into a shared function
so both jobs use it.

1. `/system/resource` → the router's resource columns. This is the same
   write `resourceUpdates` does today.
2. `/system/resource/cpu` → per-core load.
3. `/ppp/active` count-only → sessions online, counting every PPP
   service.
4. `/ip/pool` with `total`, `used` and `available`.
   - Older 7.x builds lack these fields. In that case: used comes from
     count-only on `/ip/pool/used` with `pool=<name>`, and total is
     computed from `ranges`. IPv4 ranges only; a range TikMan cannot
     parse makes that pool's total unknown.
5. `/radius`, then a monitor `once` per entry → requests, accepts,
   rejects, timeouts, pending and last-request-rtt.
6. `/queue/simple` with `dynamic=true` and
   `.proplist=name,target,rate`.
   - Parse `rate` ("upload/download" in bps) and keep the top 10 by
     download.
   - No dynamic queues means an empty list, and the page says rate
     limits are not set by RADIUS or profile.

**Storage.**

- Migration 82 adds `router_bras_samples`, one row per BRAS per minute:
  - `router_id`, `time`, `sessions`, `cpu_max`, `cpu_avg`
  - `radius_accepts`, `radius_rejects`, `radius_timeouts` (cumulative
    totals summed over servers, as read)
  - `pool_min_free` (the fewest free addresses in any pool whose total
    is known)

  It is a hypertable like `router_interface_metrics`, with 30 days of
  raw data and no rollups. That is about 43k rows per BRAS a month.
- Migration 82 also adds `bras jsonb` and `bras_at timestamptz` to
  `mikrotik_routers` for the latest snapshot: cores, pools, RADIUS
  servers, top users and uplink. This follows the `health` / `health_at`
  pattern.

**Uplink.** For a BRAS, discovery reads the active `0.0.0.0/0` route of
any origin with the existing `RoutesTo("0.0.0.0/0")`. A default learned
by iBGP, OSPF or DHCP counts as well as a static one. The uplink is
found from that route's gateway, in this order:

1. If the gateway names an interface (`%iface` or an interface name),
   that stored interface.
2. Otherwise, the stored interface whose subnet contains the gateway
   address.

It is stored as `uplink_interface_id` (migration 82, nullable). A
failed read keeps the uplink stored earlier. With no default route, or no matching interface, the uplink
section is hidden and no port is coloured upstream. On the faceplate,
the uplink port of a BRAS gets the upstream (yellow) tape that BGP gives
border ports.

### API

- `GET /api/v1/routers/:id/bras?from&to` returns:
  - the latest snapshot (`bras`, `bras_at`)
  - the samples in the range
  - the derived values:
    - `sessions15mAgo`: the sample closest to 15 minutes before the
      latest, within ±2 minutes, else null
    - `radiusRejects1h` and `radiusTimeouts1h`: the latest total minus
      the total about 60 minutes earlier. A negative difference means
      the counters reset, so it reads null.
    - `drops`: every sample where sessions fell by
      `max(100, 5% of the previous sample)` against a previous sample no
      more than 2 minutes older. Each drop carries its time and size.
- The endpoint is auth-gated like the other router reads (all roles may
  read). On a non-BRAS router it returns 404.

### Frontend

- `MikrotikRouter` gains `role`, `roleSetByAdmin`, `bras`, `brasAt` and
  `uplinkInterfaceId`.
- **Rack unit.** A role tape sits on the chassis next to the name tape:
  yellow BORDER, blue BRAS, white LAINNYA.
  - A BRAS unit's detail line shows `<n> online` and CPU instead of BGP.
  - BRAS units skip `useRouterBgpSessions`.
  - The page summary line adds `· <n> pelanggan online` when any BRAS
    exists. The BGP sum counts only routers that have BGP.
- **Ringkasan tab.** Its parts:
  - A row of four signals, each with an LED dot. Thresholds:
    - **Pelanggan online.** Red when a drop is in the last 15 minutes.
    - **RADIUS.** "Not answering" is judged per server from that
      server's own counters between two reads. The job keeps each
      server's previous counters in the `bras` snapshot it is about to
      replace.
      - **Not answering:** a server whose requests grew while its
        accepts and rejects did not, and whose timeouts grew.
      - **Red** when every server is not answering, the same as no
        logins at all.
      - **Amber** when some servers are not answering, when timeouts
        grew in the last hour, or when rejects in the hour exceed 10%
        of accepts.
      - A server with no new requests keeps its previous verdict.
    - **CPU core terberat.** Amber at 70% or more, red at 90%.
    - **Sisa alamat.** Amber below 10% free in any pool, red below 2%.
  - A 24-hour sessions line chart in recharts. Missing minutes show as
    gaps, using `connectNulls={false}` as in DailyPerformanceChart.
    Drops are marked with a dot and a "HH:mm turun N sesi" label.
  - Under the chart, in two columns:
    - left: pools (bar plus used/total/free) and RADIUS servers
    - right: top users (username with a copy button, download, upload)
      and the uplink (port, current in/out rate, capacity)
  - A stale snapshot (`bras_at` older than 5 minutes) greys out the
    signals and says when the last read was, like the health strip does.
- The Router tab's Descriptions show the role and whether TikMan guessed
  it.

## Left to the builder

- How the transport exposes count-only, monitor and proplist, as long as
  all three protocols work and are tested.
- PDU packing for the targeted interface GETs.
- Component and file split, within the line limits.
- Whether the sessions chart is a sibling of TrafficChart or TrafficChart
  with a value-formatter prop.

## Verify on the real BRAS before relying on it

The operator registers the BRAS after deploy, with the REST API on
RouterOS v7. Check on the box:

- The reply shape of count-only (`ret` as a string, or a number).
- Whether `/ip/pool` returns `total` and `used` on its build.
- The format of `last-request-rtt`.
- The size of the `/queue/simple` reply.

The RouterOS research behind these calls (manual.mikrotik.com REST API,
`/ip/pool`, `/radius/monitor`, `/system/resource/cpu`, `/queue/simple`)
rated the field names high-confidence and the reply shapes medium.
