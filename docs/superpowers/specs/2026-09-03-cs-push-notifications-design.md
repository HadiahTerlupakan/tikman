# CS Push Notifications — Design

## 1. Masalah

A CS only learns a WhatsApp message arrived by having the inbox tab open and
looking at it. Nothing reaches them if the tab is backgrounded, minimized, or
closed — the SSE stream (`useCsStream`) already carries a "message" event the
moment a customer writes, but it only ever triggers a React Query refetch,
never anything the CS can see without looking at that tab.

## 2. Keputusan yang melandasi desain

- **Siapa dinotifikasi:** setiap Admin, CS, dan Technician — the three roles
  the `/api/v1/cs/*` route group already admits — for every incoming customer
  message, with no per-assignment filtering. Simpler than presence-aware
  routing, and matches how the inbox is already described: one team, one
  inbox.
- **Isi notifikasi:** nama/nomor pelanggan plus cuplikan pesan, not a generic
  "pesan baru masuk". A CS decides whether to act from the OS notification
  itself.
- **Transport: Firebase Cloud Messaging**, chosen over a self-hosted VAPID Web
  Push implementation because Firebase may be reused for other features
  later. This pulls in a real external dependency (a Google-hosted service,
  the `firebase` npm package, and `firebase.google.com/go/v4`) that a
  from-scratch VAPID implementation would not have needed — accepted
  deliberately, not overlooked.
- **Navbar badge:** reuses the existing "awaiting reply" concept
  (`last_message_direction = 'in'`, already the "Belum dibalas" filter tab)
  rather than building a second, per-user "unread" tracking system. No new
  table, no "mark as read" endpoint. The count is global — how many
  conversations need a reply right now — not personal to the viewing CS.
- **Opt-in, not automatic:** permission is requested from an explicit control,
  never on page load. Browsers throttle auto-prompted permission requests,
  and a request the CS never asked for is the fastest way to get "Block"
  clicked once and never askable again.
- **Code ships ahead of real credentials.** Firebase config (frontend) and the
  service account (backend) are both read from environment configuration that
  can be empty. Empty means the feature is inert — `api` still starts, the
  frontend still builds and runs, nothing crashes — so implementation and
  testing do not block on the user's Firebase project existing yet.

## 3. Arsitektur

The trigger point already exists and does not change:
`backend/internal/wa/inbound.go` publishes `Event{Type: "message",
ConversationID, MessageID}` to the Redis channel `cs:events`
(`wa.EventsChannel`) the moment an inbound message is stored. Today the only
subscriber is `cs_handler_stream.go`, one subscription per open SSE
connection, relaying to that one browser tab.

This adds a second, single, long-lived subscriber inside the `api` process
itself (not a new container, not a new `cmd/`) — a background goroutine
started in `cmd/api/main.go` alongside the existing
`go wgService.RunStatusRefresher(...)` goroutine, following that same
run-until-process-exit shape rather than introducing a new
cancellation-context convention. On every `EventMessage`, it looks up the
conversation and message from the database (the same "event carries only an
id, the receiver fetches the rest" pattern the SSE side already uses) and
hands off to a notifier, which sends one push per registered device token
belonging to an Admin/CS/Technician user.

```
inbound.go --publish--> cs:events (Redis) --subscribe--> PushEventListener (new, in cmd/api)
                                                                |
                                                                v
                                                   PushNotifierService (new)
                                                     - fetch conversation + message
                                                     - build title/body/data
                                                     - PushService.TokensForRoles(...)
                                                     - push.Client.SendEach(...)
                                                     - PushService.RemoveTokens(invalid)
```

## 4. Data baru

One table, `push_subscriptions` — a device's FCM registration token, owned by
a user:

| column | type | notes |
|---|---|---|
| id | uuid PK | |
| user_id | uuid NOT NULL | FK → `users(id) ON DELETE CASCADE`, added in the numbered SQL migration, not a GORM tag — the same split this codebase already uses for `cs_conversations` and friends: AutoMigrate owns the column, a migration owns the constraint, because the constraint is only exercised under real Postgres. |
| fcm_token | varchar(1024) UNIQUE NOT NULL | one row per device/browser; re-registering the same token updates instead of duplicating |
| created_at, updated_at | timestamp | |

A user can hold several rows (several browsers/devices). Deleting a user
deletes their subscriptions with them.

## 5. Backend

**`internal/models/push_subscription.go`** — the `PushSubscription` model,
added to `AutoMigrate`.

**`backend/migrations/46_push_subscriptions.sql`** — adds the `user_id`
foreign key with `ON DELETE CASCADE`, mirroring migration 41's pattern.

**`internal/services/push_service.go`** — `PushService`:
- `Subscribe(userID uuid.UUID, token string) error` — upsert on the unique
  token.
- `Unsubscribe(userID uuid.UUID, token string) error` — deletes only a row
  matching both the token and the caller's own `userID`, so one CS cannot
  unregister another's device.
- `TokensForRoles(roles ...models.UserRole) ([]string, error)` — joins to
  `users`, returns distinct tokens for the given roles.
- `RemoveTokens(tokens []string) error` — bulk delete, used to drop tokens
  FCM reports as unregistered.

**`internal/push/client.go`** (new top-level package, sibling to `internal/wa`
— this wraps an external device/service client the way `internal/wa` wraps
whatsmeow, not business logic) — a thin wrapper around
`firebase.google.com/go/v4/messaging`:

```go
type Sender interface {
    SendEach(ctx context.Context, tokens []string, title, body string, data map[string]string) (invalidTokens []string, err error)
}
```

`Client` implements `Sender` against real Firebase, constructed at startup
from `FIREBASE_SERVICE_ACCOUNT_JSON_B64` (base64-encoded service account
JSON, decoded once). **If that env var is empty, `cmd/api` logs a warning,
skips constructing the client, and the listener goroutine is never started —
the rest of the API is unaffected.** This is what lets the feature ship
before the user's Firebase project exists.

A fake implementing `Sender` is what `internal/services` tests use — the same
shape as `services.FakePresence` already used for CS assignment tests.

**`internal/services/push_notifier_service.go`** — `PushNotifierService`,
holding a `push.Sender`, `PushService`, and read access to conversations and
messages:
- `NotifyIncomingMessage(ctx, conversationID, messageID uuid.UUID) error` —
  looks up the conversation (customer name or phone) and the message body,
  truncates the body to 120 runes (append "…" when truncated — matches how
  an OS notification body reads elsewhere, and keeps the payload well under
  FCM's 4KB per-message limit), calls
  `TokensForRoles(Admin, CS, Technician)`, calls `Sender.SendEach`, then
  `RemoveTokens` on whatever came back invalid. A send failure is logged and
  swallowed — same reasoning as `signReply`'s missing-name path: a missed
  notification is a smaller problem than blocking message storage on a
  third-party service being down.

**`internal/services/push_listener.go`** — `PushEventListener.Run(ctx)`:
subscribes to `wa.EventsChannel`, decodes each `wa.Event`, and calls
`NotifyIncomingMessage` when `Type == wa.EventMessage`. Skips entirely (never
subscribes) if the notifier has no working `Sender`.

**`internal/api/push_handler.go`** + DTOs — two endpoints, any authenticated
role (registering your own device is not a CS-specific action):
- `POST /api/v1/push/subscribe` — `{fcm_token}`, calls
  `PushService.Subscribe(currentUserID, token)`.
- `DELETE /api/v1/push/subscribe` — `{fcm_token}`, calls
  `PushService.Unsubscribe(currentUserID, token)`.

**Config** (`internal/config`) — one new field,
`FirebaseServiceAccountJSONB64`, read from `FIREBASE_SERVICE_ACCOUNT_JSON_B64`.
Documented in `.env.example` as optional.

## 6. Frontend

**`frontend/public/firebase-messaging-sw.js`** — a plain, unbundled static
file (Firebase's own requirement: the messaging service worker loads via
`importScripts`, not through Vite's module graph). Pinned to the same
`firebase` version the app installs. The `firebaseConfig` object is inlined
literally — these values are public by Firebase's own design, so committing
them is correct, not a leak.

**`frontend/src/infrastructure/firebase/messaging.ts`** — `initializeApp`,
`requestPushPermission()` (asks `Notification.requestPermission()`, registers
the service worker, calls `getToken(messaging, {vapidKey, ...})`, POSTs the
token via a new `PushRepository`), and an `onMessage()` foreground listener
that calls `showNotification` on the SW registration — FCM does not surface
foreground messages as OS notifications on its own.

This registration runs **once, at the app-shell level in `AppLayout`**, not
scoped to the CS Inbox page — a CS reading the OLT map should still see a
push arrive. It only runs for Admin/CS/Technician (the roles that can open
`/api/v1/cs/*` at all); a Viewer is never prompted.

**The opt-in control** — a button/toggle reading `Notification.permission`,
placed in the CS Inbox header next to `WaConnectionBadge` (where a CS already
looks to judge whether the inbox is working), calling
`requestPushPermission()` on click.

**Navbar badge (`AppLayout.tsx`)** — replaces the hardcoded `<Badge
count={0}>` with the length of `useCsConversations({ awaitingReply: true })`,
gated the same way (Admin/CS/Technician only; a Viewer's badge stays absent,
not zero).

**`useCsStream()` moves from `CsInboxPage` to `AppLayout`.** Both the badge
and the inbox page need live invalidation from the same SSE connection;
running it once at the shell level gives every page a current badge and
leaves `CsInboxPage` unchanged in behavior — it already relies on query
invalidation, not on holding the connection itself. This also means one SSE
connection per tab instead of a second one had the badge opened its own.

## 7. Prasyarat pengguna (di luar kode)

Firebase project registration is account-bound and cannot be done by an
agent. The user creates the project and a Web App registration, generates a
Web Push certificate (VAPID key), and generates a service account key, then
supplies:
- the `firebaseConfig` object → inlined into `firebase-messaging-sw.js` and a
  `src/shared/config/firebase.ts` constant (public values, committed)
- the VAPID key → same file (public)
- the service account JSON → base64-encoded into
  `FIREBASE_SERVICE_ACCOUNT_JSON_B64` in `.env` (radpro: root-owned, mode
  600, the same handling as `ENCRYPTION_KEY`)

Until these are supplied, the feature is inert by design (§5) — the rest of
this spec's code does not wait on them.

## 8. Error handling

- No Firebase config configured → listener never starts, `/push/subscribe`
  still accepts and stores tokens (harmless — nothing sends to them yet).
- A token FCM reports unregistered/invalid on send → deleted from
  `push_subscriptions` immediately, so the table self-cleans instead of
  accumulating dead rows.
- Notification permission denied by the browser → `requestPushPermission()`
  resolves to a state the opt-in control reflects (e.g. "Diblokir oleh
  browser"); no retry loop, since a denied permission can only be reversed
  from the browser's own site settings.
- A send failure not attributable to a specific token (Firebase unreachable,
  auth failure) is logged and does not affect message storage or the SSE
  path — push is additive, never load-bearing.

## 9. Pengujian

- `push_service_test.go` — subscribe upserts, unsubscribe only removes the
  caller's own token, `TokensForRoles` filters correctly (SQLite, in-memory,
  matching the rest of `internal/services`).
- `push_notifier_service_test.go` — a fake `Sender` records what it was
  asked to send; asserts title/body/data shape, asserts invalid tokens
  returned by the fake are removed via `PushService`.
- `push_listener_test.go` — an event with `Type != "message"` is ignored; a
  `message` event triggers exactly one notify call.
- `push_handler_test.go` — subscribe/unsubscribe scoped to the authenticated
  user; unsubscribing someone else's token is a no-op, not an error that
  leaks whether that token exists.
- Frontend: a unit test for the badge (renders the awaiting-reply count,
  absent for Viewer) and for the opt-in control's three `Notification.
  permission` states ("default", "granted", "denied"). Actual push delivery
  end-to-end (a real browser receiving a real FCM push) is not something an
  agent can verify — noted as manual verification the user performs once
  real credentials are in place.

## 10. Struktur berkas

```
backend/
  internal/models/push_subscription.go
  internal/services/push_service.go
  internal/services/push_service_test.go
  internal/services/push_notifier_service.go
  internal/services/push_notifier_service_test.go
  internal/services/push_listener.go
  internal/services/push_listener_test.go
  internal/push/client.go
  internal/api/push_handler.go
  internal/api/push_handler_test.go
  migrations/46_push_subscriptions.sql

frontend/
  public/firebase-messaging-sw.js
  src/shared/config/firebase.ts
  src/infrastructure/firebase/messaging.ts
  src/domain/repositories/IPushRepository.ts
  src/infrastructure/repositories/PushRepository.ts
  src/application/hooks/usePushNotifications.ts
  src/presentation/components/cs/PushOptInButton.tsx
  src/presentation/components/cs/__tests__/PushOptInButton.test.tsx
  src/presentation/components/layout/AppLayout.tsx (modified)
  src/presentation/pages/CsInboxPage.tsx (modified — drop the now-lifted useCsStream call)
```

## 11. Di luar cakupan

- Presence-aware routing (only notify whoever is *not* currently looking).
- Suppressing the OS notification when the relevant tab already has focus.
- Per-user unread counts or a "mark as read" concept for the badge.
- Mobile app delivery — FCM's cross-platform reach is unused here; this spec
  is web-only.
