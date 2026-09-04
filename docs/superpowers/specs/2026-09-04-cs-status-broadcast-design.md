# CS Status Broadcast — Design

## 1. Masalah

TikMan can post an update to a WhatsApp Channel the team's number administers
(see `2026-09-04-cs-channel-broadcast-design.md`, shipped and running). But on
2026-09-04 both paired numbers' channels had **zero subscribers**, while their
contact stores held **2,575 and 1,023 contacts**. An outage announcement posted
to a channel today reaches nobody.

WhatsApp Status reaches those contacts. It is not a message: nothing lands in
anyone's chat, no conversation is created, no chat notification fires. It
appears in the viewer's Status/Updates tab and disappears after 24 hours.

Announcing an outage should mean writing it once and choosing where it goes —
not writing it twice, and not remembering which surface was updated.

## 2. Keputusan yang melandasi desain

- **One action, two destinations.** A single composer with destination
  checkboxes, not two buttons. An outage announcement usually belongs in both
  places, and typing it twice is how one of them ends up forgotten.
- **One row per destination.** An announcement to both writes two
  `wa_broadcast_posts` rows. Partial failure then reads as what it is — channel
  sent, status failed with its reason — without forcing one row to carry two
  state machines and two message ids.
- **The existing table is generalized, not duplicated.** `wa_channel_posts`
  becomes `wa_broadcast_posts` with a `destination` column. The alternative — a
  parallel `wa_status_posts` with its own drainer — would be the *third* copy of
  a drain loop the channel feature's final review already flagged as
  duplication. This closes that debt instead of adding to it. The migration is
  near-free: the table holds one row.
- **`destination_jid` is empty for a status**, and a check constraint requires
  it only for channels. A status has no target beyond its own account, which
  `wa_account_id` already names; storing `status@broadcast` on every row would
  disguise the fact that the two destinations are not the same shape.
- **No expiry column.** The "sudah kedaluwarsa" note is computed on screen from
  `sent_at` + 24 hours. A stored column would claim we know something we only
  calculate.
- **Which number posts is chosen at send time**, not fixed in settings. A
  service outage may belong on CS Utama while a billing notice belongs on WA
  Billing.
- **Access is unchanged from the channel feature:** Admin, CS and Technician —
  every role that can open the inbox. Deliberately the same rule for both
  destinations, so the modal has one story rather than two.

## 3. Risiko yang diterima

- **A status cannot be withdrawn from TikMan.** Once posted it is visible to
  every viewer until it expires; removing it early means deleting it from the
  phone that holds the pairing. Deleting a status is out of scope.
- **Reach is not ours to control.** Who sees a status is decided by that
  number's status-privacy setting on its phone — all contacts, all except some,
  or a named list. TikMan reads it through WhatsApp when sending and never
  changes it. If reach is wrong, the setting is wrong, not this feature.
- **Reach depends on the contact store.** whatsmeow resolves recipients from
  its own contact store, filled by app-state sync. An incomplete store means a
  status reaches fewer people, and nothing reports that.
- **No pacing within a single status.** One status is one `SendMessage` that
  builds an encrypted copy for every recipient internally. The drainer's pace
  still separates one queued row from the next, but nothing can space out the
  fan-out inside one post.
- **Ban risk is real but ordinary.** Posting a status from an unofficial client
  performs the same per-recipient encryption the official app performs when a
  user posts a status. Once or twice a day is normal traffic, not the bulk
  pattern that gets numbers flagged. This is a smaller risk than sending
  individual messages to the same contacts, not a larger one.

## 4. Arsitektur

Nothing new crosses the process boundary. The API still holds no WhatsApp
connection: it writes rows and announces on `cs:outbox`, and the `wa` process
sends. What changes is that the drainer now has two destinations to send to.

**Sending a status** is `SendMessage(ctx, types.StatusBroadcastJID, msg)`.
whatsmeow routes `status@broadcast` through its group/broadcast path
(`send.go:403`), resolving recipients itself via `getBroadcastListParticipants`
→ `getStatusBroadcastRecipients` → `GetStatusPrivacy` plus the contact store.
Nothing about recipients is TikMan's to compute or store.

Two differences from the channel path, and only two:
- **Status media is encrypted**, so it uses the ordinary `Upload` the chat path
  uses, not `UploadNewsletter`, and carries no `MediaHandle`.
- **Status accepts text, image and video only** — no documents.

`ChannelSender` becomes `BroadcastSender` with four methods (text and media, per
destination). `ChannelDrainer` becomes `BroadcastDrainer`, whose `send` switches
on the row's `destination`. One drain loop serves both.

The SSE event is renamed too: `EventChannelPost` becomes `EventBroadcastPost`
and its wire string becomes `"broadcast_post"`, because a status update
announced as `"channel_post"` would be a lie in the one place a future reader
would trust it. The string does move, and the cost is bounded: during the
seconds between the two containers restarting, an event either side does not
recognise is ignored, costing one missed refetch that the next event closes.

## 5. Skema database

Migration `49_cs_broadcast_posts.sql`. AutoMigrate cannot rename, so unlike
migration 48 this file does structural work:

```
ALTER TABLE wa_channel_posts RENAME TO wa_broadcast_posts;
ALTER TABLE wa_broadcast_posts RENAME COLUMN channel_jid TO destination_jid;
ALTER TABLE wa_broadcast_posts ALTER COLUMN destination_jid DROP NOT NULL;
ALTER TABLE wa_broadcast_posts ADD COLUMN destination varchar(20) NOT NULL DEFAULT 'channel';
ALTER TABLE wa_broadcast_posts ALTER COLUMN destination DROP DEFAULT;
```

The default exists only to give the one existing row a value, and is dropped
immediately so no future insert can omit the column silently.

Constraints, replacing migration 48's equivalents under their new names:

| constraint | rule |
| --- | --- |
| `wa_broadcast_posts_destination_valid` | `destination IN ('channel','status')` |
| `wa_broadcast_posts_status_valid` | `status IN ('queued','sent','failed')` |
| `wa_broadcast_posts_jid_matches_destination` | `(destination = 'channel' AND destination_jid IS NOT NULL) OR (destination = 'status' AND destination_jid IS NULL)` |
| `fk_wa_broadcast_posts_account` | RESTRICT to `wa_accounts(id)`, as before |
| `idx_wa_broadcast_posts_queued` | partial index on `(wa_account_id, created_at) WHERE status = 'queued'` |

The third constraint is the one carrying the design: it makes "a status has no
target beyond its account" a rule the database enforces rather than a
convention the code remembers.

`wa_channels` is untouched — it mirrors channels, and a status has no mirror.

`models.WAChannelPost` becomes `models.WABroadcastPost`, gaining
`Destination BroadcastDestination` and a nullable `DestinationJID *string`.

## 6. Hak akses

Unchanged: the `/api/v1/cs` group's existing
`RequireRole(Admin, CS, Technician)` is the whole gate, for both destinations.
No route-level check is added.

## 7. Endpoint

```
GET  /api/v1/cs/wa-channels            unchanged
POST /api/v1/cs/wa-channels/refresh    unchanged
GET  /api/v1/cs/broadcasts             riwayat terakhir, lintas tujuan
POST /api/v1/cs/broadcasts             kirim teks (JSON)
POST /api/v1/cs/broadcasts/media       kirim lampiran (multipart)
```

The `/channel-posts` routes are replaced, not kept alongside — nothing outside
this repository calls them, and leaving both would leave two ways to say the
same thing.

A send request names its destinations explicitly:

```json
{
  "body": "Ada pemeliharaan malam ini",
  "destinations": [
    {"type": "channel", "channel_id": "<wa_channels uuid>"},
    {"type": "status",  "wa_account_id": "<wa_accounts uuid>"}
  ]
}
```

Each entry becomes one row. An empty list is refused. A `channel_id` not in the
mirror is refused, as today, before anything is queued. A `status` entry whose
account is unknown is refused the same way. The multipart form carries its
destinations as repeated query parameters —
`?channel_id=<uuid>&status_account_id=<uuid>`, each repeatable — for the reason
`sendMedia` already records: the body is wrapped in a size guard before
anything reads it, so a form field would have to be parsed ahead of that
guard.

## 8. Antarmuka

The header button becomes **"Pengumuman"** — a channel is now one destination
rather than the whole subject.

The modal opens with its destination section: a **Saluran** checkbox revealing
the channel picker, and a **Status WA** checkbox revealing the number list.
Both may be checked. Checking Status is impossible while a document is
attached, and the reason is stated there rather than after send.

The history stops being per-channel and becomes **the most recent announcements
across every destination**, each row labelled with where it went — "Saluran ·
PT Surya Bestari Lestari" or "Status · CS Utama". With one action able to reach
two places, a history filtered to one channel would hide half of what was just
sent.

Each row carries the status tag, the time, who posted it, the body or
attachment filename, and a failure reason when there is one. A status row past
24 hours keeps its **Terkirim** tag with a muted "sudah kedaluwarsa" beside it:
replacing the tag would erase that it once succeeded, and omitting the note
would leave old rows reading as though they were still live.

Sending still only queues. The modal says so, and the history is where the
outcome appears.

## 9. Penanganan galat

- **A document with a status destination** is refused at the API with 400
  before any row is written, and before the upload is stored — so no orphaned
  file is left behind. The UI prevents it first; the API is what guarantees it.
- **One destination succeeds, the other fails** — two rows, two outcomes, both
  visible. Neither hides the other.
- **The `wa` process is down** — rows stay `queued` and go out when it returns,
  including via the periodic sweep added by the channel feature's final review.
- **Status privacy cannot be read** (session up but the IQ fails) — the send
  fails and the row records WhatsApp's reason. Not silent.
- **Channel admin rights revoked** — unchanged: refused at the API against the
  mirror before queuing.

## 10. Struktur berkas

Backend:

- `migrations/49_cs_broadcast_posts.sql`
- `internal/models/cs_broadcast.go` (new): `WABroadcastPost`,
  `BroadcastDestination` (`DestinationChannel`, `DestinationStatus`).
  `cs_channel.go` keeps `WAChannel` alone — the channel mirror and the
  broadcast outbox are separate concepts and should not share a file.
- `internal/services/cs_channel_post_service.go` →
  `cs_broadcast_post_service.go`: `Queue` takes a destination; `ListRecent`
  replaces `ListFor`
- `internal/wa/status_send.go` — `SendStatusText`, `SendStatusMedia`
- `internal/wa/channel_drainer.go` → `broadcast_drainer.go`: `BroadcastSender`,
  `BroadcastDrainer`
- `internal/api/cs_handler_channels.go` → `cs_handler_broadcasts.go`
- edits: `models.go`, `router.go`, `events.go`, `cs_purge.go`,
  `cs_media_retention.go`, `cmd/wa/sessions.go`, `cmd/wa/main.go`

Frontend:

- `domain/entities/WaChannel.ts` — `BroadcastPost`, `BroadcastDestination`
- `application/hooks/useWaChannels.ts` — `useBroadcastPosts`, `useSendBroadcast`,
  `useSendBroadcastMedia`
- `application/hooks/useChannelBroadcast.ts` → `useBroadcast.ts`
- `presentation/components/cs/ChannelBroadcastModal.tsx` → `BroadcastModal.tsx`
- `presentation/components/cs/ChannelPostHistory.tsx` → `BroadcastHistory.tsx`
- edits: `InboxHeaderActions.tsx`, `CsRepository.ts`, `ICsRepository.ts`,
  `endpoints.ts`, `useCsStream.ts`

Both components stay props-driven, calling no hooks of their own — the pattern
every component under `components/cs/` follows and what keeps their tests plain
renders.

## 11. Pengujian

Go:

- `broadcast_drainer_test.go`: a `status` row is sent through the status sender
  and a `channel` row through the channel sender, told apart by `destination`;
  a partial failure across two rows leaves one sent and one failed with its
  reason; the existing concurrency and pace tests carry over unchanged.
- `cs_broadcast_post_service_test.go`: `Queue` refuses a channel row without a
  destination JID and a status row with one; `ListRecent` returns both
  destinations newest first.
- `cs_broadcast_post_postgres_test.go`: migration 49 preserves rows written
  before it. A row inserted under the old shape must survive the rename with
  `destination = 'channel'` and its `destination_jid` intact. This needs real
  Postgres — SQLite gets none of these constraints — so it joins the existing
  `TEST_POSTGRES_DSN` suite.
- `cs_handler_broadcasts_test.go`: a document with a status destination is
  refused, and leaves no file under `mediaRoot`; an empty destination list is
  refused; every inbox role may send and Viewer may not.

Frontend:

- The Status checkbox is disabled while a document is attached, and says why.
- A status row older than 24 hours shows both its Terkirim tag and the expiry
  note; a fresh one shows only the tag.
- A failed row shows its reason.

## 12. Dependensi baru

None. `go.mau.fi/whatsmeow` already carries `types.StatusBroadcastJID`, the
broadcast send path, and `GetStatusPrivacy`.

## 13. Di luar cakupan

View counts; deleting a status before it expires; scheduling; managing the
status-privacy setting from TikMan; and status text backgrounds or fonts.
