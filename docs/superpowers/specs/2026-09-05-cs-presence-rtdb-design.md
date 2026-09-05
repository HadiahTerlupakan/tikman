# CS Presence on Firebase Realtime Database — Design

**Date:** 2026-09-05
**Status:** Approved for planning
**Supersedes:** the Redis-polled presence panel shipped in `c02947c`

## Why

Presence today is a Redis key per agent with a sixty-second TTL, refreshed by
the SSE heartbeat every fifteen seconds. An agent who closes the tab, sleeps
the laptop or loses the network keeps counting as online for up to a minute.
The round-robin hands them work in that window, and the team panel shows them
at a desk they left.

Firebase Realtime Database answers this with `onDisconnect()`: the server drops
the node the moment the socket closes. Departure becomes immediate instead of
eventually-consistent within a minute.

Nothing else about presence is wrong, so nothing else changes.

## What already exists

- `firebase@12` in the frontend — RTDB is a module of a package already
  installed. No new dependency.
- `firebase.google.com/go/v4` in the backend, with a service account in
  `FIREBASE_SERVICE_ACCOUNT_JSON_B64`, used for FCM. `internal/push/client.go`
  builds the `firebase.App`; `Auth()` and `DatabaseWithURL()` come off that
  same app.
- `services.Presence` — an interface over `MarkOnline`, `Online`, `NextTurn`,
  implemented by `RedisPresence`, consumed by `CSAssignmentService`.
- `CsTeamPanel`, which takes `online: string[]` and renders the team. It does
  not change.

## What does not exist

- **No RTDB instance, and no `databaseURL`.** `firebase.ts` carries six keys
  for FCM only.
- **No Firebase Auth.** Zero `getAuth` or `signIn` in the source; sessions are
  cookies against the Go API. The browser therefore has no Firebase identity,
  and RTDB rules have no `auth.uid` to rest on.
- **No Firebase credentials in the `wa` service.** `docker-compose.yml` does
  not pass `FIREBASE_SERVICE_ACCOUNT_JSON_B64` to it.

That last point shapes the whole design. `AssignOne` — the only caller of
`Presence.Online` — runs in `wa` (`internal/wa/inbound.go:97`), and recreating
`wa` drops both WhatsApp sessions. Repeated reconnects are what get an
unofficial number blocked, so a design that forces `wa` to restart is paying a
real price for a presence feature.

## Architecture

RTDB is the single source of truth. `api` maintains a projection of it in the
Redis keys that already exist, so `wa` reads presence exactly as it does today
and is not touched.

```
browser (CS Inbox open)
   │  signInWithCustomToken
   │  set /cs-presence/{uid}, onDisconnect().remove()
   ▼
Firebase RTDB  ──────────────┐
   │  onValue                │  GetShallow, polled
   ▼                         ▼
browser (CsTeamPanel)     api (mirror)
                             │  SET present EX 60, DEL departed
                             ▼
                          Redis  ──►  wa (CSAssignmentService, unchanged)
```

The two arrows out of RTDB are not the same mechanism, and the difference is
forced rather than chosen. The browser SDK has real listeners, so the panel
updates the instant a node appears or vanishes. **The Go Admin SDK has none**:
`db.Ref` offers `Get`, `GetShallow`, `Set`, `Delete` and `Transaction` over
REST and nothing that streams. The mirror therefore polls.

Redis is a projection with exactly one writer, not a second truth. It is never
read back to decide what RTDB should contain.

### Failure behaviour

Decided by the operator: **when RTDB is unreachable, assignment stops.**

Two mechanisms, because one is not enough.

**Departure** is handled by deletion. Each poll writes `cs:online:{uid}` for
every agent in the snapshot and deletes the keys of agents no longer in it.
Letting a departed agent's key expire on its own instead would leave assignment
exactly as slow as it is today — the sixty-second TTL is the lag this whole
migration exists to remove, so the mirror must not lean on it for departures.

**Outage** is handled by the TTL. If `api` loses RTDB it stops both writing and
deleting; the surviving keys expire within sixty seconds, `Online()` returns
empty, and `AssignOne` leaves conversations unassigned for a CS to claim by
hand. No code detects the outage; the TTL does.

The panel is a separate matter and should not be described more kindly than it
is. `onValue` keeps its last snapshot when the socket drops — the SDK does not
empty it — so a browser that loses RTDB shows the team as it last saw them,
frozen, until the connection returns. It does not go blank and it does not know
it is stale. Deciding whether that needs a visible indicator is left to the
implementation plan; claiming it resolves itself would be false.

## Components

### 1. RTDB shape

```
/cs-presence/{userId}: { at: <ServerValue.TIMESTAMP> }
```

One shallow node per agent, removed by `onDisconnect()`. It carries no name and
no role: both live in Postgres, and copying them here would create a second
place for them to disagree.

### 2. Security rules

```json
{
  "rules": {
    "cs-presence": {
      ".read": "auth != null",
      "$uid": {
        ".write": "auth != null && auth.uid === $uid",
        ".validate": "newData.hasChildren(['at'])"
      }
    }
  }
}
```

Write is restricted to the agent's own node. Without this, the Firebase config —
which ships in the bundle by design — would let anyone mark another CS online or
empty the tree.

Rules are published before the code that depends on them ships.

### 3. Firebase identity

`GET /api/v1/auth/firebase-token`, behind the session middleware and the same
role gate as `/api/v1/cs` (Admin, CS, Technician). It mints a custom token for
the caller's own user id via `app.Auth().CustomToken(ctx, userID)` and returns
`{"data": {"token": "..."}}`.

The user id is taken from the session, never from the request, so a caller
cannot mint a token for somebody else.

The browser exchanges it with `signInWithCustomToken`. The SDK refreshes the
resulting ID token on its own; nothing schedules that.

When `FIREBASE_SERVICE_ACCOUNT_JSON_B64` is empty the endpoint answers `503`
with code `FIREBASE_NOT_CONFIGURED`, matching how the rest of the app treats an
absent Firebase project — inert, not broken.

### 4. Browser write

On the CS Inbox route only, under the same condition that sends `?presence=1`
today: someone reading the OLT map is not at the inbox and must not count.

Order matters. `onDisconnect().remove()` is registered *before* the value is
written, so a socket that dies between the two leaves nothing behind.

### 5. The mirror (`api`)

A goroutine started in `cmd/api/main.go` alongside the other startup wiring,
polling `GetShallow` on `/cs-presence` every fifteen seconds. `GetShallow`
returns the keys without their values, which is all the mirror needs and keeps
the response one line per agent.

Each pass writes `cs:online:{uid}` with the existing sixty-second TTL for every
agent in the snapshot, and deletes the keys of agents that were in the previous
snapshot but not this one.

Fifteen seconds is the honest bound on how stale assignment can be. The panel
is not bound by it — the browser has a real listener — so what the operator
sees and what the round-robin acts on can differ for up to that long. Making
them identical would need a streaming client the Go SDK does not provide.

When Firebase is not configured the mirror does not start, and `cs:online:*` is
never written by anything. Assignment then leaves everything unassigned, which
is the documented behaviour for an unconfigured deployment.

### 6. Panel data source

`useOnlineAgents` becomes an RTDB subscription (`onValue`) rather than a polled
query, returning the same `string[]`. `CsTeamPanel` is unchanged.

### 7. Deletions

Replaced code is removed, not left beside its replacement:

- `internal/api/cs_handler_online.go` and its test
- the `/online` route in `router.go` and in the test router in
  `cs_handler_test.go`
- `CS_ONLINE` in `endpoints.ts`, `getOnlineAgents` in `CsRepository` and
  `ICsRepository`
- `CS_PRESENCE_POLL_MS` in `limits.ts`
- `MarkOnline` from `cs_handler_stream.go`, the `?presence=1` query parameter,
  and `presence` from `CSHandler`'s dependencies

`MarkOnline` itself stays on `services.Presence`: writing a Redis key with its
TTL is exactly what the mirror does, so the method does not disappear — its
caller moves from the SSE stream to the mirror.

`Online` and `NextTurn` stay too. `NextTurn` is an `INCR` — a counter, for
which Redis is the right tool — and is not presence.

### 8. Configuration

`VITE_FIREBASE_DATABASE_URL` joins the six existing values, and must be added in
all three places the others appear: `firebase.ts`, `frontend/Dockerfile` as an
`ARG`/`ENV` pair, and `docker-compose.yml` under `build.args`. The comment in
`firebase.ts` records that missing one of these yields a bundle that silently
reads `""`.

`FIREBASE_DATABASE_URL` is added to the `api` service environment for the
mirror. It is **not** added to `wa`, which is the point of the design.

## Testing

- **Custom token endpoint:** mints for the session's own user; refuses roles
  that cannot open the inbox; answers `503` when Firebase is unconfigured.
- **Mirror:** given a snapshot of agent ids, writes exactly those keys with a
  TTL; a snapshot that drops an agent **deletes** that key rather than waiting
  for it to expire, which is the whole point of the migration; a snapshot that
  fails to load leaves the existing keys alone, so one failed poll does not
  empty the rotation. Tested against a fake snapshot source and the existing
  `FakePresence`, never against a real RTDB.
- **Assignment:** unchanged, and its existing tests must stay green untouched —
  that is the evidence `wa` was not disturbed.
- **Browser presence:** `onDisconnect` is registered before the write. Asserted
  against a fake RTDB handle; the Firebase SDK is not exercised in tests.
- **Panel:** already covered; its tests must pass unchanged, which is the
  evidence the data source swap did not change the component.

## Out of scope

- The 293 ONTs and other unrelated Firebase usage.
- Showing "last seen" times. The panel needs online or not; a timestamp would
  need a second design decision about what a stale one means.
- Moving `NextTurn` off Redis.

## Prerequisite on the operator

The RTDB instance does not exist. Before implementation can be verified, the
Firebase console needs a Realtime Database created and its URL supplied, and
the rules above published. Everything else in this spec is code.
