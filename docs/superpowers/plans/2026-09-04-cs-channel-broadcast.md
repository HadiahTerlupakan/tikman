# CS Channel Broadcast Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a button to the CS inbox that opens a modal for picking one of the WhatsApp Channels a paired number administers and posting an update to it, with a per-post status history.

**Architecture:** The API never touches WhatsApp. The `wa` process mirrors each number's admin channels into `wa_channels` on a timer, and posts are queued as `wa_channel_posts` rows that a drainer in that process claims and sends. Status flows back to the browser over the existing `cs:events` SSE stream.

**Tech Stack:** Go 1.25 (Gin, GORM, whatsmeow, Redis), React 18 + TypeScript (Ant Design, React Query, Vitest).

**Spec:** `docs/superpowers/specs/2026-09-04-cs-channel-broadcast-design.md`

## Global Constraints

- **No new dependencies.** `go.mau.fi/whatsmeow v0.0.0-20260816113502-fb386f152837` already carries `GetSubscribedNewsletters`, `UploadNewsletter`, and the newsletter send path.
- **File limit 350 lines, function limit 50 lines, max 3 levels of nesting.** Test files are exempt when the excess is individual test cases. Code that constructs `&gosnmp.GoSNMP{}` or dials directly is exempt; nothing in this plan does.
- **Comments explain why, never what.** Go doc comments on exported identifiers are required.
- **User-facing strings are Indonesian**, matching the rest of the CS module (`"tujuan tidak valid %q"`, `"percakapan ini sedang dipegang oleh %s"`).
- **Tests assert behaviour**, never that a mock was called.
- **Every task ends with a commit.** Commit messages end with:
  ```
  Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
  Claude-Session: https://claude.ai/code/session_011W57YMHYScTo9hPzpkqrY7
  ```
- **Backend gates:** `go test ./... -race`, `gofmt -s -l .` empty, `go vet ./...`.
- **Frontend gates:** `npm test -- --run`, `npm run lint`, `npm run format:check`, `npm run build`.
- **Roles:** the `/api/v1/cs` group already enforces `RequireRole(Admin, CS, Technician)`. No route in this plan adds a further role check.

---

### Task 1: Model saluran, migrasi, dan layanan cermin

**Files:**
- Create: `backend/internal/models/cs_channel.go`
- Create: `backend/migrations/48_cs_channels.sql`
- Create: `backend/internal/services/cs_channel_service.go`
- Modify: `backend/internal/models/models.go:5-29`
- Test: `backend/internal/services/cs_channel_service_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces: `models.WAChannel`, `models.WAChannelPost`, `models.ChannelRole` (`ChannelRoleOwner`, `ChannelRoleAdmin`), `models.ChannelPostStatus` (`ChannelPostQueued`, `ChannelPostSent`, `ChannelPostFailed`); `services.CSChannelService` with `NewCSChannelService(db *gorm.DB) *CSChannelService`, `List() ([]models.WAChannel, error)`, `Get(id uuid.UUID) (*models.WAChannel, error)`, `Replace(accountID uuid.UUID, channels []models.WAChannel) error`.

- [ ] **Step 1: Write the models**

Create `backend/internal/models/cs_channel.go`:

```go
package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ChannelRole is how much a paired number may do on a WhatsApp Channel. Only
// the two roles that may post are ever stored; whatsmeow also reports
// "subscriber" and "guest", which the sync drops.
type ChannelRole string

const (
	ChannelRoleOwner ChannelRole = "owner"
	ChannelRoleAdmin ChannelRole = "admin"
)

// ChannelPostStatus is how far an update has travelled. There are three, not
// the five a chat message has: a channel sends no receipts, so "delivered"
// and "read" would never arrive and must not be promised on screen.
type ChannelPostStatus string

const (
	ChannelPostQueued ChannelPostStatus = "queued"
	ChannelPostSent   ChannelPostStatus = "sent"
	ChannelPostFailed ChannelPostStatus = "failed"
)

// WAChannel is one WhatsApp Channel a paired number administers.
//
// It is a mirror of what WhatsApp answered, never a source of truth: the wa
// process replaces every row for an account on each sync, which is what makes
// a revoked admin right disappear without any removal logic.
type WAChannel struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	WAAccountID uuid.UUID `gorm:"type:uuid;not null;index" json:"wa_account_id"`
	// The column tag is load-bearing for the same reason it is on WAAccount:
	// GORM's naming strategy renders JID as "j_id", which is not the column
	// the migration and every query name.
	JID             string      `gorm:"column:jid;type:varchar(128);not null" json:"jid"`
	Name            string      `gorm:"type:varchar(255)" json:"name"`
	Role            ChannelRole `gorm:"type:varchar(20);not null" json:"role"`
	SubscriberCount int         `json:"subscriber_count"`
	SyncedAt        time.Time   `json:"synced_at"`
}

func (c *WAChannel) BeforeCreate(tx *gorm.DB) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return nil
}

// TableName specifies the table name for GORM.
func (WAChannel) TableName() string { return "wa_channels" }

// WAChannelPost is one update on its way to a channel, and afterwards the
// record that it went.
//
// A row with Status ChannelPostQueued is the outbox — there is no second
// table, the same decision CSMessage records. An update written while the wa
// process was down is still sitting here when it comes back.
//
// ChannelJID is copied rather than pointing at wa_channels, deliberately: the
// sync deletes and rebuilds those rows, and history must not follow them.
type WAChannelPost struct {
	ID            uuid.UUID         `gorm:"type:uuid;primaryKey" json:"id"`
	WAAccountID   uuid.UUID         `gorm:"type:uuid;not null;index" json:"wa_account_id"`
	ChannelJID    string            `gorm:"column:channel_jid;type:varchar(128);not null;index" json:"channel_jid"`
	SenderUserID  uuid.UUID         `gorm:"type:uuid;not null;index" json:"sender_user_id"`
	Kind          MessageKind       `gorm:"type:varchar(20);not null" json:"kind"`
	Body          string            `gorm:"type:text" json:"body"`
	MediaPath     string            `gorm:"type:text" json:"-"`
	MediaMime     string            `gorm:"type:varchar(100)" json:"media_mime,omitempty"`
	MediaSize     int64             `json:"media_size,omitempty"`
	MediaFilename string            `gorm:"type:varchar(255)" json:"media_filename,omitempty"`
	Status        ChannelPostStatus `gorm:"type:varchar(20);not null;index" json:"status"`
	FailReason    string            `gorm:"type:text" json:"fail_reason,omitempty"`
	WAMessageID   *string           `gorm:"type:varchar(128)" json:"wa_message_id,omitempty"`
	CreatedAt     time.Time         `json:"created_at"`
	SentAt        *time.Time        `json:"sent_at,omitempty"`
}

func (p *WAChannelPost) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

// TableName specifies the table name for GORM.
func (WAChannelPost) TableName() string { return "wa_channel_posts" }
```

- [ ] **Step 2: Register both models with AutoMigrate**

In `backend/internal/models/models.go`, add to the `db.AutoMigrate(...)` list, immediately after `&CSQuickReply{},`:

```go
		&WAChannel{},
		&WAChannelPost{},
```

- [ ] **Step 3: Write the migration**

Create `backend/migrations/48_cs_channels.sql`:

```sql
-- Posting updates to a WhatsApp Channel.
--
-- AutoMigrate creates both tables from the model tags and runs before this
-- file, so what is left here is everything a GORM tag cannot say — the same
-- split migration 41 describes for the rest of the CS module.

ALTER TABLE wa_channels DROP CONSTRAINT IF EXISTS wa_channels_role_valid;
ALTER TABLE wa_channels ADD CONSTRAINT wa_channels_role_valid
    CHECK (role IN ('owner', 'admin'));

-- One row per channel per number. The sync replaces an account's rows on every
-- pass; without this, a pass racing another would leave the same channel
-- listed twice in the picker and the sender would not know which to pick.
CREATE UNIQUE INDEX IF NOT EXISTS uq_wa_channels_account_jid
    ON wa_channels (wa_account_id, jid);

-- CASCADE: the mirror belongs to the number. Deleting a number should take its
-- channel list with it rather than be refused over rows WhatsApp can rebuild
-- at any time.
ALTER TABLE wa_channels DROP CONSTRAINT IF EXISTS fk_wa_channels_account;
ALTER TABLE wa_channels ADD CONSTRAINT fk_wa_channels_account
    FOREIGN KEY (wa_account_id) REFERENCES wa_accounts(id) ON DELETE CASCADE;

ALTER TABLE wa_channel_posts DROP CONSTRAINT IF EXISTS wa_channel_posts_status_valid;
ALTER TABLE wa_channel_posts ADD CONSTRAINT wa_channel_posts_status_valid
    CHECK (status IN ('queued', 'sent', 'failed'));

-- RESTRICT, matching cs_conversations in migration 41: deleting a number that
-- has broadcast history is a mistake worth refusing, and it is what keeps that
-- history readable.
ALTER TABLE wa_channel_posts DROP CONSTRAINT IF EXISTS fk_wa_channel_posts_account;
ALTER TABLE wa_channel_posts ADD CONSTRAINT fk_wa_channel_posts_account
    FOREIGN KEY (wa_account_id) REFERENCES wa_accounts(id) ON DELETE RESTRICT;

-- The drainer asks only for queued rows, and history grows without bound while
-- the queue stays near empty. A partial index keeps that claim cheap.
CREATE INDEX IF NOT EXISTS idx_wa_channel_posts_queued
    ON wa_channel_posts (wa_account_id, created_at)
    WHERE status = 'queued';
```

- [ ] **Step 4: Write the failing service test**

Create `backend/internal/services/cs_channel_service_test.go`:

```go
package services

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

func channelSetup(t *testing.T) (*CSChannelService, models.WAAccount) {
	t.Helper()
	db := setupTestDB(t)
	return NewCSChannelService(db), csAccount(t, db)
}

func channel(jid, name string, role models.ChannelRole) models.WAChannel {
	return models.WAChannel{JID: jid, Name: name, Role: role, SubscriberCount: 12}
}

// Replacing rather than merging is the whole reason a revoked admin right
// needs no removal path: the next sync simply does not mention that channel.
func TestReplaceDropsAChannelTheNumberNoLongerAdmins(t *testing.T) {
	channels, account := channelSetup(t)

	require.NoError(t, channels.Replace(account.ID, []models.WAChannel{
		channel("120363000000000001@newsletter", "Info Gangguan", models.ChannelRoleOwner),
		channel("120363000000000002@newsletter", "Promo", models.ChannelRoleAdmin),
	}))
	require.NoError(t, channels.Replace(account.ID, []models.WAChannel{
		channel("120363000000000001@newsletter", "Info Gangguan", models.ChannelRoleOwner),
	}))

	rows, err := channels.List()
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "120363000000000001@newsletter", rows[0].JID)
}

// One number's sync must not empty another's picker.
func TestReplaceLeavesAnotherNumbersChannelsAlone(t *testing.T) {
	db := setupTestDB(t)
	channels := NewCSChannelService(db)
	first := csAccount(t, db)
	second := models.WAAccount{Label: "CS Kedua", Status: models.WAAccountConnected}
	require.NoError(t, db.Create(&second).Error)

	require.NoError(t, channels.Replace(first.ID, []models.WAChannel{
		channel("120363000000000001@newsletter", "Info Gangguan", models.ChannelRoleOwner),
	}))
	require.NoError(t, channels.Replace(second.ID, []models.WAChannel{
		channel("120363000000000009@newsletter", "Kanal Kedua", models.ChannelRoleAdmin),
	}))
	require.NoError(t, channels.Replace(second.ID, nil))

	rows, err := channels.List()
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, first.ID, rows[0].WAAccountID)
}

// Replace stamps the account and a fresh sync time onto rows the caller did
// not fill in, so the sync code never has to remember to.
func TestReplaceStampsAccountAndSyncTime(t *testing.T) {
	channels, account := channelSetup(t)

	require.NoError(t, channels.Replace(account.ID, []models.WAChannel{
		channel("120363000000000003@newsletter", "Pemeliharaan", models.ChannelRoleAdmin),
	}))

	rows, err := channels.List()
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, account.ID, rows[0].WAAccountID)
	assert.False(t, rows[0].SyncedAt.IsZero())
	assert.NotEqual(t, uuid.Nil, rows[0].ID)
}

// Get is how a post request turns a picked id into the JID and the number it
// must be sent through. An id that is not in the mirror must not resolve.
func TestGetRefusesAnUnknownChannel(t *testing.T) {
	channels, _ := channelSetup(t)

	_, err := channels.Get(uuid.New())
	assert.Error(t, err)
}
```

- [ ] **Step 5: Run the test to verify it fails**

```bash
cd backend && go test ./internal/services/ -run TestReplace -v
```
Expected: FAIL to build — `undefined: NewCSChannelService`.

- [ ] **Step 6: Write the service**

Create `backend/internal/services/cs_channel_service.go`:

```go
package services

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// CSChannelService holds the mirror of the WhatsApp Channels each paired
// number administers. Nothing here talks to WhatsApp — the wa process does
// that and hands the answer to Replace.
type CSChannelService struct {
	db *gorm.DB
}

// NewCSChannelService constructs a CSChannelService.
func NewCSChannelService(db *gorm.DB) *CSChannelService {
	return &CSChannelService{db: db}
}

// List answers every channel the team may post to, across every number, named
// in the order the picker shows them.
func (s *CSChannelService) List() ([]models.WAChannel, error) {
	var rows []models.WAChannel
	if err := s.db.Order("name ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list channels: %w", err)
	}
	return rows, nil
}

// Get loads one channel. It is what turns a picked id into the JID and the
// number an update must be sent through, and what refuses an id that is no
// longer in the mirror.
func (s *CSChannelService) Get(id uuid.UUID) (*models.WAChannel, error) {
	var row models.WAChannel
	if err := s.db.First(&row, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("load channel: %w", err)
	}
	return &row, nil
}

// Replace swaps one number's channels for what WhatsApp just answered.
//
// The delete and the insert are one transaction: a sync that failed halfway
// would otherwise leave the picker empty for a number that still administers
// channels, and nothing would rebuild it until the next hour.
func (s *CSChannelService) Replace(accountID uuid.UUID, channels []models.WAChannel) error {
	syncedAt := time.Now()
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("wa_account_id = ?", accountID).Delete(&models.WAChannel{}).Error; err != nil {
			return fmt.Errorf("clear channels: %w", err)
		}
		if len(channels) == 0 {
			return nil
		}
		for i := range channels {
			channels[i].ID = uuid.Nil
			channels[i].WAAccountID = accountID
			channels[i].SyncedAt = syncedAt
		}
		if err := tx.Create(&channels).Error; err != nil {
			return fmt.Errorf("store channels: %w", err)
		}
		return nil
	})
}
```

- [ ] **Step 7: Run the tests to verify they pass**

```bash
cd backend && go test ./internal/services/ -run 'TestReplace|TestGetRefuses' -v
```
Expected: PASS, four tests.

- [ ] **Step 8: Commit**

```bash
git add backend/internal/models/cs_channel.go backend/internal/models/models.go \
        backend/migrations/48_cs_channels.sql \
        backend/internal/services/cs_channel_service.go \
        backend/internal/services/cs_channel_service_test.go
git commit -m "feat(cs): mirror the channels each WhatsApp number admins

...message body..."
```

---

### Task 2: Layanan antrean kiriman

**Files:**
- Create: `backend/internal/services/cs_channel_post_service.go`
- Test: `backend/internal/services/cs_channel_post_service_test.go`

**Interfaces:**
- Consumes: `models.WAChannelPost`, `models.ChannelPostStatus` (Task 1); `services.MediaFile` (existing, `cs_message_service.go:26`).
- Produces: `services.ChannelPost` (input struct: `WAAccountID uuid.UUID`, `ChannelJID string`, `SenderUserID uuid.UUID`, `Kind models.MessageKind`, `Body string`, `Media *MediaFile`); `services.CSChannelPostService` with `NewCSChannelPostService(db *gorm.DB) *CSChannelPostService`, `Queue(in ChannelPost) (*models.WAChannelPost, error)`, `ListFor(channelJID string, limit int) ([]models.WAChannelPost, error)`, `ClaimQueued(accountID uuid.UUID, limit int) ([]models.WAChannelPost, error)`, `MarkSent(id uuid.UUID, waMessageID string) error`, `MarkFailed(id uuid.UUID, reason string) error`.

- [ ] **Step 1: Write the failing test**

Create `backend/internal/services/cs_channel_post_service_test.go`:

```go
package services

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

const infoGangguan = "120363000000000001@newsletter"

func postSetup(t *testing.T) (*CSChannelPostService, models.WAAccount, *gorm.DB) {
	t.Helper()
	db := setupTestDB(t)
	return NewCSChannelPostService(db), csAccount(t, db), db
}

func queued(t *testing.T, s *CSChannelPostService, account models.WAAccount, body string) *models.WAChannelPost {
	t.Helper()
	post, err := s.Queue(ChannelPost{
		WAAccountID:  account.ID,
		ChannelJID:   infoGangguan,
		SenderUserID: uuid.New(),
		Kind:         models.MessageKindText,
		Body:         body,
	})
	require.NoError(t, err)
	return post
}

// A queued row is the outbox. Nothing has reached WhatsApp yet, and the row
// must say so rather than read as delivered.
func TestQueueStoresAnUpdateAsWaiting(t *testing.T) {
	posts, account, _ := postSetup(t)

	post := queued(t, posts, account, "Ada pemeliharaan malam ini")

	assert.Equal(t, models.ChannelPostQueued, post.Status)
	assert.Nil(t, post.WAMessageID)
	assert.Nil(t, post.SentAt)
}

// The drainer sends oldest first, so two announcements written in order do not
// reach subscribers reversed.
func TestClaimQueuedAnswersOldestFirst(t *testing.T) {
	posts, account, db := postSetup(t)

	first := queued(t, posts, account, "pertama")
	second := queued(t, posts, account, "kedua")
	// created_at is written by the database on insert; SQLite resolves both to
	// the same instant often enough that ordering has to be made deterministic.
	require.NoError(t, db.Model(&models.WAChannelPost{}).Where("id = ?", second.ID).
		Update("created_at", second.CreatedAt.Add(time.Second)).Error)

	waiting, err := posts.ClaimQueued(account.ID, 10)
	require.NoError(t, err)
	require.Len(t, waiting, 2)
	assert.Equal(t, first.ID, waiting[0].ID)
}

// A claim is scoped to the number, the same way the message outbox is: the
// session holding one number must never send another number's update.
func TestClaimQueuedIsScopedToItsNumber(t *testing.T) {
	posts, account, db := postSetup(t)
	other := models.WAAccount{Label: "CS Kedua", Status: models.WAAccountConnected}
	require.NoError(t, db.Create(&other).Error)

	queued(t, posts, account, "punya nomor pertama")

	waiting, err := posts.ClaimQueued(other.ID, 10)
	require.NoError(t, err)
	assert.Empty(t, waiting)
}

// A refusal must leave a reason on the row. Without it the sender watches an
// announcement disappear with nothing to act on.
func TestMarkFailedRecordsTheReason(t *testing.T) {
	posts, account, _ := postSetup(t)
	post := queued(t, posts, account, "Ada pemeliharaan malam ini")

	require.NoError(t, posts.MarkFailed(post.ID, "not authorized to post"))

	history, err := posts.ListFor(infoGangguan, 10)
	require.NoError(t, err)
	require.Len(t, history, 1)
	assert.Equal(t, models.ChannelPostFailed, history[0].Status)
	assert.Equal(t, "not authorized to post", history[0].FailReason)
}

// A retry that succeeds must not leave the previous refusal on screen beside
// a "sent" badge.
func TestMarkSentClearsAnEarlierFailure(t *testing.T) {
	posts, account, _ := postSetup(t)
	post := queued(t, posts, account, "Ada pemeliharaan malam ini")
	require.NoError(t, posts.MarkFailed(post.ID, "not authorized to post"))

	require.NoError(t, posts.MarkSent(post.ID, "3EB0F1"))

	history, err := posts.ListFor(infoGangguan, 10)
	require.NoError(t, err)
	require.Len(t, history, 1)
	assert.Equal(t, models.ChannelPostSent, history[0].Status)
	assert.Empty(t, history[0].FailReason)
	require.NotNil(t, history[0].WAMessageID)
	assert.Equal(t, "3EB0F1", *history[0].WAMessageID)
	assert.NotNil(t, history[0].SentAt)
}

// History is keyed by the JID, not by the wa_channels row, so a sync that
// rebuilds the mirror does not take the record of what was announced with it.
func TestHistorySurvivesTheChannelRowBeingRebuilt(t *testing.T) {
	posts, account, db := postSetup(t)
	channels := NewCSChannelService(db)
	require.NoError(t, channels.Replace(account.ID, []models.WAChannel{
		{JID: infoGangguan, Name: "Info Gangguan", Role: models.ChannelRoleOwner},
	}))
	queued(t, posts, account, "Ada pemeliharaan malam ini")

	require.NoError(t, channels.Replace(account.ID, nil))

	history, err := posts.ListFor(infoGangguan, 10)
	require.NoError(t, err)
	assert.Len(t, history, 1)
}
```

Add `"time"` to the import block — `TestClaimQueuedAnswersOldestFirst` uses it.

- [ ] **Step 2: Run the test to verify it fails**

```bash
cd backend && go test ./internal/services/ -run TestQueueStores -v
```
Expected: FAIL to build — `undefined: NewCSChannelPostService`.

- [ ] **Step 3: Write the service**

Create `backend/internal/services/cs_channel_post_service.go`:

```go
package services

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// defaultPostHistoryLimit is one screen of a channel's broadcast history.
const defaultPostHistoryLimit = 50

// ChannelPost is one update as the caller supplies it, before it is a row.
type ChannelPost struct {
	WAAccountID  uuid.UUID
	ChannelJID   string
	SenderUserID uuid.UUID
	Kind         models.MessageKind
	Body         string
	Media        *MediaFile
}

// CSChannelPostService is the outbox and the history of channel updates. They
// are one table: a queued row is what the drainer claims, and the same row is
// what the sender reads the outcome from afterwards.
type CSChannelPostService struct {
	db *gorm.DB
}

// NewCSChannelPostService constructs a CSChannelPostService.
func NewCSChannelPostService(db *gorm.DB) *CSChannelPostService {
	return &CSChannelPostService{db: db}
}

// Queue writes an update as waiting to be sent. The wa process claims it, and
// one written while that process was down is still here when it comes back.
func (s *CSChannelPostService) Queue(in ChannelPost) (*models.WAChannelPost, error) {
	post := models.WAChannelPost{
		WAAccountID:  in.WAAccountID,
		ChannelJID:   in.ChannelJID,
		SenderUserID: in.SenderUserID,
		Kind:         in.Kind,
		Body:         in.Body,
		Status:       models.ChannelPostQueued,
	}
	if in.Media != nil {
		post.MediaPath = in.Media.Path
		post.MediaMime = in.Media.Mime
		post.MediaFilename = in.Media.Filename
		post.MediaSize = in.Media.Size
	}
	if err := s.db.Create(&post).Error; err != nil {
		return nil, fmt.Errorf("queue channel post: %w", err)
	}
	return &post, nil
}

// ListFor returns one channel's broadcast history, newest first.
//
// Keyed by the JID rather than the wa_channels row id: the sync deletes and
// rebuilds those rows on every pass, and the record of what was announced
// must not disappear with them.
func (s *CSChannelPostService) ListFor(channelJID string, limit int) ([]models.WAChannelPost, error) {
	if limit <= 0 || limit > defaultPostHistoryLimit {
		limit = defaultPostHistoryLimit
	}
	var rows []models.WAChannelPost
	err := s.db.Where("channel_jid = ?", channelJID).
		Order("created_at DESC").Limit(limit).Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list channel posts: %w", err)
	}
	return rows, nil
}

// ClaimQueued returns the updates still waiting on one number, oldest first.
//
// Scoped to the account for the same reason the message outbox is: the session
// holding a number claims only its own work, so an update leaves through the
// number that actually administers the channel.
func (s *CSChannelPostService) ClaimQueued(accountID uuid.UUID, limit int) ([]models.WAChannelPost, error) {
	if limit <= 0 {
		limit = defaultPostHistoryLimit
	}
	var rows []models.WAChannelPost
	err := s.db.Where("status = ? AND wa_account_id = ?", models.ChannelPostQueued, accountID).
		Order("created_at ASC").Limit(limit).Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("claim queued channel posts: %w", err)
	}
	return rows, nil
}

// MarkSent records that WhatsApp accepted an update, and clears any reason a
// previous attempt left behind.
func (s *CSChannelPostService) MarkSent(id uuid.UUID, waMessageID string) error {
	sentAt := time.Now()
	return s.update(id, map[string]any{
		"status":        models.ChannelPostSent,
		"wa_message_id": waMessageID,
		"fail_reason":   "",
		"sent_at":       sentAt,
	})
}

// MarkFailed records why an update could not be sent, so the sender reads a
// reason rather than watching an announcement quietly disappear.
func (s *CSChannelPostService) MarkFailed(id uuid.UUID, reason string) error {
	return s.update(id, map[string]any{
		"status":      models.ChannelPostFailed,
		"fail_reason": reason,
	})
}

func (s *CSChannelPostService) update(id uuid.UUID, fields map[string]any) error {
	res := s.db.Model(&models.WAChannelPost{}).Where("id = ?", id).Updates(fields)
	if res.Error != nil {
		return fmt.Errorf("update channel post: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

```bash
cd backend && go test ./internal/services/ -run 'TestQueue|TestClaim|TestMark|TestHistory' -v
```
Expected: PASS, six tests.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/services/cs_channel_post_service.go \
        backend/internal/services/cs_channel_post_service_test.go
git commit -m "feat(cs): queue channel updates and keep what happened to them"
```

---

### Task 3: Membaca daftar saluran dari WhatsApp

**Files:**
- Create: `backend/internal/wa/channels.go`
- Test: `backend/internal/wa/channels_test.go`

**Interfaces:**
- Consumes: `models.WAChannel`, `models.ChannelRole` (Task 1).
- Produces: `wa.channelsFrom(metas []*types.NewsletterMetadata, accountID uuid.UUID) []models.WAChannel` (unexported, tested directly); `(*wa.Client).AdminChannels(ctx context.Context) ([]models.WAChannel, error)`.

- [ ] **Step 1: Write the failing test**

Create `backend/internal/wa/channels_test.go`:

```go
package wa

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"go.mau.fi/whatsmeow/types"
)

func newsletter(jid, name string, role types.NewsletterRole) *types.NewsletterMetadata {
	meta := &types.NewsletterMetadata{
		ID:         types.JID{User: jid, Server: types.NewsletterServer},
		ViewerMeta: &types.NewsletterViewerMetadata{Role: role},
	}
	meta.ThreadMeta.Name.Text = name
	meta.ThreadMeta.SubscriberCount = 240
	return meta
}

// GetSubscribedNewsletters answers everything the number follows, and most of
// it is followed as a plain subscriber. Keeping those would fill the picker
// with channels every post to would be refused.
func TestOnlyChannelsTheNumberCanPostToAreKept(t *testing.T) {
	account := uuid.New()

	kept := channelsFrom([]*types.NewsletterMetadata{
		newsletter("120363000000000001", "Info Gangguan", types.NewsletterRoleOwner),
		newsletter("120363000000000002", "Promo", types.NewsletterRoleAdmin),
		newsletter("120363000000000003", "Berita Tetangga", types.NewsletterRoleSubscriber),
		newsletter("120363000000000004", "Kanal Tamu", types.NewsletterRoleGuest),
	}, account)

	require.Len(t, kept, 2)
	names := []string{kept[0].Name, kept[1].Name}
	assert.ElementsMatch(t, []string{"Info Gangguan", "Promo"}, names)
	assert.Equal(t, account, kept[0].WAAccountID)
}

// ViewerMeta is a pointer in whatsmeow and is absent for a newsletter the
// server told us nothing about. Reading Role off it unguarded would panic and
// take the whole sync — and with it the session's goroutine — down.
func TestANewsletterWithoutViewerMetadataIsSkipped(t *testing.T) {
	kept := channelsFrom([]*types.NewsletterMetadata{
		{ID: types.JID{User: "120363000000000005", Server: types.NewsletterServer}},
		nil,
	}, uuid.New())

	assert.Empty(t, kept)
}

// The JID is what a post is addressed to, so it must be stored in the form
// whatsmeow parses back, not just the numeric part.
func TestTheStoredJIDIsAddressable(t *testing.T) {
	kept := channelsFrom([]*types.NewsletterMetadata{
		newsletter("120363000000000001", "Info Gangguan", types.NewsletterRoleOwner),
	}, uuid.New())

	require.Len(t, kept, 1)
	assert.Equal(t, "120363000000000001@newsletter", kept[0].JID)
	assert.Equal(t, models.ChannelRoleOwner, kept[0].Role)
	assert.Equal(t, 240, kept[0].SubscriberCount)
}
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
cd backend && go test ./internal/wa/ -run TestOnlyChannels -v
```
Expected: FAIL to build — `undefined: channelsFrom`.

- [ ] **Step 3: Write the implementation**

Create `backend/internal/wa/channels.go`:

```go
// Reading which WhatsApp Channels a number may post to. The list is mirrored
// into the database rather than asked for on demand, because the API process
// holds no WhatsApp connection.
package wa

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"go.mau.fi/whatsmeow/types"
)

// AdminChannels answers the channels this number administers.
func (c *Client) AdminChannels(ctx context.Context) ([]models.WAChannel, error) {
	metas, err := c.wa.GetSubscribedNewsletters(ctx)
	if err != nil {
		return nil, fmt.Errorf("baca daftar saluran: %w", err)
	}
	return channelsFrom(metas, c.accountID), nil
}

// channelsFrom keeps only what this number may actually post to.
//
// ViewerMeta is a pointer and is absent for a newsletter the server said
// nothing about; reading Role off it unguarded would panic inside the
// session's own goroutine.
func channelsFrom(metas []*types.NewsletterMetadata, accountID uuid.UUID) []models.WAChannel {
	out := make([]models.WAChannel, 0, len(metas))
	for _, meta := range metas {
		if meta == nil || meta.ViewerMeta == nil {
			continue
		}
		role, ok := postingRole(meta.ViewerMeta.Role)
		if !ok {
			continue
		}
		out = append(out, models.WAChannel{
			WAAccountID:     accountID,
			JID:             meta.ID.String(),
			Name:            meta.ThreadMeta.Name.Text,
			Role:            role,
			SubscriberCount: meta.ThreadMeta.SubscriberCount,
		})
	}
	return out
}

// postingRole maps the roles whatsmeow reports to the two that may post,
// answering false for a channel this number only follows.
func postingRole(role types.NewsletterRole) (models.ChannelRole, bool) {
	switch role {
	case types.NewsletterRoleOwner:
		return models.ChannelRoleOwner, true
	case types.NewsletterRoleAdmin:
		return models.ChannelRoleAdmin, true
	default:
		return "", false
	}
}
```

- [ ] **Step 4: Run the tests to verify they pass**

```bash
cd backend && go test ./internal/wa/ -run 'TestOnlyChannels|TestANewsletter|TestTheStoredJID' -v
```
Expected: PASS, three tests.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/wa/channels.go backend/internal/wa/channels_test.go
git commit -m "feat(cs): read which channels a paired number may post to"
```

---

### Task 4: Mengirim pembaruan — jalur whatsmeow dan drainer

**Files:**
- Create: `backend/internal/wa/channel_send.go`
- Create: `backend/internal/wa/channel_drainer.go`
- Test: `backend/internal/wa/channel_drainer_test.go`

**Interfaces:**
- Consumes: `services.CSChannelPostService` (Task 2); `buildMediaMessage`, `buildTextMessage`, `uploadTypeFor` (existing, `wa/media.go` and `wa/outbound.go`).
- Produces: `wa.ChannelSender` interface with `SendChannelText(ctx context.Context, channelJID, body string) (string, error)` and `SendChannelMedia(ctx context.Context, channelJID string, kind models.MessageKind, path, mime, filename, caption string) (string, error)`, both implemented by `*wa.Client`; `wa.ChannelDrainer` with `NewChannelDrainer(accountID uuid.UUID, posts *services.CSChannelPostService, sender ChannelSender, publisher announcer, mediaRoot string, pace time.Duration) *ChannelDrainer` and `Drain(ctx context.Context, limit int) (int, error)`.

- [ ] **Step 1: Write the failing drainer test**

Create `backend/internal/wa/channel_drainer_test.go`:

```go
package wa

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// fakeChannelSender stands in for the WhatsApp connection. refuse names the
// body it rejects, so a test can make exactly one post fail.
type fakeChannelSender struct {
	mu     sync.Mutex
	sent   []string
	refuse string
}

func (f *fakeChannelSender) SendChannelText(_ context.Context, _, body string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if body == f.refuse {
		return "", errors.New("not authorized to post")
	}
	f.sent = append(f.sent, body)
	return "3EB0" + body, nil
}

func (f *fakeChannelSender) SendChannelMedia(
	_ context.Context, _ string, _ models.MessageKind, _, _, _, caption string,
) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, caption)
	return "3EB0MEDIA", nil
}

func channelDrainSetup(t *testing.T) (*ChannelDrainer, *fakeChannelSender, *services.CSChannelPostService, models.WAAccount) {
	t.Helper()
	// Built here rather than borrowed from the services package, matching
	// drainSetup in outbound_test.go — including the single connection, which
	// is load-bearing for the concurrency test below: every new connection to
	// an unshared :memory: database gets its own empty copy, so a goroutine
	// that grows the pool would query tables that do not exist there and the
	// race would hide behind swallowed failures.
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, models.AutoMigrate(db))

	account := models.WAAccount{Label: "CS Utama", Status: models.WAAccountConnected}
	require.NoError(t, db.Create(&account).Error)
	posts := services.NewCSChannelPostService(db)
	sender := &fakeChannelSender{}
	return NewChannelDrainer(account.ID, posts, sender, nil, t.TempDir(), 0), sender, posts, account
}

func queuePost(t *testing.T, posts *services.CSChannelPostService, account models.WAAccount, body string) uuid.UUID {
	t.Helper()
	post, err := posts.Queue(services.ChannelPost{
		WAAccountID:  account.ID,
		ChannelJID:   "120363000000000001@newsletter",
		SenderUserID: uuid.New(),
		Kind:         models.MessageKindText,
		Body:         body,
	})
	require.NoError(t, err)
	return post.ID
}

// One channel refusing an update must not hold up the announcements queued
// behind it.
func TestDrainKeepsGoingAfterWhatsAppRefusesOne(t *testing.T) {
	drainer, sender, posts, account := channelDrainSetup(t)
	sender.refuse = "ditolak"
	queuePost(t, posts, account, "ditolak")
	queuePost(t, posts, account, "berhasil")

	sent, err := drainer.Drain(context.Background(), 10)

	require.NoError(t, err)
	assert.Equal(t, 1, sent)
	assert.Equal(t, []string{"berhasil"}, sender.sent)
}

// The refusal has to survive on the row, because the history is the only place
// the sender ever learns their announcement did not go.
func TestARefusedUpdateKeepsItsReason(t *testing.T) {
	drainer, sender, posts, account := channelDrainSetup(t)
	sender.refuse = "ditolak"
	queuePost(t, posts, account, "ditolak")

	_, err := drainer.Drain(context.Background(), 10)
	require.NoError(t, err)

	history, err := posts.ListFor("120363000000000001@newsletter", 10)
	require.NoError(t, err)
	require.Len(t, history, 1)
	assert.Equal(t, models.ChannelPostFailed, history[0].Status)
	assert.Contains(t, history[0].FailReason, "not authorized")
}

// ClaimQueued reads without locking, so two drains racing would hand both the
// same row and subscribers would receive the announcement twice.
func TestTwoDrainsNeverSendTheSameUpdateTwice(t *testing.T) {
	drainer, sender, posts, account := channelDrainSetup(t)
	queuePost(t, posts, account, "sekali saja")

	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = drainer.Drain(context.Background(), 10)
		}()
	}
	wg.Wait()

	assert.Equal(t, []string{"sekali saja"}, sender.sent)
}

// A sent update stops being claimable, so a later sweep does not repeat it.
func TestASentUpdateLeavesTheQueue(t *testing.T) {
	drainer, _, posts, account := channelDrainSetup(t)
	queuePost(t, posts, account, "sudah terkirim")
	_, err := drainer.Drain(context.Background(), 10)
	require.NoError(t, err)

	waiting, err := posts.ClaimQueued(account.ID, 10)
	require.NoError(t, err)
	assert.Empty(t, waiting)
}
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
cd backend && go test ./internal/wa/ -run TestDrainKeepsGoing -v
```
Expected: FAIL to build — `undefined: NewChannelDrainer`.

- [ ] **Step 3: Write the send path**

Create `backend/internal/wa/channel_send.go`:

```go
package wa

import (
	"context"
	"fmt"
	"os"

	"github.com/tikman/olt-provisioning/internal/models"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

// SendChannelText posts a text update and answers the id WhatsApp gave it.
func (c *Client) SendChannelText(ctx context.Context, channelJID, body string) (string, error) {
	to, err := types.ParseJID(channelJID)
	if err != nil {
		return "", fmt.Errorf("saluran tidak valid %q: %w", channelJID, err)
	}

	resp, err := c.wa.SendMessage(ctx, to, buildTextMessage(body, nil))
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}

// SendChannelMedia posts an attachment as an update.
//
// Channel media is uploaded unencrypted, so UploadNewsletter is the upload and
// the handle it returns has to travel with the send — whatsmeow requires it
// for newsletter media. The message itself is built by the same helper the
// chat path uses: it copies MediaKey and FileEncSHA256 from the response, and
// their being empty here is exactly right for a channel.
//
// The file is read whole, which is only safe because the upload boundary caps
// it: the API wraps the request body in a MaxBytesReader before a byte is
// stored.
func (c *Client) SendChannelMedia(
	ctx context.Context, channelJID string, kind models.MessageKind, path, mime, filename, caption string,
) (string, error) {
	to, err := types.ParseJID(channelJID)
	if err != nil {
		return "", fmt.Errorf("saluran tidak valid %q: %w", channelJID, err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("baca lampiran: %w", err)
	}
	uploaded, err := c.wa.UploadNewsletter(ctx, data, uploadTypeFor(kind))
	if err != nil {
		return "", fmt.Errorf("unggah lampiran: %w", err)
	}

	resp, err := c.wa.SendMessage(ctx, to,
		buildMediaMessage(kind, uploaded, mime, filename, caption, nil),
		whatsmeow.SendRequestExtra{MediaHandle: uploaded.Handle},
	)
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}
```

- [ ] **Step 4: Write the drainer**

Create `backend/internal/wa/channel_drainer.go`:

```go
package wa

import (
	"context"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
)

// ChannelSender is the part of whatsmeow that posting an update needs. It is
// its own interface rather than an addition to Sender so the message outbox
// and its tests are untouched by channel work.
type ChannelSender interface {
	SendChannelText(ctx context.Context, channelJID, body string) (waMessageID string, err error)
	SendChannelMedia(
		ctx context.Context, channelJID string, kind models.MessageKind,
		path, mime, filename, caption string,
	) (waMessageID string, err error)
}

// ChannelDrainer posts the updates waiting in the channel outbox.
//
// Only one drain runs at a time, for the reason the message Drainer records:
// ClaimQueued reads rows without locking them, so overlapping drains would
// hand both the same row — and a duplicate here reaches every subscriber.
type ChannelDrainer struct {
	mu        sync.Mutex
	accountID uuid.UUID
	posts     *services.CSChannelPostService
	sender    ChannelSender
	publisher announcer
	mediaRoot string
	pace      time.Duration
}

// NewChannelDrainer constructs a ChannelDrainer. pace is the gap left between
// two posts, for the same reason the message drainer has one: emptying a queue
// as fast as the connection allows is what gets an unofficial number flagged.
func NewChannelDrainer(
	accountID uuid.UUID,
	posts *services.CSChannelPostService,
	sender ChannelSender,
	publisher announcer,
	mediaRoot string,
	pace time.Duration,
) *ChannelDrainer {
	return &ChannelDrainer{
		accountID: accountID,
		posts:     posts,
		sender:    sender,
		publisher: publisher,
		mediaRoot: mediaRoot,
		pace:      pace,
	}
}

// Drain posts what is waiting and answers how many reached WhatsApp. An update
// WhatsApp refuses is recorded with its reason and the drain continues: one
// channel refusing must not hold up an announcement to another.
func (d *ChannelDrainer) Drain(ctx context.Context, limit int) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	waiting, err := d.posts.ClaimQueued(d.accountID, limit)
	if err != nil {
		return 0, err
	}

	sent := 0
	for i, post := range waiting {
		if i > 0 && d.pace > 0 {
			select {
			case <-ctx.Done():
				return sent, ctx.Err()
			case <-time.After(d.pace):
			}
		}

		waID, err := d.send(ctx, post)
		if err != nil {
			if markErr := d.posts.MarkFailed(post.ID, err.Error()); markErr != nil {
				return sent, markErr
			}
			d.announce(ctx, post)
			continue
		}
		if err := d.posts.MarkSent(post.ID, waID); err != nil {
			return sent, err
		}
		d.announce(ctx, post)
		sent++
	}
	return sent, nil
}

func (d *ChannelDrainer) send(ctx context.Context, post models.WAChannelPost) (string, error) {
	if post.Kind == models.MessageKindText {
		return d.sender.SendChannelText(ctx, post.ChannelJID, post.Body)
	}
	return d.sender.SendChannelMedia(
		ctx, post.ChannelJID, post.Kind,
		filepath.Join(d.mediaRoot, post.MediaPath),
		post.MediaMime, post.MediaFilename, post.Body,
	)
}

// announce tells the browsers with this channel's history open that one of its
// updates moved. Published per post rather than once at the end, for the same
// reason as the message drainer: the pace between sends is measured in
// seconds, so a batched announcement would leave the first update showing a
// clock long after it had gone.
//
// A failure to publish is returned to nobody: the update is already sent, and
// the browser's next refetch closes the gap.
func (d *ChannelDrainer) announce(ctx context.Context, post models.WAChannelPost) {
	if d.publisher == nil {
		return
	}
	_ = d.publisher.Publish(ctx, Event{
		Type:      EventChannelPost,
		ChannelID: post.ChannelJID,
	})
}
```

- [ ] **Step 5: Add the event type this drainer publishes**

In `backend/internal/wa/events.go`, beside the existing event type constants, add:

```go
	// EventChannelPost says one channel update changed status. It names the
	// channel by JID rather than a row id, because the wa_channels row is
	// deleted and recreated on every sync while the JID does not move.
	EventChannelPost = "channel_post"
```

and add the field to `Event`:

```go
	// ChannelID is the JID of the channel an EventChannelPost is about.
	ChannelID string `json:"channel_id,omitempty"`
```

- [ ] **Step 6: Run the tests to verify they pass**

```bash
cd backend && go test ./internal/wa/ -race -v
```
Expected: PASS, including the four new channel drainer tests and every existing `wa` test.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/wa/channel_send.go backend/internal/wa/channel_drainer.go \
        backend/internal/wa/channel_drainer_test.go backend/internal/wa/events.go \
        backend/internal/services/testing.go
git commit -m "feat(cs): send queued channel updates through WhatsApp"
```

---

### Task 5: Menyambungkan proses wa

**Files:**
- Create: `backend/cmd/wa/channels.go`
- Modify: `backend/cmd/wa/sessions.go:19-29` (sessionDeps), `:31-35` (session), `:112-155` (ensure), `:157-170` (feed), `:172-180` (drainAll)
- Modify: `backend/cmd/wa/main.go:150-231` (applyControl)
- Modify: `backend/internal/wa/events.go` (control action constant)

**Interfaces:**
- Consumes: `services.CSChannelService`, `services.CSChannelPostService` (Tasks 1-2); `(*wa.Client).AdminChannels`, `wa.NewChannelDrainer` (Tasks 3-4).
- Produces: `wa.ControlSyncChannels = "sync-channels"`; `syncChannels(ctx context.Context, client *wa.Client, channels *services.CSChannelService, accountID uuid.UUID, logger *zap.Logger)`.

- [ ] **Step 1: Add the control action**

In `backend/internal/wa/events.go`, in the control actions block:

```go
	// ControlSyncChannels asks this process to re-read which channels a number
	// administers. The mirror refreshes hourly on its own; this is the button
	// for an admin who has just been given a channel and does not want to wait.
	ControlSyncChannels = "sync-channels"
```

- [ ] **Step 2: Write the sync helper**

Create `backend/cmd/wa/channels.go`:

```go
package main

import (
	"context"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/services"
	"github.com/tikman/olt-provisioning/internal/wa"
	"go.uber.org/zap"
)

// syncChannels refreshes one number's channel mirror.
//
// A failure leaves the previous list in place rather than emptying the picker:
// Replace is not reached at all when WhatsApp does not answer, so a number
// whose connection is briefly down keeps the channels it had.
func syncChannels(
	ctx context.Context,
	client *wa.Client,
	channels *services.CSChannelService,
	accountID uuid.UUID,
	logger *zap.Logger,
) {
	found, err := client.AdminChannels(ctx)
	if err != nil {
		logger.Warn("Could not read the channel list", zap.Error(err))
		return
	}
	if err := channels.Replace(accountID, found); err != nil {
		logger.Error("Could not store the channel list", zap.Error(err))
		return
	}
	logger.Info("Refreshed the channel list", zap.Int("channels", len(found)))
}
```

- [ ] **Step 3: Wire the sync loop and the second drainer into sessions.go**

In `sessionDeps`, add:

```go
	channels      *services.CSChannelService
	channelPosts  *services.CSChannelPostService
```

In `session`, add:

```go
	channelDrainer *wa.ChannelDrainer
```

In `ensure`, after the existing `drainer := wa.NewDrainer(...)` construction:

```go
	channelDrainer := wa.NewChannelDrainer(account.ID, s.deps.channelPosts, client,
		publisher, s.deps.cfg.WAMediaDir,
		time.Duration(s.deps.cfg.WASendIntervalMS)*time.Millisecond)
```

and store it on the `session` literal:

```go
	s.running[account.ID] = &session{
		client: client, drainer: drainer, channelDrainer: channelDrainer, stop: stop,
	}
```

In `feed`, add the hourly refresh alongside the existing loops, and one at startup so a freshly connected number does not wait an hour to appear in the picker:

```go
	syncChannels(ctx, client, s.deps.channels, account.ID, logger)
	go every(ctx, channelSync, func() {
		syncChannels(ctx, client, s.deps.channels, account.ID, logger)
	})
```

Declare the interval beside the other sweep constants in `cmd/wa/main.go`:

```go
// channelSync is how often a number re-reads which channels it administers.
// Hourly rather than minutes: admin rights on a channel change rarely, and the
// refresh button covers the case where somebody cannot wait.
const channelSync = time.Hour
```

In `drainAll`, drain both queues:

```go
func (s *sessions) drainAll(ctx context.Context) {
	for _, live := range s.snapshot() {
		drainOutbox(ctx, live.drainer, s.deps.logger)
		drainChannelOutbox(ctx, live.channelDrainer, s.deps.logger)
	}
}
```

Add the helper next to the existing `drainOutbox` (in whichever file it lives), matching its shape:

```go
// drainChannelOutbox posts what is waiting for the channels one number
// administers, logging how many went.
func drainChannelOutbox(ctx context.Context, drainer *wa.ChannelDrainer, logger *zap.Logger) {
	sent, err := drainer.Drain(ctx, drainLimit)
	if err != nil {
		logger.Error("Could not post the queued channel updates", zap.Error(err))
		return
	}
	if sent > 0 {
		logger.Info("Posted queued channel updates", zap.Int("sent", sent))
	}
}
```

Use the same limit constant `drainOutbox` uses; read it from that function rather than inventing a second one.

- [ ] **Step 4: Handle the control action in main.go**

In `applyControl`, add a case beside `wa.ControlDisconnect`:

```go
	case wa.ControlSyncChannels:
		client := live.client(id)
		if client == nil {
			logger.Warn("Asked to refresh channels for a number with no session",
				zap.String("account_id", msg.AccountID))
			return
		}
		syncChannels(ctx, client, live.deps.channels, id, logger)
```

Match the surrounding cases for how `id` is parsed and how a missing session is reported — do not introduce a different shape.

- [ ] **Step 5: Construct the two services in main.go**

Where the other CS services are constructed in `cmd/wa/main.go`, add:

```go
	channels := services.NewCSChannelService(db)
	channelPosts := services.NewCSChannelPostService(db)
```

and pass them into the `sessionDeps` literal as `channels: channels, channelPosts: channelPosts`.

- [ ] **Step 6: Verify it builds and every test still passes**

```bash
cd backend && go build ./... && go vet ./... && go test ./... -race
```
Expected: build clean, vet clean, all tests PASS.

- [ ] **Step 7: Commit**

```bash
git add backend/cmd/wa/ backend/internal/wa/events.go
git commit -m "feat(cs): keep the channel mirror fresh and drain its outbox"
```

---

### Task 6: Endpoint API

**Files:**
- Create: `backend/internal/api/cs_handler_channels.go`
- Modify: `backend/internal/api/cs_handler.go` (the `CSHandler` struct and `NewCSHandler`; find it with `grep -rn "func NewCSHandler" backend/internal/api/`)
- Modify: `backend/internal/api/router.go:81-95` (service construction and handler call), `:230-266` (routes)
- Modify: `backend/internal/api/cs_handler_test.go:59-105` (`setupCSHandler` must pass the two new services)
- Test: `backend/internal/api/cs_handler_channels_test.go`

**Interfaces:**
- Consumes: `services.CSChannelService`, `services.CSChannelPostService` (Tasks 1-2); existing `mapCSError`, `bindJSON`, `queryInt`, `refuseUpload`, `storeUpload`, `removeOrphanedUpload`, `kindForMime`, `maxUploadBytes`, `wa.NormalizeMime`, `wa.AllowedExtension`, `h.announceEvent`, `h.redis`.
- Produces: five handler methods on `*CSHandler` — `ListChannels`, `RefreshChannels`, `ListChannelPosts`, `CreateChannelPost`, `CreateChannelPostMedia`.

- [ ] **Step 1: Write the failing handler test**

Create `backend/internal/api/cs_handler_channels_test.go`:

```go
package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

func adminChannel(t *testing.T, env *csHandlerEnv) models.WAChannel {
	t.Helper()
	row := models.WAChannel{
		WAAccountID: env.account.ID,
		JID:         "120363000000000001@newsletter",
		Name:        "Info Gangguan",
		Role:        models.ChannelRoleOwner,
	}
	require.NoError(t, env.db.Create(&row).Error)
	return row
}

// Broadcasting is open to everyone who can open the inbox — deliberately
// looser than quick replies and number management beside it. Viewer stays out
// because the whole /cs group is closed to that role.
func TestEveryInboxRoleMayPostAnUpdate(t *testing.T) {
	env := setupCSHandler(t)
	channel := adminChannel(t, env)

	for role, want := range map[models.UserRole]int{
		models.UserRoleAdmin:      http.StatusCreated,
		models.UserRoleCS:         http.StatusCreated,
		models.UserRoleTechnician: http.StatusCreated,
		models.UserRoleViewer:     http.StatusForbidden,
	} {
		body := `{"channel_id":"` + channel.ID.String() + `","body":"Ada pemeliharaan malam ini"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/cs/channel-posts", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		env.asUser(env.cs, role).ServeHTTP(rec, req)
		assert.Equal(t, want, rec.Code, string(role))
	}
}

// A channel the mirror no longer lists is a channel this number may no longer
// post to. Refusing at the API means nothing is queued that could only ever
// fail later.
func TestPostingToAnUnknownChannelIsRefused(t *testing.T) {
	env := setupCSHandler(t)

	body := `{"channel_id":"` + uuid.New().String() + `","body":"Ada pemeliharaan"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cs/channel-posts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.asUser(env.cs, models.UserRoleCS).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// An empty update is nothing to announce, and WhatsApp would refuse it several
// seconds later where the sender can no longer see the form.
func TestAnEmptyUpdateIsRefused(t *testing.T) {
	env := setupCSHandler(t)
	channel := adminChannel(t, env)

	body := `{"channel_id":"` + channel.ID.String() + `","body":"   "}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cs/channel-posts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.asUser(env.cs, models.UserRoleCS).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// The queued row must carry the JID and the number, not the picked row id: the
// mirror row it came from is deleted and recreated on every sync.
func TestAQueuedUpdateCarriesTheChannelAndItsNumber(t *testing.T) {
	env := setupCSHandler(t)
	channel := adminChannel(t, env)

	body := `{"channel_id":"` + channel.ID.String() + `","body":"Ada pemeliharaan malam ini"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cs/channel-posts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.asUser(env.cs, models.UserRoleCS).ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	var stored []models.WAChannelPost
	require.NoError(t, env.db.Find(&stored).Error)
	require.Len(t, stored, 1)
	assert.Equal(t, channel.JID, stored[0].ChannelJID)
	assert.Equal(t, env.account.ID, stored[0].WAAccountID)
	assert.Equal(t, models.ChannelPostQueued, stored[0].Status)
}

// The upload allowlist is the same one chat attachments go through, and it
// deliberately excludes the types ServeMedia would hand back from the API's
// own origin.
func TestAnUnacceptedAttachmentTypeIsRefused(t *testing.T) {
	env := setupCSHandler(t)
	channel := adminChannel(t, env)

	req := uploadRequest(t,
		"/api/v1/cs/channel-posts/media?channel_id="+channel.ID.String(), "text/html", 32)
	rec := httptest.NewRecorder()
	env.asUser(env.cs, models.UserRoleCS).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
```

`uploadRequest(t, path, contentType string, size int)` already exists at `backend/internal/api/cs_handler_upload_test.go:18` and builds exactly this multipart POST; reuse it rather than writing a second helper. `env.cs`, `env.db`, `env.account` and `env.asUser` come from `setupCSHandler`. Drop `strings` from the imports if the media test is the only one that needed it.

- [ ] **Step 2: Run the test to verify it fails**

```bash
cd backend && go test ./internal/api/ -run TestEveryInboxRole -v
```
Expected: FAIL — 404, because the routes do not exist yet.

- [ ] **Step 3: Add the two services to the handler**

In the file defining `CSHandler` and `NewCSHandler`, add two fields:

```go
	channels     *services.CSChannelService
	channelPosts *services.CSChannelPostService
```

and two parameters to `NewCSHandler`, placed immediately after `accounts` so the grouping stays readable. Update the two call sites: `router.go` and `cs_handler_test.go`.

- [ ] **Step 4: Write the handlers**

Create `backend/internal/api/cs_handler_channels.go`:

```go
package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/middleware"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"github.com/tikman/olt-provisioning/internal/wa"
	"go.uber.org/zap"
)

// CreateChannelPostRequest is one text update as the composer sends it.
type CreateChannelPostRequest struct {
	ChannelID string `json:"channel_id" binding:"required"`
	Body      string `json:"body" binding:"required"`
}

// ListChannels answers every channel the team may post to. It reads the
// mirror the wa process keeps; no WhatsApp connection is involved.
func (h *CSHandler) ListChannels(c *gin.Context) {
	rows, err := h.channels.List()
	if err != nil {
		mapCSError(c, err, "CHANNEL_LIST_FAILED")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows})
}

// RefreshChannels asks the wa process to re-read every number's channel list.
//
// One control message per number, because a session is per number and the
// process routes the action by account id. Like the other control actions this
// is fire-and-forget: the answer arrives as changed rows, not as a response.
func (h *CSHandler) RefreshChannels(c *gin.Context) {
	accounts, err := h.accounts.List()
	if err != nil {
		mapCSError(c, err, "CHANNEL_REFRESH_FAILED")
		return
	}

	for _, account := range accounts {
		msg := wa.ControlMessage{Action: wa.ControlSyncChannels, AccountID: account.ID.String()}
		if err := h.publishControl(c, msg); err != nil {
			h.logger.Warn("publish channel sync request failed",
				zap.String("account_id", account.ID.String()), zap.Error(err))
		}
	}
	c.JSON(http.StatusAccepted, gin.H{"data": gin.H{"accounts": len(accounts)}})
}

// ListChannelPosts answers one channel's broadcast history, newest first.
func (h *CSHandler) ListChannelPosts(c *gin.Context) {
	channel, ok := h.channelFromQuery(c, "CHANNEL_HISTORY_FAILED")
	if !ok {
		return
	}

	rows, err := h.channelPosts.ListFor(channel.JID, queryInt(c, "limit"))
	if err != nil {
		mapCSError(c, err, "CHANNEL_HISTORY_FAILED")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows})
}

// CreateChannelPost queues a text update.
func (h *CSHandler) CreateChannelPost(c *gin.Context) {
	var req CreateChannelPostRequest
	if !bindJSON(c, &req) {
		return
	}

	body := strings.TrimSpace(req.Body)
	if body == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "pembaruan tidak boleh kosong", Code: "EMPTY_UPDATE",
		})
		return
	}

	channel, ok := h.channelByID(c, req.ChannelID, "CHANNEL_POST_FAILED")
	if !ok {
		return
	}

	userID, _ := middleware.GetUserID(c)
	post, err := h.channelPosts.Queue(services.ChannelPost{
		WAAccountID:  channel.WAAccountID,
		ChannelJID:   channel.JID,
		SenderUserID: userID,
		Kind:         models.MessageKindText,
		Body:         body,
	})
	if err != nil {
		mapCSError(c, err, "CHANNEL_POST_FAILED")
		return
	}

	h.announceChannelPost(c, channel.JID)
	c.JSON(http.StatusCreated, gin.H{"data": post})
}

// CreateChannelPostMedia queues an update carrying an attachment. The file is
// stored before the row is written, the same order the chat path uses.
func (h *CSHandler) CreateChannelPostMedia(c *gin.Context) {
	channel, ok := h.channelFromQuery(c, "CHANNEL_POST_FAILED")
	if !ok {
		return
	}

	// The bound is applied to the body itself, not to what the multipart
	// header claims: everything downstream works on however many bytes
	// actually arrive.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadBytes)

	fileHeader, err := c.FormFile("file")
	if err != nil {
		refuseUpload(c, err)
		return
	}

	mime := wa.NormalizeMime(fileHeader.Header.Get("Content-Type"))
	ext, allowed := wa.AllowedExtension(mime)
	if !allowed {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: fmt.Sprintf("attachment type %q is not accepted", mime),
			Code:  "MEDIA_TYPE_NOT_ALLOWED",
		})
		return
	}

	media, err := h.storeUpload(fileHeader, mime, ext)
	if err != nil {
		h.logger.Error("store channel attachment failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "failed to store attachment", Code: "MEDIA_STORE_FAILED",
		})
		return
	}

	userID, _ := middleware.GetUserID(c)
	post, err := h.channelPosts.Queue(services.ChannelPost{
		WAAccountID:  channel.WAAccountID,
		ChannelJID:   channel.JID,
		SenderUserID: userID,
		Kind:         kindForMime(mime),
		Body:         strings.TrimSpace(c.PostForm("caption")),
		Media:        media,
	})
	if err != nil {
		h.removeOrphanedUpload(media.Path)
		mapCSError(c, err, "CHANNEL_POST_FAILED")
		return
	}

	h.announceChannelPost(c, channel.JID)
	c.JSON(http.StatusCreated, gin.H{"data": post})
}

// channelFromQuery resolves the channel_id query parameter.
func (h *CSHandler) channelFromQuery(c *gin.Context, code string) (*models.WAChannel, bool) {
	return h.channelByID(c, c.Query("channel_id"), code)
}

// channelByID turns a picked id into the channel it names, refusing one that
// is no longer in the mirror — which is what stops an update being queued to a
// channel this number may no longer post to.
func (h *CSHandler) channelByID(c *gin.Context, raw, code string) (*models.WAChannel, bool) {
	id, err := uuid.Parse(raw)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "saluran tidak valid", Code: "INVALID_CHANNEL_ID",
		})
		return nil, false
	}

	channel, err := h.channels.Get(id)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error: "saluran tidak ditemukan", Code: code,
		})
		return nil, false
	}
	return channel, true
}

// announceChannelPost wakes the wa process to drain now instead of waiting for
// its sweep, and tells the other browsers a row appeared. Redis carries no
// truth here — the row is already stored — so neither failure fails the
// request.
func (h *CSHandler) announceChannelPost(c *gin.Context, channelJID string) {
	ctx := c.Request.Context()
	h.announceEvent(ctx, wa.Event{Type: wa.EventChannelPost, ChannelID: channelJID})
	if err := h.redis.Publish(ctx, wa.OutboxChannel, "").Err(); err != nil {
		h.logger.Warn("publish cs outbox notice failed", zap.Error(err))
	}
}
```

If this file passes 300 lines, move `CreateChannelPostMedia` and the upload helpers into `cs_handler_channels_media.go`.

- [ ] **Step 5: Register the routes**

In `backend/internal/api/router.go`, inside the `cs` group, after the `wa-accounts` block:

```go
			cs.GET("/wa-channels", csHandler.ListChannels)
			cs.POST("/wa-channels/refresh", csHandler.RefreshChannels)
			cs.GET("/channel-posts", csHandler.ListChannelPosts)
			cs.POST("/channel-posts", csHandler.CreateChannelPost)
			cs.POST("/channel-posts/media", csHandler.CreateChannelPostMedia)
```

Construct the services beside the other CS services near line 81:

```go
	csChannelService := services.NewCSChannelService(db)
	csChannelPostService := services.NewCSChannelPostService(db)
```

and pass both into `NewCSHandler`.

- [ ] **Step 6: Run the tests to verify they pass**

```bash
cd backend && go test ./internal/api/ -race -v
```
Expected: PASS, including the five new tests and every existing API test.

- [ ] **Step 7: Full backend gate**

```bash
cd backend && gofmt -s -l . && go vet ./... && go test ./... -race
```
Expected: `gofmt` prints nothing, vet clean, all tests PASS.

- [ ] **Step 8: Commit**

```bash
git add backend/internal/api/ backend/internal/services/
git commit -m "feat(cs): expose the channel list and the update outbox over HTTP"
```

---

### Task 7: Frontend — entitas, repositori, hooks

**Files:**
- Create: `frontend/src/domain/entities/WaChannel.ts`
- Create: `frontend/src/application/hooks/useWaChannels.ts`
- Modify: `frontend/src/domain/entities/index.ts` (export the new entities)
- Modify: `frontend/src/domain/repositories/ICsRepository.ts`
- Modify: `frontend/src/infrastructure/repositories/CsRepository.ts`
- Modify: `frontend/src/infrastructure/http/endpoints.ts:99-106`
- Modify: `frontend/src/application/hooks/index.ts:17-21`

**Interfaces:**
- Consumes: the five endpoints from Task 6.
- Produces: types `WaChannel` (`id`, `waAccountId`, `jid`, `name`, `role`, `subscriberCount`, `syncedAt`) and `ChannelPost` (`id`, `waAccountId`, `channelJid`, `senderUserId`, `kind`, `body`, `mediaFilename?`, `status`, `failReason?`, `createdAt`, `sentAt?`); hooks `useWaChannels()`, `useRefreshWaChannels()`, `useChannelPosts(channelId?: string)`, `useSendChannelPost()`, `useSendChannelPostMedia()`.

Note: the axios client converts snake_case to camelCase through `humps`, so the API's `channel_jid` reaches the component as `channelJid`. Do not convert by hand.

- [ ] **Step 1: Write the entities**

Create `frontend/src/domain/entities/WaChannel.ts`:

```ts
/** How much a paired number may do on a WhatsApp Channel. Only the roles that
 * may post are ever stored, so the picker never offers one that would refuse
 * the update. */
export type ChannelRole = "owner" | "admin";

/** How far an update has travelled. There is no "delivered" or "read": a
 * channel sends no receipts, so promising either on screen would be a lie. */
export type ChannelPostStatus = "queued" | "sent" | "failed";

/** One WhatsApp Channel a paired number administers. */
export interface WaChannel {
  id: string;
  waAccountId: string;
  jid: string;
  name: string;
  role: ChannelRole;
  subscriberCount: number;
  syncedAt: string;
}

/** One update, on its way to a channel or already gone. */
export interface ChannelPost {
  id: string;
  waAccountId: string;
  channelJid: string;
  senderUserId: string;
  kind: string;
  body: string;
  mediaFilename?: string;
  status: ChannelPostStatus;
  failReason?: string;
  createdAt: string;
  sentAt?: string;
}
```

Export both from `frontend/src/domain/entities/index.ts`, matching how the neighbouring entities are exported there.

- [ ] **Step 2: Add the endpoints**

In `frontend/src/infrastructure/http/endpoints.ts`, beside the other CS entries:

```ts
  CS_WA_CHANNELS: "/api/v1/cs/wa-channels",
  CS_WA_CHANNELS_REFRESH: "/api/v1/cs/wa-channels/refresh",
  CS_CHANNEL_POSTS: "/api/v1/cs/channel-posts",
  CS_CHANNEL_POSTS_MEDIA: "/api/v1/cs/channel-posts/media",
```

- [ ] **Step 3: Add the repository methods**

In `frontend/src/infrastructure/repositories/CsRepository.ts`, after `deleteWaAccount`:

```ts
  async listWaChannels(): Promise<WaChannel[]> {
    const response = await apiClient.get(API_ENDPOINTS.CS_WA_CHANNELS);
    return response.data.data;
  }

  /** Fire-and-forget: the API asks the wa process to re-read its channel
   * lists and answers immediately. The result arrives as changed rows on the
   * next fetch, not in this response. */
  async refreshWaChannels(): Promise<void> {
    await apiClient.post(API_ENDPOINTS.CS_WA_CHANNELS_REFRESH);
  }

  async getChannelPosts(channelId: string): Promise<ChannelPost[]> {
    const response = await apiClient.get(API_ENDPOINTS.CS_CHANNEL_POSTS, {
      params: { channel_id: channelId },
    });
    return response.data.data;
  }

  async sendChannelPost(channelId: string, body: string): Promise<ChannelPost> {
    const response = await apiClient.post(API_ENDPOINTS.CS_CHANNEL_POSTS, {
      channel_id: channelId,
      body,
    });
    return response.data.data;
  }

  async sendChannelPostMedia(
    channelId: string,
    file: File,
    caption?: string,
  ): Promise<ChannelPost> {
    const form = new FormData();
    form.append("file", file);
    if (caption) {
      form.append("caption", caption);
    }

    // The channel travels in the query string, not the form, for the reason
    // sendMedia records: the API wraps the body in a size guard before
    // anything reads it, and a form field would have to be parsed ahead of it.
    const response = await apiClient.post(
      `${API_ENDPOINTS.CS_CHANNEL_POSTS_MEDIA}?channel_id=${channelId}`,
      form,
      // Dropping the header hands the boundary to the browser, which is the
      // only thing that knows it.
      { headers: { "Content-Type": false } },
    );
    return response.data.data;
  }
```

Add the same five signatures to `ICsRepository` in `frontend/src/domain/repositories/ICsRepository.ts`, and add `WaChannel` / `ChannelPost` to the type imports at the top of both files.

- [ ] **Step 4: Write the hooks**

Create `frontend/src/application/hooks/useWaChannels.ts`:

```ts
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { CsRepository } from "@/infrastructure/repositories";
import { reportCsMutationError } from "./csMutationError";

const csRepository = new CsRepository();

const CHANNELS_KEY = ["cs", "wa-channels"];

/** The channels the team may post to, mirrored from WhatsApp by the wa
 * process. This never touches a WhatsApp connection. */
export function useWaChannels() {
  return useQuery({
    queryKey: CHANNELS_KEY,
    queryFn: () => csRepository.listWaChannels(),
  });
}

/** Asks the wa process to re-read its channel lists. The mirror refreshes
 * hourly by itself; this is for an admin who has just been given a channel and
 * does not want to wait. The rows change a moment after this resolves, so the
 * list is invalidated rather than read from the response. */
export function useRefreshWaChannels() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => csRepository.refreshWaChannels(),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: CHANNELS_KEY });
    },
    onError: reportCsMutationError,
  });
}

/** One channel's broadcast history. This is where a sender learns whether
 * their update actually went: sending only queues it. */
export function useChannelPosts(channelId?: string) {
  return useQuery({
    queryKey: ["cs", "channel-posts", channelId],
    queryFn: () => csRepository.getChannelPosts(channelId as string),
    enabled: !!channelId,
  });
}

export function useSendChannelPost() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (vars: { channelId: string; body: string }) =>
      csRepository.sendChannelPost(vars.channelId, vars.body),
    onSuccess: (_post, vars) => {
      queryClient.invalidateQueries({
        queryKey: ["cs", "channel-posts", vars.channelId],
      });
    },
    onError: reportCsMutationError,
  });
}

export function useSendChannelPostMedia() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (vars: { channelId: string; file: File; caption?: string }) =>
      csRepository.sendChannelPostMedia(vars.channelId, vars.file, vars.caption),
    onSuccess: (_post, vars) => {
      queryClient.invalidateQueries({
        queryKey: ["cs", "channel-posts", vars.channelId],
      });
    },
    onError: reportCsMutationError,
  });
}
```

Add `export * from "./useWaChannels";` to `frontend/src/application/hooks/index.ts`.

- [ ] **Step 5: Verify it builds and lints**

```bash
cd frontend && npm run build && npm run lint && npm run format:check
```
Expected: build succeeds, zero lint errors, formatting clean.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/domain frontend/src/infrastructure frontend/src/application
git commit -m "feat(cs): fetch the channel list and post updates from the browser"
```

---

### Task 8: Frontend — modal, riwayat, dan header inbox

**Files:**
- Create: `frontend/src/presentation/components/cs/ChannelPostHistory.tsx`
- Create: `frontend/src/presentation/components/cs/ChannelBroadcastModal.tsx`
- Create: `frontend/src/presentation/components/cs/InboxHeaderActions.tsx`
- Modify: `frontend/src/presentation/pages/CsInboxPage.tsx` (imports, modal state, the `PageHeader` `extra` block, and the modal render)
- Modify: `frontend/src/application/hooks/useCsStream.ts:7-14` (`CsEvent`) and `:100-110` (the invalidation block)
- Test: `frontend/src/presentation/components/cs/__tests__/ChannelBroadcastModal.test.tsx`

**Interfaces:**
- Consumes: hooks and types from Task 7.
- Produces: `ChannelPostHistory({ posts, loading })`; `ChannelBroadcastModal(props)` as typed below; `InboxHeaderActions(props)` as typed below.

Both components are **props-driven and hold no hooks of their own**, matching `WaNumbersModal` and every other component under `components/cs/`. `CsInboxPage` calls the hooks and passes data down. This is what makes the tests plain renders rather than module mocks — the pattern every test in `components/cs/__tests__/` already uses.

- [ ] **Step 1: Teach the stream about channel posts**

In `frontend/src/application/hooks/useCsStream.ts`, add to the `CsEvent` type:

```ts
  channel_id?: string;
```

and in `onEvent`, immediately after the `typing` branch and before the conversation invalidations:

```ts
      if (payload.type === "channel_post") {
        // Returns rather than falling through: a channel update belongs to no
        // conversation, and the refetches below would reload the whole inbox
        // for something that changed nothing in it.
        queryClient.invalidateQueries({ queryKey: ["cs", "channel-posts"] });
        return;
      }
```

- [ ] **Step 2: Write the failing modal test**

Create `frontend/src/presentation/components/cs/__tests__/ChannelBroadcastModal.test.tsx`:

```tsx
import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ChannelBroadcastModal } from "../ChannelBroadcastModal";
import type { ChannelPost, WaChannel } from "@/domain/entities";

const channels: WaChannel[] = [
  {
    id: "c1",
    waAccountId: "a1",
    jid: "120363000000000001@newsletter",
    name: "Info Gangguan",
    role: "owner",
    subscriberCount: 240,
    syncedAt: "2026-09-04T00:00:00Z",
  },
];

function open(props: Partial<Parameters<typeof ChannelBroadcastModal>[0]> = {}) {
  return render(
    <ChannelBroadcastModal
      open
      channels={channels}
      accountLabels={{ a1: "CS Utama" }}
      posts={[]}
      loadingPosts={false}
      refreshing={false}
      sending={false}
      onSelectChannel={vi.fn()}
      onRefresh={vi.fn()}
      onSend={vi.fn().mockResolvedValue(true)}
      onClose={vi.fn()}
      {...props}
    />,
  );
}

const sendButton = () => screen.getByRole("button", { name: "Kirim Pembaruan" });

describe("ChannelBroadcastModal", () => {
  // An update reaches every subscriber and cannot be withdrawn. Sending before
  // a channel is chosen would mean sending to whichever one happened to be
  // first.
  it("keeps the send button dead until a channel and something to say exist", async () => {
    open();
    expect(sendButton()).toBeDisabled();

    await userEvent.type(screen.getByPlaceholderText(/tulis pembaruan/i), "Ada pemeliharaan");
    expect(sendButton()).toBeDisabled();

    open({ selectedChannelId: "c1" });
    expect(sendButton()).toBeDisabled();
  });

  it("arms the send button once both are present", async () => {
    open({ selectedChannelId: "c1" });

    await userEvent.type(screen.getByPlaceholderText(/tulis pembaruan/i), "Ada pemeliharaan");

    expect(sendButton()).toBeEnabled();
  });

  // Sending only queues; the outcome arrives seconds later. The history is the
  // only place a failure is ever visible, so the reason has to be on screen.
  it("shows why an update failed", () => {
    const failed: ChannelPost[] = [
      {
        id: "p1",
        waAccountId: "a1",
        channelJid: "120363000000000001@newsletter",
        senderUserId: "u1",
        kind: "text",
        body: "Ada pemeliharaan malam ini",
        status: "failed",
        failReason: "not authorized to post",
        createdAt: "2026-09-04T01:00:00Z",
      },
    ];
    open({ selectedChannelId: "c1", posts: failed });

    expect(screen.getByText("Gagal")).toBeInTheDocument();
    expect(screen.getByText("not authorized to post")).toBeInTheDocument();
  });

  // A number that admins no channel cannot be helped by this screen, and the
  // empty state has to say why rather than look like a loading failure.
  it("explains an empty channel list instead of showing a dead composer", () => {
    open({ channels: [] });

    expect(screen.getByText(/harus sudah menjadi admin sebuah saluran/i)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Kirim Pembaruan" })).toBeNull();
  });
});
```

- [ ] **Step 3: Run the test to verify it fails**

```bash
cd frontend && npm test -- --run ChannelBroadcastModal
```
Expected: FAIL — cannot resolve `../ChannelBroadcastModal`.

- [ ] **Step 4: Write the history component**

Create `frontend/src/presentation/components/cs/ChannelPostHistory.tsx`:

```tsx
import { Empty, List, Tag, Typography } from "antd";
import type { ChannelPost, ChannelPostStatus } from "@/domain/entities";

const { Text } = Typography;

/** Three states and no more: a channel sends no receipts, so there is nothing
 * honest to show between "sent" and whatever a subscriber did with it. */
const STATUS_LABEL: Record<ChannelPostStatus, string> = {
  queued: "Antre",
  sent: "Terkirim",
  failed: "Gagal",
};

const STATUS_COLOR: Record<ChannelPostStatus, string> = {
  queued: "default",
  sent: "success",
  failed: "error",
};

interface ChannelPostHistoryProps {
  posts: ChannelPost[];
  loading: boolean;
}

/** What has been announced on a channel, and what became of it.
 *
 * Not decoration: sending only queues the update, so this is the only place a
 * sender ever learns whether their announcement actually went out. */
export function ChannelPostHistory({ posts, loading }: ChannelPostHistoryProps) {
  return (
    <List
      size="small"
      loading={loading}
      dataSource={posts}
      locale={{
        emptyText: <Empty description="Belum ada pembaruan di saluran ini" />,
      }}
      renderItem={(post) => (
        <List.Item key={post.id}>
          <List.Item.Meta
            title={
              <>
                <Tag color={STATUS_COLOR[post.status]}>
                  {STATUS_LABEL[post.status]}
                </Tag>
                <Text type="secondary">
                  {new Date(post.createdAt).toLocaleString("id-ID")}
                </Text>
              </>
            }
            description={
              <>
                <div>{post.body || post.mediaFilename}</div>
                {post.failReason ? (
                  <Text type="danger">{post.failReason}</Text>
                ) : null}
              </>
            }
          />
        </List.Item>
      )}
    />
  );
}
```

- [ ] **Step 5: Write the modal**

Create `frontend/src/presentation/components/cs/ChannelBroadcastModal.tsx`:

```tsx
import { useState } from "react";
import { Alert, Button, Empty, Input, Modal, Select, Space, Upload } from "antd";
import { PaperClipOutlined, ReloadOutlined } from "@ant-design/icons";
import type { ChannelPost, WaChannel } from "@/domain/entities";
import { ChannelPostHistory } from "./ChannelPostHistory";

const { TextArea } = Input;

interface ChannelBroadcastModalProps {
  open: boolean;
  channels: WaChannel[];
  /** The number's label per wa_account_id, so one flat list can still say
   * which number administers each channel. */
  accountLabels: Record<string, string>;
  posts: ChannelPost[];
  loadingPosts: boolean;
  refreshing: boolean;
  sending: boolean;
  selectedChannelId?: string;
  onSelectChannel: (id: string) => void;
  onRefresh: () => void;
  /** Answers whether the update was queued, so the composer clears only when
   * it actually was and a rejected update is not silently thrown away. */
  onSend: (body: string, file?: File) => Promise<boolean>;
  onClose: () => void;
}

/** Posting an update to a WhatsApp Channel one of the team's numbers admins.
 *
 * One flat channel list rather than picking a number first: a channel belongs
 * to exactly one number, so asking for both would ask the sender to repeat
 * what they have already said. */
export function ChannelBroadcastModal({
  open,
  channels,
  accountLabels,
  posts,
  loadingPosts,
  refreshing,
  sending,
  selectedChannelId,
  onSelectChannel,
  onRefresh,
  onSend,
  onClose,
}: ChannelBroadcastModalProps) {
  const [body, setBody] = useState("");
  const [file, setFile] = useState<File>();

  const canSend = !!selectedChannelId && (body.trim() !== "" || !!file);

  const handleSend = async () => {
    if (!canSend) return;
    if (await onSend(body.trim(), file)) {
      setBody("");
      setFile(undefined);
    }
  };

  return (
    <Modal
      open={open}
      onCancel={onClose}
      footer={null}
      width={640}
      title="Pembaruan Saluran"
    >
      {channels.length === 0 ? (
        <Empty description="Belum ada saluran. Nomor WhatsApp harus sudah menjadi admin sebuah saluran — TikMan tidak bisa membuatkannya." />
      ) : (
        <Space direction="vertical" style={{ width: "100%" }} size="middle">
          <Space.Compact style={{ width: "100%" }}>
            <Select
              style={{ flex: 1 }}
              placeholder="Pilih saluran"
              value={selectedChannelId}
              onChange={onSelectChannel}
              options={channels.map((channel) => ({
                value: channel.id,
                label: `${channel.name} — ${accountLabels[channel.waAccountId] ?? "nomor tak dikenal"}`,
              }))}
            />
            <Button
              icon={<ReloadOutlined />}
              loading={refreshing}
              onClick={onRefresh}
            >
              Segarkan
            </Button>
          </Space.Compact>

          <TextArea
            rows={4}
            value={body}
            onChange={(event) => setBody(event.target.value)}
            placeholder="Tulis pembaruan untuk pengikut saluran"
          />

          <Upload
            maxCount={1}
            // false: the file is held until send rather than posted the moment
            // it is chosen. There is no withdrawing an update that has already
            // reached subscribers.
            beforeUpload={(chosen) => {
              setFile(chosen);
              return false;
            }}
            onRemove={() => setFile(undefined)}
          >
            <Button icon={<PaperClipOutlined />}>Lampiran</Button>
          </Upload>

          <Alert
            type="info"
            showIcon
            message="Kirim akan mengantrekan pembaruan. Statusnya muncul di riwayat di bawah beberapa saat kemudian."
          />

          <Button
            type="primary"
            disabled={!canSend}
            loading={sending}
            onClick={handleSend}
          >
            Kirim Pembaruan
          </Button>

          <ChannelPostHistory posts={posts} loading={loadingPosts} />
        </Space>
      )}
    </Modal>
  );
}
```

- [ ] **Step 6: Run the modal tests to verify they pass**

```bash
cd frontend && npm test -- --run ChannelBroadcastModal
```
Expected: PASS, four tests.

- [ ] **Step 7: Extract the inbox header actions**

Create `frontend/src/presentation/components/cs/InboxHeaderActions.tsx`, holding what is today the `extra` block of `PageHeader` in `CsInboxPage.tsx` plus the new button:

```tsx
import { Button, Space } from "antd";
import { NotificationOutlined, ThunderboltOutlined } from "@ant-design/icons";
import type { WaAccount } from "@/domain/entities";
import type { WaStreamStatus } from "@/application/hooks/useCsStream";
import type { PushPermission } from "@/infrastructure/firebase/messaging";
import { PushOptInButton } from "./PushOptInButton";
import { WaConnectionBadge } from "./WaConnectionBadge";

interface InboxHeaderActionsProps {
  isAdmin: boolean;
  accounts?: WaAccount[];
  // WaStreamStatus is already the per-number map, not one number's status.
  stream: WaStreamStatus;
  pushPermission: PushPermission;
  pushRequesting: boolean;
  onEnablePush: () => void;
  onOpenQuickReplies: () => void;
  onOpenNumbers: () => void;
  onOpenBroadcast: () => void;
}

/** The inbox header's controls, lifted out of CsInboxPage so that page stays
 * under the file-size limit it had already outgrown. */
export function InboxHeaderActions({
  isAdmin,
  accounts,
  stream,
  pushPermission,
  pushRequesting,
  onEnablePush,
  onOpenQuickReplies,
  onOpenNumbers,
  onOpenBroadcast,
}: InboxHeaderActionsProps) {
  return (
    <Space>
      {isAdmin && (
        <Button icon={<ThunderboltOutlined />} onClick={onOpenQuickReplies}>
          Balasan Cepat
        </Button>
      )}
      {/* Deliberately not behind isAdmin, unlike the button beside it:
          broadcasting is open to every role that can open the inbox. */}
      <Button icon={<NotificationOutlined />} onClick={onOpenBroadcast}>
        Pembaruan Saluran
      </Button>
      <PushOptInButton
        permission={pushPermission}
        requesting={pushRequesting}
        onEnable={onEnablePush}
      />
      <WaConnectionBadge
        accounts={accounts}
        stream={stream}
        onOpenNumbers={isAdmin ? onOpenNumbers : undefined}
      />
    </Space>
  );
}
```

The prop types above are the ones the two existing components already declare: `PushOptInButton` takes `PushPermission` from `@/infrastructure/firebase/messaging` (`PushOptInButton.tsx:7-12`), and `WaConnectionBadge` takes `WaStreamStatus`, which is itself `Record<string, WaLiveStatus>` (`useCsStream.ts:44`).

- [ ] **Step 8: Wire the page**

In `CsInboxPage.tsx`:

Add the state and hooks beside the existing modal state and queries:

```tsx
  const [broadcastOpen, setBroadcastOpen] = useState(false);
  const [broadcastChannelId, setBroadcastChannelId] = useState<string>();

  const channelsQuery = useWaChannels();
  const channelPostsQuery = useChannelPosts(broadcastChannelId);
  const refreshChannels = useRefreshWaChannels();
  const sendChannelPost = useSendChannelPost();
  const sendChannelPostMedia = useSendChannelPostMedia();
```

Add the send handler beside `handleSend` and `handleAttach`, following their shape — `mutateAsync` so the composer learns whether the update was queued, and the rejection turned into `false` because the hook has already shown the reason:

```tsx
  const handleBroadcast = async (body: string, file?: File): Promise<boolean> => {
    if (!broadcastChannelId) return false;
    try {
      if (file) {
        await sendChannelPostMedia.mutateAsync({
          channelId: broadcastChannelId,
          file,
          caption: body,
        });
      } else {
        await sendChannelPost.mutateAsync({
          channelId: broadcastChannelId,
          body,
        });
      }
      return true;
    } catch {
      return false;
    }
  };
```

Replace the `extra` block of `PageHeader` with:

```tsx
        extra={
          <InboxHeaderActions
            isAdmin={isAdmin}
            accounts={accountsQuery.data}
            stream={stream}
            pushPermission={push.permission}
            pushRequesting={push.requesting}
            onEnablePush={push.enable}
            onOpenQuickReplies={() => setQuickRepliesOpen(true)}
            onOpenNumbers={() => setNumbersOpen(true)}
            onOpenBroadcast={() => setBroadcastOpen(true)}
          />
        }
```

and render the modal beside the existing ones:

```tsx
      <ChannelBroadcastModal
        open={broadcastOpen}
        channels={channelsQuery.data ?? []}
        accountLabels={Object.fromEntries(
          accounts.map((account) => [account.id, account.label]),
        )}
        posts={channelPostsQuery.data ?? []}
        loadingPosts={channelPostsQuery.isLoading}
        refreshing={refreshChannels.isPending}
        sending={sendChannelPost.isPending || sendChannelPostMedia.isPending}
        selectedChannelId={broadcastChannelId}
        onSelectChannel={setBroadcastChannelId}
        onRefresh={() => refreshChannels.mutate()}
        onSend={handleBroadcast}
        onClose={() => setBroadcastOpen(false)}
      />
```

`accounts` is the already-derived `accountsQuery.data ?? []` near the top of the component; reuse it rather than deriving it again.

- [ ] **Step 9: Run every frontend test**

```bash
cd frontend && npm test -- --run
```
Expected: PASS, including the four new modal tests and every existing test.

- [ ] **Step 10: Confirm CsInboxPage is back under the limit**

```bash
wc -l frontend/src/presentation/pages/CsInboxPage.tsx
```
Expected: under 350. It was 380 before this task, and extracting the header block is what buys the room the new wiring spends.

- [ ] **Step 11: Full frontend gate**

```bash
cd frontend && npm run lint && npm run format:check && npm run build
```
Expected: zero lint errors, formatting clean, build succeeds.

- [ ] **Step 12: Commit**

```bash
git add frontend/src
git commit -m "feat(cs): post updates to a WhatsApp Channel from the inbox"
```

---

## Verifikasi akhir

After Task 8, run both suites from a clean tree:

```bash
cd backend && gofmt -s -l . && go vet ./... && go mod verify && go test ./... -race -cover
cd ../frontend && npm test -- --run && npm run lint && npm run format:check && npm run build
```

Then check the file-size gate on everything this plan created or grew:

```bash
find backend/internal/api backend/internal/services backend/internal/wa backend/cmd/wa \
     frontend/src/presentation/components/cs frontend/src/presentation/pages \
     -name '*.go' -o -name '*.tsx' -o -name '*.ts' | xargs wc -l | sort -rn | head -20
```

No non-test file may exceed 350 lines. If `cs_handler_channels.go` did, split it as Task 6 Step 4 describes.

Finally, run `graphify update .` to keep the knowledge graph current.
