# CS Status Broadcast Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Post an announcement to a WhatsApp Status and/or a WhatsApp Channel from one composer in the CS inbox, with one outbox row per destination.

**Architecture:** The channel outbox is generalized rather than duplicated: `wa_channel_posts` becomes `wa_broadcast_posts` with a `destination` column, and the one drainer switches on it. The API still holds no WhatsApp connection — it writes rows and announces on Redis; the `wa` process sends. A status goes out as `SendMessage(ctx, types.StatusBroadcastJID, msg)`, whose recipients whatsmeow resolves itself.

**Tech Stack:** Go 1.25 (Gin, GORM, whatsmeow, Redis), React 18 + TypeScript (Ant Design, React Query, Vitest).

**Spec:** `docs/superpowers/specs/2026-09-04-cs-status-broadcast-design.md`

## Global Constraints

- **No new dependencies.** `go.mau.fi/whatsmeow v0.0.0-20260816113502-fb386f152837` already carries `types.StatusBroadcastJID`, the broadcast send path, and `GetStatusPrivacy`.
- **Non-test files: 350 lines max, functions 50 lines max, 3 levels of nesting max.** Test files may exceed 350 when the excess is individual test cases. Network-bound code in `internal/wa` that dials or constructs clients is exempt from the line limits. `main()` in `cmd/` is exempt from the function limit.
- **Comments explain why, never what.** Go doc comments on exported identifiers are required, short and factual.
- **User-facing strings are Indonesian.**
- **Tests assert behaviour**, never that a mock was called.
- **Backend gates:** `cd backend && gofmt -s -l . && go vet ./... && go test ./... -race`
- **Frontend gates:** `cd frontend && npm test -- --run && npm run lint && npm run format:check && npm run build`
- **Access control is unchanged:** the `/api/v1/cs` group already enforces `RequireRole(Admin, CS, Technician)`. No route in this plan adds another role check.
- **Commit messages end with exactly:**
  ```
  Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
  Claude-Session: https://claude.ai/code/session_011W57YMHYScTo9hPzpkqrY7
  ```

**A live-data warning that binds every task:** `wa_broadcast_posts` exists in production with one real row (an announcement sent 2026-09-04). Migration 49 renames a live table. Nothing in this plan may drop, recreate, or truncate it.

---

### Task 1: Migrasi 49 dan penggantian nama, tanpa perubahan perilaku

The rename breaks compilation until every reference moves, so this task does the whole mechanical change at once and adds the `destination` column. Behaviour does not change: every existing test must still pass, under its new names.

**Files:**
- Create: `backend/migrations/49_cs_broadcast_posts.sql`
- Create: `backend/internal/models/cs_broadcast.go`
- Modify: `backend/internal/models/cs_channel.go` (remove the post model, keep `WAChannel`), `backend/internal/models/models.go`
- Modify: `backend/internal/services/cs_channel_post_service.go` → rename file to `cs_broadcast_post_service.go`
- Modify: `backend/internal/services/cs_purge.go`, `cs_media_retention.go`
- Modify: `backend/internal/wa/channel_drainer.go` → rename file to `broadcast_drainer.go`
- Modify: `backend/internal/api/cs_handler_channels.go`, `backend/cmd/wa/sessions.go`, `backend/cmd/wa/main.go`
- Modify: every test file naming the renamed symbols
- Test: `backend/internal/services/cs_broadcast_post_postgres_test.go` (new)

**Interfaces:**
- Consumes: nothing.
- Produces: `models.WABroadcastPost` (table `wa_broadcast_posts`) with `Destination BroadcastDestination` and `DestinationJID *string`; `models.BroadcastDestination` (`DestinationChannel = "channel"`, `DestinationStatus = "status"`); `models.BroadcastPostStatus` (`BroadcastQueued`, `BroadcastSent`, `BroadcastFailed`); `services.CSBroadcastPostService`; `wa.BroadcastSender`, `wa.BroadcastDrainer`, `wa.NewBroadcastDrainer`.

- [ ] **Step 1: Write the migration**

Create `backend/migrations/49_cs_broadcast_posts.sql`:

```sql
-- Posting to a WhatsApp Status as well as a Channel. The outbox is
-- generalized rather than duplicated: one table, one drainer, one history.
--
-- Unlike migration 48, this file does structural work — AutoMigrate cannot
-- rename a table or a column, and it must not be allowed to "fix" the drift
-- by creating a second table beside the live one.

ALTER TABLE wa_channel_posts RENAME TO wa_broadcast_posts;
ALTER TABLE wa_broadcast_posts RENAME COLUMN channel_jid TO destination_jid;
ALTER TABLE wa_broadcast_posts ALTER COLUMN destination_jid DROP NOT NULL;

-- The default exists only to give rows written before this migration a value,
-- and is dropped immediately so no later insert can omit the column silently.
ALTER TABLE wa_broadcast_posts ADD COLUMN IF NOT EXISTS destination varchar(20) NOT NULL DEFAULT 'channel';
ALTER TABLE wa_broadcast_posts ALTER COLUMN destination DROP DEFAULT;

ALTER TABLE wa_broadcast_posts DROP CONSTRAINT IF EXISTS wa_channel_posts_status_valid;
ALTER TABLE wa_broadcast_posts DROP CONSTRAINT IF EXISTS wa_broadcast_posts_status_valid;
ALTER TABLE wa_broadcast_posts ADD CONSTRAINT wa_broadcast_posts_status_valid
    CHECK (status IN ('queued', 'sent', 'failed'));

ALTER TABLE wa_broadcast_posts DROP CONSTRAINT IF EXISTS wa_broadcast_posts_destination_valid;
ALTER TABLE wa_broadcast_posts ADD CONSTRAINT wa_broadcast_posts_destination_valid
    CHECK (destination IN ('channel', 'status'));

-- The rule the whole design rests on: a channel post names its channel, a
-- status post has no target beyond the account already on the row. Enforced
-- here rather than remembered in code.
ALTER TABLE wa_broadcast_posts DROP CONSTRAINT IF EXISTS wa_broadcast_posts_jid_matches_destination;
ALTER TABLE wa_broadcast_posts ADD CONSTRAINT wa_broadcast_posts_jid_matches_destination CHECK (
    (destination = 'channel' AND destination_jid IS NOT NULL)
    OR (destination = 'status' AND destination_jid IS NULL)
);

ALTER TABLE wa_broadcast_posts DROP CONSTRAINT IF EXISTS fk_wa_channel_posts_account;
ALTER TABLE wa_broadcast_posts DROP CONSTRAINT IF EXISTS fk_wa_broadcast_posts_account;
ALTER TABLE wa_broadcast_posts ADD CONSTRAINT fk_wa_broadcast_posts_account
    FOREIGN KEY (wa_account_id) REFERENCES wa_accounts(id) ON DELETE RESTRICT;

DROP INDEX IF EXISTS idx_wa_channel_posts_queued;
CREATE INDEX IF NOT EXISTS idx_wa_broadcast_posts_queued
    ON wa_broadcast_posts (wa_account_id, created_at)
    WHERE status = 'queued';
```

- [ ] **Step 2: Write the model**

Create `backend/internal/models/cs_broadcast.go`:

```go
package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// BroadcastDestination is where one announcement goes.
type BroadcastDestination string

const (
	DestinationChannel BroadcastDestination = "channel"
	DestinationStatus  BroadcastDestination = "status"
)

// BroadcastPostStatus is how far an announcement has travelled. Three, not the
// five a chat message has: neither a channel nor a status sends receipts.
type BroadcastPostStatus string

const (
	BroadcastQueued BroadcastPostStatus = "queued"
	BroadcastSent   BroadcastPostStatus = "sent"
	BroadcastFailed BroadcastPostStatus = "failed"
)

// WABroadcastPost is one announcement on its way out, and afterwards the
// record that it went.
//
// A row with Status BroadcastQueued is the outbox — there is no second table,
// the same decision CSMessage records. An announcement written while the wa
// process was down is still sitting here when it comes back.
//
// One announcement sent to two destinations is two rows, so a partial failure
// reads as what it is rather than forcing one row to carry two outcomes.
type WABroadcastPost struct {
	ID          uuid.UUID            `gorm:"type:uuid;primaryKey" json:"id"`
	WAAccountID uuid.UUID            `gorm:"type:uuid;not null;index" json:"wa_account_id"`
	Destination BroadcastDestination `gorm:"type:varchar(20);not null;index" json:"destination"`
	// DestinationJID is the channel this went to, and nil for a status: a
	// status has no target beyond WAAccountID. It is copied rather than
	// pointing at wa_channels, because the channel sync deletes and rebuilds
	// those rows and history must not follow them.
	DestinationJID *string             `gorm:"column:destination_jid;type:varchar(128);index" json:"destination_jid,omitempty"`
	SenderUserID   uuid.UUID           `gorm:"type:uuid;not null;index" json:"sender_user_id"`
	Kind           MessageKind         `gorm:"type:varchar(20);not null" json:"kind"`
	Body           string              `gorm:"type:text" json:"body"`
	MediaPath      string              `gorm:"type:text" json:"-"`
	MediaMime      string              `gorm:"type:varchar(100)" json:"media_mime,omitempty"`
	MediaSize      int64               `json:"media_size,omitempty"`
	MediaFilename  string              `gorm:"type:varchar(255)" json:"media_filename,omitempty"`
	Status         BroadcastPostStatus `gorm:"type:varchar(20);not null;index" json:"status"`
	FailReason     string              `gorm:"type:text" json:"fail_reason,omitempty"`
	WAMessageID    *string             `gorm:"type:varchar(128)" json:"wa_message_id,omitempty"`
	CreatedAt      time.Time           `json:"created_at"`
	SentAt         *time.Time          `json:"sent_at,omitempty"`
}

func (p *WABroadcastPost) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

// TableName specifies the table name for GORM.
func (WABroadcastPost) TableName() string { return "wa_broadcast_posts" }
```

- [ ] **Step 3: Strip the old model out of `cs_channel.go`**

Delete `WAChannelPost`, `ChannelPostStatus` and its constants, and `WAChannelPost`'s `BeforeCreate`/`TableName` from `backend/internal/models/cs_channel.go`. `WAChannel`, `ChannelRole` and their constants stay — that file is now only the channel mirror.

In `backend/internal/models/models.go`, replace `&WAChannelPost{},` with `&WABroadcastPost{},` in the `AutoMigrate` list.

- [ ] **Step 4: Move every reference**

This is mechanical. The symbol map:

| old | new |
| --- | --- |
| `models.WAChannelPost` | `models.WABroadcastPost` |
| `models.ChannelPostStatus` | `models.BroadcastPostStatus` |
| `models.ChannelPostQueued` | `models.BroadcastQueued` |
| `models.ChannelPostSent` | `models.BroadcastSent` |
| `models.ChannelPostFailed` | `models.BroadcastFailed` |
| `post.ChannelJID` | `post.DestinationJID` (now `*string`) |
| `services.CSChannelPostService` | `services.CSBroadcastPostService` |
| `services.NewCSChannelPostService` | `services.NewCSBroadcastPostService` |
| `services.ChannelPost` (input struct) | `services.BroadcastPost` |
| `wa.ChannelSender` | `wa.BroadcastSender` |
| `wa.ChannelDrainer` | `wa.BroadcastDrainer` |
| `wa.NewChannelDrainer` | `wa.NewBroadcastDrainer` |
| `drainChannelOutbox` | `drainBroadcastOutbox` |
| `removeChannelPosts` | `removeBroadcastPosts` |

Rename the files too: `services/cs_channel_post_service.go` → `cs_broadcast_post_service.go`, `services/cs_channel_post_service_test.go` → `cs_broadcast_post_service_test.go`, `wa/channel_drainer.go` → `broadcast_drainer.go`, `wa/channel_drainer_test.go` → `broadcast_drainer_test.go`. Use `git mv` so history follows.

**Where `ChannelJID` was read, the pointer needs handling.** The input struct keeps its `ChannelJID string` field throughout — Task 2 adds a `Destination` beside it rather than replacing it, so nothing here needs undoing later. In `Queue`, set:

```go
	post := models.WABroadcastPost{
		WAAccountID:    in.WAAccountID,
		Destination:    models.DestinationChannel,
		DestinationJID: &in.ChannelJID,
		...
	}
```

In `ListFor`, the query column becomes `destination_jid`. In the drainer's `send` and `announce`, dereference through a nil check that fails the row rather than panicking:

```go
func (d *BroadcastDrainer) send(ctx context.Context, post models.WABroadcastPost) (string, error) {
	if post.DestinationJID == nil {
		return "", fmt.Errorf("tujuan saluran tidak ada pada kiriman %s", post.ID)
	}
	jid := *post.DestinationJID
	if post.Kind == models.MessageKindText {
		return d.sender.SendChannelText(ctx, jid, post.Body)
	}
	return d.sender.SendChannelMedia(
		ctx, jid, post.Kind,
		filepath.Join(d.mediaRoot, post.MediaPath),
		post.MediaMime, post.MediaFilename, post.Body,
	)
}
```

`announce` publishes `ChannelID: ""` when the JID is nil; Task 3 removes that field entirely.

- [ ] **Step 5: Write the migration-preservation test**

Create `backend/internal/services/cs_broadcast_post_postgres_test.go`. Find how the existing Postgres suite is set up — `grep -n "setupPostgresTestDB" backend/internal/services/*_test.go` — and follow it exactly, including how it skips without `TEST_POSTGRES_DSN`.

The test inserts a row in the pre-migration shape, runs the migrations, and asserts the row survived:

```go
// Migration 49 renames a table that is live in production with real history in
// it. A rename that drops and recreates would lose announcements already sent,
// and SQLite — which has none of these constraints — would never catch it.
func TestMigration49PreservesExistingChannelPosts(t *testing.T) {
	db := setupPostgresTestDB(t)

	account := models.WAAccount{Label: "CS Utama", Status: models.WAAccountConnected}
	require.NoError(t, db.Create(&account).Error)

	jid := "120363000000000001@newsletter"
	before := models.WABroadcastPost{
		WAAccountID:    account.ID,
		Destination:    models.DestinationChannel,
		DestinationJID: &jid,
		SenderUserID:   uuid.New(),
		Kind:           models.MessageKindText,
		Body:           "Selamat datang di chanel SBL Network",
		Status:         models.BroadcastSent,
	}
	require.NoError(t, db.Create(&before).Error)

	var after models.WABroadcastPost
	require.NoError(t, db.First(&after, "id = ?", before.ID).Error)
	assert.Equal(t, models.DestinationChannel, after.Destination)
	require.NotNil(t, after.DestinationJID)
	assert.Equal(t, jid, *after.DestinationJID)
	assert.Equal(t, "Selamat datang di chanel SBL Network", after.Body)
}

// The constraint is the design: a status names no channel, a channel must.
func TestPostgresRefusesADestinationThatContradictsItsJID(t *testing.T) {
	db := setupPostgresTestDB(t)
	account := models.WAAccount{Label: "CS Utama", Status: models.WAAccountConnected}
	require.NoError(t, db.Create(&account).Error)

	jid := "120363000000000001@newsletter"
	statusWithJID := models.WABroadcastPost{
		WAAccountID: account.ID, Destination: models.DestinationStatus,
		DestinationJID: &jid, SenderUserID: uuid.New(),
		Kind: models.MessageKindText, Body: "x", Status: models.BroadcastQueued,
	}
	assert.Error(t, db.Create(&statusWithJID).Error)

	channelWithout := models.WABroadcastPost{
		WAAccountID: account.ID, Destination: models.DestinationChannel,
		SenderUserID: uuid.New(),
		Kind:         models.MessageKindText, Body: "x", Status: models.BroadcastQueued,
	}
	assert.Error(t, db.Create(&channelWithout).Error)
}
```

- [ ] **Step 6: Run the whole backend gate**

```bash
cd backend && gofmt -s -l . && go vet ./... && go test ./... -race
```
Expected: clean, and **every pre-existing test still passing** under the new names. If a test needed its assertions changed rather than just its symbols renamed, stop and report it — that means behaviour moved, and this task is supposed to move none.

If `TEST_POSTGRES_DSN` is not set, the new Postgres tests skip. Report that they skipped rather than claiming they passed.

- [ ] **Step 7: Commit**

```bash
git add -A backend/
git commit -m "refactor(cs): generalize the channel outbox into a broadcast outbox"
```

---

### Task 2: Layanan outbox dengan tujuan

**Files:**
- Modify: `backend/internal/services/cs_broadcast_post_service.go`
- Test: `backend/internal/services/cs_broadcast_post_service_test.go`

**Interfaces:**
- Consumes: `models.WABroadcastPost`, `models.BroadcastDestination`, `models.BroadcastPostStatus` (Task 1).
- Produces: `services.BroadcastPost` input struct with `WAAccountID uuid.UUID`, `Destination models.BroadcastDestination`, `ChannelJID string` (empty for a status), `SenderUserID uuid.UUID`, `Kind models.MessageKind`, `Body string`, `Media *MediaFile`; `Queue(in BroadcastPost) (*models.WABroadcastPost, error)`; `ListRecent(limit int) ([]models.WABroadcastPost, error)` replacing `ListFor`.

- [ ] **Step 1: Write the failing tests**

Append to `backend/internal/services/cs_broadcast_post_service_test.go`:

```go
// The database enforces this too, but SQLite in the unit suite does not — and
// a bad row would only fail hours later at the drainer, recorded as a failure
// nobody could act on.
func TestQueueRefusesAChannelPostWithoutItsChannel(t *testing.T) {
	posts, account, _ := postSetup(t)

	_, err := posts.Queue(BroadcastPost{
		WAAccountID:  account.ID,
		Destination:  models.DestinationChannel,
		SenderUserID: uuid.New(),
		Kind:         models.MessageKindText,
		Body:         "Ada pemeliharaan malam ini",
	})

	assert.Error(t, err)
}

// A status has no target beyond its account. Accepting a JID here would let two
// different things be stored in one shape and read back as the same thing.
func TestQueueRefusesAStatusPostThatNamesAChannel(t *testing.T) {
	posts, account, _ := postSetup(t)

	_, err := posts.Queue(BroadcastPost{
		WAAccountID:  account.ID,
		Destination:  models.DestinationStatus,
		ChannelJID:   infoGangguan,
		SenderUserID: uuid.New(),
		Kind:         models.MessageKindText,
		Body:         "Ada pemeliharaan malam ini",
	})

	assert.Error(t, err)
}

func TestQueueStoresAStatusPostWithoutAChannel(t *testing.T) {
	posts, account, _ := postSetup(t)

	post, err := posts.Queue(BroadcastPost{
		WAAccountID:  account.ID,
		Destination:  models.DestinationStatus,
		SenderUserID: uuid.New(),
		Kind:         models.MessageKindText,
		Body:         "Ada pemeliharaan malam ini",
	})

	require.NoError(t, err)
	assert.Equal(t, models.DestinationStatus, post.Destination)
	assert.Nil(t, post.DestinationJID)
	assert.Equal(t, models.BroadcastQueued, post.Status)
}

// One announcement to both destinations is two rows, and the history is where
// the sender sees both. Filtering to one channel would hide half of what they
// just sent.
func TestListRecentReturnsBothDestinationsNewestFirst(t *testing.T) {
	posts, account, db := postSetup(t)

	channelPost, err := posts.Queue(BroadcastPost{
		WAAccountID: account.ID, Destination: models.DestinationChannel,
		ChannelJID: infoGangguan, SenderUserID: uuid.New(),
		Kind: models.MessageKindText, Body: "ke saluran",
	})
	require.NoError(t, err)
	statusPost, err := posts.Queue(BroadcastPost{
		WAAccountID: account.ID, Destination: models.DestinationStatus,
		SenderUserID: uuid.New(),
		Kind:         models.MessageKindText, Body: "ke status",
	})
	require.NoError(t, err)
	require.NoError(t, db.Model(&models.WABroadcastPost{}).Where("id = ?", statusPost.ID).
		Update("created_at", statusPost.CreatedAt.Add(time.Second)).Error)

	recent, err := posts.ListRecent(10)
	require.NoError(t, err)
	require.Len(t, recent, 2)
	assert.Equal(t, statusPost.ID, recent[0].ID)
	assert.Equal(t, channelPost.ID, recent[1].ID)
}
```

Delete the old `ListFor` tests, and update `queued()` and any other helper in that file to pass `Destination: models.DestinationChannel` and `ChannelJID: infoGangguan`.

- [ ] **Step 2: Run to verify they fail**

```bash
cd backend && go test ./internal/services/ -run 'TestQueueRefuses|TestQueueStoresAStatus|TestListRecent' -v
```
Expected: FAIL to build — `unknown field Destination in struct literal`.

- [ ] **Step 3: Implement**

In `cs_broadcast_post_service.go`, replace the `ChannelPost` struct with:

```go
// BroadcastPost is one announcement as the caller supplies it, before it is a
// row. ChannelJID is empty for a status.
type BroadcastPost struct {
	WAAccountID  uuid.UUID
	Destination  models.BroadcastDestination
	ChannelJID   string
	SenderUserID uuid.UUID
	Kind         models.MessageKind
	Body         string
	Media        *MediaFile
}
```

and `Queue` with:

```go
// Queue writes an announcement as waiting to be sent. The wa process claims it,
// and one written while that process was down is still here when it comes back.
//
// The destination and the channel are checked against each other here as well
// as by the database, because the unit suite runs on SQLite, which has none of
// migration 49's constraints — and a contradictory row would otherwise only
// fail at the drainer, hours later, as a failure nobody could act on.
func (s *CSBroadcastPostService) Queue(in BroadcastPost) (*models.WABroadcastPost, error) {
	post := models.WABroadcastPost{
		WAAccountID:  in.WAAccountID,
		Destination:  in.Destination,
		SenderUserID: in.SenderUserID,
		Kind:         in.Kind,
		Body:         in.Body,
		Status:       models.BroadcastQueued,
	}

	switch in.Destination {
	case models.DestinationChannel:
		if in.ChannelJID == "" {
			return nil, fmt.Errorf("kiriman saluran harus menyebut salurannya")
		}
		jid := in.ChannelJID
		post.DestinationJID = &jid
	case models.DestinationStatus:
		if in.ChannelJID != "" {
			return nil, fmt.Errorf("status tidak menyebut saluran")
		}
	default:
		return nil, fmt.Errorf("tujuan tidak dikenal %q", in.Destination)
	}

	if in.Media != nil {
		post.MediaPath = in.Media.Path
		post.MediaMime = in.Media.Mime
		post.MediaFilename = in.Media.Filename
		post.MediaSize = in.Media.Size
	}
	if err := s.db.Create(&post).Error; err != nil {
		return nil, fmt.Errorf("queue broadcast post: %w", err)
	}
	return &post, nil
}
```

Replace `ListFor` with:

```go
// ListRecent returns the latest announcements across every destination,
// newest first. Not filtered by channel: one action can now reach two places,
// and a per-channel history would hide half of what was just sent.
func (s *CSBroadcastPostService) ListRecent(limit int) ([]models.WABroadcastPost, error) {
	if limit <= 0 || limit > defaultPostHistoryLimit {
		limit = defaultPostHistoryLimit
	}
	var rows []models.WABroadcastPost
	err := s.db.Order("created_at DESC").Limit(limit).Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list broadcast posts: %w", err)
	}
	return rows, nil
}
```

Update the two callers of `ListFor` (`cs_handler_channels.go` and the drainer test) to `ListRecent`.

- [ ] **Step 4: Run the tests**

```bash
cd backend && go test ./internal/services/ -race -v
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/services/ backend/internal/api/ backend/internal/wa/
git commit -m "feat(cs): let the outbox carry a destination"
```

---

### Task 3: Jalur kirim status dan drainer bercabang

**Files:**
- Create: `backend/internal/wa/status_send.go`
- Modify: `backend/internal/wa/broadcast_drainer.go`, `backend/internal/wa/events.go`
- Test: `backend/internal/wa/broadcast_drainer_test.go`

**Interfaces:**
- Consumes: `models.WABroadcastPost`, `models.DestinationChannel`/`DestinationStatus` (Task 1); `services.CSBroadcastPostService` (Task 2); existing `buildTextMessage`, `buildMediaMessage`, `uploadTypeFor`.
- Produces: `wa.BroadcastSender` with `SendChannelText`, `SendChannelMedia`, `SendStatusText(ctx, body string) (string, error)`, `SendStatusMedia(ctx context.Context, kind models.MessageKind, path, mime, filename, caption string) (string, error)`; `wa.EventBroadcastPost = "broadcast_post"`.

- [ ] **Step 1: Write the failing tests**

In `backend/internal/wa/broadcast_drainer_test.go`, extend `fakeChannelSender` — rename it `fakeBroadcastSender` — with the two status methods, recording which path was taken:

```go
func (f *fakeBroadcastSender) SendStatusText(_ context.Context, body string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if body == f.refuse {
		return "", errors.New("status privacy unavailable")
	}
	f.statusSent = append(f.statusSent, body)
	return "3EBSTATUS" + body, nil
}

func (f *fakeBroadcastSender) SendStatusMedia(
	_ context.Context, _ models.MessageKind, path, _, _, caption string,
) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.statusMediaPath = path
	f.statusSent = append(f.statusSent, caption)
	return "3EBSTATUSMEDIA", nil
}
```

Add `statusSent []string` and `statusMediaPath string` to the struct, and a helper that queues a status row:

```go
func queueStatus(t *testing.T, posts *services.CSBroadcastPostService, account models.WAAccount, body string) uuid.UUID {
	t.Helper()
	post, err := posts.Queue(services.BroadcastPost{
		WAAccountID:  account.ID,
		Destination:  models.DestinationStatus,
		SenderUserID: uuid.New(),
		Kind:         models.MessageKindText,
		Body:         body,
	})
	require.NoError(t, err)
	return post.ID
}
```

Then the tests:

```go
// The destination on the row is what decides the path. Sending a status
// through the channel sender would address it to a newsletter JID that does
// not exist and fail in a way nobody could read.
func TestAStatusRowGoesThroughTheStatusSender(t *testing.T) {
	drainer, sender, posts, account := channelDrainSetup(t)
	queueStatus(t, posts, account, "Ada pemeliharaan malam ini")

	sent, err := drainer.Drain(context.Background(), 10)

	require.NoError(t, err)
	assert.Equal(t, 1, sent)
	assert.Equal(t, []string{"Ada pemeliharaan malam ini"}, sender.statusSent)
	assert.Empty(t, sender.sent, "a status must not go through the channel sender")
}

// One announcement to two destinations is two rows. One destination refusing
// must not take the other down with it, and both outcomes must be readable.
func TestOneDestinationFailingLeavesTheOtherSent(t *testing.T) {
	drainer, sender, posts, account := channelDrainSetup(t)
	sender.refuse = "Ada pemeliharaan malam ini"
	queuePost(t, posts, account, "Ada pemeliharaan malam ini")
	queueStatus(t, posts, account, "ke status")

	_, err := drainer.Drain(context.Background(), 10)
	require.NoError(t, err)

	history, err := posts.ListRecent(10)
	require.NoError(t, err)
	require.Len(t, history, 2)
	byDestination := map[models.BroadcastDestination]models.WABroadcastPost{}
	for _, row := range history {
		byDestination[row.Destination] = row
	}
	assert.Equal(t, models.BroadcastFailed, byDestination[models.DestinationChannel].Status)
	assert.NotEmpty(t, byDestination[models.DestinationChannel].FailReason)
	assert.Equal(t, models.BroadcastSent, byDestination[models.DestinationStatus].Status)
	assert.Empty(t, byDestination[models.DestinationStatus].FailReason)
}

// A status attachment is read from where the API stored it, the same join the
// channel path depends on.
func TestAStatusMediaRowIsSentFromWhereTheUploadWasStored(t *testing.T) {
	drainer, sender, posts, account := channelDrainSetup(t)
	root := drainer.mediaRoot
	rel := filepath.Join("2026", "09", "pengumuman.jpg")
	require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(root, rel)), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(root, rel), []byte("jpegbytes"), 0o640))

	_, err := posts.Queue(services.BroadcastPost{
		WAAccountID: account.ID, Destination: models.DestinationStatus,
		SenderUserID: uuid.New(), Kind: models.MessageKindImage,
		Body:  "Ada pemeliharaan",
		Media: &services.MediaFile{Path: rel, Mime: "image/jpeg", Filename: "pengumuman.jpg", Size: 9},
	})
	require.NoError(t, err)

	_, err = drainer.Drain(context.Background(), 10)
	require.NoError(t, err)

	assert.Equal(t, filepath.Join(root, rel), sender.statusMediaPath)
}
```

Write that test as the **mixed** case, which is the one worth having: give the status row the body `"ke status"` so `refuse` (which matches on body) rejects only the channel row. Then assert `BroadcastFailed` with a non-empty reason for the channel row and `BroadcastSent` for the status row. A test where both fail proves less — it cannot tell "each row keeps its own outcome" apart from "the whole drain failed".

- [ ] **Step 2: Run to verify they fail**

```bash
cd backend && go test ./internal/wa/ -run TestAStatusRow -v
```
Expected: FAIL to build — `fakeBroadcastSender` does not implement the new interface, or `SendStatusText` undefined.

- [ ] **Step 3: Write the status send path**

Create `backend/internal/wa/status_send.go`:

```go
// Posting a WhatsApp Status. Unlike a channel update, a status is encrypted
// per recipient, and whatsmeow resolves those recipients itself from the
// account's status-privacy setting and its contact store — none of that is
// TikMan's to compute or store.
package wa

import (
	"context"
	"fmt"
	"os"

	"github.com/tikman/olt-provisioning/internal/models"
	"go.mau.fi/whatsmeow/types"
)

// SendStatusText posts a text status and answers the id WhatsApp gave it.
func (c *Client) SendStatusText(ctx context.Context, body string) (string, error) {
	resp, err := c.wa.SendMessage(ctx, types.StatusBroadcastJID, buildTextMessage(body, nil))
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}

// SendStatusMedia posts an attachment as a status.
//
// The upload is the ordinary encrypted one the chat path uses, not
// UploadNewsletter: only channel media travels unencrypted. There is no media
// handle to pass on for the same reason.
//
// The file is read whole, which is only safe because the upload boundary caps
// it: the API wraps the request body in a MaxBytesReader before a byte is
// stored.
func (c *Client) SendStatusMedia(
	ctx context.Context, kind models.MessageKind, path, mime, filename, caption string,
) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("baca lampiran: %w", err)
	}
	uploaded, err := c.wa.Upload(ctx, data, uploadTypeFor(kind))
	if err != nil {
		return "", fmt.Errorf("unggah lampiran: %w", err)
	}

	resp, err := c.wa.SendMessage(ctx, types.StatusBroadcastJID,
		buildMediaMessage(kind, uploaded, mime, filename, caption, nil))
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}
```

- [ ] **Step 4: Widen the interface and branch the drainer**

In `broadcast_drainer.go`, extend the interface:

```go
// BroadcastSender is the part of whatsmeow that posting an announcement needs.
// It is its own interface rather than an addition to Sender so the chat outbox
// and its tests are untouched by broadcast work.
type BroadcastSender interface {
	SendChannelText(ctx context.Context, channelJID, body string) (waMessageID string, err error)
	SendChannelMedia(
		ctx context.Context, channelJID string, kind models.MessageKind,
		path, mime, filename, caption string,
	) (waMessageID string, err error)
	SendStatusText(ctx context.Context, body string) (waMessageID string, err error)
	SendStatusMedia(
		ctx context.Context, kind models.MessageKind,
		path, mime, filename, caption string,
	) (waMessageID string, err error)
}
```

and replace `send` with a version that switches on the destination:

```go
func (d *BroadcastDrainer) send(ctx context.Context, post models.WABroadcastPost) (string, error) {
	switch post.Destination {
	case models.DestinationStatus:
		return d.sendStatus(ctx, post)
	case models.DestinationChannel:
		return d.sendChannel(ctx, post)
	default:
		return "", fmt.Errorf("tujuan tidak dikenal %q", post.Destination)
	}
}

func (d *BroadcastDrainer) sendStatus(ctx context.Context, post models.WABroadcastPost) (string, error) {
	if post.Kind == models.MessageKindText {
		return d.sender.SendStatusText(ctx, post.Body)
	}
	return d.sender.SendStatusMedia(
		ctx, post.Kind, filepath.Join(d.mediaRoot, post.MediaPath),
		post.MediaMime, post.MediaFilename, post.Body,
	)
}

func (d *BroadcastDrainer) sendChannel(ctx context.Context, post models.WABroadcastPost) (string, error) {
	if post.DestinationJID == nil {
		return "", fmt.Errorf("kiriman saluran tanpa saluran")
	}
	jid := *post.DestinationJID
	if post.Kind == models.MessageKindText {
		return d.sender.SendChannelText(ctx, jid, post.Body)
	}
	return d.sender.SendChannelMedia(
		ctx, jid, post.Kind, filepath.Join(d.mediaRoot, post.MediaPath),
		post.MediaMime, post.MediaFilename, post.Body,
	)
}
```

- [ ] **Step 5: Rename the event and drop the field nothing reads**

In `backend/internal/wa/events.go`, replace the `EventChannelPost` constant with:

```go
	// EventBroadcastPost says one announcement changed status. It carries no
	// identifier: the history is one recent list across every destination, so
	// the browser refetches all of it and there is nothing to narrow by.
	EventBroadcastPost = "broadcast_post"
```

and delete the `ChannelID` field from `Event`. Update `announce` in the drainer and `announceChannelPost` in the API handler to publish `Event{Type: EventBroadcastPost}`.

- [ ] **Step 6: Run the wa suite**

```bash
cd backend && go test ./internal/wa/ -race -v && go build ./...
```
Expected: PASS, including the pre-existing channel, concurrency and pace tests.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/wa/ backend/internal/api/
git commit -m "feat(cs): post a WhatsApp Status from the same outbox"
```

---

### Task 4: Endpoint /broadcasts

**Files:**
- Rename: `backend/internal/api/cs_handler_channels.go` → `cs_handler_broadcasts.go` (`git mv`)
- Rename: `backend/internal/api/cs_handler_channels_test.go` → `cs_handler_broadcasts_test.go`
- Modify: `backend/internal/api/router.go`, `backend/internal/api/cs_handler_test.go` (the `asUser` route table)

**Interfaces:**
- Consumes: `services.CSBroadcastPostService.Queue`/`ListRecent`, `services.BroadcastPost` (Task 2); existing `maxUploadBytes`, `refuseUpload`, `h.storeUpload`, `h.removeOrphanedUpload`, `kindForMime`, `validateAttachmentMime`, `wa.NormalizeMime`, `mapCSError`, `bindJSON`, `queryInt`, `middleware.GetUserID`.
- Produces: `ListBroadcasts`, `CreateBroadcast`, `CreateBroadcastMedia` on `*CSHandler`. `ListChannels` and `RefreshChannels` keep their names and routes.

- [ ] **Step 1: Write the failing tests**

Replace the channel-post tests in `cs_handler_broadcasts_test.go` with these, keeping the existing `adminChannel` helper and the role test (updated to the new path):

```go
// A status accepts text, image and video — never a document. Refusing at the
// API is what makes the UI's disabled checkbox a guarantee rather than a
// suggestion, and it must refuse before the upload is stored so no orphaned
// file is left behind.
func TestADocumentIsRefusedForAStatusDestination(t *testing.T) {
	env := setupCSHandler(t)

	req := uploadRequest(t,
		"/api/v1/cs/broadcasts/media?status_account_id="+env.account.ID.String(),
		"application/pdf", 32)
	rec := httptest.NewRecorder()
	env.asUser(env.cs, models.UserRoleCS).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var stored []models.WABroadcastPost
	require.NoError(t, env.db.Find(&stored).Error)
	assert.Empty(t, stored)

	left, err := filepath.Glob(filepath.Join(env.mediaRoot, "*", "*", "*"))
	require.NoError(t, err)
	assert.Empty(t, left, "a refused upload must leave no file behind")
}

// An announcement with nowhere to go is a mistake, not an empty success.
func TestABroadcastWithNoDestinationIsRefused(t *testing.T) {
	env := setupCSHandler(t)

	body := `{"body":"Ada pemeliharaan","destinations":[]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cs/broadcasts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.asUser(env.cs, models.UserRoleCS).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// One request, two destinations, two rows — that is what lets a partial
// failure be readable afterwards.
func TestOneRequestToBothDestinationsWritesTwoRows(t *testing.T) {
	env := setupCSHandler(t)
	channel := adminChannel(t, env)

	body := `{"body":"Ada pemeliharaan malam ini","destinations":[` +
		`{"type":"channel","channel_id":"` + channel.ID.String() + `"},` +
		`{"type":"status","wa_account_id":"` + env.account.ID.String() + `"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cs/broadcasts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.asUser(env.cs, models.UserRoleCS).ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	var stored []models.WABroadcastPost
	require.NoError(t, env.db.Order("destination").Find(&stored).Error)
	require.Len(t, stored, 2)
	assert.Equal(t, models.DestinationChannel, stored[0].Destination)
	require.NotNil(t, stored[0].DestinationJID)
	assert.Equal(t, channel.JID, *stored[0].DestinationJID)
	assert.Equal(t, models.DestinationStatus, stored[1].Destination)
	assert.Nil(t, stored[1].DestinationJID)
}

// A channel the mirror no longer lists is refused before anything is queued —
// including the status row that shared the request.
func TestAnUnknownChannelRefusesTheWholeRequest(t *testing.T) {
	env := setupCSHandler(t)

	body := `{"body":"Ada pemeliharaan","destinations":[` +
		`{"type":"channel","channel_id":"` + uuid.New().String() + `"},` +
		`{"type":"status","wa_account_id":"` + env.account.ID.String() + `"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cs/broadcasts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.asUser(env.cs, models.UserRoleCS).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)

	var stored []models.WABroadcastPost
	require.NoError(t, env.db.Find(&stored).Error)
	assert.Empty(t, stored, "no row may be written when any destination is refused")
}
```

- [ ] **Step 2: Run to verify they fail**

```bash
cd backend && go test ./internal/api/ -run TestOneRequestToBoth -v
```
Expected: FAIL with 404 — the route does not exist yet.

- [ ] **Step 3: Implement the handlers**

In `cs_handler_broadcasts.go`, replace `CreateChannelPost`/`CreateChannelPostMedia`/`ListChannelPosts` with:

```go
// BroadcastDestinationRequest names one place an announcement should go.
// Exactly one of the two id fields is used, decided by Type.
type BroadcastDestinationRequest struct {
	Type        string `json:"type" binding:"required"`
	ChannelID   string `json:"channel_id"`
	WAAccountID string `json:"wa_account_id"`
}

// CreateBroadcastRequest is one announcement as the composer sends it.
type CreateBroadcastRequest struct {
	Body         string                        `json:"body" binding:"required"`
	Destinations []BroadcastDestinationRequest `json:"destinations" binding:"required"`
}

// ListBroadcasts answers the most recent announcements across every
// destination, newest first.
func (h *CSHandler) ListBroadcasts(c *gin.Context) {
	rows, err := h.broadcasts.ListRecent(queryInt(c, "limit"))
	if err != nil {
		mapCSError(c, err, "BROADCAST_HISTORY_FAILED")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows})
}

// CreateBroadcast queues one text announcement per destination.
//
// Every destination is resolved before any row is written, so a request naming
// one good and one bad destination queues nothing — half an announcement is
// worse than none, because the sender would believe both went.
func (h *CSHandler) CreateBroadcast(c *gin.Context) {
	var req CreateBroadcastRequest
	if !bindJSON(c, &req) {
		return
	}

	body := strings.TrimSpace(req.Body)
	if body == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "pengumuman tidak boleh kosong", Code: "EMPTY_BROADCAST",
		})
		return
	}

	targets, ok := h.resolveDestinations(c, req.Destinations, models.MessageKindText)
	if !ok {
		return
	}

	userID, _ := middleware.GetUserID(c)
	queued, err := h.queueAll(targets, userID, models.MessageKindText, body, nil)
	if err != nil {
		mapCSError(c, err, "BROADCAST_FAILED")
		return
	}

	h.announceBroadcast(c)
	c.JSON(http.StatusCreated, gin.H{"data": queued})
}
```

The helpers, in the same file:

```go
// broadcastTarget is one resolved destination, ready to become a row.
type broadcastTarget struct {
	accountID   uuid.UUID
	destination models.BroadcastDestination
	channelJID  string
}

// resolveDestinations turns the request's destinations into targets, refusing
// the whole request if any one of them cannot be honoured — including a
// document aimed at a status, which WhatsApp does not accept.
func (h *CSHandler) resolveDestinations(
	c *gin.Context, requested []BroadcastDestinationRequest, kind models.MessageKind,
) ([]broadcastTarget, bool) {
	if len(requested) == 0 {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "pilih setidaknya satu tujuan", Code: "NO_DESTINATION",
		})
		return nil, false
	}

	targets := make([]broadcastTarget, 0, len(requested))
	for _, want := range requested {
		switch want.Type {
		case string(models.DestinationChannel):
			channel, ok := h.channelByID(c, want.ChannelID, "BROADCAST_FAILED")
			if !ok {
				return nil, false
			}
			targets = append(targets, broadcastTarget{
				accountID: channel.WAAccountID, destination: models.DestinationChannel,
				channelJID: channel.JID,
			})
		case string(models.DestinationStatus):
			if !statusAccepts(kind) {
				c.JSON(http.StatusBadRequest, ErrorResponse{
					Error: "status hanya menerima teks, gambar, dan video",
					Code:  "STATUS_KIND_NOT_ALLOWED",
				})
				return nil, false
			}
			account, ok := h.accountByID(c, want.WAAccountID)
			if !ok {
				return nil, false
			}
			targets = append(targets, broadcastTarget{
				accountID: account.ID, destination: models.DestinationStatus,
			})
		default:
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error: fmt.Sprintf("tujuan tidak dikenal %q", want.Type),
				Code:  "UNKNOWN_DESTINATION",
			})
			return nil, false
		}
	}
	return targets, true
}

// statusAccepts answers whether WhatsApp will take this kind as a status. A
// document will not go up, and refusing here is what stops one being stored.
func statusAccepts(kind models.MessageKind) bool {
	switch kind {
	case models.MessageKindText, models.MessageKindImage, models.MessageKindVideo:
		return true
	default:
		return false
	}
}

// queueAll writes one row per target.
func (h *CSHandler) queueAll(
	targets []broadcastTarget, userID uuid.UUID,
	kind models.MessageKind, body string, media *services.MediaFile,
) ([]models.WABroadcastPost, error) {
	queued := make([]models.WABroadcastPost, 0, len(targets))
	for _, target := range targets {
		post, err := h.broadcasts.Queue(services.BroadcastPost{
			WAAccountID:  target.accountID,
			Destination:  target.destination,
			ChannelJID:   target.channelJID,
			SenderUserID: userID,
			Kind:         kind,
			Body:         body,
			Media:        media,
		})
		if err != nil {
			return nil, err
		}
		queued = append(queued, *post)
	}
	return queued, nil
}
```

`accountByID` mirrors `channelByID`: parse the uuid, load through `h.accounts.Get`, answer 400 on a bad uuid and 404 on an unknown account.

`CreateBroadcastMedia` follows the shape `CreateChannelPostMedia` already has — `MaxBytesReader`, `c.FormFile`, `wa.NormalizeMime`, `validateAttachmentMime`, `h.storeUpload` — with one change in order: **resolve the destinations after the MIME is known but before `storeUpload` runs**, so a document aimed at a status is refused with no file written. Its destinations come from repeated query parameters: `c.QueryArray("channel_id")` and `c.QueryArray("status_account_id")`, each turned into a `BroadcastDestinationRequest`.

Rename `announceChannelPost` to `announceBroadcast`, publishing `wa.Event{Type: wa.EventBroadcastPost}` and the `cs:outbox` notice as before.

If the file passes 300 lines, move `CreateBroadcastMedia` and the upload helpers to `cs_handler_broadcasts_media.go`.

- [ ] **Step 4: Move the routes**

In `router.go`, replace the three `channel-posts` routes with:

```go
			cs.GET("/broadcasts", csHandler.ListBroadcasts)
			cs.POST("/broadcasts", csHandler.CreateBroadcast)
			cs.POST("/broadcasts/media", csHandler.CreateBroadcastMedia)
```

`wa-channels` and `wa-channels/refresh` are unchanged. Mirror all five in `cs_handler_test.go`'s `asUser` route table — that table is a second copy of the router, and a route missing from it makes tests pass against a surface production does not have.

- [ ] **Step 5: Run the API suite and the full gate**

```bash
cd backend && go test ./internal/api/ -race -v && gofmt -s -l . && go vet ./... && go test ./... -race
```
Expected: PASS throughout.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/api/
git commit -m "feat(cs): one request, one row per destination"
```

---

### Task 5: Frontend — tipe, repositori, hooks

**Files:**
- Modify: `frontend/src/domain/entities/WaChannel.ts`, `frontend/src/domain/repositories/ICsRepository.ts`
- Modify: `frontend/src/infrastructure/http/endpoints.ts`, `frontend/src/infrastructure/repositories/CsRepository.ts`
- Modify: `frontend/src/application/hooks/useWaChannels.ts`, `frontend/src/application/hooks/useCsStream.ts`

**Interfaces:**
- Consumes: the endpoints from Task 4.
- Produces: types `BroadcastDestination = "channel" | "status"`, `BroadcastPost` (`id`, `waAccountId`, `destination`, `destinationJid?`, `senderUserId`, `kind`, `body`, `mediaFilename?`, `status`, `failReason?`, `createdAt`, `sentAt?`); `BroadcastTarget = {type: "channel", channelId: string} | {type: "status", waAccountId: string}`; hooks `useBroadcasts()`, `useSendBroadcast()`, `useSendBroadcastMedia()`. `useWaChannels` and `useRefreshWaChannels` keep their names.

- [ ] **Step 1: Update the types**

In `frontend/src/domain/entities/WaChannel.ts`, rename `ChannelPost` to `BroadcastPost`, rename `ChannelPostStatus` to `BroadcastPostStatus`, and add:

```ts
/** Where one announcement goes. A channel names its channel; a status names
 * nothing beyond the number it goes out from. */
export type BroadcastDestination = "channel" | "status";

/** One destination as the composer asks for it. */
export type BroadcastTarget =
  | { type: "channel"; channelId: string }
  | { type: "status"; waAccountId: string };
```

`BroadcastPost` gains `destination: BroadcastDestination` and `destinationJid?: string`, and loses `channelJid`.

Update the barrel export in `frontend/src/domain/entities/index.ts`.

- [ ] **Step 2: Update the endpoints and repository**

In `endpoints.ts`, replace the three `CS_CHANNEL_POSTS*` entries with:

```ts
  CS_BROADCASTS: "/api/v1/cs/broadcasts",
  CS_BROADCASTS_MEDIA: "/api/v1/cs/broadcasts/media",
```

In `CsRepository.ts`, replace `getChannelPosts`, `sendChannelPost` and `sendChannelPostMedia` with:

```ts
  async getBroadcasts(): Promise<BroadcastPost[]> {
    const response = await apiClient.get(API_ENDPOINTS.CS_BROADCASTS);
    return response.data.data;
  }

  async sendBroadcast(
    body: string,
    targets: BroadcastTarget[],
  ): Promise<BroadcastPost[]> {
    const response = await apiClient.post(API_ENDPOINTS.CS_BROADCASTS, {
      body,
      destinations: targets.map(broadcastDestinationPayload),
    });
    return response.data.data;
  }

  async sendBroadcastMedia(
    file: File,
    caption: string,
    targets: BroadcastTarget[],
  ): Promise<BroadcastPost[]> {
    const form = new FormData();
    form.append("file", file);
    if (caption) {
      form.append("caption", caption);
    }

    // Destinations travel in the query string, not the form, for the reason
    // sendMedia records: the API wraps the body in a size guard before
    // anything reads it, and a form field would have to be parsed ahead of it.
    const params = new URLSearchParams();
    for (const target of targets) {
      if (target.type === "channel") params.append("channel_id", target.channelId);
      else params.append("status_account_id", target.waAccountId);
    }

    const response = await apiClient.post(
      `${API_ENDPOINTS.CS_BROADCASTS_MEDIA}?${params.toString()}`,
      form,
      // Dropping the header hands the boundary to the browser, which is the
      // only thing that knows it.
      { headers: { "Content-Type": false } },
    );
    return response.data.data;
  }
```

with, above the class:

```ts
/** The wire shape the API binds. Written by hand because apiClient's
 * decamelizeKeys never touches a nested array's object keys the way the
 * handler's binding tags expect. */
function broadcastDestinationPayload(target: BroadcastTarget) {
  return target.type === "channel"
    ? { type: "channel", channel_id: target.channelId }
    : { type: "status", wa_account_id: target.waAccountId };
}
```

**Verify that claim before relying on it:** read `frontend/src/infrastructure/http/apiClient.ts` and confirm what `decamelizeKeys` does to nested arrays of objects. If it converts them correctly, drop the helper and send camelCase — and say so in your report. Getting this wrong produces a request the backend rejects while the build and tests stay green.

Mirror the three methods in `ICsRepository.ts`.

- [ ] **Step 3: Update the hooks**

In `useWaChannels.ts`, replace `useChannelPosts`, `useSendChannelPost` and `useSendChannelPostMedia` with:

```ts
const BROADCASTS_KEY = ["cs", "broadcasts"];

/** The most recent announcements across every destination. This is where a
 * sender learns whether theirs actually went: sending only queues it. */
export function useBroadcasts() {
  return useQuery({
    queryKey: BROADCASTS_KEY,
    queryFn: () => csRepository.getBroadcasts(),
  });
}

export function useSendBroadcast() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (vars: { body: string; targets: BroadcastTarget[] }) =>
      csRepository.sendBroadcast(vars.body, vars.targets),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: BROADCASTS_KEY });
    },
    onError: reportCsMutationError,
  });
}

export function useSendBroadcastMedia() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (vars: { file: File; caption: string; targets: BroadcastTarget[] }) =>
      csRepository.sendBroadcastMedia(vars.file, vars.caption, vars.targets),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: BROADCASTS_KEY });
    },
    onError: reportCsMutationError,
  });
}
```

In `useCsStream.ts`, change the branch to match the new event and key:

```ts
      if (payload.type === "broadcast_post") {
        // Returns rather than falling through: an announcement belongs to no
        // conversation, and the refetches below would reload the whole inbox
        // for something that changed nothing in it.
        queryClient.invalidateQueries({ queryKey: ["cs", "broadcasts"] });
        return;
      }
```

- [ ] **Step 4: Verify**

```bash
cd frontend && npm run build && npm run lint && npm run format:check
```
Expected: build succeeds, zero lint errors, formatting clean. Existing tests will fail until Task 6 — that is expected, and Step 5 does not run them.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/domain frontend/src/infrastructure frontend/src/application
git commit -m "feat(cs): fetch and send announcements by destination"
```

---

### Task 6: Frontend — komposer dua tujuan

**Files:**
- Rename: `ChannelBroadcastModal.tsx` → `BroadcastModal.tsx`, `ChannelPostHistory.tsx` → `BroadcastHistory.tsx` (`git mv`)
- Rename: `useChannelBroadcast.ts` → `useBroadcast.ts`
- Rename: `__tests__/ChannelBroadcastModal.test.tsx` → `__tests__/BroadcastModal.test.tsx`
- Modify: `InboxHeaderActions.tsx`, `CsInboxPage.tsx`, `__tests__/CsInboxPageView.test.tsx`, `application/__tests__/useCsStream.test.ts`

**Interfaces:**
- Consumes: everything from Task 5.
- Produces: `BroadcastModal(props)` and `BroadcastHistory({ posts, loading, senderNames, accountLabels, channelNames })`, both props-driven and calling no hooks of their own.

- [ ] **Step 1: Write the failing tests**

In `__tests__/BroadcastModal.test.tsx`, keep the existing disabled-button and failure-reason tests (renaming the import and the send button's label if it changed), and add:

```tsx
// A document cannot be posted as a status. Disabling the checkbox is what
// makes that visible before the sender commits to it, rather than seconds
// later in the history.
it("refuses a status destination once a document is attached", async () => {
  open({ selectedChannelId: "c1", attachedKind: "document" });

  expect(screen.getByRole("checkbox", { name: /status wa/i })).toBeDisabled();
  expect(screen.getByText(/status hanya menerima teks, gambar, dan video/i)).toBeInTheDocument();
});

// One action can now reach two places, so the history has to say which is
// which — otherwise a partial failure is unreadable.
it("labels each history row with where it went", () => {
  open({
    posts: [
      { ...basePost, id: "p1", destination: "channel", destinationJid: "120363000000000001@newsletter" },
      { ...basePost, id: "p2", destination: "status" },
    ],
  });

  expect(screen.getByText(/Saluran · Info Gangguan/)).toBeInTheDocument();
  expect(screen.getByText(/Status · CS Utama/)).toBeInTheDocument();
});

// A status is gone after 24 hours. Dropping the Terkirim tag would erase that
// it once succeeded; dropping the note would leave old rows reading as live.
it("marks a status older than a day as expired while keeping its sent tag", () => {
  const twoDaysAgo = new Date(Date.now() - 48 * 60 * 60 * 1000).toISOString();
  open({
    posts: [{ ...basePost, id: "p3", destination: "status", status: "sent", sentAt: twoDaysAgo }],
  });

  expect(screen.getByText("Terkirim")).toBeInTheDocument();
  expect(screen.getByText(/sudah kedaluwarsa/i)).toBeInTheDocument();
});

// A channel post does not expire, so the note must not appear on one.
it("does not mark a channel post as expired", () => {
  const twoDaysAgo = new Date(Date.now() - 48 * 60 * 60 * 1000).toISOString();
  open({
    posts: [{
      ...basePost, id: "p4", destination: "channel",
      destinationJid: "120363000000000001@newsletter", status: "sent", sentAt: twoDaysAgo,
    }],
  });

  expect(screen.queryByText(/sudah kedaluwarsa/i)).toBeNull();
});
```

`basePost` is a module-level `BroadcastPost` with `waAccountId: "a1"`, `senderUserId: "u1"`, `kind: "text"`, `body: "Ada pemeliharaan malam ini"`, `status: "sent"`, `createdAt` set. `open()` grows props for `attachedKind`, `posts`, `channelNames` and `accountLabels`; give it defaults so the existing tests keep working unchanged.

- [ ] **Step 2: Run to verify they fail**

```bash
cd frontend && npm test -- --run BroadcastModal
```
Expected: FAIL — cannot resolve `../BroadcastModal`.

- [ ] **Step 3: Write the history component**

`BroadcastHistory.tsx` takes `{ posts, loading, senderNames, accountLabels, channelNames }` and renders each row with:

- a destination label: `Saluran · ${channelNames[post.destinationJid ?? ""] ?? "saluran tak dikenal"}` for a channel, `Status · ${accountLabels[post.waAccountId] ?? "nomor tak dikenal"}` for a status;
- the existing status `Tag` (`Antre`/`Terkirim`/`Gagal`) and time;
- the sender's name from `senderNames`, degrading to "pengguna tak dikenal";
- the body or, when empty, `mediaFilename`;
- `failReason` in danger text when present;
- for a **status** row whose `sentAt` is more than 24 hours old, a muted "sudah kedaluwarsa" beside the tag. Put the age rule in one exported helper so the test and the component agree on it:

```ts
/** A status disappears from WhatsApp after a day. Computed rather than stored:
 * a column would claim we know something we only calculate. */
const STATUS_LIFETIME_MS = 24 * 60 * 60 * 1000;

export function statusHasExpired(post: BroadcastPost, now = Date.now()): boolean {
  if (post.destination !== "status" || !post.sentAt) return false;
  return now - new Date(post.sentAt).getTime() > STATUS_LIFETIME_MS;
}
```

Keep the file under 150 lines.

- [ ] **Step 4: Write the modal**

`BroadcastModal.tsx` keeps its current composer — text area, upload, send button, the "sending only queues" Alert, and the history below — and gains a destination section above them:

- a `Checkbox` **Saluran** that reveals the existing channel `Select` and the Segarkan button;
- a `Checkbox` **Status WA** that reveals a `Checkbox.Group` of connected numbers, labelled from `accountLabels`;
- the Status checkbox is `disabled` when the attached file's kind is a document, with the reason rendered beside it: "Status hanya menerima teks, gambar, dan video";
- the send button is enabled only when at least one destination is chosen **and** there is text or a file.

`onSend` becomes `(body: string, targets: BroadcastTarget[], file?: File) => Promise<boolean>`. The modal builds the target list from its own checkbox state; the page decides what to do with it.

Keep the file under 250 lines. If the destination section pushes past that, extract it as `BroadcastDestinations.tsx` taking the same props and callbacks.

- [ ] **Step 5: Rewire the hook and the page**

`useBroadcast.ts` replaces `useChannelBroadcast.ts`: it owns the modal's open state, the selected channel, the selected status accounts, and calls `useWaChannels`, `useRefreshWaChannels`, `useBroadcasts`, `useSendBroadcast`, `useSendBroadcastMedia`. Its return type stays tied to the modal's props via `ComponentProps<typeof BroadcastModal>` — and the import must be `import type`, so the application layer gains no runtime edge to the presentation layer.

It also builds `channelNames` (JID → channel name) from the channel list, so the history can label a channel row without a second query.

In `CsInboxPage.tsx`, rename the hook call and the modal element. In `InboxHeaderActions.tsx`, the button label becomes **"Pengumuman"** and stays outside `isAdmin`.

Update `__tests__/CsInboxPageView.test.tsx`'s hook mock list to the new hook names, and `application/__tests__/useCsStream.test.ts` to the `broadcast_post` event and the `["cs","broadcasts"]` key.

- [ ] **Step 6: Run the full frontend gate**

```bash
cd frontend && npm test -- --run && npm run lint && npm run format:check && npm run build
```
Expected: all tests pass, zero lint errors, formatting clean, build succeeds.

- [ ] **Step 7: Check the page did not grow**

```bash
wc -l frontend/src/presentation/pages/CsInboxPage.tsx
```
It was 374 before this task. Report the number. If it grew past 374, say so rather than reaching into unrelated page logic to compensate — the file is already above the project's 350 limit for reasons that predate this feature.

- [ ] **Step 8: Commit**

```bash
git add frontend/src
git commit -m "feat(cs): choose where an announcement goes"
```

---

## Verifikasi akhir

```bash
cd backend && gofmt -s -l . && go vet ./... && go mod verify && go test ./... -race -cover
cd ../frontend && npm test -- --run && npm run lint && npm run format:check && npm run build
```

Then the file-size gate on everything created or grown:

```bash
find backend/internal backend/cmd frontend/src/presentation/components/cs \
     frontend/src/application/hooks -name '*.go' -o -name '*.tsx' -o -name '*.ts' \
  | xargs wc -l | sort -rn | head -20
```

No non-test file may exceed 350 lines except `CsInboxPage.tsx`, whose overage predates this work.

**The Postgres suite is not optional for this plan.** Migration 49 renames a live table, and SQLite has none of its constraints — run the backend suite with `TEST_POSTGRES_DSN` set at least once and report that you did, or report plainly that the migration tests skipped.

Finally, run `graphify update .` to keep the knowledge graph current.
