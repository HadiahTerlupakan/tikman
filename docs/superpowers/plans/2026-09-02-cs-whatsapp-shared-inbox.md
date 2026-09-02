# Modul CS — Shared WhatsApp Inbox: Rencana Implementasi

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Memindahkan satu nomor WhatsApp CS ke dalam TikMan sebagai inbox bersama, sehingga banyak petugas membalas pelanggan dari satu tempat tanpa saling melempar keluar dari sesi WhatsApp.

**Architecture:** Container `wa` tersendiri memegang koneksi whatsmeow dan menjadi satu-satunya proses yang berbicara ke WhatsApp. Postgres adalah sumber kebenaran — baris pesan berstatus `queued` sekaligus berfungsi sebagai outbox. Redis hanya mempercepat: mengumumkan peristiwa ke SSE dan menyimpan penanda CS yang sedang online. Kalau Redis mati, penyapu berkala tetap membuat sistem benar, hanya terlambat.

**Tech Stack:** Go 1.25 (Gin, GORM, go-redis, zap, viper), `go.mau.fi/whatsmeow`, PostgreSQL 15/TimescaleDB, Redis 7, React 18 + TypeScript + Ant Design + React Query.

**Spec:** `docs/superpowers/specs/2026-09-02-cs-whatsapp-shared-inbox-design.md`

## Global Constraints

- Berkas maksimal 350 baris; fungsi maksimal 50 baris; kedalaman nesting maksimal 3. Berkas tes dikecualikan bila kelebihannya berupa kasus uji. `main()` di `cmd/` dikecualikan dari batas fungsi.
- Model GORM **tidak boleh** punya field relasi (`Site *Site`); hanya ID asing. Ambil data terkait lewat query manual.
- `AutoMigrate` membuat tabel dari tag model dan berjalan **sebelum** migrasi SQL. Migrasi SQL hanya berisi yang tidak bisa dinyatakan tag: CHECK, foreign key, kolom terkalkulasi, dan indeks parsial.
- Migrasi SQL memakai penamaan datar `NN_nama.sql` di `backend/migrations/` (bukan pola `.up/.down`), mengikuti berkas yang sudah ada.
- Tes backend memakai SQLite in-memory lewat `setupTestDB(t)`. Karena SQLite tidak mendapat indeks parsial dari migrasi Postgres, aturan keunikan yang penting **juga** ditegakkan di Go — pola yang sama dipakai `DistributionService`.
- Prefix API: `/api/v1`. Frontend memakai `API_ENDPOINTS` di `src/infrastructure/http/endpoints.ts`.
- Frontend ↔ backend: JSON snake_case, dikonversi ke camelCase oleh `humps` di interceptor axios. Tulis DTO Go dalam snake_case, tipe TypeScript dalam camelCase.
- Dependensi baru yang disetujui **hanya** `go.mau.fi/whatsmeow` beserta turunannya (`go.mau.fi/libsignal`, `google.golang.org/protobuf`). Tidak ada dependensi lain — SSE memakai Gin, sesi memakai Postgres, media memakai pustaka standar.
- Komentar menjelaskan **mengapa**, bukan **apa**. Tidak ada `TODO`, kode mati, atau berkas markdown baru.
- Setiap task berakhir dengan `cd backend && go test ./... && gofmt -s -l .` bersih (atau padanan frontend) sebelum commit.

## Peta Berkas

**Backend — baru**

| Berkas | Tanggung jawab |
|---|---|
| `internal/models/cs_conversation.go` | `WAAccount`, `CSConversation`, dan enum statusnya |
| `internal/models/cs_message.go` | `CSMessage`, `CSQuickReply`, dan enumnya |
| `internal/utils/phone.go` | Normalisasi nomor HP ke bentuk `628xxx` |
| `internal/services/cs_presence.go` | Siapa yang sedang membuka inbox (Redis), di balik antarmuka |
| `internal/services/cs_assignment.go` | Pembagian round-robin |
| `internal/services/cs_conversation_service.go` | Cari/buat percakapan, penautan ONT, status, penugasan |
| `internal/services/cs_message_service.go` | Simpan pesan masuk (idempoten), antrekan pesan keluar, tanda terima |
| `internal/services/cs_quick_reply_service.go` | Template balasan cepat |
| `internal/services/cs_media_retention.go` | Hapus media lebih tua dari batas retensi |
| `internal/wa/sender.go` | Antarmuka `Sender` — batas yang membuat pengiriman bisa diuji |
| `internal/wa/client.go` | Koneksi whatsmeow, pairing, sambung ulang |
| `internal/wa/inbound.go` | Terjemahan event whatsmeow → pesan tersimpan |
| `internal/wa/outbound.go` | Pengurasan antrean, pembatas laju |
| `internal/wa/media.go` | Unduh dan simpan berkas media |
| `internal/wa/receipts.go` | Pembaruan status delivered/read |
| `internal/wa/events.go` | Penerbitan peristiwa ke Redis |
| `internal/api/cs_dto.go` | DTO permintaan/jawaban |
| `internal/api/cs_handler_conversations.go` | Daftar, penugasan, status, penautan ONT |
| `internal/api/cs_handler_messages.go` | Riwayat, kirim teks, kirim media, sajikan media |
| `internal/api/cs_handler_stream.go` | SSE |
| `internal/api/cs_handler_wa.go` | Akun WhatsApp: status, pairing, putus |
| `internal/api/cs_handler_quick_replies.go` | CRUD balasan cepat |
| `cmd/wa/main.go` | Perangkaian proses `wa` |
| `backend/Dockerfile.wa` | Image proses `wa` |
| `backend/migrations/41_add_cs_module.sql` | CHECK, FK, tsvector, indeks parsial |

**Backend — diubah**

| Berkas | Perubahan |
|---|---|
| `internal/models/user.go` | Tambah `UserRoleCS` |
| `internal/models/ont.go` | Tambah kolom `Phone` |
| `internal/models/models.go` | Daftarkan model baru ke `AutoMigrate` |
| `internal/config/config.go` | `WAMediaDir`, `WASendIntervalMS`, `WAMediaRetentionDays` |
| `internal/api/router.go` | Grup rute `/cs` |
| `internal/api/ont_dto.go`, `internal/services/ont_service.go` | Terima dan simpan nomor HP |
| `docker-compose.yml`, `docker-compose.dev.yml` | Service `wa`, volume `cs_media` |
| `.github/workflows/ci.yml` | Bangun binary `wa` |

**Frontend — baru**

`domain/entities/CsConversation.ts`, `domain/entities/CsMessage.ts`, `domain/repositories/ICsRepository.ts`, `infrastructure/repositories/CsRepository.ts`, `application/hooks/useCsInbox.ts`, `application/hooks/useCsStream.ts`, `application/hooks/useCsQuickReplies.ts`, `presentation/pages/CsInboxPage.tsx`, dan komponen di `presentation/components/cs/`: `ConversationList.tsx`, `MessageThread.tsx`, `MessageComposer.tsx`, `CustomerPanel.tsx`, `QuickReplyPicker.tsx`, `WaConnectionBadge.tsx`, `WaPairingModal.tsx`.

**Frontend — diubah**

`infrastructure/http/endpoints.ts`, `presentation/routes/index.tsx`, `presentation/components/layout/navigationRoutes.tsx`, dan form ONT untuk isian nomor HP.

---

### Task 1: Model, migrasi, dan role `cs`

Fondasi database. Setelah task ini `AutoMigrate` membuat tabelnya dan migrasi SQL menambahkan aturan yang tidak bisa dinyatakan tag GORM.

**Files:**
- Create: `backend/internal/models/cs_conversation.go`
- Create: `backend/internal/models/cs_message.go`
- Create: `backend/migrations/41_add_cs_module.sql`
- Modify: `backend/internal/models/user.go`
- Modify: `backend/internal/models/ont.go`
- Modify: `backend/internal/models/models.go`
- Test: `backend/internal/models/cs_test.go`

**Interfaces:**
- Produces: tipe `models.WAAccount`, `models.CSConversation`, `models.CSMessage`, `models.CSQuickReply`; konstanta `models.ConversationUnassigned/Open/Closed`, `models.WAAccountDisconnected/Pairing/Connected/Banned`, `models.MessageIn/MessageOut`, `models.MessageKindText/Image/Document/Audio/Video`, `models.MessageQueued/Sent/Delivered/Read/Failed`, `models.UserRoleCS`; kolom `ONT.Phone`.

- [ ] **Step 1: Tulis tes yang gagal**

`backend/internal/models/cs_test.go`:

```go
package models

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func csTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, AutoMigrate(db))
	return db
}

func TestCSConversationGetsAnIDOnCreate(t *testing.T) {
	db := csTestDB(t)

	account := WAAccount{Label: "CS Utama", Status: WAAccountDisconnected}
	require.NoError(t, db.Create(&account).Error)
	require.NotEqual(t, uuid.Nil, account.ID)

	conv := CSConversation{
		WAAccountID:   account.ID,
		CustomerJID:   "628111@s.whatsapp.net",
		CustomerPhone: "628111",
		Status:        ConversationUnassigned,
	}
	require.NoError(t, db.Create(&conv).Error)
	require.NotEqual(t, uuid.Nil, conv.ID)
}

// One customer may hold only one thread per number: a second row would split
// their history in two, and the CS reading one half would answer blind.
func TestOneThreadPerCustomerPerAccount(t *testing.T) {
	db := csTestDB(t)

	account := WAAccount{Label: "CS Utama", Status: WAAccountDisconnected}
	require.NoError(t, db.Create(&account).Error)

	first := CSConversation{
		WAAccountID:   account.ID,
		CustomerJID:   "628111@s.whatsapp.net",
		CustomerPhone: "628111",
		Status:        ConversationUnassigned,
	}
	require.NoError(t, db.Create(&first).Error)

	second := CSConversation{
		WAAccountID:   account.ID,
		CustomerJID:   "628111@s.whatsapp.net",
		CustomerPhone: "628111",
		Status:        ConversationUnassigned,
	}
	require.Error(t, db.Create(&second).Error)
}

// An outbound message has no WhatsApp id until it is actually sent, so the
// column must hold many empty values at once. A plain unique index would let
// the second queued message collide with the first.
func TestManyMessagesMayWaitWithoutAWhatsAppID(t *testing.T) {
	db := csTestDB(t)

	account := WAAccount{Label: "CS Utama", Status: WAAccountDisconnected}
	require.NoError(t, db.Create(&account).Error)
	conv := CSConversation{
		WAAccountID:   account.ID,
		CustomerJID:   "628111@s.whatsapp.net",
		CustomerPhone: "628111",
		Status:        ConversationOpen,
	}
	require.NoError(t, db.Create(&conv).Error)

	for i := 0; i < 2; i++ {
		msg := CSMessage{
			ConversationID: conv.ID,
			Direction:      MessageOut,
			Kind:           MessageKindText,
			Body:           "halo",
			Status:         MessageQueued,
		}
		require.NoError(t, db.Create(&msg).Error)
		require.Nil(t, msg.WAMessageID)
	}
}
```

- [ ] **Step 2: Jalankan tes, pastikan gagal**

Run: `cd backend && go test ./internal/models/ -run TestCS -v`
Expected: FAIL — `undefined: WAAccount`.

- [ ] **Step 3: Tulis model**

`backend/internal/models/cs_conversation.go`:

```go
package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// WAAccountStatus is where a connected WhatsApp number stands.
type WAAccountStatus string

const (
	WAAccountDisconnected WAAccountStatus = "disconnected"
	WAAccountPairing      WAAccountStatus = "pairing"
	WAAccountConnected    WAAccountStatus = "connected"
	WAAccountBanned       WAAccountStatus = "banned"
)

// WAAccount is one WhatsApp number the team answers from. More than one row is
// allowed from the start, so adding a second number later to spread the load —
// or to survive one number being blocked — costs a row rather than a migration.
type WAAccount struct {
	ID              uuid.UUID       `gorm:"type:uuid;primaryKey" json:"id"`
	Label           string          `gorm:"type:varchar(100);not null" json:"label"`
	JID             string          `gorm:"type:varchar(64)" json:"jid"`
	Status          WAAccountStatus `gorm:"type:varchar(20);not null" json:"status"`
	LastConnectedAt *time.Time      `json:"last_connected_at,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

func (a *WAAccount) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}

// TableName specifies the table name for GORM.
func (WAAccount) TableName() string { return "wa_accounts" }

// ConversationStatus is how far a customer's thread has got.
type ConversationStatus string

const (
	ConversationUnassigned ConversationStatus = "unassigned"
	ConversationOpen       ConversationStatus = "open"
	ConversationClosed     ConversationStatus = "closed"
)

// CSConversation is one customer's thread on one WhatsApp number.
type CSConversation struct {
	ID             uuid.UUID          `gorm:"type:uuid;primaryKey" json:"id"`
	WAAccountID    uuid.UUID          `gorm:"type:uuid;not null;uniqueIndex:uq_cs_conversations_peer,priority:1" json:"wa_account_id"`
	CustomerJID    string             `gorm:"type:varchar(64);not null;uniqueIndex:uq_cs_conversations_peer,priority:2" json:"customer_jid"`
	CustomerPhone  string             `gorm:"type:varchar(20);not null;index" json:"customer_phone"`
	CustomerName   string             `gorm:"type:varchar(255)" json:"customer_name"`
	AssignedUserID *uuid.UUID         `gorm:"type:uuid;index" json:"assigned_user_id,omitempty"`
	Status         ConversationStatus `gorm:"type:varchar(20);not null;index" json:"status"`
	ONTID          *uuid.UUID         `gorm:"type:uuid;index" json:"ont_id,omitempty"`
	LastMessageAt  time.Time          `gorm:"index" json:"last_message_at"`
	UnreadCount    int                `gorm:"not null;default:0" json:"unread_count"`
	CreatedAt      time.Time          `json:"created_at"`
	UpdatedAt      time.Time          `json:"updated_at"`
}

func (c *CSConversation) BeforeCreate(tx *gorm.DB) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return nil
}

// TableName specifies the table name for GORM.
func (CSConversation) TableName() string { return "cs_conversations" }
```

`backend/internal/models/cs_message.go`:

```go
package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// MessageDirection says who wrote a message.
type MessageDirection string

const (
	MessageIn  MessageDirection = "in"
	MessageOut MessageDirection = "out"
)

// MessageKind is what a message carries.
type MessageKind string

const (
	MessageKindText     MessageKind = "text"
	MessageKindImage    MessageKind = "image"
	MessageKindDocument MessageKind = "document"
	MessageKindAudio    MessageKind = "audio"
	MessageKindVideo    MessageKind = "video"
)

// MessageStatus is how far an outbound message has travelled. Inbound messages
// are stored as delivered.
type MessageStatus string

const (
	MessageQueued    MessageStatus = "queued"
	MessageSent      MessageStatus = "sent"
	MessageDelivered MessageStatus = "delivered"
	MessageRead      MessageStatus = "read"
	MessageFailed    MessageStatus = "failed"
)

// CSMessage is one WhatsApp message in a thread.
//
// A row with Status MessageQueued is the outbox: there is no second table. A
// reply written while the WhatsApp process was down is still sitting here when
// it comes back, which is what stops a CS reply from vanishing silently.
//
// WAMessageID is a pointer because an outbound message has no WhatsApp id until
// it is sent, and many may wait at once. The uniqueness that makes inbound
// storage idempotent is a partial index added in migration 41; SQLite tests do
// not get it, so CSMessageService checks for the duplicate in Go as well.
type CSMessage struct {
	ID             uuid.UUID        `gorm:"type:uuid;primaryKey" json:"id"`
	ConversationID uuid.UUID        `gorm:"type:uuid;not null;index" json:"conversation_id"`
	WAMessageID    *string          `gorm:"type:varchar(128);index" json:"wa_message_id,omitempty"`
	Direction      MessageDirection `gorm:"type:varchar(3);not null" json:"direction"`
	SenderUserID   *uuid.UUID       `gorm:"type:uuid;index" json:"sender_user_id,omitempty"`
	Kind           MessageKind      `gorm:"type:varchar(20);not null" json:"kind"`
	Body           string           `gorm:"type:text" json:"body"`
	MediaPath      string           `gorm:"type:text" json:"-"`
	MediaMime      string           `gorm:"type:varchar(100)" json:"media_mime,omitempty"`
	MediaSize      int64            `json:"media_size,omitempty"`
	MediaFilename  string           `gorm:"type:varchar(255)" json:"media_filename,omitempty"`
	Status         MessageStatus    `gorm:"type:varchar(20);not null;index" json:"status"`
	FailReason     string           `gorm:"type:text" json:"fail_reason,omitempty"`
	WATimestamp    time.Time        `gorm:"index" json:"wa_timestamp"`
	CreatedAt      time.Time        `json:"created_at"`
}

func (m *CSMessage) BeforeCreate(tx *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}

// TableName specifies the table name for GORM.
func (CSMessage) TableName() string { return "cs_messages" }

// CSQuickReply is a canned answer a CS can insert instead of retyping it.
type CSQuickReply struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	Title     string    `gorm:"type:varchar(100);not null" json:"title"`
	Body      string    `gorm:"type:text;not null" json:"body"`
	CreatedBy uuid.UUID `gorm:"type:uuid;not null" json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (q *CSQuickReply) BeforeCreate(tx *gorm.DB) error {
	if q.ID == uuid.Nil {
		q.ID = uuid.New()
	}
	return nil
}

// TableName specifies the table name for GORM.
func (CSQuickReply) TableName() string { return "cs_quick_replies" }
```

Di `backend/internal/models/user.go`, tambahkan setelah `UserRoleViewer`:

```go
	UserRoleCS         UserRole = "cs"
```

Di `backend/internal/models/ont.go`, tambahkan setelah field `Description`:

```go
	// Phone is the subscriber's WhatsApp number in 628xxx form. It is what ties
	// an incoming chat to the ONT whose light the CS is about to check.
	Phone string `gorm:"type:varchar(20)" json:"phone"`
```

Di `backend/internal/models/models.go`, tambahkan ke daftar `AutoMigrate` setelah `&ODP{}`:

```go
		&WAAccount{},
		&CSConversation{},
		&CSMessage{},
		&CSQuickReply{},
```

- [ ] **Step 4: Jalankan tes, pastikan lulus**

Run: `cd backend && go test ./internal/models/ -run TestCS -v && go test ./internal/models/ -run TestManyMessages -v && go test ./internal/models/ -run TestOneThread -v`
Expected: PASS ketiganya.

- [ ] **Step 5: Tulis migrasi SQL**

`backend/migrations/41_add_cs_module.sql`:

```sql
-- The CS module: one WhatsApp number answered by many hands.
--
-- AutoMigrate creates these tables from the model tags and runs before this
-- file, so what is left here is everything a GORM tag cannot say: the enum
-- checks, the foreign keys, the full-text column search leans on, and the
-- partial indexes that make waiting rows cheap to find.

ALTER TABLE wa_accounts DROP CONSTRAINT IF EXISTS wa_accounts_status_valid;
ALTER TABLE wa_accounts ADD CONSTRAINT wa_accounts_status_valid
    CHECK (status IN ('disconnected', 'pairing', 'connected', 'banned'));

ALTER TABLE cs_conversations DROP CONSTRAINT IF EXISTS cs_conversations_status_valid;
ALTER TABLE cs_conversations ADD CONSTRAINT cs_conversations_status_valid
    CHECK (status IN ('unassigned', 'open', 'closed'));

-- A thread with nobody holding it is exactly a thread that is unassigned.
-- Without this an assignment could be cleared while the row still called itself
-- open, and the conversation would sit in no inbox while looking answered.
ALTER TABLE cs_conversations DROP CONSTRAINT IF EXISTS cs_conversations_holder_matches_status;
ALTER TABLE cs_conversations ADD CONSTRAINT cs_conversations_holder_matches_status
    CHECK (status <> 'unassigned' OR assigned_user_id IS NULL);

-- RESTRICT on the account: deleting a number that still holds conversations is
-- a mistake worth refusing. SET NULL on the others: a CS can leave the company
-- and an ONT can be replaced without taking the customer's history with them.
ALTER TABLE cs_conversations DROP CONSTRAINT IF EXISTS fk_cs_conversations_account;
ALTER TABLE cs_conversations ADD CONSTRAINT fk_cs_conversations_account
    FOREIGN KEY (wa_account_id) REFERENCES wa_accounts(id) ON DELETE RESTRICT;

ALTER TABLE cs_conversations DROP CONSTRAINT IF EXISTS fk_cs_conversations_user;
ALTER TABLE cs_conversations ADD CONSTRAINT fk_cs_conversations_user
    FOREIGN KEY (assigned_user_id) REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE cs_conversations DROP CONSTRAINT IF EXISTS fk_cs_conversations_ont;
ALTER TABLE cs_conversations ADD CONSTRAINT fk_cs_conversations_ont
    FOREIGN KEY (ont_id) REFERENCES onts(id) ON DELETE SET NULL;

ALTER TABLE cs_messages DROP CONSTRAINT IF EXISTS cs_messages_direction_valid;
ALTER TABLE cs_messages ADD CONSTRAINT cs_messages_direction_valid
    CHECK (direction IN ('in', 'out'));

ALTER TABLE cs_messages DROP CONSTRAINT IF EXISTS cs_messages_status_valid;
ALTER TABLE cs_messages ADD CONSTRAINT cs_messages_status_valid
    CHECK (status IN ('queued', 'sent', 'delivered', 'read', 'failed'));

ALTER TABLE cs_messages DROP CONSTRAINT IF EXISTS fk_cs_messages_conversation;
ALTER TABLE cs_messages ADD CONSTRAINT fk_cs_messages_conversation
    FOREIGN KEY (conversation_id) REFERENCES cs_conversations(id) ON DELETE CASCADE;

-- Partial, because an outbound message carries no WhatsApp id until it is sent
-- and many may be waiting at once. What this does enforce is the rule that
-- matters: WhatsApp re-delivering an event must not store the message twice.
CREATE UNIQUE INDEX IF NOT EXISTS uq_cs_messages_wa_id ON cs_messages (wa_message_id)
    WHERE wa_message_id IS NOT NULL;

-- Full text over message bodies. At thousands of messages a day this table
-- passes a million rows within a year, where ILIKE crawls.
ALTER TABLE cs_messages ADD COLUMN IF NOT EXISTS tsv tsvector
    GENERATED ALWAYS AS (to_tsvector('simple', coalesce(body, ''))) STORED;
CREATE INDEX IF NOT EXISTS idx_cs_messages_tsv ON cs_messages USING GIN (tsv);

-- The sweeper asks only for messages still waiting; a partial index keeps that
-- question off the millions of rows that were sent long ago.
CREATE INDEX IF NOT EXISTS idx_cs_messages_queued ON cs_messages (created_at)
    WHERE status = 'queued';

-- Inbox ordering is always newest-first within one account.
CREATE INDEX IF NOT EXISTS idx_cs_conversations_recent
    ON cs_conversations (wa_account_id, last_message_at DESC);

-- One phone number belongs to one subscriber's ONT: two ONTs claiming it would
-- send the CS to the wrong house. Partial, because most ONTs have no number
-- recorded yet and empty values must not collide.
CREATE UNIQUE INDEX IF NOT EXISTS uq_onts_phone ON onts (phone)
    WHERE phone IS NOT NULL AND phone <> '';
```

- [ ] **Step 6: Terapkan migrasi ke Postgres sungguhan**

Run:
```bash
docker-compose -f docker-compose.dev.yml up -d
cd backend && TEST_POSTGRES_DSN="host=localhost port=5437 user=tikman password=$POSTGRES_PASSWORD dbname=tikman sslmode=disable" \
  go test ./internal/database/ -run TestEveryMigrationAppliesToAFreshSchema -v
```
Expected: PASS. Kalau gagal karena `ADD COLUMN IF NOT EXISTS ... GENERATED`, periksa versi Postgres — kolom terkalkulasi butuh Postgres 12+, dan image proyek ini PG15.

- [ ] **Step 7: Commit**

```bash
cd /Users/rohadimraja/Documents/tikman
git add backend/internal/models backend/migrations/41_add_cs_module.sql
git commit -m "feat(cs): give a shared inbox its tables, and one number to many hands"
```

---

### Task 2: Normalisasi nomor HP

Nomor yang sama ditulis `0812…`, `+62812…`, dan `62812…`. Tanpa satu bentuk baku, pencocokan chat ke ONT gagal diam-diam.

**Files:**
- Create: `backend/internal/utils/phone.go`
- Test: `backend/internal/utils/phone_test.go`

**Interfaces:**
- Produces: `utils.NormalizePhone(raw string) (string, error)` — mengembalikan bentuk `628xxx`, atau galat bila nomornya tidak masuk akal.

- [ ] **Step 1: Tulis tes yang gagal**

`backend/internal/utils/phone_test.go`:

```go
package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizePhoneAcceptsTheThreeWaysIndonesiansWriteANumber(t *testing.T) {
	for _, raw := range []string{"081234567890", "+6281234567890", "6281234567890", "0812-3456-7890", "0812 3456 7890"} {
		got, err := NormalizePhone(raw)
		require.NoError(t, err, raw)
		assert.Equal(t, "6281234567890", got, raw)
	}
}

func TestNormalizePhoneRejectsWhatCannotBeANumber(t *testing.T) {
	for _, raw := range []string{"", "  ", "abcdefg", "0812", "0" , "62"} {
		_, err := NormalizePhone(raw)
		assert.Error(t, err, raw)
	}
}
```

- [ ] **Step 2: Jalankan tes, pastikan gagal**

Run: `cd backend && go test ./internal/utils/ -run TestNormalizePhone -v`
Expected: FAIL — `undefined: NormalizePhone`.

- [ ] **Step 3: Tulis implementasi**

`backend/internal/utils/phone.go`:

```go
package utils

import (
	"fmt"
	"strings"
)

// minPhoneDigits is the shortest Indonesian mobile number worth accepting:
// country code plus a subscriber number. Anything shorter is a typo, and
// storing it would attach a chat to the wrong subscriber.
const minPhoneDigits = 10

// NormalizePhone rewrites the three ways an Indonesian number gets typed —
// 0812…, +62812…, 62812… — into the single 62812… form WhatsApp itself uses,
// so a chat can be matched to an ONT whichever way the number was recorded.
func NormalizePhone(raw string) (string, error) {
	var digits strings.Builder
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}

	n := digits.String()
	switch {
	case strings.HasPrefix(n, "0"):
		n = "62" + strings.TrimPrefix(n, "0")
	case strings.HasPrefix(n, "62"):
	default:
		return "", fmt.Errorf("nomor %q tidak dikenali sebagai nomor Indonesia", raw)
	}

	if len(n) < minPhoneDigits {
		return "", fmt.Errorf("nomor %q terlalu pendek", raw)
	}
	return n, nil
}
```

- [ ] **Step 4: Jalankan tes, pastikan lulus**

Run: `cd backend && go test ./internal/utils/ -run TestNormalizePhone -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/utils/phone.go backend/internal/utils/phone_test.go
git commit -m "feat(cs): read the three ways a number gets typed as one number"
```

---

### Task 3: Presence CS

Round-robin hanya boleh menyasar orang yang benar-benar ada di depan layar. Presence disimpan di Redis dengan TTL, dan dipisahkan di balik antarmuka agar pembagian bisa diuji tanpa Redis.

**Files:**
- Create: `backend/internal/services/cs_presence.go`
- Test: `backend/internal/services/cs_presence_test.go`

**Interfaces:**
- Produces:
  - `type Presence interface { MarkOnline(ctx context.Context, userID uuid.UUID) error; Online(ctx context.Context) ([]uuid.UUID, error); NextTurn(ctx context.Context) (uint64, error) }`
  - `services.NewRedisPresence(client *redis.Client) *RedisPresence` — implementasi Redis
  - `services.NewFakePresence(ids ...uuid.UUID) *FakePresence` — implementasi dalam memori untuk tes task berikutnya

- [ ] **Step 1: Tulis tes yang gagal**

`backend/internal/services/cs_presence_test.go`:

```go
package services

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The fake is what every assignment test runs against, so its own behaviour is
// worth pinning: turns must advance, or a round-robin test could pass while
// handing every conversation to the same person.
func TestFakePresenceAdvancesItsTurn(t *testing.T) {
	ctx := context.Background()
	a, b := uuid.New(), uuid.New()
	p := NewFakePresence(a, b)

	first, err := p.NextTurn(ctx)
	require.NoError(t, err)
	second, err := p.NextTurn(ctx)
	require.NoError(t, err)

	assert.Equal(t, first+1, second)

	online, err := p.Online(ctx)
	require.NoError(t, err)
	assert.ElementsMatch(t, []uuid.UUID{a, b}, online)
}

func TestFakePresenceForgetsWhoWentOffline(t *testing.T) {
	ctx := context.Background()
	a := uuid.New()
	p := NewFakePresence(a)

	p.SetOnline()

	online, err := p.Online(ctx)
	require.NoError(t, err)
	assert.Empty(t, online)
}
```

- [ ] **Step 2: Jalankan tes, pastikan gagal**

Run: `cd backend && go test ./internal/services/ -run TestFakePresence -v`
Expected: FAIL — `undefined: NewFakePresence`.

- [ ] **Step 3: Tulis implementasi**

`backend/internal/services/cs_presence.go`:

```go
package services

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// presenceTTL outlives the fifteen-second SSE heartbeat several times over, so
// one slow network moment does not drop a CS out of the rotation mid-shift.
const presenceTTL = 60 * time.Second

const (
	presenceKeyPrefix = "cs:online:"
	turnKey           = "cs:rr:pointer"
)

// Presence is who currently has the CS inbox open. It sits behind an interface
// because assignment is the logic worth testing, and Redis is not.
type Presence interface {
	MarkOnline(ctx context.Context, userID uuid.UUID) error
	Online(ctx context.Context) ([]uuid.UUID, error)
	NextTurn(ctx context.Context) (uint64, error)
}

// RedisPresence keeps the online set as keys that expire on their own, so a CS
// whose browser died simply stops being counted; nothing has to clean up.
type RedisPresence struct {
	client *redis.Client
}

// NewRedisPresence constructs a RedisPresence.
func NewRedisPresence(client *redis.Client) *RedisPresence {
	return &RedisPresence{client: client}
}

// MarkOnline records that a CS still has the inbox open.
func (p *RedisPresence) MarkOnline(ctx context.Context, userID uuid.UUID) error {
	return p.client.Set(ctx, presenceKeyPrefix+userID.String(), "1", presenceTTL).Err()
}

// Online lists the CS currently at their desks, sorted so that the rotation is
// the same order for every caller.
func (p *RedisPresence) Online(ctx context.Context) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	iter := p.client.Scan(ctx, 0, presenceKeyPrefix+"*", 100).Iterator()
	for iter.Next(ctx) {
		id, err := uuid.Parse(strings.TrimPrefix(iter.Val(), presenceKeyPrefix))
		if err != nil {
			continue
		}
		ids = append(ids, id)
	}
	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("scan presence: %w", err)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
	return ids, nil
}

// NextTurn hands out the next position in the rotation.
func (p *RedisPresence) NextTurn(ctx context.Context) (uint64, error) {
	n, err := p.client.Incr(ctx, turnKey).Result()
	if err != nil {
		return 0, fmt.Errorf("advance rotation: %w", err)
	}
	return uint64(n), nil
}

// FakePresence is the in-memory stand-in the assignment tests run against.
type FakePresence struct {
	mu     sync.Mutex
	online []uuid.UUID
	turn   uint64
}

// NewFakePresence constructs a FakePresence with the given users online.
func NewFakePresence(ids ...uuid.UUID) *FakePresence {
	return &FakePresence{online: ids}
}

// SetOnline replaces who is online, which is how a test moves a shift along.
func (p *FakePresence) SetOnline(ids ...uuid.UUID) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.online = ids
}

// MarkOnline adds a user to the online set.
func (p *FakePresence) MarkOnline(_ context.Context, userID uuid.UUID) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, id := range p.online {
		if id == userID {
			return nil
		}
	}
	p.online = append(p.online, userID)
	return nil
}

// Online lists who is at their desk.
func (p *FakePresence) Online(_ context.Context) ([]uuid.UUID, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	ids := make([]uuid.UUID, len(p.online))
	copy(ids, p.online)
	sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
	return ids, nil
}

// NextTurn hands out the next position in the rotation.
func (p *FakePresence) NextTurn(_ context.Context) (uint64, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.turn++
	return p.turn, nil
}
```

- [ ] **Step 4: Jalankan tes, pastikan lulus**

Run: `cd backend && go test ./internal/services/ -run TestFakePresence -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/services/cs_presence.go backend/internal/services/cs_presence_test.go
git commit -m "feat(cs): count only the agents actually sitting in front of the inbox"
```

---

### Task 4: Layanan percakapan

Percakapan adalah tempat semua keputusan berlabuh: siapa pelanggannya, ONT mana miliknya, siapa yang memegangnya, dan kapan ia dianggap selesai.

**Files:**
- Create: `backend/internal/services/cs_conversation_service.go`
- Test: `backend/internal/services/cs_conversation_service_test.go`

**Interfaces:**
- Consumes: `models.CSConversation`, `models.ConversationStatus`, `utils.NormalizePhone`
- Produces:
  - `services.NewCSConversationService(db *gorm.DB) *CSConversationService`
  - `type IncomingPeer struct { WAAccountID uuid.UUID; JID, Phone, Name string }`
  - `(*CSConversationService).FindOrCreate(p IncomingPeer) (*models.CSConversation, error)`
  - `(*CSConversationService).List(f ConversationFilter) ([]models.CSConversation, error)`
  - `(*CSConversationService).Get(id uuid.UUID) (*models.CSConversation, error)`
  - `(*CSConversationService).Assign(conversationID, holderID uuid.UUID) error`
  - `(*CSConversationService).Close(conversationID uuid.UUID) error`
  - `(*CSConversationService).LinkONT(conversationID uuid.UUID, ontID *uuid.UUID) error`
  - `(*CSConversationService).EnsureHolder(conversationID, userID uuid.UUID) error`
  - `type ConversationFilter struct { Mine *uuid.UUID; Unassigned, Closed bool; Search string; Limit, Offset int }`
  - `services.ErrNotHolder`

- [ ] **Step 1: Tulis tes yang gagal**

`backend/internal/services/cs_conversation_service_test.go`:

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

func csAccount(t *testing.T, db *gorm.DB) models.WAAccount {
	account := models.WAAccount{Label: "CS Utama", Status: models.WAAccountConnected}
	require.NoError(t, db.Create(&account).Error)
	return account
}

func peer(accountID uuid.UUID) IncomingPeer {
	return IncomingPeer{
		WAAccountID: accountID,
		JID:         "6281234567890@s.whatsapp.net",
		Phone:       "081234567890",
		Name:        "Pak Budi",
	}
}

func TestFindOrCreateStartsAThreadNobodyHolds(t *testing.T) {
	db := setupTestDB(t)
	svc := NewCSConversationService(db)
	account := csAccount(t, db)

	conv, err := svc.FindOrCreate(peer(account.ID))
	require.NoError(t, err)

	assert.Equal(t, models.ConversationUnassigned, conv.Status)
	assert.Nil(t, conv.AssignedUserID)
	assert.Equal(t, "6281234567890", conv.CustomerPhone, "the number is stored in one form regardless of how it arrived")
}

func TestFindOrCreateReturnsTheSameThreadTwice(t *testing.T) {
	db := setupTestDB(t)
	svc := NewCSConversationService(db)
	account := csAccount(t, db)

	first, err := svc.FindOrCreate(peer(account.ID))
	require.NoError(t, err)
	second, err := svc.FindOrCreate(peer(account.ID))
	require.NoError(t, err)

	assert.Equal(t, first.ID, second.ID)
}

// A customer whose ONT is known should arrive with it already attached, so the
// CS sees the light levels without hunting for the subscriber first.
func TestFindOrCreateAttachesTheONTThatOwnsTheNumber(t *testing.T) {
	db := setupTestDB(t)
	svc := NewCSConversationService(db)
	account := csAccount(t, db)

	ont := models.ONT{
		OLTID: uuid.New(), PortID: 1, ONTID: 1,
		SerialNumber: "ZTEG12345678", Name: "Budi", Phone: "6281234567890",
	}
	require.NoError(t, db.Create(&ont).Error)

	conv, err := svc.FindOrCreate(peer(account.ID))
	require.NoError(t, err)

	require.NotNil(t, conv.ONTID)
	assert.Equal(t, ont.ID, *conv.ONTID)
}

// A customer who writes again after their case was closed is a new problem, not
// a footnote to the old one — and it must land in somebody's queue again.
func TestFindOrCreateReopensAClosedThreadAndDropsItsFormerHolder(t *testing.T) {
	db := setupTestDB(t)
	svc := NewCSConversationService(db)
	account := csAccount(t, db)

	conv, err := svc.FindOrCreate(peer(account.ID))
	require.NoError(t, err)

	holder := uuid.New()
	require.NoError(t, svc.Assign(conv.ID, holder))
	require.NoError(t, svc.Close(conv.ID))

	reopened, err := svc.FindOrCreate(peer(account.ID))
	require.NoError(t, err)

	assert.Equal(t, conv.ID, reopened.ID)
	assert.Equal(t, models.ConversationUnassigned, reopened.Status)
	assert.Nil(t, reopened.AssignedUserID)
}

func TestEnsureHolderRefusesSomeoneElsesThread(t *testing.T) {
	db := setupTestDB(t)
	svc := NewCSConversationService(db)
	account := csAccount(t, db)

	conv, err := svc.FindOrCreate(peer(account.ID))
	require.NoError(t, err)

	holder, intruder := uuid.New(), uuid.New()
	require.NoError(t, svc.Assign(conv.ID, holder))

	require.NoError(t, svc.EnsureHolder(conv.ID, holder))
	assert.ErrorIs(t, svc.EnsureHolder(conv.ID, intruder), ErrNotHolder)
}

func TestEnsureHolderRefusesAThreadNobodyHolds(t *testing.T) {
	db := setupTestDB(t)
	svc := NewCSConversationService(db)
	account := csAccount(t, db)

	conv, err := svc.FindOrCreate(peer(account.ID))
	require.NoError(t, err)

	assert.ErrorIs(t, svc.EnsureHolder(conv.ID, uuid.New()), ErrNotHolder)
}

func TestListSeparatesMineFromUnheld(t *testing.T) {
	db := setupTestDB(t)
	svc := NewCSConversationService(db)
	account := csAccount(t, db)
	me := uuid.New()

	mine, err := svc.FindOrCreate(IncomingPeer{
		WAAccountID: account.ID, JID: "628111@s.whatsapp.net", Phone: "628111222333", Name: "A",
	})
	require.NoError(t, err)
	require.NoError(t, svc.Assign(mine.ID, me))

	_, err = svc.FindOrCreate(IncomingPeer{
		WAAccountID: account.ID, JID: "628222@s.whatsapp.net", Phone: "628222333444", Name: "B",
	})
	require.NoError(t, err)

	held, err := svc.List(ConversationFilter{Mine: &me})
	require.NoError(t, err)
	require.Len(t, held, 1)
	assert.Equal(t, mine.ID, held[0].ID)

	waiting, err := svc.List(ConversationFilter{Unassigned: true})
	require.NoError(t, err)
	require.Len(t, waiting, 1)
	assert.Equal(t, "628222333444", waiting[0].CustomerPhone)
}
```

- [ ] **Step 2: Jalankan tes, pastikan gagal**

Run: `cd backend && go test ./internal/services/ -run 'TestFindOrCreate|TestEnsureHolder|TestListSeparates' -v`
Expected: FAIL — `undefined: NewCSConversationService`.

- [ ] **Step 3: Tulis implementasi**

`backend/internal/services/cs_conversation_service.go`:

```go
package services

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/utils"
	"gorm.io/gorm"
)

// ErrNotHolder is returned when someone tries to answer a conversation another
// CS is holding. Round-robin decides who answers; this is what enforces it.
var ErrNotHolder = errors.New("percakapan sedang dipegang orang lain")

// defaultConversationLimit keeps one inbox page bounded; at thousands of chats
// a day an unbounded list would fetch a year of history to draw one screen.
const defaultConversationLimit = 50

// CSConversationService owns a customer's thread: who they are, which ONT is
// theirs, who is holding the thread, and when it is done.
type CSConversationService struct {
	db *gorm.DB
}

// NewCSConversationService constructs a CSConversationService.
func NewCSConversationService(db *gorm.DB) *CSConversationService {
	return &CSConversationService{db: db}
}

// IncomingPeer is a customer as WhatsApp describes them.
type IncomingPeer struct {
	WAAccountID uuid.UUID
	JID         string
	Phone       string
	Name        string
}

// ConversationFilter narrows the inbox to one of the views a CS switches
// between: their own threads, the ones nobody holds, or the finished ones.
type ConversationFilter struct {
	Mine       *uuid.UUID
	Unassigned bool
	Closed     bool
	Search     string
	Limit      int
	Offset     int
}

// FindOrCreate returns the thread for one customer on one number, creating it
// on first contact. A thread that was closed is reopened and released, because
// a customer writing again has a new problem and it must reach somebody's queue.
func (s *CSConversationService) FindOrCreate(p IncomingPeer) (*models.CSConversation, error) {
	phone, err := utils.NormalizePhone(p.Phone)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrValidation, err.Error())
	}

	var conv models.CSConversation
	err = s.db.Where("wa_account_id = ? AND customer_jid = ?", p.WAAccountID, p.JID).First(&conv).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return s.create(p, phone)
	}
	if err != nil {
		return nil, fmt.Errorf("load conversation: %w", err)
	}

	updates := map[string]any{}
	if p.Name != "" && p.Name != conv.CustomerName {
		updates["customer_name"] = p.Name
	}
	if conv.Status == models.ConversationClosed {
		updates["status"] = models.ConversationUnassigned
		updates["assigned_user_id"] = nil
	}
	if len(updates) > 0 {
		if err := s.db.Model(&conv).Updates(updates).Error; err != nil {
			return nil, fmt.Errorf("reopen conversation: %w", err)
		}
	}
	return s.Get(conv.ID)
}

func (s *CSConversationService) create(p IncomingPeer, phone string) (*models.CSConversation, error) {
	conv := models.CSConversation{
		WAAccountID:   p.WAAccountID,
		CustomerJID:   p.JID,
		CustomerPhone: phone,
		CustomerName:  p.Name,
		Status:        models.ConversationUnassigned,
		LastMessageAt: time.Now(),
		ONTID:         s.ontOwning(phone),
	}
	if err := s.db.Create(&conv).Error; err != nil {
		return nil, fmt.Errorf("create conversation: %w", err)
	}
	return &conv, nil
}

// ontOwning finds the subscriber's ONT by number. A number nobody recorded is
// the normal case early on, so a miss is silence rather than an error.
func (s *CSConversationService) ontOwning(phone string) *uuid.UUID {
	var ont models.ONT
	if err := s.db.Where("phone = ?", phone).First(&ont).Error; err != nil {
		return nil
	}
	return &ont.ID
}

// Get loads one conversation.
func (s *CSConversationService) Get(id uuid.UUID) (*models.CSConversation, error) {
	var conv models.CSConversation
	if err := s.db.First(&conv, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("load conversation: %w", err)
	}
	return &conv, nil
}

// List draws one page of the inbox, newest first.
func (s *CSConversationService) List(f ConversationFilter) ([]models.CSConversation, error) {
	q := s.db.Model(&models.CSConversation{})

	switch {
	case f.Mine != nil:
		q = q.Where("assigned_user_id = ? AND status <> ?", *f.Mine, models.ConversationClosed)
	case f.Unassigned:
		q = q.Where("status = ?", models.ConversationUnassigned)
	case f.Closed:
		q = q.Where("status = ?", models.ConversationClosed)
	}

	if f.Search != "" {
		like := "%" + f.Search + "%"
		q = q.Where("customer_name LIKE ? OR customer_phone LIKE ?", like, like)
	}

	limit := f.Limit
	if limit <= 0 || limit > defaultConversationLimit {
		limit = defaultConversationLimit
	}

	var rows []models.CSConversation
	if err := q.Order("last_message_at DESC").Limit(limit).Offset(f.Offset).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list conversations: %w", err)
	}
	return rows, nil
}

// Assign hands a conversation to one CS. Taking over someone else's thread goes
// through here too; the audit trail is written by the handler that called it.
func (s *CSConversationService) Assign(conversationID, holderID uuid.UUID) error {
	return s.update(conversationID, map[string]any{
		"assigned_user_id": holderID,
		"status":           models.ConversationOpen,
	})
}

// Close marks a conversation finished. The holder stays on the row, so the
// history still says who dealt with it.
func (s *CSConversationService) Close(conversationID uuid.UUID) error {
	return s.update(conversationID, map[string]any{"status": models.ConversationClosed})
}

// LinkONT ties a thread to a subscriber's ONT, or unties it when ontID is nil.
func (s *CSConversationService) LinkONT(conversationID uuid.UUID, ontID *uuid.UUID) error {
	return s.update(conversationID, map[string]any{"ont_id": ontID})
}

// Touch records that a conversation just saw traffic, which is what orders the
// inbox.
func (s *CSConversationService) Touch(conversationID uuid.UUID, at time.Time) error {
	return s.update(conversationID, map[string]any{"last_message_at": at})
}

// EnsureHolder reports whether this user may answer this conversation.
func (s *CSConversationService) EnsureHolder(conversationID, userID uuid.UUID) error {
	conv, err := s.Get(conversationID)
	if err != nil {
		return err
	}
	if conv.AssignedUserID == nil || *conv.AssignedUserID != userID {
		return ErrNotHolder
	}
	return nil
}

func (s *CSConversationService) update(conversationID uuid.UUID, fields map[string]any) error {
	res := s.db.Model(&models.CSConversation{}).Where("id = ?", conversationID).Updates(fields)
	if res.Error != nil {
		return fmt.Errorf("update conversation: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
```

- [ ] **Step 4: Jalankan tes, pastikan lulus**

Run: `cd backend && go test ./internal/services/ -run 'TestFindOrCreate|TestEnsureHolder|TestListSeparates' -v`
Expected: PASS ketujuhnya. Kalau `ErrValidation` belum ada di paket, ambil dari `distribution_service.go` — jangan bikin yang baru.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/services/cs_conversation_service.go backend/internal/services/cs_conversation_service_test.go
git commit -m "feat(cs): keep one thread per customer, and reopen it when they write again"
```

---

### Task 5: Pembagian round-robin

**Files:**
- Create: `backend/internal/services/cs_assignment.go`
- Test: `backend/internal/services/cs_assignment_test.go`

**Interfaces:**
- Consumes: `Presence`, `*CSConversationService`, `models.CSConversation`
- Produces:
  - `services.NewCSAssignmentService(db *gorm.DB, conversations *CSConversationService, presence Presence) *CSAssignmentService`
  - `(*CSAssignmentService).AssignOne(ctx context.Context, conversationID uuid.UUID) (*uuid.UUID, error)` — nil bila tidak ada yang online
  - `(*CSAssignmentService).AssignWaiting(ctx context.Context) (int, error)` — bagikan semua yang menunggu, kembalikan jumlahnya

- [ ] **Step 1: Tulis tes yang gagal**

`backend/internal/services/cs_assignment_test.go`:

```go
package services

import (
	"context"
	"sort"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

func waitingConversation(t *testing.T, svc *CSConversationService, accountID uuid.UUID, jid, phone string) *models.CSConversation {
	conv, err := svc.FindOrCreate(IncomingPeer{WAAccountID: accountID, JID: jid, Phone: phone, Name: "X"})
	require.NoError(t, err)
	return conv
}

func assignmentSetup(t *testing.T, online ...uuid.UUID) (*gorm.DB, *CSConversationService, *CSAssignmentService, models.WAAccount, *FakePresence) {
	db := setupTestDB(t)
	conversations := NewCSConversationService(db)
	presence := NewFakePresence(online...)
	return db, conversations, NewCSAssignmentService(db, conversations, presence), csAccount(t, db), presence
}

func TestAssignmentSharesThreadsRoundTheTeam(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	_, conversations, assignment, account, _ := assignmentSetup(t, a, b)
	ctx := context.Background()

	first := waitingConversation(t, conversations, account.ID, "628111@s.whatsapp.net", "628111222333")
	second := waitingConversation(t, conversations, account.ID, "628222@s.whatsapp.net", "628222333444")
	third := waitingConversation(t, conversations, account.ID, "628333@s.whatsapp.net", "628333444555")

	var got []uuid.UUID
	for _, conv := range []*models.CSConversation{first, second, third} {
		holder, err := assignment.AssignOne(ctx, conv.ID)
		require.NoError(t, err)
		require.NotNil(t, holder)
		got = append(got, *holder)
	}

	// Two agents, three threads: each agent gets at least one, and the third
	// wraps back round rather than piling onto whoever was picked first.
	assert.NotEqual(t, got[0], got[1], "consecutive threads must not land on the same agent")
	assert.Equal(t, got[0], got[2], "the rotation wraps")

	team := []uuid.UUID{a, b}
	sort.Slice(team, func(i, j int) bool { return team[i].String() < team[j].String() })
	assert.Contains(t, team, got[0])
}

// A thread arriving at night must wait, not disappear into the inbox of
// somebody who went home four hours ago.
func TestAssignmentLeavesAThreadWaitingWhenNobodyIsOnline(t *testing.T) {
	_, conversations, assignment, account, _ := assignmentSetup(t)
	ctx := context.Background()

	conv := waitingConversation(t, conversations, account.ID, "628111@s.whatsapp.net", "628111222333")

	holder, err := assignment.AssignOne(ctx, conv.ID)
	require.NoError(t, err)
	assert.Nil(t, holder)

	after, err := conversations.Get(conv.ID)
	require.NoError(t, err)
	assert.Equal(t, models.ConversationUnassigned, after.Status)
	assert.Nil(t, after.AssignedUserID)
}

func TestAssignWaitingHandsOutEverythingLeftOvernight(t *testing.T) {
	a := uuid.New()
	_, conversations, assignment, account, presence := assignmentSetup(t)
	ctx := context.Background()

	first := waitingConversation(t, conversations, account.ID, "628111@s.whatsapp.net", "628111222333")
	second := waitingConversation(t, conversations, account.ID, "628222@s.whatsapp.net", "628222333444")

	// Nobody was online when these arrived.
	for _, conv := range []*models.CSConversation{first, second} {
		holder, err := assignment.AssignOne(ctx, conv.ID)
		require.NoError(t, err)
		require.Nil(t, holder)
	}

	presence.SetOnline(a)

	n, err := assignment.AssignWaiting(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, n)

	for _, conv := range []*models.CSConversation{first, second} {
		after, err := conversations.Get(conv.ID)
		require.NoError(t, err)
		assert.Equal(t, models.ConversationOpen, after.Status)
		require.NotNil(t, after.AssignedUserID)
		assert.Equal(t, a, *after.AssignedUserID)
	}
}

// A closed thread is finished; the morning sweep must not drag it back out.
func TestAssignWaitingIgnoresClosedThreads(t *testing.T) {
	a := uuid.New()
	_, conversations, assignment, account, _ := assignmentSetup(t, a)
	ctx := context.Background()

	conv := waitingConversation(t, conversations, account.ID, "628111@s.whatsapp.net", "628111222333")
	require.NoError(t, conversations.Close(conv.ID))

	n, err := assignment.AssignWaiting(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, n)
}
```

- [ ] **Step 2: Jalankan tes, pastikan gagal**

Run: `cd backend && go test ./internal/services/ -run TestAssign -v`
Expected: FAIL — `undefined: NewCSAssignmentService`.

- [ ] **Step 3: Tulis implementasi**

`backend/internal/services/cs_assignment.go`:

```go
package services

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// CSAssignmentService shares incoming conversations round the agents who
// actually have the inbox open.
type CSAssignmentService struct {
	db            *gorm.DB
	conversations *CSConversationService
	presence      Presence
}

// NewCSAssignmentService constructs a CSAssignmentService.
func NewCSAssignmentService(db *gorm.DB, conversations *CSConversationService, presence Presence) *CSAssignmentService {
	return &CSAssignmentService{db: db, conversations: conversations, presence: presence}
}

// AssignOne hands one waiting conversation to the next agent in the rotation.
// It answers nil when nobody is online: the thread then waits rather than
// landing in the inbox of someone who went home, and AssignWaiting picks it up
// when the next shift opens the page.
func (s *CSAssignmentService) AssignOne(ctx context.Context, conversationID uuid.UUID) (*uuid.UUID, error) {
	online, err := s.presence.Online(ctx)
	if err != nil {
		return nil, fmt.Errorf("read online agents: %w", err)
	}
	if len(online) == 0 {
		return nil, nil
	}

	turn, err := s.presence.NextTurn(ctx)
	if err != nil {
		return nil, fmt.Errorf("advance rotation: %w", err)
	}

	holder := online[turn%uint64(len(online))]
	if err := s.conversations.Assign(conversationID, holder); err != nil {
		return nil, err
	}
	return &holder, nil
}

// AssignWaiting shares out everything that arrived while nobody was watching.
func (s *CSAssignmentService) AssignWaiting(ctx context.Context) (int, error) {
	var waiting []models.CSConversation
	err := s.db.Where("status = ?", models.ConversationUnassigned).
		Order("last_message_at ASC").
		Find(&waiting).Error
	if err != nil {
		return 0, fmt.Errorf("list waiting conversations: %w", err)
	}

	assigned := 0
	for _, conv := range waiting {
		holder, err := s.AssignOne(ctx, conv.ID)
		if err != nil {
			return assigned, err
		}
		if holder == nil {
			break
		}
		assigned++
	}
	return assigned, nil
}
```

- [ ] **Step 4: Jalankan tes, pastikan lulus**

Run: `cd backend && go test ./internal/services/ -run TestAssign -v`
Expected: PASS keempatnya.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/services/cs_assignment.go backend/internal/services/cs_assignment_test.go
git commit -m "feat(cs): share incoming chats round the agents who are actually there"
```

---

### Task 6: Layanan pesan

Termasuk idempotensi pesan masuk dan antrean keluar. Di sinilah janji "tidak ada balasan CS yang menguap" ditepati.

**Files:**
- Create: `backend/internal/services/cs_message_service.go`
- Test: `backend/internal/services/cs_message_service_test.go`
- Test: `backend/internal/services/cs_message_postgres_test.go`

**Interfaces:**
- Consumes: `models.CSMessage`, `*CSConversationService`
- Produces:
  - `services.NewCSMessageService(db *gorm.DB, conversations *CSConversationService) *CSMessageService`
  - `type InboundMessage struct { ConversationID uuid.UUID; WAMessageID string; Kind models.MessageKind; Body string; Media *MediaFile; At time.Time }`
  - `type MediaFile struct { Path, Mime, Filename string; Size int64 }`
  - `(*CSMessageService).SaveInbound(in InboundMessage) (*models.CSMessage, bool, error)` — bool `false` bila pesan sudah pernah tersimpan
  - `(*CSMessageService).Queue(conversationID, senderUserID uuid.UUID, kind models.MessageKind, body string, media *MediaFile) (*models.CSMessage, error)`
  - `(*CSMessageService).ClaimQueued(limit int) ([]models.CSMessage, error)`
  - `(*CSMessageService).MarkSent(id uuid.UUID, waMessageID string) error`
  - `(*CSMessageService).MarkFailed(id uuid.UUID, reason string) error`
  - `(*CSMessageService).ApplyReceipt(waMessageID string, status models.MessageStatus) error`
  - `(*CSMessageService).History(conversationID uuid.UUID, limit, offset int) ([]models.CSMessage, error)`
  - `(*CSMessageService).Search(term string, limit int) ([]models.CSMessage, error)`

- [ ] **Step 1: Tulis tes yang gagal**

`backend/internal/services/cs_message_service_test.go`:

```go
package services

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

func messageSetup(t *testing.T) (*CSMessageService, *CSConversationService, *models.CSConversation) {
	db := setupTestDB(t)
	conversations := NewCSConversationService(db)
	account := csAccount(t, db)
	conv, err := conversations.FindOrCreate(IncomingPeer{
		WAAccountID: account.ID, JID: "628111@s.whatsapp.net", Phone: "628111222333", Name: "Budi",
	})
	require.NoError(t, err)
	return NewCSMessageService(db, conversations), conversations, conv
}

// WhatsApp re-delivers events it is unsure about. Storing the second copy would
// show the customer's message twice and, worse, count it twice as unread.
func TestSaveInboundStoresARepeatedMessageOnlyOnce(t *testing.T) {
	messages, _, conv := messageSetup(t)

	in := InboundMessage{
		ConversationID: conv.ID,
		WAMessageID:    "3EB0ABC123",
		Kind:           models.MessageKindText,
		Body:           "internet saya mati",
		At:             time.Now(),
	}

	first, created, err := messages.SaveInbound(in)
	require.NoError(t, err)
	assert.True(t, created)

	second, created, err := messages.SaveInbound(in)
	require.NoError(t, err)
	assert.False(t, created, "the same WhatsApp message must not be stored twice")
	assert.Equal(t, first.ID, second.ID)

	history, err := messages.History(conv.ID, 10, 0)
	require.NoError(t, err)
	assert.Len(t, history, 1)
}

func TestSaveInboundMovesTheThreadToTheTopOfTheInbox(t *testing.T) {
	messages, conversations, conv := messageSetup(t)

	before, err := conversations.Get(conv.ID)
	require.NoError(t, err)

	later := time.Now().Add(time.Hour)
	_, _, err = messages.SaveInbound(InboundMessage{
		ConversationID: conv.ID, WAMessageID: "3EB0ABC124",
		Kind: models.MessageKindText, Body: "halo", At: later,
	})
	require.NoError(t, err)

	after, err := conversations.Get(conv.ID)
	require.NoError(t, err)
	assert.True(t, after.LastMessageAt.After(before.LastMessageAt))
	assert.Equal(t, 1, after.UnreadCount)
}

func TestQueuedMessageWaitsWithoutAWhatsAppID(t *testing.T) {
	messages, _, conv := messageSetup(t)
	sender := uuid.New()

	msg, err := messages.Queue(conv.ID, sender, models.MessageKindText, "sudah kami cek", nil)
	require.NoError(t, err)

	assert.Equal(t, models.MessageQueued, msg.Status)
	assert.Equal(t, models.MessageOut, msg.Direction)
	assert.Nil(t, msg.WAMessageID)
	require.NotNil(t, msg.SenderUserID)
	assert.Equal(t, sender, *msg.SenderUserID)
}

// This is the promise that no CS reply disappears: whatever was queued while
// the WhatsApp process was down is still here to be claimed when it returns.
func TestClaimQueuedReturnsOnlyWhatIsStillWaiting(t *testing.T) {
	messages, _, conv := messageSetup(t)
	sender := uuid.New()

	waiting, err := messages.Queue(conv.ID, sender, models.MessageKindText, "menunggu", nil)
	require.NoError(t, err)
	gone, err := messages.Queue(conv.ID, sender, models.MessageKindText, "terkirim", nil)
	require.NoError(t, err)
	require.NoError(t, messages.MarkSent(gone.ID, "3EB0SENT"))

	claimed, err := messages.ClaimQueued(10)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	assert.Equal(t, waiting.ID, claimed[0].ID)
}

func TestMarkFailedKeepsTheReasonWhereTheCSCanReadIt(t *testing.T) {
	messages, _, conv := messageSetup(t)

	msg, err := messages.Queue(conv.ID, uuid.New(), models.MessageKindText, "halo", nil)
	require.NoError(t, err)
	require.NoError(t, messages.MarkFailed(msg.ID, "nomor tidak terdaftar di WhatsApp"))

	history, err := messages.History(conv.ID, 10, 0)
	require.NoError(t, err)
	require.Len(t, history, 1)
	assert.Equal(t, models.MessageFailed, history[0].Status)
	assert.Equal(t, "nomor tidak terdaftar di WhatsApp", history[0].FailReason)
}

func TestApplyReceiptWalksAMessageForwardOnly(t *testing.T) {
	messages, _, conv := messageSetup(t)

	msg, err := messages.Queue(conv.ID, uuid.New(), models.MessageKindText, "halo", nil)
	require.NoError(t, err)
	require.NoError(t, messages.MarkSent(msg.ID, "3EB0AAA"))

	require.NoError(t, messages.ApplyReceipt("3EB0AAA", models.MessageRead))
	require.NoError(t, messages.ApplyReceipt("3EB0AAA", models.MessageDelivered))

	history, err := messages.History(conv.ID, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, models.MessageRead, history[0].Status,
		"a late delivered receipt must not undo a read one")
}
```

`backend/internal/services/cs_message_postgres_test.go`:

```go
package services

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

// Full-text search is the one part of the message service SQLite cannot answer
// for: the tsvector column and its GIN index exist only in migration 41.
func TestSearchFindsAMessageByItsWordsOnPostgres(t *testing.T) {
	if os.Getenv("TEST_POSTGRES_DSN") == "" {
		t.Skip("set TEST_POSTGRES_DSN to search against Postgres")
	}

	db := setupPostgresTestDB(t)
	conversations := NewCSConversationService(db)
	messages := NewCSMessageService(db, conversations)

	account := models.WAAccount{Label: "CS Utama", Status: models.WAAccountConnected}
	require.NoError(t, db.Create(&account).Error)

	conv, err := conversations.FindOrCreate(IncomingPeer{
		WAAccountID: account.ID, JID: "628111@s.whatsapp.net", Phone: "628111222333", Name: "Budi",
	})
	require.NoError(t, err)

	_, _, err = messages.SaveInbound(InboundMessage{
		ConversationID: conv.ID, WAMessageID: "3EB0SEARCH",
		Kind: models.MessageKindText, Body: "lampu los merah berkedip", At: time.Now(),
	})
	require.NoError(t, err)

	found, err := messages.Search("los", 10)
	require.NoError(t, err)
	require.Len(t, found, 1)
	assert.Equal(t, "lampu los merah berkedip", found[0].Body)
}
```

Catatan: `setupPostgresTestDB(t)` mengikuti pola yang sudah dipakai `distribution_postgres_test.go`. Baca berkas itu dan pakai pembantu yang sama — jangan buat versi kedua.

- [ ] **Step 2: Jalankan tes, pastikan gagal**

Run: `cd backend && go test ./internal/services/ -run 'TestSaveInbound|TestQueued|TestClaimQueued|TestMarkFailed|TestApplyReceipt' -v`
Expected: FAIL — `undefined: NewCSMessageService`.

- [ ] **Step 3: Tulis implementasi**

`backend/internal/services/cs_message_service.go`:

```go
package services

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// defaultHistoryLimit is one screen of a thread.
const defaultHistoryLimit = 50

// receiptRank orders the receipts WhatsApp sends. They can arrive out of order,
// and a late "delivered" must not walk a message back from "read".
var receiptRank = map[models.MessageStatus]int{
	models.MessageQueued:    0,
	models.MessageSent:      1,
	models.MessageDelivered: 2,
	models.MessageRead:      3,
}

// MediaFile is a stored attachment.
type MediaFile struct {
	Path     string
	Mime     string
	Filename string
	Size     int64
}

// InboundMessage is a message as it arrived from WhatsApp.
type InboundMessage struct {
	ConversationID uuid.UUID
	WAMessageID    string
	Kind           models.MessageKind
	Body           string
	Media          *MediaFile
	At             time.Time
}

// CSMessageService stores the traffic in a thread, in both directions.
type CSMessageService struct {
	db            *gorm.DB
	conversations *CSConversationService
}

// NewCSMessageService constructs a CSMessageService.
func NewCSMessageService(db *gorm.DB, conversations *CSConversationService) *CSMessageService {
	return &CSMessageService{db: db, conversations: conversations}
}

// SaveInbound stores an incoming message, answering false when this WhatsApp
// message was already stored. WhatsApp re-delivers events it is unsure about,
// and the duplicate would otherwise be shown to the CS and counted as unread.
//
// The lookup is done here as well as by the partial unique index in migration
// 41, because SQLite tests never get that index.
func (s *CSMessageService) SaveInbound(in InboundMessage) (*models.CSMessage, bool, error) {
	var existing models.CSMessage
	err := s.db.Where("wa_message_id = ?", in.WAMessageID).First(&existing).Error
	if err == nil {
		return &existing, false, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, fmt.Errorf("look for existing message: %w", err)
	}

	waID := in.WAMessageID
	msg := models.CSMessage{
		ConversationID: in.ConversationID,
		WAMessageID:    &waID,
		Direction:      models.MessageIn,
		Kind:           in.Kind,
		Body:           in.Body,
		Status:         models.MessageDelivered,
		WATimestamp:    in.At,
	}
	applyMedia(&msg, in.Media)

	if err := s.db.Create(&msg).Error; err != nil {
		return nil, false, fmt.Errorf("store inbound message: %w", err)
	}

	if err := s.bumpConversation(in.ConversationID, in.At); err != nil {
		return nil, false, err
	}
	return &msg, true, nil
}

// Queue writes a CS reply as waiting to be sent. The row is the outbox: the wa
// process claims it, and one that was written while that process was down is
// still here when it comes back.
func (s *CSMessageService) Queue(
	conversationID, senderUserID uuid.UUID,
	kind models.MessageKind, body string, media *MediaFile,
) (*models.CSMessage, error) {
	sender := senderUserID
	msg := models.CSMessage{
		ConversationID: conversationID,
		Direction:      models.MessageOut,
		SenderUserID:   &sender,
		Kind:           kind,
		Body:           body,
		Status:         models.MessageQueued,
		WATimestamp:    time.Now(),
	}
	applyMedia(&msg, media)

	if err := s.db.Create(&msg).Error; err != nil {
		return nil, fmt.Errorf("queue message: %w", err)
	}
	if err := s.conversations.Touch(conversationID, msg.WATimestamp); err != nil {
		return nil, err
	}
	return &msg, nil
}

// ClaimQueued returns the messages still waiting to reach WhatsApp, oldest
// first so a thread's replies arrive in the order the CS wrote them.
func (s *CSMessageService) ClaimQueued(limit int) ([]models.CSMessage, error) {
	if limit <= 0 {
		limit = defaultHistoryLimit
	}
	var rows []models.CSMessage
	err := s.db.Where("status = ?", models.MessageQueued).
		Order("created_at ASC").Limit(limit).Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("claim queued messages: %w", err)
	}
	return rows, nil
}

// MarkSent records that WhatsApp accepted a message.
func (s *CSMessageService) MarkSent(id uuid.UUID, waMessageID string) error {
	return s.updateMessage(id, map[string]any{
		"status":        models.MessageSent,
		"wa_message_id": waMessageID,
		"fail_reason":   "",
	})
}

// MarkFailed records why a message could not be sent, so the CS reads a reason
// rather than watching a reply quietly disappear.
func (s *CSMessageService) MarkFailed(id uuid.UUID, reason string) error {
	return s.updateMessage(id, map[string]any{
		"status":      models.MessageFailed,
		"fail_reason": reason,
	})
}

// ApplyReceipt walks a message forward through sent, delivered and read, and
// refuses to walk it back when receipts arrive out of order.
func (s *CSMessageService) ApplyReceipt(waMessageID string, status models.MessageStatus) error {
	var msg models.CSMessage
	err := s.db.Where("wa_message_id = ?", waMessageID).First(&msg).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("load message for receipt: %w", err)
	}
	if receiptRank[status] <= receiptRank[msg.Status] {
		return nil
	}
	return s.updateMessage(msg.ID, map[string]any{"status": status})
}

// History returns one page of a thread, newest first.
func (s *CSMessageService) History(conversationID uuid.UUID, limit, offset int) ([]models.CSMessage, error) {
	if limit <= 0 || limit > defaultHistoryLimit {
		limit = defaultHistoryLimit
	}
	var rows []models.CSMessage
	err := s.db.Where("conversation_id = ?", conversationID).
		Order("wa_timestamp DESC").Limit(limit).Offset(offset).Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("load history: %w", err)
	}
	return rows, nil
}

// Search finds messages by their words. It leans on the tsvector column added
// in migration 41, so it answers on Postgres only.
func (s *CSMessageService) Search(term string, limit int) ([]models.CSMessage, error) {
	if limit <= 0 || limit > defaultHistoryLimit {
		limit = defaultHistoryLimit
	}
	var rows []models.CSMessage
	err := s.db.Where("tsv @@ plainto_tsquery('simple', ?)", term).
		Order("wa_timestamp DESC").Limit(limit).Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("search messages: %w", err)
	}
	return rows, nil
}

func (s *CSMessageService) bumpConversation(conversationID uuid.UUID, at time.Time) error {
	err := s.db.Model(&models.CSConversation{}).Where("id = ?", conversationID).
		Updates(map[string]any{
			"last_message_at": at,
			"unread_count":    gorm.Expr("unread_count + 1"),
		}).Error
	if err != nil {
		return fmt.Errorf("bump conversation: %w", err)
	}
	return nil
}

func (s *CSMessageService) updateMessage(id uuid.UUID, fields map[string]any) error {
	res := s.db.Model(&models.CSMessage{}).Where("id = ?", id).Updates(fields)
	if res.Error != nil {
		return fmt.Errorf("update message: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func applyMedia(msg *models.CSMessage, media *MediaFile) {
	if media == nil {
		return
	}
	msg.MediaPath = media.Path
	msg.MediaMime = media.Mime
	msg.MediaFilename = media.Filename
	msg.MediaSize = media.Size
}
```

- [ ] **Step 4: Jalankan tes, pastikan lulus**

Run: `cd backend && go test ./internal/services/ -run 'TestSaveInbound|TestQueued|TestClaimQueued|TestMarkFailed|TestApplyReceipt' -v`
Expected: PASS keenamnya.

- [ ] **Step 5: Jalankan tes Postgres**

Run:
```bash
cd backend && TEST_POSTGRES_DSN="host=localhost port=5437 user=tikman password=$POSTGRES_PASSWORD dbname=tikman sslmode=disable" \
  go test ./internal/services/ -run TestSearchFindsAMessage -v
```
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/services/cs_message_service.go backend/internal/services/cs_message_service_test.go backend/internal/services/cs_message_postgres_test.go
git commit -m "feat(cs): store what arrives once, and keep what was queued until it is sent"
```

---

### Task 7: Balasan cepat

**Files:**
- Create: `backend/internal/services/cs_quick_reply_service.go`
- Test: `backend/internal/services/cs_quick_reply_service_test.go`

**Interfaces:**
- Produces:
  - `services.NewCSQuickReplyService(db *gorm.DB) *CSQuickReplyService`
  - `(*CSQuickReplyService).List() ([]models.CSQuickReply, error)`
  - `(*CSQuickReplyService).Create(title, body string, createdBy uuid.UUID) (*models.CSQuickReply, error)`
  - `(*CSQuickReplyService).Update(id uuid.UUID, title, body string) (*models.CSQuickReply, error)`
  - `(*CSQuickReplyService).Delete(id uuid.UUID) error`

- [ ] **Step 1: Tulis tes yang gagal**

`backend/internal/services/cs_quick_reply_service_test.go`:

```go
package services

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQuickReplyRoundTrip(t *testing.T) {
	db := setupTestDB(t)
	svc := NewCSQuickReplyService(db)
	author := uuid.New()

	created, err := svc.Create("Cek LOS", "Mohon cek lampu LOS pada modem, apakah menyala merah?", author)
	require.NoError(t, err)
	assert.Equal(t, "Cek LOS", created.Title)

	updated, err := svc.Update(created.ID, "Cek LOS", "Mohon foto lampu pada modem.")
	require.NoError(t, err)
	assert.Equal(t, "Mohon foto lampu pada modem.", updated.Body)

	list, err := svc.List()
	require.NoError(t, err)
	require.Len(t, list, 1)

	require.NoError(t, svc.Delete(created.ID))
	list, err = svc.List()
	require.NoError(t, err)
	assert.Empty(t, list)
}

// A template with no body inserts nothing, which looks to the CS like the
// button is broken.
func TestQuickReplyRefusesAnEmptyBody(t *testing.T) {
	db := setupTestDB(t)
	svc := NewCSQuickReplyService(db)

	_, err := svc.Create("Kosong", "   ", uuid.New())
	assert.ErrorIs(t, err, ErrValidation)

	_, err = svc.Create("  ", "isi", uuid.New())
	assert.ErrorIs(t, err, ErrValidation)
}
```

- [ ] **Step 2: Jalankan tes, pastikan gagal**

Run: `cd backend && go test ./internal/services/ -run TestQuickReply -v`
Expected: FAIL — `undefined: NewCSQuickReplyService`.

- [ ] **Step 3: Tulis implementasi**

`backend/internal/services/cs_quick_reply_service.go`:

```go
package services

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// CSQuickReplyService owns the canned answers a CS inserts instead of retyping
// the same sentence forty times a day.
type CSQuickReplyService struct {
	db *gorm.DB
}

// NewCSQuickReplyService constructs a CSQuickReplyService.
func NewCSQuickReplyService(db *gorm.DB) *CSQuickReplyService {
	return &CSQuickReplyService{db: db}
}

// List returns every template, by title.
func (s *CSQuickReplyService) List() ([]models.CSQuickReply, error) {
	var rows []models.CSQuickReply
	if err := s.db.Order("title ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list quick replies: %w", err)
	}
	return rows, nil
}

// Create records a template.
func (s *CSQuickReplyService) Create(title, body string, createdBy uuid.UUID) (*models.CSQuickReply, error) {
	title, body, err := validQuickReply(title, body)
	if err != nil {
		return nil, err
	}

	row := models.CSQuickReply{Title: title, Body: body, CreatedBy: createdBy}
	if err := s.db.Create(&row).Error; err != nil {
		return nil, fmt.Errorf("create quick reply: %w", err)
	}
	return &row, nil
}

// Update rewrites a template.
func (s *CSQuickReplyService) Update(id uuid.UUID, title, body string) (*models.CSQuickReply, error) {
	title, body, err := validQuickReply(title, body)
	if err != nil {
		return nil, err
	}

	res := s.db.Model(&models.CSQuickReply{}).Where("id = ?", id).
		Updates(map[string]any{"title": title, "body": body})
	if res.Error != nil {
		return nil, fmt.Errorf("update quick reply: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	var row models.CSQuickReply
	if err := s.db.First(&row, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("load quick reply: %w", err)
	}
	return &row, nil
}

// Delete removes a template.
func (s *CSQuickReplyService) Delete(id uuid.UUID) error {
	res := s.db.Delete(&models.CSQuickReply{}, "id = ?", id)
	if res.Error != nil {
		return fmt.Errorf("delete quick reply: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func validQuickReply(title, body string) (string, string, error) {
	title, body = strings.TrimSpace(title), strings.TrimSpace(body)
	if title == "" {
		return "", "", fmt.Errorf("%w: judul balasan cepat wajib diisi", ErrValidation)
	}
	if body == "" {
		return "", "", fmt.Errorf("%w: isi balasan cepat wajib diisi", ErrValidation)
	}
	return title, body, nil
}
```

- [ ] **Step 4: Jalankan tes, pastikan lulus**

Run: `cd backend && go test ./internal/services/ -run TestQuickReply -v`
Expected: PASS keduanya.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/services/cs_quick_reply_service.go backend/internal/services/cs_quick_reply_service_test.go
git commit -m "feat(cs): let a CS insert the sentence they type forty times a day"
```

---

### Task 8: Retensi media

**Files:**
- Create: `backend/internal/services/cs_media_retention.go`
- Test: `backend/internal/services/cs_media_retention_test.go`

**Interfaces:**
- Produces:
  - `services.NewCSMediaRetention(db *gorm.DB, root string, keepDays int) *CSMediaRetention`
  - `(*CSMediaRetention).Sweep() (int, error)` — jumlah berkas yang dihapus

- [ ] **Step 1: Tulis tes yang gagal**

`backend/internal/services/cs_media_retention_test.go`:

```go
package services

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

// The message stays; only the file goes. A CS reading old history should see
// that the customer sent a photo, even when the photo itself is long gone.
func TestSweepDeletesOldFilesButKeepsTheMessages(t *testing.T) {
	db := setupTestDB(t)
	conversations := NewCSConversationService(db)
	messages := NewCSMessageService(db, conversations)
	account := csAccount(t, db)

	conv, err := conversations.FindOrCreate(IncomingPeer{
		WAAccountID: account.ID, JID: "628111@s.whatsapp.net", Phone: "628111222333", Name: "Budi",
	})
	require.NoError(t, err)

	root := t.TempDir()
	oldPath := filepath.Join("2026", "01", "old.jpg")
	freshPath := filepath.Join("2026", "09", "fresh.jpg")
	for _, rel := range []string{oldPath, freshPath} {
		require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(root, rel)), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(root, rel), []byte("x"), 0o644))
	}

	old, _, err := messages.SaveInbound(InboundMessage{
		ConversationID: conv.ID, WAMessageID: "3EB0OLD", Kind: models.MessageKindImage,
		Media: &MediaFile{Path: oldPath, Mime: "image/jpeg", Filename: "old.jpg", Size: 1},
		At:    time.Now().Add(-100 * 24 * time.Hour),
	})
	require.NoError(t, err)

	_, _, err = messages.SaveInbound(InboundMessage{
		ConversationID: conv.ID, WAMessageID: "3EB0FRESH", Kind: models.MessageKindImage,
		Media: &MediaFile{Path: freshPath, Mime: "image/jpeg", Filename: "fresh.jpg", Size: 1},
		At:    time.Now(),
	})
	require.NoError(t, err)

	removed, err := NewCSMediaRetention(db, root, 90).Sweep()
	require.NoError(t, err)
	assert.Equal(t, 1, removed)

	_, err = os.Stat(filepath.Join(root, oldPath))
	assert.True(t, os.IsNotExist(err), "the old file is gone")
	_, err = os.Stat(filepath.Join(root, freshPath))
	assert.NoError(t, err, "the recent file stays")

	var stored models.CSMessage
	require.NoError(t, db.First(&stored, "id = ?", old.ID).Error)
	assert.Equal(t, models.MessageKindImage, stored.Kind, "the message survives its file")
	assert.Empty(t, stored.MediaPath, "but no longer points at one")
}
```

- [ ] **Step 2: Jalankan tes, pastikan gagal**

Run: `cd backend && go test ./internal/services/ -run TestSweepDeletesOldFiles -v`
Expected: FAIL — `undefined: NewCSMediaRetention`.

- [ ] **Step 3: Tulis implementasi**

`backend/internal/services/cs_media_retention.go`:

```go
package services

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// CSMediaRetention removes attachments once they are old enough that nobody
// opens them, which is what keeps the VPS disk from filling up at a few
// gigabytes a month.
type CSMediaRetention struct {
	db       *gorm.DB
	root     string
	keepDays int
}

// NewCSMediaRetention constructs a CSMediaRetention.
func NewCSMediaRetention(db *gorm.DB, root string, keepDays int) *CSMediaRetention {
	return &CSMediaRetention{db: db, root: root, keepDays: keepDays}
}

// Sweep deletes attachments past the retention window and forgets their paths.
// The message rows stay: a CS reading old history should still see that the
// customer sent a photo, even when the photo itself is gone.
func (r *CSMediaRetention) Sweep() (int, error) {
	cutoff := time.Now().AddDate(0, 0, -r.keepDays)

	var rows []models.CSMessage
	err := r.db.Where("media_path <> '' AND wa_timestamp < ?", cutoff).Find(&rows).Error
	if err != nil {
		return 0, fmt.Errorf("list expired media: %w", err)
	}

	removed := 0
	for _, row := range rows {
		if err := os.Remove(filepath.Join(r.root, row.MediaPath)); err != nil && !os.IsNotExist(err) {
			return removed, fmt.Errorf("remove %s: %w", row.MediaPath, err)
		}
		err := r.db.Model(&models.CSMessage{}).Where("id = ?", row.ID).
			Updates(map[string]any{"media_path": "", "media_size": 0}).Error
		if err != nil {
			return removed, fmt.Errorf("forget media path: %w", err)
		}
		removed++
	}
	return removed, nil
}
```

- [ ] **Step 4: Jalankan tes, pastikan lulus**

Run: `cd backend && go test ./internal/services/ -run TestSweepDeletesOldFiles -v`
Expected: PASS.

- [ ] **Step 5: Jalankan seluruh tes layanan**

Run: `cd backend && go test ./internal/services/... -race`
Expected: PASS semuanya.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/services/cs_media_retention.go backend/internal/services/cs_media_retention_test.go
git commit -m "feat(cs): let old attachments go, and keep the messages that carried them"
```

---

### Task 9: Pengurasan antrean dan batas pengiriman

Batas antara logika pengiriman dan koneksi WhatsApp sungguhan. `Sender` ada supaya bagian yang bisa salah — memilih pesan, menandai berhasil, menandai gagal — dapat diuji tanpa nomor WhatsApp.

**Files:**
- Create: `backend/internal/wa/sender.go`
- Create: `backend/internal/wa/outbound.go`
- Test: `backend/internal/wa/outbound_test.go`

**Interfaces:**
- Consumes: `*services.CSMessageService`, `*services.CSConversationService`, `models.CSMessage`
- Produces:
  - `type Sender interface { SendText(ctx context.Context, jid, body string) (string, error); SendMedia(ctx context.Context, jid string, kind models.MessageKind, path, mime, filename, caption string) (string, error) }`
  - `wa.NewDrainer(messages *services.CSMessageService, conversations *services.CSConversationService, sender Sender, mediaRoot string, pace time.Duration) *Drainer`
  - `(*Drainer).Drain(ctx context.Context, limit int) (int, error)`

- [ ] **Step 1: Tulis tes yang gagal**

`backend/internal/wa/outbound_test.go`:

```go
package wa

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type fakeSender struct {
	sent []string
	err  error
}

func (f *fakeSender) SendText(_ context.Context, _, body string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	f.sent = append(f.sent, body)
	return "3EB0" + body, nil
}

func (f *fakeSender) SendMedia(_ context.Context, _ string, _ models.MessageKind, path, _, _, _ string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	f.sent = append(f.sent, path)
	return "3EB0MEDIA", nil
}

func drainSetup(t *testing.T) (*gorm.DB, *services.CSMessageService, *services.CSConversationService, *models.CSConversation) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, models.AutoMigrate(db))

	account := models.WAAccount{Label: "CS Utama", Status: models.WAAccountConnected}
	require.NoError(t, db.Create(&account).Error)

	conversations := services.NewCSConversationService(db)
	conv, err := conversations.FindOrCreate(services.IncomingPeer{
		WAAccountID: account.ID, JID: "628111@s.whatsapp.net", Phone: "628111222333", Name: "Budi",
	})
	require.NoError(t, err)

	return db, services.NewCSMessageService(db, conversations), conversations, conv
}

func TestDrainSendsWhatIsWaitingAndMarksItSent(t *testing.T) {
	_, messages, conversations, conv := drainSetup(t)
	sender := &fakeSender{}

	_, err := messages.Queue(conv.ID, uuid.New(), models.MessageKindText, "sudah kami cek", nil)
	require.NoError(t, err)

	n, err := NewDrainer(messages, conversations, sender, t.TempDir(), 0).Drain(context.Background(), 10)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Equal(t, []string{"sudah kami cek"}, sender.sent)

	history, err := messages.History(conv.ID, 10, 0)
	require.NoError(t, err)
	require.Len(t, history, 1)
	assert.Equal(t, models.MessageSent, history[0].Status)
	require.NotNil(t, history[0].WAMessageID)
}

// A refusal from WhatsApp must end up in front of the CS as a sentence, not as
// a reply that silently never arrives.
func TestDrainRecordsWhyAMessageCouldNotBeSent(t *testing.T) {
	_, messages, conversations, conv := drainSetup(t)
	sender := &fakeSender{err: errors.New("nomor tidak terdaftar di WhatsApp")}

	_, err := messages.Queue(conv.ID, uuid.New(), models.MessageKindText, "halo", nil)
	require.NoError(t, err)

	n, err := NewDrainer(messages, conversations, sender, t.TempDir(), 0).Drain(context.Background(), 10)
	require.NoError(t, err, "one bad message must not stop the drain")
	assert.Equal(t, 0, n)

	history, err := messages.History(conv.ID, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, models.MessageFailed, history[0].Status)
	assert.Contains(t, history[0].FailReason, "nomor tidak terdaftar")
}

// Draining twice must not send the same reply twice: a customer receiving the
// same answer repeatedly is worse than a slow answer.
func TestDrainDoesNotSendTheSameMessageTwice(t *testing.T) {
	_, messages, conversations, conv := drainSetup(t)
	sender := &fakeSender{}
	drainer := NewDrainer(messages, conversations, sender, t.TempDir(), 0)

	_, err := messages.Queue(conv.ID, uuid.New(), models.MessageKindText, "halo", nil)
	require.NoError(t, err)

	_, err = drainer.Drain(context.Background(), 10)
	require.NoError(t, err)
	_, err = drainer.Drain(context.Background(), 10)
	require.NoError(t, err)

	assert.Len(t, sender.sent, 1)
}
```

- [ ] **Step 2: Jalankan tes, pastikan gagal**

Run: `cd backend && go test ./internal/wa/ -v`
Expected: FAIL — paket `wa` belum ada.

- [ ] **Step 3: Tulis implementasi**

`backend/internal/wa/sender.go`:

```go
// Package wa holds the one process that talks to WhatsApp.
package wa

import (
	"context"

	"github.com/tikman/olt-provisioning/internal/models"
)

// Sender is the part of whatsmeow that sending needs. It exists so the logic
// worth getting right — which message goes next, what happens when WhatsApp
// refuses — can be tested without a WhatsApp connection, while the connection
// itself stays in client.go where the network-code exemption applies.
type Sender interface {
	SendText(ctx context.Context, jid, body string) (waMessageID string, err error)
	SendMedia(ctx context.Context, jid string, kind models.MessageKind, path, mime, filename, caption string) (waMessageID string, err error)
}
```

`backend/internal/wa/outbound.go`:

```go
package wa

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
)

// Drainer sends the replies waiting in the outbox.
type Drainer struct {
	messages      *services.CSMessageService
	conversations *services.CSConversationService
	sender        Sender
	mediaRoot     string
	pace          time.Duration
}

// NewDrainer constructs a Drainer. pace is the gap left between two sends:
// emptying the queue as fast as the connection allows is the pattern that gets
// an unofficial number flagged fastest, so the queue is drained deliberately
// slowly.
func NewDrainer(
	messages *services.CSMessageService,
	conversations *services.CSConversationService,
	sender Sender,
	mediaRoot string,
	pace time.Duration,
) *Drainer {
	return &Drainer{
		messages:      messages,
		conversations: conversations,
		sender:        sender,
		mediaRoot:     mediaRoot,
		pace:          pace,
	}
}

// Drain sends what is waiting and answers how many reached WhatsApp. A message
// WhatsApp refuses is recorded with its reason and the drain continues: one bad
// number must not hold up every other customer's reply.
func (d *Drainer) Drain(ctx context.Context, limit int) (int, error) {
	waiting, err := d.messages.ClaimQueued(limit)
	if err != nil {
		return 0, err
	}

	sent := 0
	for i, msg := range waiting {
		if i > 0 && d.pace > 0 {
			select {
			case <-ctx.Done():
				return sent, ctx.Err()
			case <-time.After(d.pace):
			}
		}

		waID, err := d.send(ctx, msg)
		if err != nil {
			if markErr := d.messages.MarkFailed(msg.ID, err.Error()); markErr != nil {
				return sent, markErr
			}
			continue
		}
		if err := d.messages.MarkSent(msg.ID, waID); err != nil {
			return sent, err
		}
		sent++
	}
	return sent, nil
}

func (d *Drainer) send(ctx context.Context, msg models.CSMessage) (string, error) {
	conv, err := d.conversations.Get(msg.ConversationID)
	if err != nil {
		return "", fmt.Errorf("percakapan tidak ditemukan: %w", err)
	}

	if msg.Kind == models.MessageKindText {
		return d.sender.SendText(ctx, conv.CustomerJID, msg.Body)
	}
	return d.sender.SendMedia(
		ctx, conv.CustomerJID, msg.Kind,
		filepath.Join(d.mediaRoot, msg.MediaPath),
		msg.MediaMime, msg.MediaFilename, msg.Body,
	)
}
```

- [ ] **Step 4: Jalankan tes, pastikan lulus**

Run: `cd backend && go test ./internal/wa/ -v`
Expected: PASS ketiganya.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/wa
git commit -m "feat(cs): drain the outbox deliberately, and say why a message did not go"
```

---

### Task 10: Koneksi whatsmeow dan proses `wa`

Bagian yang menyentuh jaringan sungguhan. Sesuai pengecualian di CLAUDE.md, `client.go` tidak diuji unit; yang diuji adalah semua yang sudah dipisahkan darinya di task-task sebelumnya.

**Files:**
- Create: `backend/internal/wa/client.go`
- Create: `backend/internal/wa/inbound.go`
- Create: `backend/internal/wa/media.go`
- Create: `backend/internal/wa/receipts.go`
- Create: `backend/internal/wa/events.go`
- Create: `backend/cmd/wa/main.go`
- Create: `backend/Dockerfile.wa`
- Modify: `backend/internal/config/config.go`
- Modify: `backend/go.mod`, `backend/go.sum`

**Interfaces:**
- Consumes: `Sender`, `*Drainer`, `*services.CSMessageService`, `*services.CSConversationService`, `*services.CSAssignmentService`, `*services.CSMediaRetention`
- Produces:
  - `wa.NewClient(...) (*Client, error)` yang memenuhi `Sender`
  - `(*Client).Connect(ctx context.Context) error`, `(*Client).QRChannel(ctx context.Context) (<-chan string, error)`, `(*Client).Logout(ctx context.Context) error`
  - `wa.NewPublisher(client *redis.Client) *Publisher` dengan `(*Publisher).Publish(ctx context.Context, event Event) error`
  - `type Event struct { Type string `json:"type"`; ConversationID string `json:"conversation_id,omitempty"`; MessageID string `json:"message_id,omitempty"`; AccountStatus string `json:"account_status,omitempty"` }`
  - Konstanta `wa.EventsChannel = "cs:events"`
  - Config: `cfg.WAMediaDir`, `cfg.WASendIntervalMS`, `cfg.WAMediaRetentionDays`, `cfg.WADrainIntervalSeconds`

- [ ] **Step 1: Pasang dependensi**

Run:
```bash
cd backend
go get go.mau.fi/whatsmeow@latest
go mod tidy && go mod verify
git diff go.mod
```
Catat versi persis yang masuk ke `go.mod` — itulah versi yang dipatok. Kalau ada dependensi lain selain `go.mau.fi/whatsmeow`, `go.mau.fi/libsignal`, dan `google.golang.org/protobuf` beserta turunan wajibnya, berhenti dan laporkan sebelum lanjut: hanya ketiganya yang disetujui.

- [ ] **Step 2: Tambah konfigurasi**

Di `backend/internal/config/config.go`, tambahkan ke struct `Config`:

```go
	// WAMediaDir is where attachments from WhatsApp are written.
	WAMediaDir string
	// WASendIntervalMS is the gap left between two outgoing messages. Emptying
	// the queue at full speed is what gets an unofficial number flagged.
	WASendIntervalMS int
	// WADrainIntervalSeconds is how often the outbox is swept even when no Redis
	// announcement arrived, so a lost announcement costs latency, not a reply.
	WADrainIntervalSeconds int
	// WAMediaRetentionDays is how long an attachment is kept on disk.
	WAMediaRetentionDays int
```

Defaults dan pembacaannya:

```go
	viper.SetDefault("WA_MEDIA_DIR", "/data/cs-media")
	viper.SetDefault("WA_SEND_INTERVAL_MS", 1200)
	viper.SetDefault("WA_DRAIN_INTERVAL_SECONDS", 30)
	viper.SetDefault("WA_MEDIA_RETENTION_DAYS", 90)
```

```go
		WAMediaDir:             viper.GetString("WA_MEDIA_DIR"),
		WASendIntervalMS:       viper.GetInt("WA_SEND_INTERVAL_MS"),
		WADrainIntervalSeconds: viper.GetInt("WA_DRAIN_INTERVAL_SECONDS"),
		WAMediaRetentionDays:   viper.GetInt("WA_MEDIA_RETENTION_DAYS"),
```

- [ ] **Step 3: Tulis penerbit peristiwa**

`backend/internal/wa/events.go`:

```go
package wa

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// EventsChannel carries inbox changes to whichever API processes have CS
// browsers attached.
const EventsChannel = "cs:events"

// Event is one inbox change worth waking a browser for.
type Event struct {
	Type           string `json:"type"`
	ConversationID string `json:"conversation_id,omitempty"`
	MessageID      string `json:"message_id,omitempty"`
	AccountStatus  string `json:"account_status,omitempty"`
}

// Event types.
const (
	EventMessage       = "message"
	EventAssignment    = "assignment"
	EventStatus        = "status"
	EventAccountStatus = "account_status"
)

// Publisher announces inbox changes. Redis carries no truth here — it only
// saves the browser from waiting for its next poll, so a failure to publish is
// worth logging and nothing more.
type Publisher struct {
	client *redis.Client
}

// NewPublisher constructs a Publisher.
func NewPublisher(client *redis.Client) *Publisher {
	return &Publisher{client: client}
}

// Publish announces one change.
func (p *Publisher) Publish(ctx context.Context, event Event) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode event: %w", err)
	}
	return p.client.Publish(ctx, EventsChannel, payload).Err()
}
```

- [ ] **Step 4: Tulis client, inbound, media, dan receipts**

`client.go` memegang `*whatsmeow.Client`, memenuhi `Sender`, dan menyediakan pairing lewat `QRChannel`. Sambung ulang memakai jeda yang membesar dari 5 detik hingga maksimum 5 menit — bukan percobaan tanpa henti, karena itu justru memancing pemblokiran. Setiap perubahan koneksi menulis `wa_accounts.status` dan menerbitkan `Event{Type: EventAccountStatus}`.

`inbound.go` menerjemahkan `*events.Message` dari whatsmeow menjadi pemanggilan yang sudah diuji di task sebelumnya, dengan urutan tetap:

```go
// handleMessage turns one WhatsApp event into a stored message. The order
// matters: the thread must exist before the message lands in it, and the
// message must be stored before anyone is told to come and read it.
func (h *InboundHandler) handleMessage(ctx context.Context, evt *events.Message) error {
	if evt.Info.IsGroup || evt.Info.IsFromMe {
		return nil // this inbox answers customers, not groups or its own echo
	}

	conv, err := h.conversations.FindOrCreate(services.IncomingPeer{
		WAAccountID: h.accountID,
		JID:         evt.Info.Sender.ToNonAD().String(),
		Phone:       evt.Info.Sender.User,
		Name:        evt.Info.PushName,
	})
	if err != nil {
		return err
	}

	kind, body, media, err := h.extract(ctx, evt)
	if err != nil {
		return err
	}

	msg, created, err := h.messages.SaveInbound(services.InboundMessage{
		ConversationID: conv.ID,
		WAMessageID:    evt.Info.ID,
		Kind:           kind,
		Body:           body,
		Media:          media,
		At:             evt.Info.Timestamp,
	})
	if err != nil {
		return err
	}
	if !created {
		return nil // WhatsApp re-delivered one it had already given us
	}

	if _, err := h.assignment.AssignOne(ctx, conv.ID); err != nil {
		return err
	}

	return h.publisher.Publish(ctx, Event{
		Type:           EventMessage,
		ConversationID: conv.ID.String(),
		MessageID:      msg.ID.String(),
	})
}
```

`media.go` mengunduh lampiran lewat `client.Download`, menulisnya ke `<root>/<tahun>/<bulan>/<uuid><ext>`, dan mengembalikan `*services.MediaFile`. Kegagalan unduh **bukan** kegagalan pesan: kembalikan `nil` media beserta galat yang dicatat pemanggil, dan simpan pesannya dengan `Body` diberi keterangan `"[media gagal diunduh]"` supaya CS tetap tahu pelanggan mengirim sesuatu.

`receipts.go` menerjemahkan `*events.Receipt` menjadi `messages.ApplyReceipt(id, models.MessageDelivered)` untuk `types.ReceiptTypeDelivered` dan `models.MessageRead` untuk `types.ReceiptTypeRead`, satu panggilan per id di `evt.MessageIDs`.

Rujuk README whatsmeow untuk nama tipe yang tepat pada versi yang dipatok; nama di atas berlaku untuk API v0.0.x yang berjalan sekarang. Kalau sebuah nama berbeda, ikuti yang ada di pustaka dan jangan membungkusnya dengan lapisan tambahan.

- [ ] **Step 5: Tulis `cmd/wa/main.go`**

Perangkaian linear, dikecualikan dari batas 50 baris. Urutannya: muat config → logger → hubungkan database → `models.AutoMigrate` **tidak** dipanggil di sini (API yang memilikinya) → siapkan `sqlstore` whatsmeow di Postgres → buat client → daftarkan handler inbound dan receipt → `Connect` → jalankan tiga gelung berkala dengan `time.Ticker`:

1. **Penguras antrean** setiap `WADrainIntervalSeconds`, sekaligus berlangganan `cs:outbox` di Redis agar balasan terkirim dalam hitungan detik. Gelung berkalanya adalah jaring pengaman: kalau pengumuman Redis hilang, pesan tetap terkirim, hanya terlambat.
2. **Pembagi yang tertinggal** (`assignment.AssignWaiting`) setiap satu menit, yang mengambil chat semalam begitu ada CS membuka inbox.
3. **Penyapu media** sekali sehari.

Tutup dengan penanganan `SIGINT`/`SIGTERM` yang memutus client secara rapi, mengikuti pola `cmd/worker/main.go`.

- [ ] **Step 6: Tulis Dockerfile.wa**

Salin `backend/Dockerfile.worker` dan ubah target build menjadi `./cmd/wa`. Tidak ada `NET_ADMIN` dan tidak ada `iproute2`: proses ini hanya butuh jalan keluar ke internet biasa.

- [ ] **Step 7: Pastikan semuanya terkompilasi dan tes lama tetap hijau**

Run: `cd backend && go build ./... && go vet ./... && gofmt -s -l . && go test ./... -race`
Expected: build sukses, `gofmt -s -l .` tidak mengeluarkan apa pun, seluruh tes lulus.

- [ ] **Step 8: Commit**

```bash
git add backend/internal/wa backend/cmd/wa backend/Dockerfile.wa backend/internal/config/config.go backend/go.mod backend/go.sum
git commit -m "feat(cs): hold the WhatsApp session in a process of its own"
```

---

### Task 11: Endpoint percakapan dan pesan

**Files:**
- Create: `backend/internal/api/cs_dto.go`
- Create: `backend/internal/api/cs_handler_conversations.go`
- Create: `backend/internal/api/cs_handler_messages.go`
- Modify: `backend/internal/api/router.go`
- Test: `backend/internal/api/cs_handler_test.go`

**Interfaces:**
- Consumes: seluruh layanan CS, `middleware.AuthMiddleware`, `middleware.RequireRole`
- Produces: `api.NewCSHandler(...) *CSHandler` dengan method `ListConversations`, `Assign`, `SetStatus`, `LinkONT`, `History`, `Send`, `SendMedia`, `ServeMedia`, `SearchMessages`

- [ ] **Step 1: Tulis tes yang gagal**

`backend/internal/api/cs_handler_test.go` — pakai pembantu yang sudah ada di `test_helpers.go`; baca berkas itu dulu dan ikuti pola `setupONTListHandler` untuk merangkai router uji.

```go
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

// A CS may read the whole inbox — the team seeing each other is what stops two
// of them answering the same customer — but may only send on a thread they hold.
func TestSendIsRefusedOnSomeoneElsesThread(t *testing.T) {
	env := setupCSHandler(t)

	conv := env.conversation(t, "628111@s.whatsapp.net", "628111222333")
	require.NoError(t, env.conversations.Assign(conv.ID, env.otherCS))

	body := strings.NewReader(`{"body":"halo"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cs/conversations/"+conv.ID.String()+"/messages", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.asUser(env.cs, models.UserRoleCS).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Contains(t, rec.Body.String(), "dipegang")
}

func TestSendQueuesAMessageOnMyOwnThread(t *testing.T) {
	env := setupCSHandler(t)

	conv := env.conversation(t, "628111@s.whatsapp.net", "628111222333")
	require.NoError(t, env.conversations.Assign(conv.ID, env.cs))

	body := strings.NewReader(`{"body":"sudah kami cek"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cs/conversations/"+conv.ID.String()+"/messages", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.asUser(env.cs, models.UserRoleCS).ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)

	var payload struct {
		Data struct {
			Status string `json:"status"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	assert.Equal(t, string(models.MessageQueued), payload.Data.Status)
}

// Taking over is the way out when an agent leaves mid-shift, and it must be
// recorded — an assignment that changes with no trace is how blame lands on the
// wrong person.
func TestTakingOverIsAllowedAndAudited(t *testing.T) {
	env := setupCSHandler(t)

	conv := env.conversation(t, "628111@s.whatsapp.net", "628111222333")
	require.NoError(t, env.conversations.Assign(conv.ID, env.otherCS))

	body := strings.NewReader(`{"user_id":"` + env.cs.String() + `"}`)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/cs/conversations/"+conv.ID.String()+"/assign", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.asUser(env.cs, models.UserRoleCS).ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	after, err := env.conversations.Get(conv.ID)
	require.NoError(t, err)
	require.NotNil(t, after.AssignedUserID)
	assert.Equal(t, env.cs, *after.AssignedUserID)

	var logs []models.AuditLog
	require.NoError(t, env.db.Find(&logs).Error)
	assert.NotEmpty(t, logs, "a handover leaves a trail")
}

func TestViewerIsKeptOutOfTheInbox(t *testing.T) {
	env := setupCSHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cs/conversations", nil)
	rec := httptest.NewRecorder()
	env.asUser(uuid.New(), models.UserRoleViewer).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestTechnicianMayReadAndSend(t *testing.T) {
	env := setupCSHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cs/conversations", nil)
	rec := httptest.NewRecorder()
	env.asUser(uuid.New(), models.UserRoleTechnician).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}
```

Tulis pembantu `setupCSHandler(t)` di berkas tes yang sama: membuat SQLite in-memory, satu `WAAccount`, seluruh layanan CS dengan `NewFakePresence()`, dan sebuah `*gin.Engine` yang menyuntikkan `user_id` serta `user_role` ke context lewat middleware uji — persis seperti yang dilakukan `router_middleware_test.go`. Baca berkas itu lebih dulu dan pakai cara yang sama, jangan membuat mekanisme kedua.

- [ ] **Step 2: Jalankan tes, pastikan gagal**

Run: `cd backend && go test ./internal/api/ -run 'TestSend|TestTakingOver|TestViewerIsKeptOut|TestTechnicianMay' -v`
Expected: FAIL — `undefined: setupCSHandler`.

- [ ] **Step 3: Tulis DTO dan handler**

`cs_dto.go` berisi:

```go
// SendMessageRequest is a CS reply on a thread they hold.
type SendMessageRequest struct {
	Body string `json:"body" binding:"required"`
}

// AssignRequest hands a thread to one CS. Sending your own id is taking it over.
type AssignRequest struct {
	UserID uuid.UUID `json:"user_id" binding:"required"`
}

// SetStatusRequest closes or reopens a thread.
type SetStatusRequest struct {
	Status models.ConversationStatus `json:"status" binding:"required"`
}

// LinkONTRequest ties a thread to a subscriber's ONT, or unties it when null.
type LinkONTRequest struct {
	ONTID *uuid.UUID `json:"ont_id"`
}
```

`cs_handler_conversations.go` dan `cs_handler_messages.go` mengikuti pola `distribution_handler.go`: handler tipis, memanggil layanan, memetakan galat. Aturan pemetaan:

| Galat layanan | HTTP |
|---|---|
| `services.ErrValidation` | 400 lewat `badRequest` yang sudah ada |
| `services.ErrNotHolder` | 409 dengan pesan yang menyebut nama pemegangnya |
| `gorm.ErrRecordNotFound` | 404 |
| lainnya | 500 |

`Send` memanggil `conversations.EnsureHolder` sebelum `messages.Queue`, lalu menerbitkan dua hal ke Redis: `Event{Type: EventMessage}` ke `cs:events` agar CS lain langsung melihatnya, dan sebuah pemberitahuan kosong ke `cs:outbox` agar proses `wa` menguras antrean dalam hitungan detik alih-alih menunggu penyapu tiga puluh detik. Kegagalan menerbitkan keduanya dicatat dan tidak menggagalkan permintaan — pesannya sudah tersimpan, dan penyapu tetap akan mengirimnya. `Assign` memanggil `AuditService` yang sudah ada — baca `audit_service.go` untuk tanda tangan tepatnya dan pakai apa adanya.

`ServeMedia` menyajikan berkas dari `cfg.WAMediaDir` dengan `c.File`, setelah memastikan pesan itu ada dan `MediaPath` tidak kosong. Gabungkan jalurnya dengan `filepath.Join(root, path)` lalu **pastikan hasilnya masih berada di bawah root** sebelum membuka berkas; jalur media berasal dari database, dan pemeriksaan ini yang menjaga sebuah baris rusak tidak berubah menjadi pembacaan berkas sembarang.

- [ ] **Step 4: Daftarkan rute**

Di `backend/internal/api/router.go`, setelah grup `odcs`:

```go
		// The CS inbox is read by the whole team on purpose: seeing each other's
		// threads is what keeps two agents off one customer. Only replying is
		// restricted, and that check lives in the handler because it depends on
		// who holds the thread rather than on a role.
		cs := api.Group("/cs")
		cs.Use(middleware.AuthMiddleware(authStore, logger))
		cs.Use(middleware.RequireRole(models.UserRoleAdmin, models.UserRoleCS, models.UserRoleTechnician))
		{
			cs.GET("/conversations", csHandler.ListConversations)
			cs.GET("/conversations/:id/messages", csHandler.History)
			cs.POST("/conversations/:id/messages", csHandler.Send)
			cs.POST("/conversations/:id/media", csHandler.SendMedia)
			cs.PUT("/conversations/:id/assign", csHandler.Assign)
			cs.PUT("/conversations/:id/status", csHandler.SetStatus)
			cs.PUT("/conversations/:id/ont", csHandler.LinkONT)
			cs.GET("/media/:message_id", csHandler.ServeMedia)
			cs.GET("/messages/search", csHandler.SearchMessages)
		}
```

- [ ] **Step 5: Jalankan tes, pastikan lulus**

Run: `cd backend && go test ./internal/api/ -run 'TestSend|TestTakingOver|TestViewerIsKeptOut|TestTechnicianMay' -v`
Expected: PASS kelimanya.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/api/cs_dto.go backend/internal/api/cs_handler_conversations.go backend/internal/api/cs_handler_messages.go backend/internal/api/cs_handler_test.go backend/internal/api/router.go
git commit -m "feat(cs): open the inbox to the team, and the send button to its holder"
```

---

### Task 12: SSE

**Files:**
- Create: `backend/internal/api/cs_handler_stream.go`
- Modify: `backend/internal/api/router.go`
- Test: `backend/internal/api/cs_handler_stream_test.go`

**Interfaces:**
- Consumes: `wa.EventsChannel`, `wa.Event`, `services.Presence`
- Produces: `(*CSHandler).Stream(c *gin.Context)`

- [ ] **Step 1: Tulis tes yang gagal**

```go
// The heartbeat is what keeps a CS in the rotation: presence expires after a
// minute, so a stream that forgets to refresh it quietly drops its own agent
// out of round-robin mid-shift.
func TestStreamMarksItsAgentOnline(t *testing.T) {
	env := setupCSHandler(t)
	agent := uuid.New()

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/cs/stream", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		env.asUser(agent, models.UserRoleCS).ServeHTTP(rec, req)
		close(done)
	}()

	require.Eventually(t, func() bool {
		online, err := env.presence.Online(context.Background())
		return err == nil && len(online) == 1 && online[0] == agent
	}, time.Second, 10*time.Millisecond)

	cancel()
	<-done
}
```

- [ ] **Step 2: Jalankan tes, pastikan gagal**

Run: `cd backend && go test ./internal/api/ -run TestStreamMarks -v`
Expected: FAIL — rute `/cs/stream` belum ada.

- [ ] **Step 3: Tulis implementasi**

`cs_handler_stream.go`:

```go
// heartbeatInterval is well inside the sixty-second presence TTL, so one slow
// moment on the network does not drop a CS out of the rotation.
const heartbeatInterval = 15 * time.Second

// Stream keeps one CS browser up to date. It carries no truth of its own: every
// event is a nudge to refetch, which is why a dropped connection costs nothing
// but a moment of staleness.
//
// Holding the connection open is also what marks this agent online, so the
// rotation only ever hands work to somebody with the inbox actually open.
func (h *CSHandler) Stream(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthenticated", Code: "UNAUTHENTICATED"})
		return
	}

	ctx := c.Request.Context()
	if err := h.presence.MarkOnline(ctx, userID); err != nil {
		h.logger.Warn("mark CS online", zap.Error(err))
	}

	sub := h.redis.Subscribe(ctx, wa.EventsChannel)
	defer func() { _ = sub.Close() }()
	incoming := sub.Channel()

	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	c.Stream(func(w io.Writer) bool {
		select {
		case <-ctx.Done():
			return false
		case msg, open := <-incoming:
			if !open {
				return false
			}
			c.SSEvent("cs", msg.Payload)
			return true
		case <-ticker.C:
			if err := h.presence.MarkOnline(ctx, userID); err != nil {
				h.logger.Warn("refresh CS presence", zap.Error(err))
			}
			c.SSEvent("ping", "")
			return true
		}
	})
}
```

Daftarkan di grup `cs`: `cs.GET("/stream", csHandler.Stream)`.

- [ ] **Step 4: Jalankan tes, pastikan lulus**

Run: `cd backend && go test ./internal/api/ -run TestStreamMarks -v -race`
Expected: PASS, tanpa peringatan race.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/api/cs_handler_stream.go backend/internal/api/cs_handler_stream_test.go backend/internal/api/router.go
git commit -m "feat(cs): keep the inbox live, and count an open stream as an agent present"
```

---

### Task 13: Akun WhatsApp dan balasan cepat

**Files:**
- Create: `backend/internal/api/cs_handler_wa.go`
- Create: `backend/internal/api/cs_handler_quick_replies.go`
- Modify: `backend/internal/api/router.go`
- Test: `backend/internal/api/cs_handler_wa_test.go`

**Interfaces:**
- Produces: `(*CSHandler).ListAccounts`, `Connect`, `Disconnect`, `ListQuickReplies`, `CreateQuickReply`, `UpdateQuickReply`, `DeleteQuickReply`

- [ ] **Step 1: Tulis tes yang gagal**

```go
// Pairing a number, and cutting it off, are admin decisions: a CS doing either
// by accident takes the whole team off WhatsApp.
func TestOnlyAdminMayConnectANumber(t *testing.T) {
	env := setupCSHandler(t)

	for role, want := range map[models.UserRole]int{
		models.UserRoleCS:         http.StatusForbidden,
		models.UserRoleTechnician: http.StatusForbidden,
		models.UserRoleViewer:     http.StatusForbidden,
	} {
		req := httptest.NewRequest(http.MethodPost,
			"/api/v1/cs/wa-accounts/"+env.account.ID.String()+"/connect", nil)
		rec := httptest.NewRecorder()
		env.asUser(uuid.New(), role).ServeHTTP(rec, req)
		assert.Equal(t, want, rec.Code, string(role))
	}
}

func TestOnlyAdminMayChangeQuickReplies(t *testing.T) {
	env := setupCSHandler(t)

	body := strings.NewReader(`{"title":"Cek LOS","body":"Mohon cek lampu LOS."}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cs/quick-replies", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.asUser(uuid.New(), models.UserRoleCS).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// Reading them is not an admin matter: a CS who cannot read the templates
// cannot use them.
func TestAnyoneInTheInboxMayReadQuickReplies(t *testing.T) {
	env := setupCSHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cs/quick-replies", nil)
	rec := httptest.NewRecorder()
	env.asUser(uuid.New(), models.UserRoleCS).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}
```

- [ ] **Step 2: Jalankan tes, pastikan gagal**

Run: `cd backend && go test ./internal/api/ -run 'TestOnlyAdminMay|TestAnyoneInTheInbox' -v`
Expected: FAIL — rute belum ada.

- [ ] **Step 3: Tulis handler dan rute**

`Connect` menandai akun `pairing` lalu menerbitkan permintaan pairing ke proses `wa` lewat Redis channel `cs:control`; QR-nya kembali ke browser sebagai `Event{Type: EventAccountStatus}` di stream SSE yang sama. API tidak pernah memegang koneksi WhatsApp sendiri — itulah alasan proses `wa` dipisahkan sejak awal.

Rute:

```go
			cs.GET("/quick-replies", csHandler.ListQuickReplies)
			cs.POST("/quick-replies", middleware.RequireRole(models.UserRoleAdmin), csHandler.CreateQuickReply)
			cs.PUT("/quick-replies/:id", middleware.RequireRole(models.UserRoleAdmin), csHandler.UpdateQuickReply)
			cs.DELETE("/quick-replies/:id", middleware.RequireRole(models.UserRoleAdmin), csHandler.DeleteQuickReply)

			cs.GET("/wa-accounts", middleware.RequireRole(models.UserRoleAdmin), csHandler.ListAccounts)
			cs.POST("/wa-accounts/:id/connect", middleware.RequireRole(models.UserRoleAdmin), csHandler.Connect)
			cs.POST("/wa-accounts/:id/disconnect", middleware.RequireRole(models.UserRoleAdmin), csHandler.Disconnect)
```

- [ ] **Step 4: Jalankan tes, pastikan lulus**

Run: `cd backend && go test ./internal/api/... -race`
Expected: PASS semuanya.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/api/cs_handler_wa.go backend/internal/api/cs_handler_quick_replies.go backend/internal/api/cs_handler_wa_test.go backend/internal/api/router.go
git commit -m "feat(cs): pair a number and keep canned answers, both behind admin"
```

---

### Task 14: Nomor HP pada ONT

Prasyarat panel pelanggan. Tanpa ini tidak ada satu pun chat yang bisa dikaitkan ke ONT.

**Files:**
- Modify: `backend/internal/api/ont_dto.go`
- Modify: `backend/internal/services/ont_service.go`
- Modify: `frontend/src/domain/entities/Ont.ts`
- Modify: form ONT di `frontend/src/presentation/components/onts/`
- Test: `backend/internal/services/ont_service_phone_test.go`

**Interfaces:**
- Consumes: `utils.NormalizePhone`
- Produces: field `phone` pada DTO buat/ubah ONT dan pada entitas `Ont` di frontend

- [ ] **Step 1: Tulis tes yang gagal**

```go
func TestONTPhoneIsStoredInOneForm(t *testing.T) {
	db := setupTestDB(t)
	svc := NewONTService(db)

	ont, err := svc.Create(/* argumen sesuai tanda tangan Create yang ada */)
	require.NoError(t, err)
	assert.Equal(t, "6281234567890", ont.Phone, "however the technician typed it")
}

// Two ONTs claiming one number would send the CS to the wrong house. The
// partial unique index enforces it on Postgres; this check is what makes the
// rule hold in the SQLite tests and gives the operator a sentence rather than a
// constraint name.
func TestONTPhoneCannotBeClaimedTwice(t *testing.T) {
	db := setupTestDB(t)
	svc := NewONTService(db)

	_, err := svc.Create(/* nomor 081234567890 */)
	require.NoError(t, err)

	_, err = svc.Create(/* ONT lain, nomor +6281234567890 */)
	assert.ErrorIs(t, err, ErrValidation)
}

// Most ONTs have no number recorded, and empty is not a value that can collide.
func TestManyONTsMayHaveNoPhoneAtAll(t *testing.T) {
	db := setupTestDB(t)
	svc := NewONTService(db)

	for i := 0; i < 2; i++ {
		_, err := svc.Create(/* tanpa nomor */)
		require.NoError(t, err)
	}
}
```

Lengkapi argumen sesuai tanda tangan `ONTService.Create` yang ada; baca `ont_service.go` lebih dulu.

- [ ] **Step 2: Jalankan tes, pastikan gagal**

Run: `cd backend && go test ./internal/services/ -run TestONTPhone -v`
Expected: FAIL — nomor belum dinormalisasi dan duplikat belum ditolak.

- [ ] **Step 3: Tulis implementasi**

Di `ONTService`, normalkan nomor lewat `utils.NormalizePhone` saat tidak kosong, dan tolak nomor yang sudah dipakai ONT lain dengan `ErrValidation`. Kosongkan menjadi `""` bila operator menghapusnya.

- [ ] **Step 4: Jalankan tes, pastikan lulus**

Run: `cd backend && go test ./internal/services/ -run TestONTPhone -v && go test ./internal/api/... -race`
Expected: PASS.

- [ ] **Step 5: Tambahkan isian di frontend**

Tambah `phone?: string` pada entitas `Ont`, dan satu `Form.Item` bernama `phone` pada form ONT dengan label "Nomor HP Pelanggan" dan placeholder `08xxxxxxxxxx`. Ikuti komponen form yang sudah ada di berkas itu.

- [ ] **Step 6: Jalankan tes frontend**

Run: `cd frontend && npm test -- --run && npm run build`
Expected: PASS dan build sukses.

- [ ] **Step 7: Commit**

```bash
git add backend/internal frontend/src
git commit -m "feat(cs): record the number that ties a chat to a subscriber's ONT"
```

---

### Task 15: Lapisan data frontend

**Files:**
- Create: `frontend/src/domain/entities/CsConversation.ts`
- Create: `frontend/src/domain/entities/CsMessage.ts`
- Create: `frontend/src/domain/repositories/ICsRepository.ts`
- Create: `frontend/src/infrastructure/repositories/CsRepository.ts`
- Create: `frontend/src/application/hooks/useCsInbox.ts`
- Create: `frontend/src/application/hooks/useCsStream.ts`
- Create: `frontend/src/application/hooks/useCsQuickReplies.ts`
- Modify: `frontend/src/infrastructure/http/endpoints.ts`
- Modify: `frontend/src/domain/entities/index.ts`, `frontend/src/application/hooks/index.ts`
- Test: `frontend/src/application/__tests__/useCsStream.test.ts`

**Interfaces:**
- Produces:
  - `type CsConversation = { id: string; customerPhone: string; customerName: string; assignedUserId?: string; status: "unassigned" | "open" | "closed"; ontId?: string; lastMessageAt: string; unreadCount: number }`
  - `type CsMessage = { id: string; conversationId: string; direction: "in" | "out"; senderUserId?: string; kind: "text" | "image" | "document" | "audio" | "video"; body: string; mediaMime?: string; mediaFilename?: string; status: "queued" | "sent" | "delivered" | "read" | "failed"; failReason?: string; waTimestamp: string }`
  - `useCsConversations(filter)`, `useCsHistory(conversationId)`, `useSendCsMessage()`, `useAssignConversation()`, `useSetConversationStatus()`, `useLinkConversationOnt()`, `useCsStream()`, `useCsQuickReplies()`

- [ ] **Step 1: Tulis tes yang gagal**

`frontend/src/application/__tests__/useCsStream.test.ts`:

```ts
import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";
import { renderHook } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import React from "react";
import { useCsStream } from "../hooks/useCsStream";

class FakeEventSource {
  static instances: FakeEventSource[] = [];
  listeners: Record<string, ((e: MessageEvent) => void)[]> = {};
  closed = false;

  constructor(public url: string) {
    FakeEventSource.instances.push(this);
  }

  addEventListener(type: string, fn: (e: MessageEvent) => void) {
    (this.listeners[type] ||= []).push(fn);
  }

  emit(type: string, data: string) {
    for (const fn of this.listeners[type] ?? []) {
      fn(new MessageEvent(type, { data }));
    }
  }

  close() {
    this.closed = true;
  }
}

function wrapper(client: QueryClient) {
  return ({ children }: { children: React.ReactNode }) =>
    React.createElement(QueryClientProvider, { client }, children);
}

describe("useCsStream", () => {
  beforeEach(() => {
    FakeEventSource.instances = [];
    vi.stubGlobal("EventSource", FakeEventSource);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  // The stream carries no truth of its own: an event is a nudge to refetch.
  // Trusting its payload instead would leave the inbox wrong for any message
  // that arrived while the connection was down.
  it("refetches the inbox when a message event arrives", () => {
    const client = new QueryClient();
    const invalidate = vi.spyOn(client, "invalidateQueries");

    renderHook(() => useCsStream(), { wrapper: wrapper(client) });

    const source = FakeEventSource.instances[0];
    expect(source).toBeDefined();

    source.emit("cs", JSON.stringify({ type: "message", conversation_id: "abc" }));

    expect(invalidate).toHaveBeenCalledWith(
      expect.objectContaining({ queryKey: ["cs", "conversations"] }),
    );
    expect(invalidate).toHaveBeenCalledWith(
      expect.objectContaining({ queryKey: ["cs", "messages", "abc"] }),
    );
  });

  it("closes the connection when the page goes away", () => {
    const client = new QueryClient();
    const { unmount } = renderHook(() => useCsStream(), { wrapper: wrapper(client) });

    unmount();

    expect(FakeEventSource.instances[0].closed).toBe(true);
  });
});
```

- [ ] **Step 2: Jalankan tes, pastikan gagal**

Run: `cd frontend && npm test -- --run useCsStream`
Expected: FAIL — modul `useCsStream` belum ada.

- [ ] **Step 3: Tulis endpoint, entitas, repository, dan hook**

Tambahkan ke `endpoints.ts`:

```ts
  // CS inbox
  CS_STREAM: "/api/v1/cs/stream",
  CS_CONVERSATIONS: "/api/v1/cs/conversations",
  CS_MESSAGES: (id: string) => `/api/v1/cs/conversations/${id}/messages`,
  CS_MEDIA_UPLOAD: (id: string) => `/api/v1/cs/conversations/${id}/media`,
  CS_ASSIGN: (id: string) => `/api/v1/cs/conversations/${id}/assign`,
  CS_STATUS: (id: string) => `/api/v1/cs/conversations/${id}/status`,
  CS_LINK_ONT: (id: string) => `/api/v1/cs/conversations/${id}/ont`,
  CS_MEDIA: (messageId: string) => `/api/v1/cs/media/${messageId}`,
  CS_QUICK_REPLIES: "/api/v1/cs/quick-replies",
  CS_QUICK_REPLY_BY_ID: (id: string) => `/api/v1/cs/quick-replies/${id}`,
  CS_WA_ACCOUNTS: "/api/v1/cs/wa-accounts",
  CS_WA_CONNECT: (id: string) => `/api/v1/cs/wa-accounts/${id}/connect`,
  CS_WA_DISCONNECT: (id: string) => `/api/v1/cs/wa-accounts/${id}/disconnect`,
```

`useCsStream.ts`:

```ts
import { useEffect } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { API_ENDPOINTS } from "@/infrastructure/http/endpoints";
import { env } from "@/shared/config/env";

type CsEvent = {
  type: string;
  conversation_id?: string;
};

/**
 * Keeps the inbox current while the page is open, and marks this agent online
 * for as long as the connection lasts — round-robin only ever hands work to
 * somebody who actually has the inbox in front of them.
 *
 * Events carry no data of their own. Each one is a nudge to refetch, so a
 * connection that dropped and came back cannot leave the inbox showing a stale
 * thread: the refetch closes whatever gap the outage opened.
 */
export function useCsStream() {
  const queryClient = useQueryClient();

  useEffect(() => {
    const source = new EventSource(`${env.apiUrl}${API_ENDPOINTS.CS_STREAM}`, {
      withCredentials: true,
    });

    const onEvent = (event: MessageEvent) => {
      let payload: CsEvent;
      try {
        payload = JSON.parse(event.data);
      } catch {
        return;
      }

      queryClient.invalidateQueries({ queryKey: ["cs", "conversations"] });
      if (payload.conversation_id) {
        queryClient.invalidateQueries({
          queryKey: ["cs", "messages", payload.conversation_id],
        });
      }
    };

    source.addEventListener("cs", onEvent);
    return () => {
      source.removeEventListener("cs", onEvent);
      source.close();
    };
  }, [queryClient]);
}
```

`CsRepository.ts` dan `useCsInbox.ts` mengikuti persis pola `SiteRepository.ts` dan `useSites.ts`: kelas repository memanggil `apiClient`, hook membungkusnya dengan `useQuery`/`useMutation` dan membatalkan `["cs", …]` pada keberhasilan.

- [ ] **Step 4: Jalankan tes, pastikan lulus**

Run: `cd frontend && npm test -- --run useCsStream`
Expected: PASS keduanya.

- [ ] **Step 5: Commit**

```bash
git add frontend/src
git commit -m "feat(cs): wire the inbox to the API, and treat a stream event as a nudge"
```

---

### Task 16: Halaman inbox

**Files:**
- Create: `frontend/src/presentation/pages/CsInboxPage.tsx`
- Create: `frontend/src/presentation/components/cs/ConversationList.tsx`
- Create: `frontend/src/presentation/components/cs/MessageThread.tsx`
- Create: `frontend/src/presentation/components/cs/MessageComposer.tsx`
- Create: `frontend/src/presentation/components/cs/CustomerPanel.tsx`
- Create: `frontend/src/presentation/components/cs/QuickReplyPicker.tsx`
- Create: `frontend/src/presentation/components/cs/WaConnectionBadge.tsx`
- Create: `frontend/src/presentation/components/cs/WaPairingModal.tsx`
- Modify: `frontend/src/presentation/routes/index.tsx`
- Modify: `frontend/src/presentation/components/layout/navigationRoutes.tsx`
- Test: `frontend/src/presentation/components/cs/__tests__/MessageComposer.test.tsx`
- Test: `frontend/src/presentation/components/cs/__tests__/ConversationList.test.tsx`

**Interfaces:**
- Consumes: hook dari Task 15
- Produces: rute `/cs`, entri navigasi "CS Inbox"

- [ ] **Step 1: Tulis tes yang gagal**

`MessageComposer.test.tsx`:

```tsx
import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { MessageComposer } from "../MessageComposer";

const conversation = {
  id: "c1",
  customerPhone: "628111222333",
  customerName: "Budi",
  status: "open" as const,
  lastMessageAt: new Date().toISOString(),
  unreadCount: 0,
};

describe("MessageComposer", () => {
  // A greyed-out button with no explanation reads as a broken page. The CS
  // needs to know it is held by someone, and that taking over is the way in.
  it("says who holds the thread instead of just disabling the button", () => {
    render(
      <MessageComposer
        conversation={{ ...conversation, assignedUserId: "someone-else" }}
        currentUserId="me"
        holderName="Budi CS"
        onSend={vi.fn()}
        onTakeOver={vi.fn()}
      />,
    );

    expect(screen.getByText(/Dipegang Budi CS/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /ambil alih/i })).toBeEnabled();
    expect(screen.queryByRole("button", { name: /^kirim$/i })).toBeNull();
  });

  it("lets the holder send", () => {
    render(
      <MessageComposer
        conversation={{ ...conversation, assignedUserId: "me" }}
        currentUserId="me"
        holderName="Saya"
        onSend={vi.fn()}
        onTakeOver={vi.fn()}
      />,
    );

    expect(screen.getByRole("button", { name: /^kirim$/i })).toBeInTheDocument();
  });
});
```

`ConversationList.test.tsx`:

```tsx
import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { ConversationList } from "../ConversationList";

const rows = [
  {
    id: "c1",
    customerPhone: "628111222333",
    customerName: "Budi",
    status: "unassigned" as const,
    lastMessageAt: new Date().toISOString(),
    unreadCount: 2,
  },
  {
    id: "c2",
    customerPhone: "628222333444",
    customerName: "Siti",
    assignedUserId: "me",
    status: "open" as const,
    lastMessageAt: new Date().toISOString(),
    unreadCount: 0,
  },
];

describe("ConversationList", () => {
  it("marks a thread nobody holds", () => {
    render(
      <ConversationList
        conversations={rows}
        selectedId="c1"
        holderNames={{ me: "Saya" }}
        currentUserId="me"
        onSelect={vi.fn()}
      />,
    );

    expect(screen.getByText(/belum dipegang/i)).toBeInTheDocument();
    expect(screen.getByText("Saya")).toBeInTheDocument();
  });

  it("shows how many messages are unread", () => {
    render(
      <ConversationList
        conversations={rows}
        selectedId="c1"
        holderNames={{ me: "Saya" }}
        currentUserId="me"
        onSelect={vi.fn()}
      />,
    );

    expect(screen.getByText("2")).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Jalankan tes, pastikan gagal**

Run: `cd frontend && npm test -- --run cs/`
Expected: FAIL — komponennya belum ada.

- [ ] **Step 3: Tulis komponen**

`MessageComposer.tsx` memegang aturan yang diuji di atas:

```tsx
const isHolder = conversation.assignedUserId === currentUserId;

if (!isHolder) {
  return (
    <Space direction="vertical" style={{ width: "100%" }}>
      <Alert
        type="info"
        showIcon
        message={
          conversation.assignedUserId
            ? `Dipegang ${holderName} — ambil alih dulu untuk membalas`
            : "Belum dipegang siapa pun — ambil alih untuk membalas"
        }
      />
      <Button onClick={onTakeOver}>Ambil alih</Button>
    </Space>
  );
}
```

`ConversationList.tsx` menampilkan `Belum dipegang` untuk `status === "unassigned"`, nama pemegang untuk sisanya, dan `Badge count={unreadCount}`.

`MessageThread.tsx` menggambar gelembung berdasarkan `direction`, menampilkan `failReason` beserta tombol kirim ulang pada `status === "failed"`, dan menyajikan gambar dari `API_ENDPOINTS.CS_MEDIA(message.id)`.

`CustomerPanel.tsx` menampilkan nama, nomor, dan — bila `ontId` terisi — status ONT lewat hook ONT yang sudah ada, dengan tautan ke halaman ONT. Bila kosong, tampilkan pencarian ONT yang memanggil `useLinkConversationOnt`.

`WaConnectionBadge.tsx` menampilkan lencana merah mencolok di kepala halaman ketika `status !== "connected"`, karena CS harus langsung tahu bahwa pesannya tidak akan sampai.

`CsInboxPage.tsx` merangkai tiga kolom, memanggil `useCsStream()` sekali, dan menyimpan `selectedId` di state.

- [ ] **Step 4: Daftarkan rute dan navigasi**

Tambah `{ path: "cs", element: <CsInboxPage /> }` di `routes/index.tsx`, dan `{ path: "/cs", name: "CS Inbox", icon: <MessageOutlined /> }` di `navigationRoutes.tsx`.

- [ ] **Step 5: Jalankan tes, pastikan lulus**

Run: `cd frontend && npm test -- --run && npm run lint && npm run format:check && npm run build`
Expected: PASS, tanpa galat lint, format bersih, build sukses.

- [ ] **Step 6: Commit**

```bash
git add frontend/src
git commit -m "feat(cs): give the team one inbox, and say why the send button is closed"
```

---

### Task 17: Deployment dan verifikasi akhir

**Files:**
- Modify: `docker-compose.yml`
- Modify: `docker-compose.dev.yml`
- Modify: `.github/workflows/ci.yml`
- Modify: `.env.example`

- [ ] **Step 1: Tambahkan service `wa`**

Di `docker-compose.yml`:

```yaml
  wa:
    build:
      context: ./backend
      dockerfile: Dockerfile.wa
    container_name: tikman-wa
    # Deliberately not network_mode: service:api. That namespace carries wg0,
    # and restarting api would take the WhatsApp session down with it — repeated
    # reconnects are exactly the pattern that gets an unofficial number flagged.
    #
    # Deliberately not scalable either: two processes on one WhatsApp session is
    # the fastest way to lose the number.
    environment:
      - DB_HOST=postgres
      - DB_PORT=5432
      - DB_USER=tikman
      - DB_PASSWORD=${POSTGRES_PASSWORD}
      - DB_NAME=tikman
      - REDIS_HOST=redis
      - REDIS_PORT=6379
      - REDIS_PASSWORD=${REDIS_PASSWORD}
      - ENCRYPTION_KEY=${ENCRYPTION_KEY}
      - LOG_LEVEL=info
      - WA_MEDIA_DIR=/data/cs-media
      - WA_SEND_INTERVAL_MS=${WA_SEND_INTERVAL_MS:-1200}
      - WA_MEDIA_RETENTION_DAYS=${WA_MEDIA_RETENTION_DAYS:-90}
    volumes:
      - cs_media:/data/cs-media
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
    networks:
      - tikman-network
    restart: unless-stopped
```

Tambahkan `cs_media:` ke blok `volumes:` di bawah, dan pasang volume yang sama ke service `api` (mode baca) agar `ServeMedia` bisa membaca berkasnya.

- [ ] **Step 2: Tambahkan variabel ke `.env.example`**

```
# CS WhatsApp module
WA_SEND_INTERVAL_MS=1200
WA_MEDIA_RETENTION_DAYS=90
```

- [ ] **Step 3: Tambahkan build `wa` ke CI**

Di `.github/workflows/ci.yml`, di tempat binary `api` dan `worker` dibangun, tambahkan `go build -o wa ./cmd/wa`, dan tambahkan image `wa` ke langkah push GHCR mengikuti pola `worker`.

- [ ] **Step 4: Jalankan seluruh gerbang**

Run:
```bash
cd backend && go test ./... -race && go vet ./... && gofmt -s -l . && go mod verify
cd ../frontend && npm test -- --run && npm run lint && npm run format:check && npm run build
```
Expected: semua lulus, `gofmt -s -l .` kosong.

- [ ] **Step 5: Jalankan tumpukan dan pasangkan nomor sungguhan**

Run:
```bash
docker compose up -d --build
curl -s http://localhost:8080/health
```
Lalu di browser: masuk sebagai admin → Pengaturan → WhatsApp → Sambungkan → pindai QR dari HP pemegang nomor CS. Kirim satu pesan dari nomor lain dan pastikan ia muncul di `/cs` tanpa memuat ulang halaman, dan bahwa balasan sampai ke HP penguji.

Ini satu-satunya langkah yang tidak bisa dibuktikan oleh tes otomatis mana pun, dan tanpa langkah ini modul ini belum terbukti bekerja.

- [ ] **Step 6: Perbarui graph dan commit**

```bash
graphify update .
git add docker-compose.yml docker-compose.dev.yml .github/workflows/ci.yml .env.example
git commit -m "feat(cs): give the WhatsApp session a container that outlives a deploy"
```

---

## Catatan untuk pelaksana

**Yang gampang salah:**

1. **`WAMessageID` adalah pointer.** Kolom biasa bertipe string akan membuat pesan keluar kedua bertabrakan dengan yang pertama pada indeks unik, karena keduanya belum punya id WhatsApp.
2. **Migrasi berjalan setelah `AutoMigrate`.** Jangan `CREATE TABLE` di `41_add_cs_module.sql`; tabelnya sudah ada dari tag model.
3. **Kolom `tsv` tidak boleh menjadi field model.** Ia kolom terkalkulasi milik Postgres; menaruhnya di struct akan membuat `AutoMigrate` mencoba membuatnya di SQLite dan seluruh tes layanan gagal.
4. **Proses `wa` tidak boleh di-scale.** Dua proses pada satu sesi WhatsApp adalah cara tercepat kehilangan nomor.
5. **SSE bukan sumber kebenaran.** Setiap peristiwa hanya pemicu `invalidateQueries`; jangan menaruh isi pesan di dalamnya.
6. **`ErrValidation` dan `AuditService` sudah ada.** Pakai yang ada; jangan membuat versi kedua.
