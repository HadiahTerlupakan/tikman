# CS Channel Broadcast — Design

## 1. Masalah

TikMan's WhatsApp numbers can be admins of WhatsApp Channels (Saluran), but
nothing in TikMan can post to one. Announcing an outage or a maintenance
window to subscribers means opening WhatsApp on the phone that holds the
pairing — outside the system that already knows about the outage, and
unavailable to a CS who does not have that phone.

The CS inbox is where the team already works and already holds the connection
to those numbers. It is the right place for the button.

## 2. Keputusan yang melandasi desain

- **Sasaran adalah Saluran, bukan grup.** A Channel is one-way: subscribers
  cannot reply. That is why a post needs no thread, no assignment, and no
  read receipts — and why it does not belong in the conversation list.
- **Isi kiriman: teks dan lampiran** (gambar, video, dokumen), matching what
  `MessageComposer` already offers for chat, rather than text alone.
- **Hak akses: setiap peran yang bisa membuka CS Inbox** — Admin, CS, and
  Technician, the three the `/api/cs/*` group already admits
  (`router.go:232`). No extra `RequireRole`. This is deliberately looser than
  the neighbouring "Balasan Cepat" and number management buttons, which are
  admin-only. Accepted with its consequence stated: a post reaches every
  subscriber and cannot be withdrawn.
- **Daftar saluran dicerminkan ke database, bukan ditanyakan saat diminta.**
  Listing channels needs a live WhatsApp connection, which lives in the `wa`
  process, not the API. Mirroring keeps the existing one-way control channel
  one-way. The alternative — request/response over Redis with correlation IDs
  and timeouts — would introduce a pattern the codebase does not have, make an
  HTTP request depend on another process being alive, and still need an outbox
  for sending. Channel admin rights change rarely, so a mirror that refreshes
  hourly is not meaningfully stale.
- **Pengiriman lewat outbox, seperti balasan chat.** A queued row is the
  outbox; the `wa` process drains it. This is not a new mechanism — it is the
  decision already recorded in the `CSMessage` doc comment, applied again.
- **Riwayat berstatus, bukan sekadar notifikasi.** Because sending is
  asynchronous, a toast can only ever say "queued". Without a history showing
  `sent`/`failed` and the reason, a sender never learns whether their
  announcement arrived. The history is what keeps the feature honest.

## 3. Risiko yang diterima

- **Saluran lewat klien tidak resmi.** whatsmeow's newsletter support is
  reverse-engineered. Posting adds surface for a number to be flagged, on top
  of the risk the CS module already accepts for chat.
- **Tidak ada pembatalan.** Once the drainer hands a post to WhatsApp it has
  reached every subscriber. A queued row could technically be cancelled, but
  the window is seconds; a cancel button that is almost always too late
  misleads more than its absence.
- **At-least-once.** A crash between WhatsApp accepting a post and the row
  being marked `sent` resends it on restart. Same trade as the message outbox,
  for the same reason: a duplicate is visible, a silent loss is not.

## 4. Arsitektur

Three flows, all on Redis channels that already exist. No new channel.

**Sinkronisasi daftar saluran (`wa` → DB).** When a number's session connects,
and hourly after that, `cmd/wa` calls `Client.AdminChannels`, which wraps
whatsmeow's `GetSubscribedNewsletters` and keeps only those whose
`ViewerMeta.Role` is `owner` or `admin` (`types.NewsletterRole`). The result
replaces that account's rows in `wa_channels`. Replacing rather than merging
means a channel whose admin rights were revoked disappears on its own, with no
separate removal logic.

The "Segarkan" button publishes `ControlMessage{Action: "sync-channels"}` on
`cs:control`, one message per account row — a fourth action beside `connect`,
`disconnect` and `delete`. `ControlMessage` keeps its current shape.

**Kirim pembaruan (API → DB → `wa`).** The handler writes one
`wa_channel_posts` row with status `queued` and announces on `cs:outbox`, the
channel already used to say a reply is waiting. On the `wa` side, `drainAll`
now empties two queues instead of one. The API never touches WhatsApp.

Sending uses whatsmeow's newsletter path: `SendMessage` to a JID whose server
is `types.NewsletterServer` dispatches to `sendNewsletter` (`send.go:411`).
Attachments are uploaded with `UploadNewsletter`, which returns the same
`UploadResponse` as `Upload` but with `MediaKey` and `FileEncSHA256` empty,
because channel media is not encrypted. `buildMediaMessage`
(`wa/media.go:257`) copies those fields from the response as-is, so it is
reused unchanged; the only addition is passing
`whatsmeow.SendRequestExtra{MediaHandle: up.Handle}`, which whatsmeow
documents as required when sending media to newsletters.

**Status kembali (`wa` → browser).** After each post is marked `sent` or
`failed`, the drainer publishes an `Event` on `cs:events`. `Event` gains one
field, `ChannelID`, for the same reason `WAAccountID` is already there: a
browser must know which channel the event is about. `useCsStream` turns it
into a React Query invalidation, exactly as it already does for conversations
(`useCsStream.ts:102`).

## 5. Skema database

Migration `48_cs_channels.sql`, following the flat numbering the repository
actually uses.

**`wa_channels`** — a mirror, not a source of truth. It may be dropped and
rebuilt at any time.

| kolom | tipe | catatan |
| --- | --- | --- |
| `id` | uuid pk | |
| `wa_account_id` | uuid not null | → `wa_accounts` |
| `jid` | varchar(128) not null | the `@newsletter` JID |
| `name` | varchar(255) | |
| `role` | varchar(20) not null | `owner` atau `admin` |
| `subscriber_count` | int | |
| `synced_at` | timestamptz not null | |

Unique on `(wa_account_id, jid)`.

**`wa_channel_posts`** — queue and history in one table, repeating the
decision recorded in the `CSMessage` doc comment: a `queued` row *is* the
outbox, there is no second table. A post written while the `wa` process was
down is still sitting here when it comes back.

| kolom | tipe | catatan |
| --- | --- | --- |
| `id` | uuid pk | |
| `wa_account_id` | uuid not null | |
| `channel_jid` | varchar(128) not null, idx | stored as text, deliberately not a foreign key, so history survives a sync that removes the `wa_channels` row |
| `sender_user_id` | uuid not null | |
| `kind` | varchar(20) not null | `text`/`image`/`video`/`document`/`audio` |
| `body` | text | |
| `media_path` | text | |
| `media_mime` | varchar(100) | |
| `media_size` | bigint | |
| `media_filename` | varchar(255) | |
| `status` | varchar(20) not null, idx | `queued`/`sent`/`failed` |
| `fail_reason` | text | |
| `wa_message_id` | varchar(128) | null until sent |
| `created_at` | timestamptz not null | |
| `sent_at` | timestamptz | |

Three statuses, not the five a chat message has: a channel sends no receipts,
so `delivered` and `read` will never arrive and must not be promised on screen.

## 6. Hak akses

The `/api/cs` group's existing `RequireRole(Admin, CS, Technician)` is the
whole gate. No route-level role check is added, and the button in the inbox
header is not wrapped in `isAdmin` — unlike "Balasan Cepat" beside it.

## 7. Endpoint

```
GET  /api/cs/wa-channels                     daftar saluran yang diadmin
POST /api/cs/wa-channels/refresh             minta sinkronisasi ulang
GET  /api/cs/channel-posts?channel_id=<uuid> riwayat satu saluran
POST /api/cs/channel-posts                   kirim teks (JSON)
POST /api/cs/channel-posts/media             kirim lampiran (multipart)
```

A request names a channel by the `wa_channels` row id, not by its JID. The
handler resolves it to `channel_jid` and `wa_account_id` and copies both onto
the post row, and refuses an id that is not in the mirror — so a channel whose
admin rights were revoked between the browser loading the list and the sender
pressing send is rejected at the API, before anything is queued.

Every segment is static; no `:id` parameters. That is deliberate. It leaves no
static-versus-parameter pair at the same position — the class of collision the
comment at `router.go:142` warns about.

Uploads reuse what `cs_handler_messages.go` already does: the body is wrapped
in `http.MaxBytesReader` with `maxUploadBytes`, the declared MIME is checked
against `wa.AllowedExtension` (which deliberately excludes html/svg/xml,
because the API serves these files back from its own origin), and the file is
written by `storeUpload` before the row is created.

## 8. Antarmuka

One button in the CS inbox header, beside "Balasan Cepat", visible to every
role that can open the page. It opens a modal with:

- a single channel picker listing every admin channel across every connected
  number, each row labelled with the channel name and the number that admins
  it, so the sender never picks a number separately;
- a text field and an attachment control;
- a "Segarkan" action for the channel list;
- below them, the history for the selected channel: who posted, when, and the
  status — with the failure reason shown when there is one.

The history is the part that carries the asynchrony. Pressing send queues;
the real outcome arrives seconds later over SSE.

An account with no admin channels contributes nothing to the list. When no
number has any, the modal shows an `Empty` stating the requirement: the number
must already be an admin of a channel, and TikMan cannot create one.

## 9. Penanganan galat

- **Admin rights revoked between sync and drain** — WhatsApp refuses, the row
  becomes `failed` with the reason, and the history shows it. Not silent.
- **`wa` process down** — posts stay `queued` and go out when it returns. The
  history shows "Antre", never a false success.
- **Number not paired** — that number's channels simply do not appear.
- **Rejected upload** — the existing MIME allowlist answers, with the existing
  error codes.

## 10. Struktur berkas

Backend:

- `internal/models/cs_channel.go` — `WAChannel`, `WAChannelPost`,
  `ChannelPostStatus`, `BeforeCreate`, `TableName`
- `migrations/48_cs_channels.sql`
- `internal/services/cs_channel_service.go` — `List`, `Replace`, `Get`
- `internal/services/cs_channel_post_service.go` — `Queue`, `ListFor`,
  `ClaimQueued`, `MarkSent`, `MarkFailed`
- `internal/api/cs_handler_channels.go` — the five handlers; the media handler
  moves to its own file if the total passes 300 lines
- `internal/wa/channels.go` — `Client.AdminChannels`
- `internal/wa/channel_send.go` — `Client.SendChannelText`,
  `Client.SendChannelMedia`
- `internal/wa/channel_drainer.go` — `ChannelDrainer` and the `ChannelSender`
  interface it sends through
- `cmd/wa/channels.go` — the hourly sync loop and the `sync-channels` action
- edits: `internal/wa/events.go` (one action constant, one `Event` field),
  `internal/api/router.go` (five routes), `cmd/wa/sessions.go` (drain both
  queues, start the sync loop)

Frontend:

- `domain/entities/WaChannel.ts`
- `infrastructure/repositories/CsRepository.ts` — five methods
- `application/hooks/useWaChannels.ts` — `useWaChannels`,
  `useRefreshWaChannels`, `useChannelPosts`, `useSendChannelPost`,
  `useSendChannelPostMedia`
- `presentation/components/cs/ChannelBroadcastModal.tsx`
- `presentation/components/cs/ChannelPostHistory.tsx` — separate from the
  start; together they would clearly exceed 350 lines
- `presentation/components/cs/InboxHeaderActions.tsx`
- edits: `CsInboxPage.tsx`, `useCsStream.ts`

`ChannelSender` is its own interface rather than an extension of `Sender`, so
the existing outbox tests are untouched by this work.

`InboxHeaderActions.tsx` is a targeted improvement to code this change already
touches: `CsInboxPage.tsx` is 380 lines, above the 350-line limit, and moving
the `PageHeader` `extra` block out brings it back under instead of pushing it
further over.

## 11. Pengujian

Go:

- `channel_drainer_test.go` with a fake `ChannelSender`, following
  `outbound_test.go`: a claim empties the queue, a post WhatsApp refuses is
  recorded with its reason and the drain continues to the next, and two
  concurrent drains never send the same row twice.
- `cs_channel_service_test.go`: `Replace` really replaces, so a channel whose
  admin rights were revoked disappears while its post history survives.
- `cs_handler_channels_test.go`: a MIME outside the allowlist is refused, an
  unknown `channel_id` is refused, and a Viewer cannot reach the routes.

Frontend:

- `ChannelBroadcastModal.test.tsx`: the send button is disabled until a channel
  is chosen, and a `failed` post shows its reason.

Each test asserts behaviour, not that a mock was called.

## 12. Dependensi baru

None. `go.mau.fi/whatsmeow` is already a dependency and already carries
`GetSubscribedNewsletters`, `UploadNewsletter`, and the newsletter send path.

## 13. Di luar cakupan

Editing or deleting a post after it is sent; reactions; creating a channel from
TikMan; showing view counts; scheduling posts.
