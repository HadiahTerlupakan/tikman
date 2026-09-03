# CS Push Notifications Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A CS, Admin, or Technician gets an OS-level push notification (via Firebase Cloud Messaging) for every incoming WhatsApp message, plus a live "awaiting reply" count on the navbar bell — both working whether or not a TikMan tab is currently open.

**Architecture:** The existing `wa/inbound.go` → Redis `cs:events` → SSE pipeline gains a second, permanent subscriber living inside `cmd/api` (not a new container). On every incoming-message event it fetches the conversation/message and pushes to every registered device token for Admin/CS/Technician users via Firebase. The frontend gains a service worker, an opt-in control, and a navbar badge fed by data that already exists (`last_message_direction = 'in'`).

**Tech Stack:** Go backend (`firebase.google.com/go/v4/messaging`, verified against the real package — not from memory), React/TypeScript frontend (`firebase` npm package, modular v9+ API), GORM/Postgres, Redis pub/sub, Ant Design.

**Spec:** `docs/superpowers/specs/2026-09-03-cs-push-notifications-design.md`

## Global Constraints

- Every Firebase-touching piece of code must degrade to a safe no-op when its configuration is empty — no crash, no blocked startup. Real Firebase project credentials do not exist yet (Task 15 is the one task that needs them, and is the last task for exactly this reason).
- Push notifications go to Admin, CS, and Technician roles only — the same three roles `/api/v1/cs/*` already admits. No per-assignment filtering (every eligible user is notified for every incoming message).
- The message preview truncates to 120 runes with a trailing "…".
- `push_subscriptions.user_id` gets its foreign key from a numbered SQL migration (`ON DELETE CASCADE`), never a GORM tag — this codebase always splits it that way (see migration 41), because the constraint is only exercised under real Postgres, not the SQLite test database.
- **`internal/services` must never import `internal/push`.** The `PushSender` interface lives in `internal/services` (the consumer) and `push.Client` satisfies it structurally. `cmd/wa`, `cmd/worker`, `cmd/trapd`, and `cmd/seed-events` all import `internal/services`; an import edge the other way would drag the Firebase SDK, gRPC, and protobuf into all four of their builds for code they never run. `cmd/api` is the only place that imports both packages.
- No file this plan creates should exceed ~300 lines; none needs to, since the design already splits by responsibility.

---

## Task 1: `push_subscriptions` table

**Files:**
- Create: `backend/internal/models/push_subscription.go`
- Modify: `backend/internal/models/models.go` (register in `AutoMigrate`)
- Create: `backend/migrations/46_push_subscriptions.sql`

**Interfaces:**
- Produces: `models.PushSubscription{ID uuid.UUID, UserID uuid.UUID, FCMToken string, CreatedAt, UpdatedAt time.Time}`, table name `push_subscriptions`.

- [ ] **Step 1: Write the model**

```go
// backend/internal/models/push_subscription.go
package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PushSubscription is one browser/device's Firebase Cloud Messaging
// registration token, owned by the user who registered it. A user can hold
// several rows — one per browser or device.
type PushSubscription struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	UserID    uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`
	FCMToken  string    `gorm:"column:fcm_token;type:varchar(1024);uniqueIndex;not null" json:"fcm_token"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (p *PushSubscription) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

func (PushSubscription) TableName() string { return "push_subscriptions" }
```

- [ ] **Step 2: Register it in AutoMigrate**

In `backend/internal/models/models.go`, add `&PushSubscription{},` to the end of the `db.AutoMigrate(...)` argument list (after `&CSQuickReply{},`).

- [ ] **Step 3: Write the migration**

```sql
-- backend/migrations/46_push_subscriptions.sql
-- AutoMigrate creates the table and its columns; this adds the constraint
-- that only real Postgres enforces (see migration 41 for the same split on
-- cs_conversations). A user deleted with their subscriptions still owned is
-- a device that should simply stop hearing from an account that is gone —
-- CASCADE, not RESTRICT.
ALTER TABLE push_subscriptions
    ADD CONSTRAINT fk_push_subscriptions_user
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
```

- [ ] **Step 4: Verify AutoMigrate and the migration both apply cleanly**

Run: `cd backend && go build ./... && go test ./internal/database/... -run TestEveryMigrationAppliesToAFreshSchema -v`

This test is skipped without `TEST_POSTGRES_DSN`. To actually exercise it, start a disposable Postgres exactly like CI does — do not point it at any real deployment's database:

```bash
docker run -d --name push-migration-check -e POSTGRES_PASSWORD=test -e POSTGRES_DB=tikman_test -p 15433:5432 timescale/timescaledb:latest-pg15
sleep 3
TEST_POSTGRES_DSN="host=localhost port=15433 user=postgres password=test dbname=tikman_test sslmode=disable" go test ./internal/database/... -run TestEveryMigrationAppliesToAFreshSchema -v
docker rm -f push-migration-check
```

Expected: PASS, and (for your own confidence, not a scripted assertion) `psql` into that same container to confirm `push_subscriptions` exists with the FK — mirror exactly how migration 45 was verified in this same project's history if you want the precedent.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/models/push_subscription.go backend/internal/models/models.go backend/migrations/46_push_subscriptions.sql
git commit -m "feat(push): add the push_subscriptions table"
```

---

## Task 2: `PushService`

**Files:**
- Create: `backend/internal/services/push_service.go`
- Test: `backend/internal/services/push_service_test.go`

**Interfaces:**
- Consumes: `models.PushSubscription`, `models.UserRole` (from Task 1 / existing).
- Produces: `NewPushService(db *gorm.DB) *PushService`, `(*PushService).Subscribe(userID uuid.UUID, token string) error`, `(*PushService).Unsubscribe(userID uuid.UUID, token string) error`, `(*PushService).TokensForRoles(roles ...models.UserRole) ([]string, error)`, `(*PushService).RemoveTokens(tokens []string) error`.

- [ ] **Step 1: Write the failing tests**

```go
// backend/internal/services/push_service_test.go
package services

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

func TestPushServiceSubscribeUpsertsByToken(t *testing.T) {
	db := setupTestDB(t)
	service := NewPushService(db)
	user := uuid.New()

	require.NoError(t, service.Subscribe(user, "token-a"))
	require.NoError(t, service.Subscribe(user, "token-a"))

	var count int64
	require.NoError(t, db.Model(&models.PushSubscription{}).Where("fcm_token = ?", "token-a").Count(&count).Error)
	assert.Equal(t, int64(1), count, "re-registering the same token must not duplicate it")
}

// A device that logs in as a different user on a shared machine should be
// heard by whoever is using it now, not whoever registered it first.
func TestPushServiceSubscribeReassignsAnExistingTokenToANewUser(t *testing.T) {
	db := setupTestDB(t)
	service := NewPushService(db)
	first, second := uuid.New(), uuid.New()

	require.NoError(t, service.Subscribe(first, "shared-token"))
	require.NoError(t, service.Subscribe(second, "shared-token"))

	var row models.PushSubscription
	require.NoError(t, db.Where("fcm_token = ?", "shared-token").First(&row).Error)
	assert.Equal(t, second, row.UserID)
}

func TestPushServiceUnsubscribeOnlyRemovesTheCallersOwnToken(t *testing.T) {
	db := setupTestDB(t)
	service := NewPushService(db)
	owner, someoneElse := uuid.New(), uuid.New()
	require.NoError(t, service.Subscribe(owner, "token-a"))

	// someoneElse does not own "token-a" — this must not delete it.
	require.NoError(t, service.Unsubscribe(someoneElse, "token-a"))
	var count int64
	require.NoError(t, db.Model(&models.PushSubscription{}).Where("fcm_token = ?", "token-a").Count(&count).Error)
	assert.Equal(t, int64(1), count)

	require.NoError(t, service.Unsubscribe(owner, "token-a"))
	require.NoError(t, db.Model(&models.PushSubscription{}).Where("fcm_token = ?", "token-a").Count(&count).Error)
	assert.Equal(t, int64(0), count)
}

func TestPushServiceTokensForRolesFiltersByRole(t *testing.T) {
	db := setupTestDB(t)
	push := NewPushService(db)
	users := NewUserService(db)

	admin, err := users.Create("admin1", "admin1@example.com", "password123", "", models.UserRoleAdmin)
	require.NoError(t, err)
	viewer, err := users.Create("viewer1", "viewer1@example.com", "password123", "", models.UserRoleViewer)
	require.NoError(t, err)

	require.NoError(t, push.Subscribe(admin.ID, "admin-token"))
	require.NoError(t, push.Subscribe(viewer.ID, "viewer-token"))

	tokens, err := push.TokensForRoles(models.UserRoleAdmin, models.UserRoleCS, models.UserRoleTechnician)
	require.NoError(t, err)
	assert.Equal(t, []string{"admin-token"}, tokens)
}

func TestPushServiceRemoveTokensDeletesEveryRowNamed(t *testing.T) {
	db := setupTestDB(t)
	service := NewPushService(db)
	user := uuid.New()
	require.NoError(t, service.Subscribe(user, "dead-token"))
	require.NoError(t, service.Subscribe(user, "live-token"))

	require.NoError(t, service.RemoveTokens([]string{"dead-token"}))

	var remaining []string
	require.NoError(t, db.Model(&models.PushSubscription{}).Pluck("fcm_token", &remaining).Error)
	assert.Equal(t, []string{"live-token"}, remaining)
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd backend && go test ./internal/services/... -run TestPushService -v`
Expected: FAIL — `NewPushService` undefined (compile error).

- [ ] **Step 3: Write the implementation**

```go
// backend/internal/services/push_service.go
package services

import (
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PushService owns the device tokens push notifications are sent to.
type PushService struct {
	db *gorm.DB
}

func NewPushService(db *gorm.DB) *PushService {
	return &PushService{db: db}
}

// Subscribe registers a device. Re-registering the same token — the normal
// case on every app load once permission is already granted — updates the
// owner instead of creating a duplicate row, so a token that outlives a
// logout on a shared machine follows whoever is using it now.
func (s *PushService) Subscribe(userID uuid.UUID, token string) error {
	sub := models.PushSubscription{UserID: userID, FCMToken: token}
	return s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "fcm_token"}},
		DoUpdates: clause.AssignmentColumns([]string{"user_id", "updated_at"}),
	}).Create(&sub).Error
}

// Unsubscribe removes a device, scoped to the caller's own userID so one CS
// cannot silently unregister another's.
func (s *PushService) Unsubscribe(userID uuid.UUID, token string) error {
	return s.db.Where("user_id = ? AND fcm_token = ?", userID, token).
		Delete(&models.PushSubscription{}).Error
}

// TokensForRoles returns every distinct device token belonging to a user
// holding one of the given roles.
func (s *PushService) TokensForRoles(roles ...models.UserRole) ([]string, error) {
	var tokens []string
	err := s.db.Model(&models.PushSubscription{}).
		Joins("JOIN users ON users.id = push_subscriptions.user_id").
		Where("users.role IN ?", roles).
		Distinct().
		Pluck("push_subscriptions.fcm_token", &tokens).Error
	return tokens, err
}

// RemoveTokens deletes every row naming one of these tokens — used after a
// send reports them as no longer registered, so the table does not
// accumulate dead devices forever.
func (s *PushService) RemoveTokens(tokens []string) error {
	if len(tokens) == 0 {
		return nil
	}
	return s.db.Where("fcm_token IN ?", tokens).Delete(&models.PushSubscription{}).Error
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cd backend && go test ./internal/services/... -run TestPushService -v`
Expected: PASS (5 tests).

These five tests are also what proves the two GORM constructs above actually
work on the SQLite the test suite uses, rather than only in theory:
`clause.OnConflict` needs the unique index AutoMigrate builds from the
`uniqueIndex` tag, and `Distinct().Pluck(...)` with a table-qualified column
is the fussier of the two. If `TokensForRoles` comes back with a SQL error
rather than a wrong answer, swap that one statement for the explicit form and
re-run — the behaviour under test does not change:

```go
	err := s.db.Model(&models.PushSubscription{}).
		Select("DISTINCT push_subscriptions.fcm_token").
		Joins("JOIN users ON users.id = push_subscriptions.user_id").
		Where("users.role IN ?", roles).
		Find(&tokens).Error
```

- [ ] **Step 5: Commit**

```bash
git add backend/internal/services/push_service.go backend/internal/services/push_service_test.go
git commit -m "feat(push): add PushService for managing device subscriptions"
```

---

## Task 3: Firebase Cloud Messaging client

**Files:**
- Create: `backend/internal/push/client.go`
- Modify: `backend/go.mod`, `backend/go.sum` (via `go get`)

**Interfaces:**
- Produces: `push.NewClient(ctx, serviceAccountJSONB64 string) (*push.Client, error)` — returns `(nil, nil)` when `serviceAccountJSONB64` is empty — and `(*push.Client).SendEach(ctx, tokens []string, title, body string, data map[string]string) (invalidTokens []string, err error)`.

**This package deliberately declares no interface.** The interface it
satisfies (`services.PushSender`) is declared by its consumer in Task 4, the
Go way round. That is not a style preference here — it is load-bearing:
`cmd/wa`, `cmd/worker`, `cmd/trapd`, and `cmd/seed-events` all import
`internal/services`, so if `internal/services` imported `internal/push`,
every one of those four binaries would pull the entire Firebase SDK (gRPC,
protobuf, the Google API client libraries) into its build for code it never
runs. With the interface on the consumer's side, only `cmd/api` ever imports
`internal/push`.

This file wraps an external device/service client the way `internal/wa` wraps
whatsmeow — network-bound glue, not business logic. It is exempt from unit
testing under this project's existing exemption for code that constructs a
real external client and cannot be exercised without live credentials (see
CLAUDE.md's `internal/connectivity` exemption; the same reasoning applies
here). `PushNotifierService` (Task 4) is what gets tested, against a fake.

**Note on package choice:** `option.WithCredentialsJSON` exists but is
deprecated in the currently published SDK in favor of
`option.WithAuthCredentialsJSON(option.ServiceAccount, json)` — verified
directly against the installed package's `go doc` output, not from memory.
Use the non-deprecated form.

- [ ] **Step 1: Add the dependency**

Run: `cd backend && go get firebase.google.com/go/v4@latest`

This pulls in a real dependency tree (gRPC, protobuf, Google API client
libraries — roughly a dozen transitive modules). That cost was accepted
explicitly when Firebase was chosen over a self-hosted VAPID implementation
(see the spec, §2) — it is not an accident to flag as a problem now.

- [ ] **Step 2: Write the client**

```go
// backend/internal/push/client.go
package push

import (
	"context"
	"encoding/base64"
	"fmt"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"
)

// Client sends push notifications through Firebase Cloud Messaging. It
// satisfies services.PushSender, which is declared there rather than here on
// purpose — see the note in this task, and keep this package free of any
// interface so internal/services never has to import it.
type Client struct {
	fcm *messaging.Client
}

// NewClient builds a Client from a base64-encoded service account JSON key.
// An empty serviceAccountJSONB64 returns (nil, nil), never an error — a
// fresh checkout or a deploy before the user's Firebase project exists must
// still start normally. The caller treats a nil *Client as "push is not
// configured" (see cmd/api/main.go).
func NewClient(ctx context.Context, serviceAccountJSONB64 string) (*Client, error) {
	if serviceAccountJSONB64 == "" {
		return nil, nil
	}

	raw, err := base64.StdEncoding.DecodeString(serviceAccountJSONB64)
	if err != nil {
		return nil, fmt.Errorf("decode FIREBASE_SERVICE_ACCOUNT_JSON_B64: %w", err)
	}

	app, err := firebase.NewApp(ctx, nil, option.WithAuthCredentialsJSON(option.ServiceAccount, raw))
	if err != nil {
		return nil, fmt.Errorf("initialize firebase app: %w", err)
	}

	fcm, err := app.Messaging(ctx)
	if err != nil {
		return nil, fmt.Errorf("initialize firebase messaging client: %w", err)
	}

	return &Client{fcm: fcm}, nil
}

// SendEach implements Sender.
func (c *Client) SendEach(ctx context.Context, tokens []string, title, body string, data map[string]string) ([]string, error) {
	messages := make([]*messaging.Message, len(tokens))
	for i, token := range tokens {
		messages[i] = &messaging.Message{
			Token: token,
			Notification: &messaging.Notification{
				Title: title,
				Body:  body,
			},
			Data: data,
		}
	}

	resp, err := c.fcm.SendEach(ctx, messages)
	if err != nil {
		return nil, err
	}

	var invalid []string
	for i, r := range resp.Responses {
		if !r.Success && messaging.IsRegistrationTokenNotRegistered(r.Error) {
			invalid = append(invalid, tokens[i])
		}
	}
	return invalid, nil
}
```

- [ ] **Step 3: Verify it builds**

Run: `cd backend && go build ./...`
Expected: exit 0. (If it does not — the SDK's API surface may have moved since this plan was verified against it; run `go doc firebase.google.com/go/v4/messaging Client` and `go doc google.golang.org/api/option` and adjust the method/option names to match what is actually installed, rather than guessing.)

- [ ] **Step 4: Commit**

```bash
git add backend/internal/push/client.go backend/go.mod backend/go.sum
git commit -m "feat(push): add a Firebase Cloud Messaging client"
```

---

## Task 4: `PushNotifierService`

**Files:**
- Create: `backend/internal/services/push_notifier_service.go`
- Test: `backend/internal/services/push_notifier_service_test.go`

**Interfaces:**
- Consumes: `*PushService` (Task 2), `*CSConversationService.Get(id) (*models.CSConversation, error)`, `*CSMessageService.Get(id) (*models.CSMessage, error)` (both already exist). Note it does **not** consume `internal/push` — see below.
- Produces: `services.PushSender` interface (`SendEach(ctx, tokens []string, title, body string, data map[string]string) (invalidTokens []string, err error)`), `NewPushNotifierService(sender PushSender, subscriptions *PushService, conversations *CSConversationService, messages *CSMessageService) *PushNotifierService`, `(*PushNotifierService).NotifyIncomingMessage(ctx, conversationID, messageID uuid.UUID) error`. Also produces the test-only `FakePushSender` type, reused by Task 5's tests (same package, same directory).

`PushSender` is declared here, in the consumer, and `push.Client` (Task 3)
satisfies it structurally without either package importing the other. This
keeps the Firebase SDK out of the `wa`, `worker`, `trapd`, and `seed-events`
builds, all four of which import `internal/services`.

- [ ] **Step 1: Write the failing tests**

```go
// backend/internal/services/push_notifier_service_test.go
package services

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// FakePushSender records what it was asked to send instead of reaching a
// real Firebase project, the same role FakePresence plays for Presence.
type FakePushSender struct {
	Tokens  []string
	Title   string
	Body    string
	Data    map[string]string
	Invalid []string
}

func (f *FakePushSender) SendEach(_ context.Context, tokens []string, title, body string, data map[string]string) ([]string, error) {
	f.Tokens = tokens
	f.Title = title
	f.Body = body
	f.Data = data
	return f.Invalid, nil
}

// pushTestSetup wires a notifier against a fake Sender and returns the db
// and account id too, since every test also needs to create its own
// conversation (via IncomingPeer.WAAccountID) and, in most cases, real users
// (via NewUserService(db)) sharing this same database.
func pushTestSetup(t *testing.T) (notifier *PushNotifierService, sender *FakePushSender, conversations *CSConversationService, messages *CSMessageService, pushService *PushService, accountID uuid.UUID, db *gorm.DB) {
	t.Helper()
	db = setupTestDB(t)
	account := models.WAAccount{Label: "CS Utama", Status: models.WAAccountConnected}
	require.NoError(t, db.Create(&account).Error)

	conversations = NewCSConversationService(db)
	messages = NewCSMessageService(db, conversations)
	pushService = NewPushService(db)
	sender = &FakePushSender{}
	notifier = NewPushNotifierService(sender, pushService, conversations, messages)
	return notifier, sender, conversations, messages, pushService, account.ID, db
}

func TestNotifyIncomingMessageSendsToEveryEligibleRole(t *testing.T) {
	notifier, sender, conversations, messages, pushService, accountID, db := pushTestSetup(t)
	users := NewUserService(db)

	conv, err := conversations.FindOrCreate(IncomingPeer{
		WAAccountID: accountID,
		JID:         "628111@s.whatsapp.net",
		Phone:       "628111222333",
		Name:        "Budi",
	})
	require.NoError(t, err)
	msg, _, err := messages.SaveInbound(InboundMessage{
		ConversationID: conv.ID,
		WAMessageID:    "wamid-1",
		Kind:           models.MessageKindText,
		Body:           "Internetnya mati sejak semalam",
	})
	require.NoError(t, err)

	admin, err := users.Create("admin1", "admin1@example.com", "password123", "", models.UserRoleAdmin)
	require.NoError(t, err)
	viewer, err := users.Create("viewer1", "viewer1@example.com", "password123", "", models.UserRoleViewer)
	require.NoError(t, err)
	require.NoError(t, pushService.Subscribe(admin.ID, "admin-token"))
	require.NoError(t, pushService.Subscribe(viewer.ID, "viewer-token"))

	require.NoError(t, notifier.NotifyIncomingMessage(context.Background(), conv.ID, msg.ID))

	assert.Equal(t, []string{"admin-token"}, sender.Tokens)
	assert.Equal(t, "Budi", sender.Title)
	assert.Equal(t, "Internetnya mati sejak semalam", sender.Body)
	assert.Equal(t, conv.ID.String(), sender.Data["conversation_id"])
}

func TestNotifyIncomingMessageFallsBackToPhoneWithNoName(t *testing.T) {
	notifier, sender, conversations, messages, pushService, accountID, db := pushTestSetup(t)
	users := NewUserService(db)
	admin, err := users.Create("admin1", "admin1@example.com", "password123", "", models.UserRoleAdmin)
	require.NoError(t, err)
	require.NoError(t, pushService.Subscribe(admin.ID, "admin-token"))

	conv, err := conversations.FindOrCreate(IncomingPeer{
		WAAccountID: accountID,
		JID:         "628222@s.whatsapp.net",
		Phone:       "628222333444",
		Name:        "",
	})
	require.NoError(t, err)
	msg, _, err := messages.SaveInbound(InboundMessage{
		ConversationID: conv.ID, WAMessageID: "wamid-2", Kind: models.MessageKindText, Body: "Halo",
	})
	require.NoError(t, err)

	require.NoError(t, notifier.NotifyIncomingMessage(context.Background(), conv.ID, msg.ID))
	assert.Equal(t, "628222333444", sender.Title)
}

func TestNotifyIncomingMessageTruncatesALongBody(t *testing.T) {
	notifier, sender, conversations, messages, pushService, accountID, db := pushTestSetup(t)
	users := NewUserService(db)
	admin, err := users.Create("admin1", "admin1@example.com", "password123", "", models.UserRoleAdmin)
	require.NoError(t, err)
	require.NoError(t, pushService.Subscribe(admin.ID, "admin-token"))

	conv, err := conversations.FindOrCreate(IncomingPeer{
		WAAccountID: accountID, JID: "628333@s.whatsapp.net", Phone: "628333444555",
	})
	require.NoError(t, err)
	longBody := strings.Repeat("a", 200)
	msg, _, err := messages.SaveInbound(InboundMessage{
		ConversationID: conv.ID, WAMessageID: "wamid-3", Kind: models.MessageKindText, Body: longBody,
	})
	require.NoError(t, err)

	require.NoError(t, notifier.NotifyIncomingMessage(context.Background(), conv.ID, msg.ID))
	assert.Equal(t, strings.Repeat("a", 120)+"…", sender.Body)
}

func TestNotifyIncomingMessageRemovesTokensTheSenderReportsInvalid(t *testing.T) {
	notifier, sender, conversations, messages, pushService, accountID, db := pushTestSetup(t)
	users := NewUserService(db)
	admin, err := users.Create("admin1", "admin1@example.com", "password123", "", models.UserRoleAdmin)
	require.NoError(t, err)
	require.NoError(t, pushService.Subscribe(admin.ID, "dead-token"))
	sender.Invalid = []string{"dead-token"}

	conv, err := conversations.FindOrCreate(IncomingPeer{
		WAAccountID: accountID, JID: "628444@s.whatsapp.net", Phone: "628444555666",
	})
	require.NoError(t, err)
	msg, _, err := messages.SaveInbound(InboundMessage{
		ConversationID: conv.ID, WAMessageID: "wamid-4", Kind: models.MessageKindText, Body: "Halo",
	})
	require.NoError(t, err)

	require.NoError(t, notifier.NotifyIncomingMessage(context.Background(), conv.ID, msg.ID))

	tokens, err := pushService.TokensForRoles(models.UserRoleAdmin)
	require.NoError(t, err)
	assert.Empty(t, tokens)
}

func TestNotifyIncomingMessageSendsNothingWithNoEligibleTokens(t *testing.T) {
	notifier, sender, conversations, messages, _, accountID, _ := pushTestSetup(t)
	conv, err := conversations.FindOrCreate(IncomingPeer{
		WAAccountID: accountID, JID: "628555@s.whatsapp.net", Phone: "628555666777",
	})
	require.NoError(t, err)
	msg, _, err := messages.SaveInbound(InboundMessage{
		ConversationID: conv.ID, WAMessageID: "wamid-5", Kind: models.MessageKindText, Body: "Halo",
	})
	require.NoError(t, err)

	require.NoError(t, notifier.NotifyIncomingMessage(context.Background(), conv.ID, msg.ID))
	assert.Nil(t, sender.Tokens)
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd backend && go test ./internal/services/... -run TestNotifyIncomingMessage -v`
Expected: FAIL — `NewPushNotifierService` undefined.

- [ ] **Step 3: Write the implementation**

```go
// backend/internal/services/push_notifier_service.go
package services

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
)

// PushSender delivers a push notification to a set of device tokens and
// reports which of them the push service considers dead, so the caller can
// stop holding onto them.
//
// Declared here rather than in internal/push so that this package — imported
// by the wa, worker, trapd and seed-events binaries — never pulls the
// Firebase SDK into their builds. push.Client satisfies it structurally.
type PushSender interface {
	SendEach(ctx context.Context, tokens []string, title, body string, data map[string]string) (invalidTokens []string, err error)
}

// pushPreviewRunes is how much of a message body reaches an OS notification —
// enough to judge urgency, short enough to stay well under FCM's per-message
// size limit.
const pushPreviewRunes = 120

// pushEligibleRoles are the roles that can open the CS inbox at all — the
// same three /api/v1/cs/* already admits.
var pushEligibleRoles = []models.UserRole{
	models.UserRoleAdmin, models.UserRoleCS, models.UserRoleTechnician,
}

// PushNotifierService turns one incoming customer message into a push
// notification for every eligible device registered.
type PushNotifierService struct {
	sender        PushSender
	subscriptions *PushService
	conversations *CSConversationService
	messages      *CSMessageService
}

func NewPushNotifierService(sender PushSender, subscriptions *PushService, conversations *CSConversationService, messages *CSMessageService) *PushNotifierService {
	return &PushNotifierService{
		sender:        sender,
		subscriptions: subscriptions,
		conversations: conversations,
		messages:      messages,
	}
}

// NotifyIncomingMessage looks up the conversation and message an EventMessage
// named, then pushes a notification to everyone who can answer the inbox. A
// send failure here is logged by the caller (see PushEventListener) and never
// blocks message storage — push is additive, the same tolerance
// wa/inbound.go already applies to its own SSE announcement.
func (s *PushNotifierService) NotifyIncomingMessage(ctx context.Context, conversationID, messageID uuid.UUID) error {
	conv, err := s.conversations.Get(conversationID)
	if err != nil {
		return fmt.Errorf("look up conversation: %w", err)
	}
	msg, err := s.messages.Get(messageID)
	if err != nil {
		return fmt.Errorf("look up message: %w", err)
	}

	tokens, err := s.subscriptions.TokensForRoles(pushEligibleRoles...)
	if err != nil {
		return fmt.Errorf("list push tokens: %w", err)
	}
	if len(tokens) == 0 {
		return nil
	}

	title := conv.CustomerName
	if title == "" {
		title = conv.CustomerPhone
	}

	invalid, err := s.sender.SendEach(ctx, tokens, title, previewOf(msg.Body), map[string]string{
		"conversation_id": conversationID.String(),
	})
	if err != nil {
		return fmt.Errorf("send push: %w", err)
	}
	if len(invalid) > 0 {
		if err := s.subscriptions.RemoveTokens(invalid); err != nil {
			return fmt.Errorf("remove invalid push tokens: %w", err)
		}
	}
	return nil
}

// previewOf truncates a message body to what an OS notification should show,
// marking the cut with an ellipsis rather than slicing a word in half.
func previewOf(body string) string {
	runes := []rune(body)
	if len(runes) <= pushPreviewRunes {
		return body
	}
	return string(runes[:pushPreviewRunes]) + "…"
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cd backend && go test ./internal/services/... -run "TestNotifyIncomingMessage|TestPushService" -v`
Expected: PASS (10 tests total between the two files).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/services/push_notifier_service.go backend/internal/services/push_notifier_service_test.go
git commit -m "feat(push): add PushNotifierService"
```

---

## Task 5: `PushEventListener`

**Files:**
- Create: `backend/internal/services/push_listener.go`
- Test: `backend/internal/services/push_listener_test.go`

**Interfaces:**
- Consumes: `*redis.Client` (existing), `*PushNotifierService` (Task 4), `wa.Event` / `wa.EventMessage` / `wa.EventsChannel` (existing, `backend/internal/wa/events.go`), `FakePushSender` (Task 4, same package).
- Produces: `NewPushEventListener(redisClient *redis.Client, notifier *PushNotifierService, logger *zap.Logger) *PushEventListener`, `(*PushEventListener).Run(ctx context.Context)`, `(*PushEventListener).HandlePayload(ctx context.Context, payload string)`.

`Run` is thin Redis-subscribe wiring in the same shape as
`cs_handler_stream.go`'s `Stream` handler, and like that handler is not
unit-tested directly — this codebase does not stand up a real Redis pub/sub
in tests (see how `cs_handler_test.go` points its Redis client at an
unreachable address on purpose). `HandlePayload` is where the actual logic
lives, split out specifically so it can be tested without Redis at all.

- [ ] **Step 1: Write the failing tests**

```go
// backend/internal/services/push_listener_test.go
package services

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/wa"
	"go.uber.org/zap"
)

func TestHandlePayloadNotifiesOnAnIncomingMessageEvent(t *testing.T) {
	notifier, sender, conversations, messages, pushService, accountID, db := pushTestSetup(t)
	users := NewUserService(db)
	admin, err := users.Create("admin1", "admin1@example.com", "password123", "", models.UserRoleAdmin)
	require.NoError(t, err)
	require.NoError(t, pushService.Subscribe(admin.ID, "admin-token"))

	conv, err := conversations.FindOrCreate(IncomingPeer{
		WAAccountID: accountID, JID: "628666@s.whatsapp.net", Phone: "628666777888",
	})
	require.NoError(t, err)
	msg, _, err := messages.SaveInbound(InboundMessage{
		ConversationID: conv.ID, WAMessageID: "wamid-6", Kind: models.MessageKindText, Body: "Halo",
	})
	require.NoError(t, err)

	listener := NewPushEventListener(nil, notifier, zap.NewNop())
	payload, err := json.Marshal(wa.Event{
		Type:           wa.EventMessage,
		ConversationID: conv.ID.String(),
		MessageID:      msg.ID.String(),
	})
	require.NoError(t, err)

	listener.HandlePayload(context.Background(), string(payload))
	assert.Equal(t, []string{"admin-token"}, sender.Tokens)
}

func TestHandlePayloadIgnoresEveryEventTypeButMessage(t *testing.T) {
	notifier, sender, _, _, _, _, _ := pushTestSetup(t)
	listener := NewPushEventListener(nil, notifier, zap.NewNop())

	payload, err := json.Marshal(wa.Event{Type: wa.EventStatus})
	require.NoError(t, err)

	listener.HandlePayload(context.Background(), string(payload))
	assert.Nil(t, sender.Tokens, "a non-message event must never trigger a send")
}

func TestHandlePayloadIgnoresUndecodablePayloads(t *testing.T) {
	notifier, sender, _, _, _, _, _ := pushTestSetup(t)
	listener := NewPushEventListener(nil, notifier, zap.NewNop())

	listener.HandlePayload(context.Background(), "not json")
	assert.Nil(t, sender.Tokens)
}

func TestHandlePayloadIgnoresAMessageEventWithAnUnparsableID(t *testing.T) {
	notifier, sender, _, _, _, _, _ := pushTestSetup(t)
	listener := NewPushEventListener(nil, notifier, zap.NewNop())

	payload, err := json.Marshal(wa.Event{
		Type: wa.EventMessage, ConversationID: "not-a-uuid", MessageID: uuid.New().String(),
	})
	require.NoError(t, err)

	listener.HandlePayload(context.Background(), string(payload))
	assert.Nil(t, sender.Tokens)
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd backend && go test ./internal/services/... -run TestHandlePayload -v`
Expected: FAIL — `NewPushEventListener` undefined.

- [ ] **Step 3: Write the implementation**

```go
// backend/internal/services/push_listener.go
package services

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/tikman/olt-provisioning/internal/wa"
	"go.uber.org/zap"
)

// PushEventListener relays incoming-message events from the same cs:events
// channel the SSE stream reads, turning each into a push notification.
type PushEventListener struct {
	redis    *redis.Client
	notifier *PushNotifierService
	logger   *zap.Logger
}

func NewPushEventListener(redisClient *redis.Client, notifier *PushNotifierService, logger *zap.Logger) *PushEventListener {
	return &PushEventListener{redis: redisClient, notifier: notifier, logger: logger}
}

// Run subscribes to wa.EventsChannel and pushes a notification for every
// incoming customer message, until ctx is done or the connection drops — the
// same run-until-stopped shape as cs_handler_stream.go's Stream handler.
func (l *PushEventListener) Run(ctx context.Context) {
	sub := l.redis.Subscribe(ctx, wa.EventsChannel)
	defer func() { _ = sub.Close() }()
	incoming := sub.Channel()

	for {
		select {
		case <-ctx.Done():
			return
		case msg, open := <-incoming:
			if !open {
				return
			}
			l.HandlePayload(ctx, msg.Payload)
		}
	}
}

// HandlePayload decodes one cs:events message and, if it announces an
// incoming customer message, triggers a push notification. Kept separate
// from Run so it is testable without a real Redis connection.
func (l *PushEventListener) HandlePayload(ctx context.Context, payload string) {
	var event wa.Event
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		l.logger.Warn("Could not decode a cs:events payload", zap.Error(err))
		return
	}
	if event.Type != wa.EventMessage {
		return
	}

	convID, err := uuid.Parse(event.ConversationID)
	if err != nil {
		l.logger.Warn("cs:events message carried an invalid conversation id", zap.Error(err))
		return
	}
	msgID, err := uuid.Parse(event.MessageID)
	if err != nil {
		l.logger.Warn("cs:events message carried an invalid message id", zap.Error(err))
		return
	}

	if err := l.notifier.NotifyIncomingMessage(ctx, convID, msgID); err != nil {
		l.logger.Warn("Could not send push notifications for an incoming message", zap.Error(err))
	}
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cd backend && go test ./internal/services/... -run TestHandlePayload -v`
Expected: PASS (4 tests).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/services/push_listener.go backend/internal/services/push_listener_test.go
git commit -m "feat(push): add PushEventListener"
```

---

## Task 6: `/api/v1/push` endpoints

**Files:**
- Create: `backend/internal/api/push_handler.go`
- Test: `backend/internal/api/push_handler_test.go`
- Modify: `backend/internal/config/config.go` (new field)
- Modify: `backend/internal/api/router.go` (route registration)
- Modify: `backend/.env.example`, `docker-compose.yml`

**Interfaces:**
- Consumes: `*services.PushService` (Task 2), `middleware.GetUserID(c) (uuid.UUID, bool)` (existing), `bindJSON(c, target) bool` (existing, `distribution_dto.go`).
- Produces: `PushHandler` with `Subscribe(c *gin.Context)` and `Unsubscribe(c *gin.Context)`, routes `POST /api/v1/push/subscribe` and `DELETE /api/v1/push/subscribe`, `Config.FirebaseServiceAccountJSONB64`.

- [ ] **Step 1: Add the config field**

In `backend/internal/config/config.go`, add to the `Config` struct (after `WAMediaRetentionDays int`):

```go
	// FirebaseServiceAccountJSONB64 is the base64-encoded Firebase service
	// account key used to send push notifications. Empty means the feature is
	// not configured yet — cmd/api must still start normally (see
	// internal/push.NewClient).
	FirebaseServiceAccountJSONB64 string
```

And inside `Load()`, add to the `cfg := &Config{...}` literal (after `WAMediaRetentionDays: ...`):

```go
		FirebaseServiceAccountJSONB64: viper.GetString("FIREBASE_SERVICE_ACCOUNT_JSON_B64"),
```

No `viper.SetDefault` and no `validateConfig` check — an empty value is valid.

- [ ] **Step 2: Write the failing tests**

```go
// backend/internal/api/push_handler_test.go
package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
)

// asPushUser builds the router as one authenticated request would see it —
// the same fake-session-then-real-routes shape cs_handler_test.go's asUser
// uses, minus RequireRole, since any logged-in role may manage its own
// device token.
func asPushUser(handler *PushHandler, userID uuid.UUID) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Next()
	})
	router.POST("/api/v1/push/subscribe", handler.Subscribe)
	router.DELETE("/api/v1/push/subscribe", handler.Unsubscribe)
	return router
}

func setupPushHandler(t *testing.T) (*PushHandler, *gorm.DB) {
	t.Helper()
	db := TestDB(t)
	return NewPushHandler(services.NewPushService(db)), db
}

// tokensStored reads push_subscriptions directly, deliberately bypassing
// PushService.TokensForRoles — that method inner-joins to users, and these
// handler tests authenticate with a bare uuid.New() rather than a real User
// row, exactly to keep them about the handler's own scoping, not about
// TokensForRoles's join (already covered by Task 2's tests, which do create
// real users).
func tokensStored(t *testing.T, db *gorm.DB) []string {
	t.Helper()
	var tokens []string
	require.NoError(t, db.Model(&models.PushSubscription{}).Pluck("fcm_token", &tokens).Error)
	return tokens
}

func pushRequest(method, body string) *http.Request {
	req := httptest.NewRequest(method, "/api/v1/push/subscribe", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func TestPushSubscribeStoresTheCallersToken(t *testing.T) {
	handler, db := setupPushHandler(t)
	router := asPushUser(handler, uuid.New())

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, pushRequest(http.MethodPost, `{"fcm_token":"token-a"}`))

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, []string{"token-a"}, tokensStored(t, db))
}

func TestPushSubscribeRejectsAMissingToken(t *testing.T) {
	handler, _ := setupPushHandler(t)
	router := asPushUser(handler, uuid.New())

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, pushRequest(http.MethodPost, `{}`))

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestPushUnsubscribeIsANoOpForATokenTheCallerDoesNotOwn(t *testing.T) {
	handler, db := setupPushHandler(t)
	owner := uuid.New()
	require.NoError(t, services.NewPushService(db).Subscribe(owner, "token-a"))
	router := asPushUser(handler, uuid.New())

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, pushRequest(http.MethodDelete, `{"fcm_token":"token-a"}`))

	assert.Equal(t, http.StatusOK, rec.Code, "unsubscribing someone else's token must not error and reveal it exists")
	assert.Equal(t, []string{"token-a"}, tokensStored(t, db), "the token must still be there")
}

func TestPushUnsubscribeRemovesTheCallersOwnToken(t *testing.T) {
	handler, db := setupPushHandler(t)
	owner := uuid.New()
	require.NoError(t, services.NewPushService(db).Subscribe(owner, "token-a"))
	router := asPushUser(handler, owner)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, pushRequest(http.MethodDelete, `{"fcm_token":"token-a"}`))

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, tokensStored(t, db))
}
```

Add `"gorm.io/gorm"` to this file's import block (needed for the `*gorm.DB`
now threaded through `setupPushHandler` and `tokensStored`).

- [ ] **Step 3: Run the tests to verify they fail**

Run: `cd backend && go test ./internal/api/... -run TestPushSubscribe -v && go test ./internal/api/... -run TestPushUnsubscribe -v`
Expected: FAIL — `NewPushHandler` undefined.

- [ ] **Step 4: Write the handler**

```go
// backend/internal/api/push_handler.go
package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tikman/olt-provisioning/internal/middleware"
	"github.com/tikman/olt-provisioning/internal/services"
)

// PushTokenRequest is the body both push endpoints share — subscribing and
// unsubscribing name exactly the same thing, the caller's own device token.
type PushTokenRequest struct {
	FCMToken string `json:"fcm_token" binding:"required"`
}

// PushHandler registers and removes the devices push notifications are sent
// to. Registering your own device is not a CS-specific action, so unlike the
// rest of /api/v1/cs/*, no role is required beyond being logged in.
type PushHandler struct {
	push *services.PushService
}

func NewPushHandler(push *services.PushService) *PushHandler {
	return &PushHandler{push: push}
}

// Subscribe registers the caller's device token.
func (h *PushHandler) Subscribe(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthenticated", Code: "UNAUTHENTICATED"})
		return
	}
	var req PushTokenRequest
	if !bindJSON(c, &req) {
		return
	}
	if err := h.push.Subscribe(userID, req.FCMToken); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to subscribe", Code: "PUSH_SUBSCRIBE_FAILED"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"status": "subscribed"}})
}

// Unsubscribe removes the caller's device token. Removing a token the caller
// does not own succeeds silently rather than erroring — the same non-answer
// either way keeps the endpoint from confirming whether some other token
// exists at all.
func (h *PushHandler) Unsubscribe(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthenticated", Code: "UNAUTHENTICATED"})
		return
	}
	var req PushTokenRequest
	if !bindJSON(c, &req) {
		return
	}
	if err := h.push.Unsubscribe(userID, req.FCMToken); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to unsubscribe", Code: "PUSH_UNSUBSCRIBE_FAILED"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"status": "unsubscribed"}})
}
```

- [ ] **Step 5: Wire the route**

In `backend/internal/api/router.go`, construct the service and handler near
where `csConversationService` and friends are built (Task 7 wires the real
`PushService` used by the notifier — this handler can share that same
instance once Task 7 lands; for now, construct its own):

```go
	pushService := services.NewPushService(db)
	pushHandler := NewPushHandler(pushService)
```

Then, alongside the other `api.Group(...)` blocks, add:

```go
		push := api.Group("/push")
		push.Use(middleware.AuthMiddleware(authStore, logger))
		{
			push.POST("/subscribe", pushHandler.Subscribe)
			push.DELETE("/subscribe", pushHandler.Unsubscribe)
		}
```

- [ ] **Step 6: Document the new env var and pass it into the container**

Append to `backend/.env.example` (after the `CS WhatsApp module` block):

```
# Push notifications (Firebase Cloud Messaging). Base64-encoded service
# account JSON from Firebase Console > Project Settings > Service accounts >
# Generate new private key. Empty disables push — the API still starts.
FIREBASE_SERVICE_ACCOUNT_JSON_B64=
```

In `docker-compose.yml`, inside the `api` service's `environment:` list (next
to `ENCRYPTION_KEY=${ENCRYPTION_KEY}`), add:

```yaml
      - FIREBASE_SERVICE_ACCOUNT_JSON_B64=${FIREBASE_SERVICE_ACCOUNT_JSON_B64:-}
```

The `:-` default matters here — unlike `ENCRYPTION_KEY` (required, no
default), this one must not make `docker compose config` warn about an unset
variable on every deploy before the user's Firebase project exists.

- [ ] **Step 7: Run the tests to verify they pass**

Run: `cd backend && go build ./... && go test ./internal/api/... -run "TestPushSubscribe|TestPushUnsubscribe" -v`
Expected: PASS (4 tests).

- [ ] **Step 8: Commit**

```bash
git add backend/internal/api/push_handler.go backend/internal/api/push_handler_test.go backend/internal/config/config.go backend/internal/api/router.go backend/.env.example docker-compose.yml
git commit -m "feat(push): add push subscription endpoints"
```

---

## Task 7: Wire the listener into `cmd/api`

**Files:**
- Modify: `backend/cmd/api/main.go`
- Modify: `backend/internal/api/router.go` (reuse one `PushService` instance instead of two)

**Interfaces:**
- Consumes: `push.NewClient` (Task 3), `services.NewPushNotifierService` (Task 4), `services.NewPushEventListener` (Task 5), `cfg.FirebaseServiceAccountJSONB64` (Task 6), the existing `csConversationService`, `csMessageService`, `csRedisClient` already constructed in `router.Setup`.

Task 6 built its own `PushService` inside `router.Setup` for the handler.
This task threads one shared instance through instead, and starts the
listener only when a real Sender exists.

- [ ] **Step 1: Share one `PushService` and expose what `main.go` needs**

`router.Setup` currently returns only `*gin.Engine`. Change its signature to
also return the pieces `main.go` needs to start the listener, since those
services are constructed inside `Setup`, not in `main.go` itself:

```go
func Setup(ginEngine *gin.Engine, cfg *config.Config, db *gorm.DB, authStore *auth.Store, logger *zap.Logger, wgService *services.WireGuardService) (*gin.Engine, *services.PushNotifierService, *redis.Client) {
```

Inside `Setup`, replace the `pushService := services.NewPushService(db)`
line Task 6 added with:

```go
	pushService := services.NewPushService(db)
	pushHandler := NewPushHandler(pushService)
	// nil Sender for now: main.go is the one place that knows whether a real
	// Firebase client exists, and sets it via SetSender once Setup returns —
	// Setup only wires the notifier's other dependencies.
	pushNotifier := services.NewPushNotifierService(nil, pushService, csConversationService, csMessageService)
```

`PushNotifierService`'s `sender` field is unexported, so `main.go` needs a
setter to fill it in after the fact. Add this to
`backend/internal/services/push_notifier_service.go`:

```go
// SetSender replaces the Sender after construction — used by cmd/api, which
// only knows whether a real Firebase client exists after Setup has already
// built the notifier alongside everything else it depends on.
func (s *PushNotifierService) SetSender(sender PushSender) {
	s.sender = sender
}
```

`Setup`'s parameter is named `ginEngine`, but its very first line is `router
:= ginEngine`, and everything after uses `router`. Its last line today is
`return router` — change that one line to
`return router, pushNotifier, csRedisClient`.

- [ ] **Step 2: Update `main.go`'s call site and start the listener**

In `backend/cmd/api/main.go`, the line `router := api.Setup(engine, cfg, db,
sessionStore, log, wgService)` becomes:

```go
	router, pushNotifier, csRedisClient := api.Setup(engine, cfg, db, sessionStore, log, wgService)

	pushClient, err := push.NewClient(context.Background(), cfg.FirebaseServiceAccountJSONB64)
	if err != nil {
		log.Warn("Push notifications are not available", zap.Error(err))
	} else if pushClient != nil {
		pushNotifier.SetSender(pushClient)
		pushListener := services.NewPushEventListener(csRedisClient, pushNotifier, log)
		go pushListener.Run(context.Background())
		log.Info("Push notification listener started")
	} else {
		log.Info("FIREBASE_SERVICE_ACCOUNT_JSON_B64 not set — push notifications disabled")
	}
```

placed right after the `router := ...` line (now `router, pushNotifier,
csRedisClient := ...`) and before the existing `addr := fmt.Sprintf(...)`
line. Add `"github.com/tikman/olt-provisioning/internal/push"` to `main.go`'s
imports; `context` is already imported (used by `wgService.RunStatusRefresher`).

A `pushClient` error is logged and swallowed, exactly like a missing
`FIREBASE_SERVICE_ACCOUNT_JSON_B64` — a malformed value should not stop the
whole API from starting, the same tolerance the rest of this feature gives
an absent one.

- [ ] **Step 3: Verify it builds and every existing test still passes**

Run: `cd backend && go build ./... && go vet ./... && go test ./... 2>&1 | tail -20`
Expected: build succeeds, `go vet` silent, all packages `ok`.

- [ ] **Step 4: Verify the inert path by hand**

Run: `cd backend && FIREBASE_SERVICE_ACCOUNT_JSON_B64= go run cmd/api/main.go` (with whatever other env vars your local setup already needs — same as any other local run) and confirm the log line reads `FIREBASE_SERVICE_ACCOUNT_JSON_B64 not set — push notifications disabled`, and that `/health` still answers. Stop it with Ctrl-C.

- [ ] **Step 5: Commit**

```bash
git add backend/cmd/api/main.go backend/internal/api/router.go backend/internal/services/push_notifier_service.go
git commit -m "feat(push): start the push listener when Firebase is configured"
```

---

## Task 8: `useCsConversations` and `useCsStream` gain an `enabled` switch

**Files:**
- Modify: `frontend/src/application/hooks/useCsInbox.ts`
- Modify: `frontend/src/application/hooks/useCsStream.ts`

**Interfaces:**
- Produces: `useCsConversations(filter?, options?: {enabled?: boolean})` (backward compatible — existing call sites pass nothing and are unaffected), `useCsStream(enabled = true): WaStreamStatus`.

This is a small prerequisite Task 14 needs: once the navbar badge and the SSE
stream run at the app-shell level (every page, every role), a Viewer must not
have their browser silently retry a 403'd `/api/v1/cs/conversations` request
or reconnect forever to a 403'd `/api/v1/cs/stream` `EventSource` on every
page load.

- [ ] **Step 1: Write the failing tests**

Two files. First, the stream — this is the one that matters most, since an
`EventSource` pointed at a route the viewer gets 403 from reconnects on its
own, forever. Add to the **existing** `useCsStream.test.ts`, inside its
existing `describe("useCsStream", ...)` block, reusing the `FakeEventSource`
and `wrapper` helpers already defined at the top of that file:

```ts
  // AppLayout runs this hook on every page for every role, so a Viewer — who
  // gets 403 from /api/v1/cs/stream — must never open the connection at all.
  // EventSource reconnects on its own after an error, so "it fails harmlessly"
  // is not true: it fails in a loop.
  it("opens no connection when disabled", () => {
    const client = new QueryClient();

    renderHook(() => useCsStream(false), { wrapper: wrapper(client) });

    expect(FakeEventSource.instances).toHaveLength(0);
  });
```

Second, a new file for the query hook. Note it deliberately does **not**
import `./setupMocks`: that helper mocks `@/infrastructure/repositories`
with a fixed set of repos that does not include `CsRepository`, so importing
it would make `new CsRepository()` (at module scope in `useCsInbox.ts`) throw
"not a constructor". No repository mock is needed here anyway — a disabled
query never calls its `queryFn`, so nothing reaches the real repository.
This mirrors how `useCsStream.test.ts` already stands on its own.

```ts
// frontend/src/application/__tests__/useCsInbox.test.ts
import { describe, expect, it } from "vitest";
import { renderHook } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import React from "react";
import { useCsConversations } from "../hooks/useCsInbox";

function wrapper(client: QueryClient) {
  return ({ children }: { children: React.ReactNode }) =>
    React.createElement(QueryClientProvider, { client }, children);
}

describe("useCsConversations", () => {
  // AppLayout asks for the awaiting-reply count on every page, including for
  // a Viewer, who gets 403 from the endpoint. A disabled query must not send
  // that request at all.
  it("does not fetch when disabled", () => {
    const client = new QueryClient();

    const { result } = renderHook(
      () => useCsConversations({ awaitingReply: true }, { enabled: false }),
      { wrapper: wrapper(client) },
    );

    // In TanStack Query v5 a disabled query sits at status "pending" with
    // fetchStatus "idle" — it is the fetchStatus that says no request went out.
    expect(result.current.fetchStatus).toBe("idle");
  });
});
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd frontend && npx vitest run src/application/__tests__/useCsInbox.test.ts src/application/__tests__/useCsStream.test.ts`
Expected: both new tests FAIL — neither hook accepts the new argument yet, so
`useCsStream(false)` still opens a connection and `useCsConversations(...,
{enabled: false})` is a TypeScript error. The two pre-existing
`useCsStream` tests must still pass.

- [ ] **Step 3: Update the hooks**

In `frontend/src/application/hooks/useCsInbox.ts`:

```ts
export function useCsConversations(
  filter?: CsConversationFilter,
  options?: { enabled?: boolean },
) {
  return useQuery({
    queryKey: ["cs", "conversations", filter],
    queryFn: () => csRepository.getConversations(filter),
    enabled: options?.enabled,
  });
}
```

In `frontend/src/application/hooks/useCsStream.ts`, change the signature and
guard the effect:

```ts
export function useCsStream(enabled = true): WaStreamStatus {
  const queryClient = useQueryClient();
  const [waStatus, setWaStatus] = useState<WaStreamStatus>({});

  useEffect(() => {
    if (!enabled) return;
    const source = new EventSource(`${env.apiUrl}${API_ENDPOINTS.CS_STREAM}`, {
      withCredentials: true,
    });
    // ... body unchanged from here ...
    source.addEventListener("cs", onEvent);
    return () => source.close();
  }, [queryClient, enabled]);

  return waStatus;
}
```

(Keep the existing `onEvent` function body exactly as it is — only the
`if (!enabled) return;` guard and the `enabled` dependency are new.)

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cd frontend && npx vitest run src/application/__tests__/useCsInbox.test.ts src/application/__tests__/useCsStream.test.ts`
Expected: PASS — including the two tests that already existed in
`useCsStream.test.ts`, which call `useCsStream()` with no argument and must
keep working on the new default.

Then the full suite: `npx vitest run`
Expected: green — confirms no existing caller of either hook broke.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/application/hooks/useCsInbox.ts frontend/src/application/hooks/useCsStream.ts frontend/src/application/__tests__/useCsInbox.test.ts frontend/src/application/__tests__/useCsStream.test.ts
git commit -m "feat(push): let useCsConversations and useCsStream be turned off"
```

---

## Task 9: Firebase config and the messaging wrapper

**Files:**
- Create: `frontend/src/shared/config/firebase.ts`
- Create: `frontend/src/infrastructure/firebase/messaging.ts`
- Modify: `frontend/src/vite-env.d.ts` (declare the new env vars — required, see Step 2)
- Modify: `frontend/.env.example`
- Modify: `frontend/package.json` (via `npm install`)

**Interfaces:**
- Produces: `firebaseConfig`, `firebaseVapidKey`, `isFirebaseConfigured` (from `shared/config/firebase.ts`); `type PushPermission = "default" | "granted" | "denied" | "unsupported"`, `requestPushPermission(): Promise<{permission: PushPermission; token?: string}>`, `refreshTokenIfGranted(): Promise<string | undefined>`, `listenForForegroundMessages(onIncoming: (title: string, body: string) => void): Promise<() => void>` (from `infrastructure/firebase/messaging.ts`).

No automated test for this file: everything in it either talks to a real
browser API (`Notification`, `navigator.serviceWorker`) or a real Firebase
project neither exists in a test environment nor exists yet at all for this
project. Manual verification happens once Task 15 supplies real credentials
(see the spec, §9).

- [ ] **Step 1: Install the dependency**

Run: `cd frontend && npm install firebase`

- [ ] **Step 2: Declare the new env vars to TypeScript**

`frontend/src/vite-env.d.ts` declares `ImportMetaEnv` explicitly, with only
`VITE_API_URL` and `VITE_APP_NAME`, and its local `interface ImportMeta`
replaces the permissive one from `vite/client`. Reading any other
`import.meta.env.VITE_*` is therefore a compile error, not a silent
`undefined` — Step 4's `firebase.ts` will not compile without this. Add the
seven new entries:

```ts
/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_API_URL: string;
  readonly VITE_APP_NAME: string;
  readonly VITE_FIREBASE_API_KEY: string;
  readonly VITE_FIREBASE_AUTH_DOMAIN: string;
  readonly VITE_FIREBASE_PROJECT_ID: string;
  readonly VITE_FIREBASE_STORAGE_BUCKET: string;
  readonly VITE_FIREBASE_MESSAGING_SENDER_ID: string;
  readonly VITE_FIREBASE_APP_ID: string;
  readonly VITE_FIREBASE_VAPID_KEY: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
```

They are typed `string` rather than `string | undefined` to match the two
that already exist; `firebase.ts` still applies `|| ""` to each, because at
runtime an unset variable really is `undefined` whatever the type says.

- [ ] **Step 3: Add the env vars**

Append to `frontend/.env.example`:

```
# Firebase Cloud Messaging (push notifications). All of these are public by
# Firebase's own design — safe to commit once real values exist. Leave blank
# and the app runs with push notifications simply turned off.
VITE_FIREBASE_API_KEY=
VITE_FIREBASE_AUTH_DOMAIN=
VITE_FIREBASE_PROJECT_ID=
VITE_FIREBASE_STORAGE_BUCKET=
VITE_FIREBASE_MESSAGING_SENDER_ID=
VITE_FIREBASE_APP_ID=
VITE_FIREBASE_VAPID_KEY=
```

- [ ] **Step 4: Write the config module**

```ts
// frontend/src/shared/config/firebase.ts
export const firebaseConfig = {
  apiKey: import.meta.env.VITE_FIREBASE_API_KEY || "",
  authDomain: import.meta.env.VITE_FIREBASE_AUTH_DOMAIN || "",
  projectId: import.meta.env.VITE_FIREBASE_PROJECT_ID || "",
  storageBucket: import.meta.env.VITE_FIREBASE_STORAGE_BUCKET || "",
  messagingSenderId: import.meta.env.VITE_FIREBASE_MESSAGING_SENDER_ID || "",
  appId: import.meta.env.VITE_FIREBASE_APP_ID || "",
} as const;

export const firebaseVapidKey = import.meta.env.VITE_FIREBASE_VAPID_KEY || "";

/** False until a real Firebase project is configured. Every function in
 * infrastructure/firebase/messaging.ts checks this first and resolves to a
 * safe "nothing happened" instead of throwing against an unconfigured app. */
export const isFirebaseConfigured = Boolean(firebaseConfig.apiKey);
```

- [ ] **Step 5: Write the messaging wrapper**

```ts
// frontend/src/infrastructure/firebase/messaging.ts
import { initializeApp, type FirebaseApp } from "firebase/app";
import {
  getMessaging,
  getToken,
  isSupported,
  onMessage,
  type Messaging,
} from "firebase/messaging";
import {
  firebaseConfig,
  firebaseVapidKey,
  isFirebaseConfigured,
} from "@/shared/config/firebase";

let app: FirebaseApp | undefined;
let messagingInstance: Messaging | undefined;

/** Lazily creates the Firebase app and Messaging instance on first real use,
 * never at module load — importing this file must have no effect on a
 * checkout with no Firebase project configured. */
async function messaging(): Promise<Messaging | undefined> {
  if (!isFirebaseConfigured) return undefined;
  if (!(await isSupported())) return undefined;
  if (!messagingInstance) {
    app ??= initializeApp(firebaseConfig);
    messagingInstance = getMessaging(app);
  }
  return messagingInstance;
}

export type PushPermission = "default" | "granted" | "denied" | "unsupported";

function currentPermission(): PushPermission {
  if (typeof Notification === "undefined") return "unsupported";
  return Notification.permission;
}

async function registerAndGetToken(): Promise<string | undefined> {
  const instance = await messaging();
  if (!instance) return undefined;
  const registration = await navigator.serviceWorker.register(
    "/firebase-messaging-sw.js",
  );
  return getToken(instance, {
    vapidKey: firebaseVapidKey,
    serviceWorkerRegistration: registration,
  });
}

/** Asks the browser for permission, registers the service worker, and
 * returns the device token to hand the backend. Only ever called from an
 * explicit user action — never on page load. */
export async function requestPushPermission(): Promise<{
  permission: PushPermission;
  token?: string;
}> {
  if (typeof Notification === "undefined") {
    return { permission: "unsupported" };
  }
  const permission = await Notification.requestPermission();
  if (permission !== "granted") {
    return { permission };
  }
  const token = await registerAndGetToken();
  return { permission, token };
}

/** Re-fetches the device token without prompting, for a session where
 * permission was already granted on a previous visit. Resolves undefined
 * whenever permission is not already granted or Firebase is not configured. */
export async function refreshTokenIfGranted(): Promise<string | undefined> {
  if (currentPermission() !== "granted") return undefined;
  return registerAndGetToken();
}

/** Shows a notification for a push that arrives while a tab is focused — FCM
 * only runs the service worker's background handler otherwise. Resolves a
 * no-op unsubscribe when Firebase is not configured, so a caller can always
 * treat the return value as safe to call in a cleanup function. */
export async function listenForForegroundMessages(
  onIncoming: (title: string, body: string) => void,
): Promise<() => void> {
  const instance = await messaging();
  if (!instance) return () => {};
  return onMessage(instance, (payload) => {
    onIncoming(
      payload.notification?.title ?? "",
      payload.notification?.body ?? "",
    );
  });
}
```

- [ ] **Step 6: Verify it builds**

Run: `cd frontend && npm run build`
Expected: exit 0. (If the `firebase/messaging` exports have moved since this
plan was written, run `npx tsc --noEmit` and read the exact error — adjust
import names to match what the installed version actually exports rather
than guessing.)

- [ ] **Step 7: Commit**

```bash
git add frontend/src/shared/config/firebase.ts frontend/src/infrastructure/firebase/messaging.ts frontend/src/vite-env.d.ts frontend/.env.example frontend/package.json frontend/package-lock.json
git commit -m "feat(push): add the Firebase messaging wrapper"
```

---

## Task 10: The messaging service worker

**Files:**
- Create: `frontend/public/firebase-messaging-sw.js`

**Interfaces:**
- Produces: a static file served at `/firebase-messaging-sw.js`, registered by `registerAndGetToken()` (Task 9).

This file cannot import `src/shared/config/firebase.ts` — it loads via
`importScripts`, outside Vite's module graph — so the same six config values
are duplicated here on purpose (noted in a comment, so a future edit to one
does not silently forget the other). The literal values below are
placeholders because the user's Firebase project does not exist yet; Task 15
is the one place they get filled in for real, and no other step in this plan
depends on them being correct yet.

- [ ] **Step 1: Write the file**

```js
// frontend/public/firebase-messaging-sw.js
// Keep the six config values below identical to src/shared/config/firebase.ts
// — a service worker cannot import that module, so this is a deliberate
// duplicate, not a second source of truth. Filled in for real in the task
// that runs once the Firebase project exists (see
// docs/superpowers/specs/2026-09-03-cs-push-notifications-design.md §7).
importScripts("https://www.gstatic.com/firebasejs/11.6.1/firebase-app-compat.js");
importScripts("https://www.gstatic.com/firebasejs/11.6.1/firebase-messaging-compat.js");

firebase.initializeApp({
  apiKey: "REPLACE_WITH_FIREBASE_API_KEY",
  authDomain: "REPLACE_WITH_FIREBASE_AUTH_DOMAIN",
  projectId: "REPLACE_WITH_FIREBASE_PROJECT_ID",
  storageBucket: "REPLACE_WITH_FIREBASE_STORAGE_BUCKET",
  messagingSenderId: "REPLACE_WITH_FIREBASE_MESSAGING_SENDER_ID",
  appId: "REPLACE_WITH_FIREBASE_APP_ID",
});

const messaging = firebase.messaging();

messaging.onBackgroundMessage((payload) => {
  const title = payload.notification?.title || "TikMan";
  const body = payload.notification?.body || "Pesan baru masuk";
  self.registration.showNotification(title, {
    body,
    icon: "/favicon.ico",
    data: payload.data,
  });
});

self.addEventListener("notificationclick", (event) => {
  event.notification.close();
  event.waitUntil(
    clients.matchAll({ type: "window", includeUncontrolled: true }).then((windowClients) => {
      for (const client of windowClients) {
        if (client.url.includes("/cs") && "focus" in client) {
          return client.focus();
        }
      }
      return clients.openWindow("/cs");
    }),
  );
});
```

The path is `/cs`, not `/cs-inbox` — verified in
`frontend/src/presentation/routes/index.tsx`, where the route is `path: "cs"`
nested under `/` → `ProtectedRoute` → `AppLayout`. Both places above already
use it; do not "correct" them to a longer name.

- [ ] **Step 2: Verify it is served**

Run: `cd frontend && npm run dev` in one terminal, then in another:
`curl -sS http://localhost:5173/firebase-messaging-sw.js | head -5`
(adjust the port to whatever the dev server actually prints)
Expected: the file's own content, proving Vite serves `public/` files verbatim at the root.

- [ ] **Step 3: Commit**

```bash
git add frontend/public/firebase-messaging-sw.js
git commit -m "feat(push): add the Firebase messaging service worker"
```

---

## Task 11: `PushRepository`

**Files:**
- Create: `frontend/src/domain/repositories/IPushRepository.ts`
- Create: `frontend/src/infrastructure/repositories/PushRepository.ts`
- Modify: `frontend/src/domain/repositories/index.ts`
- Modify: `frontend/src/infrastructure/repositories/index.ts`
- Modify: `frontend/src/infrastructure/http/endpoints.ts`

**Interfaces:**
- Produces: `IPushRepository{subscribe(fcmToken): Promise<void>; unsubscribe(fcmToken): Promise<void>}`, `PushRepository` implementing it, `API_ENDPOINTS.PUSH_SUBSCRIBE`.

- [ ] **Step 1: Add the endpoint**

In `frontend/src/infrastructure/http/endpoints.ts`, add (near the other `CS_*` entries):

```ts
  PUSH_SUBSCRIBE: "/api/v1/push/subscribe",
```

- [ ] **Step 2: Write the interface and implementation**

```ts
// frontend/src/domain/repositories/IPushRepository.ts
export interface IPushRepository {
  subscribe(fcmToken: string): Promise<void>;
  unsubscribe(fcmToken: string): Promise<void>;
}
```

```ts
// frontend/src/infrastructure/repositories/PushRepository.ts
import { apiClient } from "../http/apiClient";
import { API_ENDPOINTS } from "../http/endpoints";
import type { IPushRepository } from "@/domain/repositories";

export class PushRepository implements IPushRepository {
  async subscribe(fcmToken: string): Promise<void> {
    await apiClient.post(API_ENDPOINTS.PUSH_SUBSCRIBE, { fcmToken });
  }

  async unsubscribe(fcmToken: string): Promise<void> {
    await apiClient.delete(API_ENDPOINTS.PUSH_SUBSCRIBE, {
      data: { fcmToken },
    });
  }
}
```

- [ ] **Step 3: Export from both barrels**

In `frontend/src/domain/repositories/index.ts`, add:
`export * from "./IPushRepository";`

In `frontend/src/infrastructure/repositories/index.ts`, add:
`export * from "./PushRepository";`

- [ ] **Step 4: Verify it builds**

Run: `cd frontend && npm run build`
Expected: exit 0.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/domain/repositories/IPushRepository.ts frontend/src/infrastructure/repositories/PushRepository.ts frontend/src/domain/repositories/index.ts frontend/src/infrastructure/repositories/index.ts frontend/src/infrastructure/http/endpoints.ts
git commit -m "feat(push): add PushRepository"
```

---

## Task 12: `usePushNotifications` hook

**Files:**
- Create: `frontend/src/application/hooks/usePushNotifications.ts`
- Modify: `frontend/src/application/hooks/index.ts`

**Interfaces:**
- Consumes: `requestPushPermission`, `type PushPermission` (Task 9), `PushRepository` (Task 11).
- Produces: `usePushNotifications(): {permission: PushPermission; requesting: boolean; enable: () => Promise<void>}`.

- [ ] **Step 1: Write the hook**

```ts
// frontend/src/application/hooks/usePushNotifications.ts
import { useState } from "react";
import {
  requestPushPermission,
  type PushPermission,
} from "@/infrastructure/firebase/messaging";
import { PushRepository } from "@/infrastructure/repositories";

const pushRepository = new PushRepository();

interface UsePushNotificationsResult {
  permission: PushPermission;
  requesting: boolean;
  enable: () => Promise<void>;
}

/** Drives the CS Inbox "Aktifkan notifikasi" control. Registering the device
 * on the backend happens here — infrastructure/firebase/messaging.ts only
 * knows about the browser and Firebase side of push. */
export function usePushNotifications(): UsePushNotificationsResult {
  const [permission, setPermission] = useState<PushPermission>(
    typeof Notification === "undefined"
      ? "unsupported"
      : Notification.permission,
  );
  const [requesting, setRequesting] = useState(false);

  const enable = async () => {
    setRequesting(true);
    try {
      const result = await requestPushPermission();
      setPermission(result.permission);
      if (result.token) {
        await pushRepository.subscribe(result.token);
      }
    } finally {
      setRequesting(false);
    }
  };

  return { permission, requesting, enable };
}
```

- [ ] **Step 2: Export it**

In `frontend/src/application/hooks/index.ts`, add:
`export * from "./usePushNotifications";`

- [ ] **Step 3: Verify it builds**

Run: `cd frontend && npm run build`
Expected: exit 0.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/application/hooks/usePushNotifications.ts frontend/src/application/hooks/index.ts
git commit -m "feat(push): add usePushNotifications"
```

---

## Task 13: `PushOptInButton`

**Files:**
- Create: `frontend/src/presentation/components/cs/PushOptInButton.tsx`
- Test: `frontend/src/presentation/components/cs/__tests__/PushOptInButton.test.tsx`
- Modify: `frontend/src/presentation/pages/CsInboxPage.tsx`

**Interfaces:**
- Consumes: `usePushNotifications()` (Task 12).
- Produces: `PushOptInButton({permission, requesting, onEnable})` — a presentational component, the same pattern `WaConnectionBadge` already uses (props in, no hooks called internally, so the test needs no `QueryClientProvider`).

- [ ] **Step 1: Write the failing tests**

```tsx
// frontend/src/presentation/components/cs/__tests__/PushOptInButton.test.tsx
import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { PushOptInButton } from "../PushOptInButton";

describe("PushOptInButton", () => {
  it("asks to enable notifications when permission has not been decided", () => {
    const onEnable = vi.fn();
    render(
      <PushOptInButton permission="default" requesting={false} onEnable={onEnable} />,
    );
    fireEvent.click(screen.getByRole("button", { name: /aktifkan notifikasi/i }));
    expect(onEnable).toHaveBeenCalledTimes(1);
  });

  it("shows notifications are active and does not offer to enable them again", () => {
    render(<PushOptInButton permission="granted" requesting={false} onEnable={vi.fn()} />);
    expect(screen.getByRole("button", { name: /notifikasi aktif/i })).toBeDisabled();
  });

  it("says the browser blocked it rather than offering a retry that cannot work", () => {
    render(<PushOptInButton permission="denied" requesting={false} onEnable={vi.fn()} />);
    expect(screen.getByRole("button", { name: /notifikasi diblokir/i })).toBeDisabled();
  });

  it("renders nothing when the browser cannot do push at all", () => {
    const { container } = render(
      <PushOptInButton permission="unsupported" requesting={false} onEnable={vi.fn()} />,
    );
    expect(container).toBeEmptyDOMElement();
  });
});
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd frontend && npx vitest run src/presentation/components/cs/__tests__/PushOptInButton.test.tsx`
Expected: FAIL — module not found.

- [ ] **Step 3: Write the component**

```tsx
// frontend/src/presentation/components/cs/PushOptInButton.tsx
import { Button, Tooltip } from "antd";
import { BellOutlined, CheckCircleOutlined, StopOutlined } from "@ant-design/icons";
import type { PushPermission } from "@/infrastructure/firebase/messaging";

interface PushOptInButtonProps {
  permission: PushPermission;
  requesting: boolean;
  onEnable: () => void;
}

/**
 * Never prompts on its own — permission is only ever asked for from this
 * explicit click. Browsers throttle auto-prompted permission requests, and a
 * request the CS never asked for is the fastest way to get "Block" clicked
 * once and never askable again.
 */
export function PushOptInButton({
  permission,
  requesting,
  onEnable,
}: PushOptInButtonProps) {
  if (permission === "granted") {
    return (
      <Button icon={<CheckCircleOutlined />} disabled>
        Notifikasi aktif
      </Button>
    );
  }

  if (permission === "denied") {
    return (
      <Tooltip title="Diblokir oleh browser — aktifkan lagi lewat pengaturan situs">
        <Button icon={<StopOutlined />} disabled>
          Notifikasi diblokir
        </Button>
      </Tooltip>
    );
  }

  if (permission === "unsupported") {
    return null;
  }

  return (
    <Button icon={<BellOutlined />} loading={requesting} onClick={onEnable}>
      Aktifkan notifikasi
    </Button>
  );
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cd frontend && npx vitest run src/presentation/components/cs/__tests__/PushOptInButton.test.tsx`
Expected: PASS (4 tests).

- [ ] **Step 5: Wire it into `CsInboxPage`**

In `frontend/src/presentation/pages/CsInboxPage.tsx`:

Add to the imports:
```tsx
import { usePushNotifications } from "@/application/hooks";
import { PushOptInButton } from "@/presentation/components/cs/PushOptInButton";
```

Inside `CsInboxPage`, alongside the other hook calls (near `const stream = useCsStream();` — which Task 14 is about to change, so place this call right where that line currently is):
```tsx
  const push = usePushNotifications();
```

In the header's `<Space>` (`extra={<Space>...}`), add `<PushOptInButton
permission={push.permission} requesting={push.requesting}
onEnable={push.enable} />` right after the `Balasan Cepat` button and before
`<WaConnectionBadge ... />`.

- [ ] **Step 6: Run the frontend test suite**

Run: `cd frontend && npx vitest run`
Expected: every test passes, including the existing `CsInboxPage` coverage if any exists (`grep -rn "CsInboxPage" frontend/src --include="*.test.tsx"` to check first).

- [ ] **Step 7: Commit**

```bash
git add frontend/src/presentation/components/cs/PushOptInButton.tsx frontend/src/presentation/components/cs/__tests__/PushOptInButton.test.tsx frontend/src/presentation/pages/CsInboxPage.tsx
git commit -m "feat(push): add the notification opt-in control to CS Inbox"
```

---

## Task 14: Live navbar badge, and lifting the stream

**Files:**
- Modify: `frontend/src/presentation/components/layout/AppLayout.tsx`
- Modify: `frontend/src/presentation/pages/CsInboxPage.tsx`

**Interfaces:**
- Consumes: `useCsStream(enabled)`, `useCsConversations(filter, options)` (Task 8), `listenForForegroundMessages`, `refreshTokenIfGranted` (Task 9), `PushRepository` (Task 11), `UserRole` (existing, `@/domain/entities`).
- Produces: `AppLayout` renders `<Outlet context={{csStream}} />`; `CsInboxPage` reads `csStream` via `useOutletContext` instead of calling `useCsStream()` itself.

Why this shape: the badge must update live from any page, not only while
CS Inbox is open — a CS looking at the OLT map still needs to see the count
change. That means the SSE connection driving both the badge and the inbox's
own live updates has to run once, at the app-shell level, not per-page. This
introduces `useOutletContext` (react-router's own mechanism for a layout to
share state with whatever route is nested inside it) — new to this codebase,
but it is the standard tool for exactly this situation, not a fresh
abstraction invented for it.

- [ ] **Step 1: Confirm the CS Inbox route path** (needed for the role-eligible-only Outlet context typing, and reused from Task 10)

```bash
grep -n "path.*[Cc]s.*[Ii]nbox\|CsInboxPage" frontend/src/presentation/routes/index.tsx
```

- [ ] **Step 2: Rewrite `AppLayout.tsx`**

Replace the full file with:

```tsx
import { Outlet, useNavigate, useLocation } from "react-router-dom";
import { ProLayout } from "@ant-design/pro-components";
import { UserOutlined, LogoutOutlined, BellOutlined } from "@ant-design/icons";
import { Dropdown, Avatar, Badge, App, Grid } from "antd";
import type { MenuProps } from "antd";
import { useEffect } from "react";
import { useAuthStore } from "@/application/stores";
import { useLogout, useCsConversations } from "@/application/hooks";
import { useCsStream, type WaStreamStatus } from "@/application/hooks/useCsStream";
import { UserRole } from "@/domain/entities";
import {
  listenForForegroundMessages,
  refreshTokenIfGranted,
} from "@/infrastructure/firebase/messaging";
import { PushRepository } from "@/infrastructure/repositories";
import { buildNavigationRoutes } from "./navigationRoutes";
import { layoutPadding } from "./layoutPadding";

const pushRepository = new PushRepository();

// The three roles that can open /api/v1/cs/* at all — everything push- and
// badge-related is inert for anyone else, the same gate the backend enforces.
const CS_ROLES: UserRole[] = [UserRole.ADMIN, UserRole.CS, UserRole.TECHNICIAN];

/** What CsInboxPage reads back via useOutletContext, since the stream that
 * feeds the navbar badge has to run here, not on that page. */
export interface AppLayoutContext {
  csStream: WaStreamStatus;
}

export function AppLayout() {
  const navigate = useNavigate();
  const location = useLocation();
  const user = useAuthStore((state) => state.user);
  const logoutMutation = useLogout();
  const canUseCs = !!user && CS_ROLES.includes(user.role);

  const stream = useCsStream(canUseCs);
  const awaitingQuery = useCsConversations(
    { awaitingReply: true },
    { enabled: canUseCs },
  );
  const awaitingCount = awaitingQuery.data?.length ?? 0;

  // Runs once per app-shell mount, not per click: a CS who already granted
  // permission on a previous visit gets their token silently refreshed, and
  // the foreground listener has to be live on every page — a push can arrive
  // while looking at the OLT map, not only while CS Inbox is open.
  useEffect(() => {
    if (!canUseCs) return;
    let unsubscribe: (() => void) | undefined;

    refreshTokenIfGranted().then((token) => {
      if (token) void pushRepository.subscribe(token);
    });

    listenForForegroundMessages((title, body) => {
      new Notification(title, { body });
    }).then((unsub) => {
      unsubscribe = unsub;
    });

    return () => unsubscribe?.();
  }, [canUseCs]);

  const handleLogout = () => {
    logoutMutation.mutate();
  };

  const userMenuItems: MenuProps["items"] = [
    {
      key: "profile",
      icon: <UserOutlined />,
      label: `${user?.username} (${user?.role})`,
    },
    { type: "divider" },
    {
      key: "logout",
      icon: <LogoutOutlined />,
      label: "Logout",
      onClick: handleLogout,
      danger: true,
    },
  ];

  const routes = buildNavigationRoutes(user?.role);
  const padding = layoutPadding(Grid.useBreakpoint());

  return (
    <div
      style={{
        background: "#0a0a0a",
        minHeight: "100vh",
        backgroundImage: `
        linear-gradient(rgba(39, 39, 42, 0.3) 1px, transparent 1px),
        linear-gradient(90deg, rgba(39, 39, 42, 0.3) 1px, transparent 1px)
      `,
        backgroundSize: "20px 20px",
      }}
    >
      <App>
        <ProLayout
          title="TikMan"
          logo={
            <div
              style={{
                width: 32,
                height: 32,
                background: "linear-gradient(135deg, #3ecf8e 0%, #2fb574 100%)",
                borderRadius: 8,
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
              }}
            >
              <svg
                style={{ width: 20, height: 20, color: "white" }}
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                strokeWidth={2.5}
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  d="M13 10V3L4 14h7v7l9-11h-7z"
                />
              </svg>
            </div>
          }
          layout="mix"
          splitMenus={false}
          navTheme="realDark"
          fixedHeader
          fixSiderbar
          location={location}
          route={{ routes }}
          siderWidth={256}
          contentStyle={{ paddingInline: padding.contentInline }}
          token={{
            bgLayout: "transparent",
            sider: {
              colorMenuBackground: "#18181b",
              colorBgMenuItemSelected: "rgba(62, 207, 142, 0.1)",
              colorTextMenuSelected: "#3ecf8e",
              colorTextMenu: "#a1a1aa",
              colorTextMenuItemHover: "#ffffff",
            },
            header: {
              colorBgHeader: "#18181b",
              colorHeaderTitle: "#ffffff",
              colorTextMenu: "#a1a1aa",
              colorTextMenuSelected: "#ffffff",
              heightLayoutHeader: 56,
            },
          }}
          menuItemRender={(item, dom) => (
            <div onClick={() => navigate(item.path || "/")}>{dom}</div>
          )}
          avatarProps={{
            src: undefined,
            size: "default",
            title: user?.username,
            render: () => {
              return (
                <Dropdown
                  menu={{ items: userMenuItems }}
                  placement="bottomRight"
                >
                  <div
                    style={{
                      display: "flex",
                      alignItems: "center",
                      gap: 16,
                      cursor: "pointer",
                    }}
                  >
                    {canUseCs && (
                      <Badge count={awaitingCount}>
                        <BellOutlined
                          style={{ fontSize: 18, color: "#a1a1aa" }}
                        />
                      </Badge>
                    )}
                    <Avatar style={{ backgroundColor: "#3ecf8e" }}>
                      {user?.initials ||
                        user?.username?.charAt(0).toUpperCase()}
                    </Avatar>
                  </div>
                </Dropdown>
              );
            },
          }}
          actionsRender={() => []}
          menuFooterRender={() => (
            <div
              style={{
                padding: "16px",
                borderTop: "1px solid #27272a",
                fontSize: 12,
                color: "#71717a",
              }}
            >
              OLT Provisioning System
            </div>
          )}
        >
          <div
            style={{
              padding: padding.page,
              minHeight: "calc(100vh - 56px)",
              background: "transparent",
            }}
          >
            <Outlet context={{ csStream: stream } satisfies AppLayoutContext} />
          </div>
        </ProLayout>
      </App>
    </div>
  );
}
```

The current file ends exactly at the closing `}` above — it has no `export
default`, only the named `export function AppLayout()`, and this plan does
not add one. The only substantive differences from the current file: the new imports; the
`canUseCs` computation; the `useCsStream(canUseCs)` and `useCsConversations`
calls replacing nothing (they are new); the `useEffect` block (new); the
`{canUseCs && (<Badge>...)}` wrapper around the existing `<Badge>` JSX
(previously unconditional, `count={0}`); and `<Outlet
context={{csStream: stream} satisfies AppLayoutContext} />` replacing the
bare `<Outlet />`. Everything else — the `ProLayout` props, the logo SVG, the
menu items — is unchanged from the file as it exists today.

- [ ] **Step 3: Update `CsInboxPage.tsx` to read the stream from context**

Change the import line
`import { useState } from "react";`
to
```tsx
import { useState } from "react";
import { useOutletContext } from "react-router-dom";
```

Remove `useCsStream` from the `@/application/hooks` import list (it is no
longer imported directly — `CsInboxPage` gets the stream from context now).
Add the import:
```tsx
import type { AppLayoutContext } from "@/presentation/components/layout/AppLayout";
```

Replace the line
```tsx
  const stream = useCsStream();
```
with
```tsx
  const { csStream: stream } = useOutletContext<AppLayoutContext>();
```

Everything else in `CsInboxPage.tsx` that reads `stream` (the
`WaConnectionBadge`, `WaNumbersModal`, and `pairingLive` lookup) is unchanged
— they all consume the same `WaStreamStatus` value, now sourced from the
layout instead of a second connection.

- [ ] **Step 4: Verify the build and full test suite**

Run: `cd frontend && npm run build && npx vitest run`
Expected: build succeeds; every test passes.

No existing test renders `CsInboxPage` or `AppLayout` — verified while this
plan was written (`grep -rln "CsInboxPage\|RouterProvider\|createMemoryRouter"
frontend/src --include="*.test.tsx" --include="*.test.ts"` returns nothing),
so neither the new `useOutletContext` call nor `AppLayout`'s new hooks can
break a test that already exists. If that ever stops being true, the failure
mode to expect is `useOutletContext` returning `undefined` outside a matching
route, which would make `const { csStream } = useOutletContext(...)` throw —
the fix then is to render the component under a `createMemoryRouter` whose
parent route supplies `<Outlet context={{ csStream: {} }} />`.

- [ ] **Step 5: Manual verification**

Run: `cd frontend && npm run dev`, log in as a CS or Admin, and confirm:
- The bell badge shows a count matching the "Belum dibalas" tab's conversation count.
- Navigating away from CS Inbox to another page keeps the badge visible and current (send a test message from another number if one is available, or trust the SSE wiring verified in Task 8/14's automated tests if not).
- Logging in as a Viewer (if a Viewer test account exists) shows no bell/badge at all.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/presentation/components/layout/AppLayout.tsx frontend/src/presentation/pages/CsInboxPage.tsx
git commit -m "feat(push): wire the navbar badge and lift the CS event stream"
```

---

## Task 15: Fill in real Firebase credentials (manual — blocked on the user)

**This task is not executed by an agent.** It exists so nothing earlier in
this plan is mistaken for "done, but not really working."

Once the user has created the Firebase project (spec §7 has the exact
console steps) and supplied:
- the `firebaseConfig` object → replace the six `REPLACE_WITH_*` literals in
  `frontend/public/firebase-messaging-sw.js`, and set the six matching
  `VITE_FIREBASE_*` values in the real `.env` (or `.env.development`) the
  frontend build reads
- the VAPID key → `VITE_FIREBASE_VAPID_KEY`
- the service account JSON → base64-encode it
  (`base64 -i service-account.json | tr -d '\n'`) into
  `FIREBASE_SERVICE_ACCOUNT_JSON_B64` in `/opt/tikman/.env` on radpro (mode
  600, root-owned — the same handling as `ENCRYPTION_KEY`)

then rebuild and redeploy both `frontend` and `api` (see how the CS initials
feature was deployed earlier in this project's history for the exact
`docker compose build` / `up -d --no-deps` sequence), and manually confirm in
a real browser: the opt-in button asks for permission, a real WhatsApp
message produces both an OS notification and a live badge update.

No code from Tasks 1–14 needs to change for this — that was the point of
making every Firebase-touching piece read from configuration that can be
empty.

---

## Verification Notes

Everything below was checked against the real codebase or the real installed
package while writing this plan, rather than assumed. Five things were wrong
on the first pass and are already fixed above; they are listed because each
one would have stopped an executor mid-task:

1. **`option.WithCredentialsJSON` is deprecated** in the currently published
   `google.golang.org/api` in favour of
   `option.WithAuthCredentialsJSON(option.ServiceAccount, json)` — found by
   running `go doc` against the actually-installed module, not from memory.
2. **`internal/services` importing `internal/push` would have put the whole
   Firebase SDK into the `wa`, `worker`, `trapd`, and `seed-events` builds**
   — all four import `internal/services`. The interface moved to the
   consumer side (Global Constraints, Tasks 3/4/7).
3. **Task 6's handler tests originally asserted through
   `PushService.TokensForRoles`**, which inner-joins to `users` — but those
   tests authenticate as a bare `uuid.New()` with no user row, so the join
   would have matched nothing and every assertion would have failed for a
   reason unrelated to what was being tested. They now read
   `push_subscriptions` directly.
4. **`frontend/src/vite-env.d.ts` declares `ImportMetaEnv` explicitly** with
   only two variables, and its local `interface ImportMeta` replaces
   `vite/client`'s permissive one — so every new `VITE_FIREBASE_*` read would
   have been a `tsc` error. Task 9 now updates that file first.
5. **Task 8's new hook test originally imported `./setupMocks`**, which
   mocks `@/infrastructure/repositories` with a fixed set of repositories
   that does not include `CsRepository` — `new CsRepository()` at module
   scope would have thrown "not a constructor". It now stands alone, the way
   `useCsStream.test.ts` already does.

Also confirmed, and relied on above: `setupTestDB`/`TestDB` both run
`models.AutoMigrate`; `InboundMessage`'s fields are
`ConversationID/WAMessageID/Kind/Body/Media/At/ReplyToWAID`;
`models.MessageKindText` is the right constant; `router.Setup` aliases its
`ginEngine` parameter to a local `router` and ends with `return router`;
`csRedisClient`, `csConversationService`, and `csMessageService` are already
in scope where Tasks 6/7 insert code; no test renders `AppLayout` or
`CsInboxPage`; `useCsStream.test.ts` exists and calls `useCsStream()` with no
argument, so the new `enabled = true` default keeps it green; TanStack Query
is v5, where a disabled query reports `fetchStatus: "idle"`.

## Self-Review Notes

- **Spec coverage:** every section of the design doc maps to a task —
  §4 (data) → Task 1, §5 (backend) → Tasks 2–7, §6 (frontend) → Tasks 9–14,
  §7 (prerequisites) → Task 15, §9 (testing) → a test step inside every task
  that has one, with the two genuinely untestable pieces (Task 3's Firebase
  client, Task 9's browser/Firebase wrapper) explicitly called out with why.
- **Type consistency:** `Sender.SendEach` (Task 3) →
  `PushNotifierService.sender.SendEach` (Task 4) →
  `FakePushSender.SendEach` (Task 4's tests, reused by Task 5) all share the
  exact signature `(ctx, tokens []string, title, body string, data
  map[string]string) ([]string, error)`. `WaStreamStatus` (existing type) is
  what `useCsStream` (Task 8) returns and `AppLayoutContext.csStream` (Task
  14) carries — the same name, not a re-declared shape.
- **Task 6/7 handoff:** Task 6 builds a standalone `PushService` and
  `PushHandler` inside `router.Setup` so its own tests pass in isolation;
  Task 7 explicitly replaces that with the shared instance the notifier also
  uses, and changes `Setup`'s return signature. This ordering (build it
  simple, then thread it through) was chosen over designing the final
  three-way shared wiring in Task 6, so each task's diff stays reviewable on
  its own — flagged here so it reads as deliberate, not as Task 7 papering
  over a Task 6 mistake.
