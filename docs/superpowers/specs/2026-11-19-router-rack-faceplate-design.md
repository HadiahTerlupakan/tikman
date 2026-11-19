# Router rack and faceplate view

## Intent

The `/routers` page is a table, and the router detail spreads its ports across
the "Interface & VLAN" and "Upstream & BGP" tabs. The operator wants to see each
MikroTik the way it looks in the NOC: a front panel with its ports, which ones
are live, and what each one carries. With more than one router, the list should
read as a rack. The look must be specific to this subject (chassis, jacks,
LEDs, cable labels), not a generic card grid.

The mockups the operator approved on 2026-11-19 are the reference:
`.superpowers/brainstorm/39339-1795087692/content/router-faceplate-v4.html`
(rack, detail, VLAN trunks, an unknown model, an unreachable router). They are
gitignored and use invented data.

## Goals

- `/routers` shows every router as a 1U unit in one rack, in a set order.
  Each port shows its live state: a plug in its role colour, a link LED
  (green up, amber silent or a dead BGP session behind it, red down), and an
  activity LED that blinks faster with more inbound traffic. Hovering a port
  names it. Beside each unit: name, a one-line status, model, CPU, BGP x/y,
  and "Ubah" for admins. Above the rack: router count, how many are
  reachable, BGP established/total.
- Clicking a unit opens the router detail. Above the existing tabs, the detail
  shows the faceplate with hanging cable labels (the port's comment, inbound
  rate) and, for a port that carries VLANs, the VLANs under its label (first
  three, then "+N VLAN lain"). Clicking a port or label shows its panel: state,
  speed, in/out, utilisation, 24 h inbound chart, BGP sessions through it
  (including via its VLANs) and its VLANs.
- Any board renders. Known boards use a template that mirrors the real
  faceplate; any other board is laid out from its interface names (ether,
  sfp/combo, qsfp groups, two rows) with a note saying so.

## Non-goals

- No writes to the router. This only reads what discovery and polling store.
- No change to the BGP priority work discussed the same day; that is a
  separate design.
- No new polling. Everything shown is already collected.

## Anti-goals

- Not a templated dashboard: no card grid, no generic KPI tiles.
- A port must never look healthy when it is not: an unreachable router shows
  every LED off and the unit dimmed, not the last known state.

## Decisions

**Data comes from what exists.** The rack calls the existing per-router reads
(`/routers`, `/routers/:id/interfaces`, `/routers/:id/bgp/sessions`) for each
router. There are a handful of routers, so no aggregate endpoint is added.
VLANs come from `parentName`/`vlanId`, BGP-to-port from the session's
`interfaceId`, the model from `boardName`, the 24 h chart from the existing
traffic endpoint.

**Port role is derived, not typed:**
- upstream: a BGP session runs on the port or on one of its VLANs;
- management: one of the port's (or its VLAN's) addresses is the address
  TikMan polls the router at;
- distribution: everything else that is up or has a comment;
- a port with VLANs of several roles is a trunk: dark label, plug striped
  with each role's colour.

**Rack order is set by hand (Claude's call, offered to the operator and not
objected to).** A new nullable `rack_position` on the router, edited in the
existing router form as "Posisi di rak". Routers without one follow, sorted
by name. Gaps between positions are drawn as blank panels, so the screen can
match the physical rack.

**Templates at launch:** `CCR2116-12G-4S+` and `CCR2004-16G-2S+`. Everything
else uses the automatic layout.

**Calls made after the builder check (Claude's, for the operator to check):**
- *Unreachable* means `status` is `down` or `unknown`: the unit is dimmed and
  every LED is off. With SNMP up but `apiStatus` not `ok`, ports stay live
  (they come from SNMP), and the BGP line beside the unit reads as stale
  rather than counting sessions.
- The label text is the interface comment; with no comment, the port name.
  The mockup's separate label and comment were invented data.
- The chassis keeps one status LED tied to the router's `status`. PWR2 and
  FAULT are dropped, since nothing collects them.
- Any session that is not established turns its port amber, including a
  peer left down on purpose. `BgpState` has no "disabled" to tell them apart.
- When no port carries the polled address (DNS name, NAT, or a WireGuard
  tunnel), no port is marked management. There is no guessing from comments.
- `interfaceId` is TikMan's subnet match, not the router's own binding. A
  session it cannot place adds no role to any port.
- The faceplate sits above all five detail tabs.

## Approaches considered

1. **Draw with HTML/CSS elements, laid out in code (chosen).** What the
   mockup does: absolutely positioned jacks and labels, an SVG layer for the
   cables. Each port is a real button, so keyboard focus and screen readers
   work, and it reuses Ant Design's tokens around it.
2. **One SVG per faceplate.** Crisper scaling, but every interactive part
   needs hand-built focus handling, and text measurement for label layout is
   awkward in SVG.
3. **Photographs of each model with hotspots.** Most realistic, but needs an
   image per model and per revision, and MikroTik's product photos are not
   ours to ship. Rules out unknown models entirely.

## Left to the builder

- Splitting the drawing code into files under the 350-line limit
  (layout/templates, chassis, labels, rack, detail panel).
- How the rack recomputes its scale on resize.
- Tests: role derivation, automatic layout grouping, rack ordering with gaps,
  "unreachable shows nothing live", and the VLAN overflow label.
