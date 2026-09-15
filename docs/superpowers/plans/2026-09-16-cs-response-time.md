# Waktu Balas CS — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Setiap giliran pelanggan menunggu balasan dicatat saat kejadian di tabel `cs_waits`, lalu ditampilkan sebagai laporan "Kinerja CS": angka tim, angka per CS (dihitung sejak CS itu mulai aktif), angka per hari, dan daftar giliran yang bisa dibuka sampai ke thread-nya.

**Architecture:** Pencatatan menumpang di transaksi yang sudah ada (`SaveInbound`, `SaveFromPhone`, `Queue`, `Close`) di bawah savepoint, lewat fungsi-fungsi di `internal/services/cs_wait.go`. Kegagalan pencatatan dicatat oleh logger yang dipasang dengan `CSConversationService.SetWaitLogger` dan tidak pernah menggagalkan penyimpanan pesan. Laporan dihitung saat dibuka oleh `CSPerformanceService` (query) dan fungsi murni di `cs_performance_calc.go`, disajikan oleh `CSPerformanceHandler` di grup rute `/api/v1/cs`, dan ditampilkan halaman React `CsPerformancePage` di `/cs-performance`.

**Tech Stack:** Go 1.25 (GORM, gin, zap, testify; SQLite untuk tes unit, Postgres/TimescaleDB untuk tes `OnPostgres`), React 18 + TypeScript (Ant Design 5, recharts 3, @tanstack/react-query 5, Vitest 4, Testing Library).

**Spec:** `docs/superpowers/specs/2026-09-15-cs-response-time-design.md`

## Global Constraints

- Fakta dicatat saat kejadian di `cs_waits`. Target 15 menit, ambang sesi 3 jam, dan ambang tertunda sistem 30 menit diterapkan saat laporan dibuka. Aturan diam 24 jam diterapkan saat pencatatan.
- `cs_waits` tanpa foreign key. Menghapus pesan, mengosongkan thread, atau menghapus nomor tidak menyentuh baris giliran.
- Pencatatan giliran berjalan di bawah savepoint transaksi yang sudah ada. Kegagalannya dicatat log, dan pesan, balasan, atau penutupan thread tetap tersimpan.
- WIB memakai variabel `wib` yang sudah ada (`time.FixedZone("WIB", 7*60*60)`, `backend/internal/services/worker_alert.go`). Image tidak memuat tzdata: jangan pakai `time.LoadLocation`.
- Median dan persentil 90 memakai interpolasi linear seperti `percentile_cont`. Tidak ada rata-rata.
- Admin melihat semua CS. CS dan teknisi melihat angka tim dan barisnya sendiri. Viewer mendapat 403 dari grup rute `/api/v1/cs` yang sudah ada.
- `from`/`to` adalah tanggal WIB `YYYY-MM-DD` inklusif, rentang paling panjang 366 hari. Giliran masuk periode menurut `ended_at`.
- Halaman di `/cs-performance`, bukan `/cs/performance`: ProLayout mencocokkan awalan path sehingga "CS Inbox" ikut tersorot. API: `GET /api/v1/cs/performance/summary` dan `GET /api/v1/cs/performance/waits`.
- Daftar giliran memakai `paginationParams` yang sudah ada (`backend/internal/api/provision_handler.go`: default 20, maksimum 100).
- Tidak ada dependensi baru.
- Frontend menulis query param dalam snake_case secara manual; `apiClient` hanya mengubah body, bukan `params`.
- Semua waktu di fixture tes SQLite memakai UTC: SQLite membandingkan waktu sebagai teks, jadi zona yang berbeda merusak perbandingan.
- File maksimal 350 baris, fungsi maksimal 50 baris, nesting maksimal 3 (CLAUDE.md). Komentar menjelaskan kenapa. Kode dan nama tes dalam bahasa Inggris; teks UI dalam bahasa Indonesia.
- `gofmt -s -l .` kosong, `go vet ./...` bersih, `npm run lint`, `npm run format:check`, dan `npm run build` bersih. Semua tes lama lulus, kecuali 30 tes WireGuard yang sudah gagal di Mac ini karena antarmuka `ppp0` memegang 10.0.0.0/8 (TestCreatePeer*, TestDeletePeer*, TestDisabledPeerIsNotAppliedToDevice, TestGetPeerConfig*, TestListPeersMarksConnectionState, TestPeerConfig*, TestReachability*, TestReconcile*, TestRecoveryReconcileFailureDoesNotMaskTheOriginalError, TestRefreshStatusStoresHandshake, TestTestReachability*, TestUpdatePeer*).
- Tes `OnPostgres` dan `internal/database` butuh `TEST_POSTGRES_DSN`. Di Mac ini jalankan kontainer sementara:
  `docker run -d --rm --name tikman-test-pg -e POSTGRES_PASSWORD=test -e POSTGRES_DB=tikman_test -p 5439:5432 timescale/timescaledb:latest-pg15`, tunggu `docker exec tikman-test-pg sh -c 'until pg_isready -h 127.0.0.1 -U postgres -q; do sleep 1; done'`, pakai `TEST_POSTGRES_DSN="host=localhost port=5439 user=postgres password=test dbname=tikman_test sslmode=disable"`, lalu `docker stop tikman-test-pg`.
- Kerja di branch `feat/cs-response-time`. Merge, push, dan deploy hanya dengan persetujuan eksplisit user.
- Setiap commit diakhiri baris `Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>`.

---

### Task 1: Tabel `cs_waits`

**Files:**
- Create: `backend/internal/models/cs_wait.go`
- Modify: `backend/internal/models/models.go` (daftar `AutoMigrate`)
- Create: `backend/migrations/52_cs_waits.sql`
- Test: `backend/internal/database/migrations_cs_waits_postgres_test.go`

**Interfaces:**
- Consumes: `freshPostgres(t *testing.T) *gorm.DB` dari `backend/internal/database/migrations_postgres_test.go` (package `database`; membangun skema dengan AutoMigrate lalu migrasi SQL, sekali per run).
- Produces:
  - `type models.WaitEndReason string` dengan konstanta `models.WaitReplied` (`"replied"`), `models.WaitPhone` (`"phone"`), `models.WaitClosed` (`"closed"`), `models.WaitAbandoned` (`"abandoned"`).
  - `type models.CSWait struct` dengan field `ID uuid.UUID`, `ConversationID uuid.UUID`, `WAAccountID uuid.UUID`, `StartedAt time.Time`, `CustomerSentAt time.Time`, `LastCustomerSentAt time.Time`, `EndedAt *time.Time`, `EndReason *models.WaitEndReason`, `EndedBy *uuid.UUID`, `ReplyMessageID *uuid.UUID`, `CreatedAt time.Time`; tabel `cs_waits`.
  - Constraint Postgres: indeks unik parsial `uq_cs_waits_one_open_per_thread`, CHECK `cs_waits_end_reason_valid`, `cs_waits_ended_together`, `cs_waits_actor_matches_reason`.

- [ ] **Step 0: Siapkan branch**

```bash
cd /Users/rohadimraja/Documents/tikman
git switch -c feat/cs-response-time
git add docs/superpowers/plans/2026-09-16-cs-response-time.md
git commit -m "$(cat <<'EOF'
docs(cs): plan how CS wait times are recorded and reported

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
EOF
)"
```

- [ ] **Step 1: Tulis tes constraint yang gagal**

Buat `backend/internal/database/migrations_cs_waits_postgres_test.go`:

```go
package database

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

// openThreadWait is a wait still open, on a thread of its own: these tests
// share one schema (see freshPostgres), so no two may use one conversation id.
func openThreadWait(conversationID uuid.UUID) *models.CSWait {
	now := time.Now()
	return &models.CSWait{
		ConversationID: conversationID, WAAccountID: uuid.New(),
		StartedAt: now, CustomerSentAt: now, LastCustomerSentAt: now,
	}
}

// endedAs marks a wait ended for a reason, by a CS, with a message.
func endedAs(w *models.CSWait, reason models.WaitEndReason, by, message *uuid.UUID) *models.CSWait {
	at := time.Now()
	w.EndedAt, w.EndReason, w.EndedBy, w.ReplyMessageID = &at, &reason, by, message
	return w
}

// A thread waits on one thing at a time. The recording never opens a second
// wait, and the database refuses one regardless: a second open wait would be
// answered twice and counted twice.
func TestDatabaseRefusesASecondOpenWaitOnOneThread(t *testing.T) {
	db := freshPostgres(t)
	thread := uuid.New()
	require.NoError(t, db.Create(openThreadWait(thread)).Error)

	err := db.Create(openThreadWait(thread)).Error

	require.Error(t, err)
	assert.Contains(t, err.Error(), "uq_cs_waits_one_open_per_thread")
}

func TestDatabaseAllowsANewWaitOnceTheLastOneEnded(t *testing.T) {
	db := freshPostgres(t)
	thread := uuid.New()
	require.NoError(t, db.Create(endedAs(openThreadWait(thread), models.WaitAbandoned, nil, nil)).Error)

	assert.NoError(t, db.Create(openThreadWait(thread)).Error)
}

func TestDatabaseRefusesAReplyThatNamesNoCS(t *testing.T) {
	db := freshPostgres(t)
	message := uuid.New()

	err := db.Create(endedAs(openThreadWait(uuid.New()), models.WaitReplied, nil, &message)).Error

	require.Error(t, err)
	assert.Contains(t, err.Error(), "cs_waits_actor_matches_reason")
}

func TestDatabaseRefusesAPhoneAnswerThatNamesACS(t *testing.T) {
	db := freshPostgres(t)
	someone, message := uuid.New(), uuid.New()

	err := db.Create(endedAs(openThreadWait(uuid.New()), models.WaitPhone, &someone, &message)).Error

	require.Error(t, err)
	assert.Contains(t, err.Error(), "cs_waits_actor_matches_reason")
}

func TestDatabaseRefusesAnOpenWaitThatNamesACS(t *testing.T) {
	db := freshPostgres(t)
	wait := openThreadWait(uuid.New())
	someone := uuid.New()
	wait.EndedBy = &someone

	err := db.Create(wait).Error

	require.Error(t, err)
	assert.Contains(t, err.Error(), "cs_waits_actor_matches_reason")
}

func TestDatabaseRefusesAReasonWithoutAnEndTime(t *testing.T) {
	db := freshPostgres(t)
	wait := openThreadWait(uuid.New())
	reason := models.WaitAbandoned
	wait.EndReason = &reason

	err := db.Create(wait).Error

	require.Error(t, err)
	assert.Contains(t, err.Error(), "cs_waits_ended_together")
}
```

- [ ] **Step 2: Jalankan tes, pastikan gagal**

Run: `cd backend && go test ./internal/database/ -run 'Wait' -v`
Expected: FAIL saat kompilasi: `undefined: models.CSWait` dan `undefined: models.WaitEndReason`.

- [ ] **Step 3: Tulis model**

Buat `backend/internal/models/cs_wait.go`:

```go
package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// WaitEndReason is how a customer's wait for an answer ended.
type WaitEndReason string

const (
	// WaitReplied is a CS answering through TikMan.
	WaitReplied WaitEndReason = "replied"
	// WaitPhone is an answer typed outside TikMan, on a device linked to the
	// number, so nobody is recorded as having given it.
	WaitPhone WaitEndReason = "phone"
	// WaitClosed is a CS closing the thread without answering.
	WaitClosed WaitEndReason = "closed"
	// WaitAbandoned is a wait the customer left: they went quiet for more than
	// a day and then wrote about something new.
	WaitAbandoned WaitEndReason = "abandoned"
)

// CSWait is one stretch of a customer waiting for an answer, recorded as it
// happens rather than worked out from the messages later. Deleting messages,
// emptying a thread or removing a number leaves these rows alone, which is why
// there is no foreign key: a figure built on them cannot be changed by deleting
// what it was built from.
type CSWait struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	ConversationID uuid.UUID `gorm:"type:uuid;not null;index" json:"conversation_id"`
	WAAccountID    uuid.UUID `gorm:"type:uuid;not null" json:"wa_account_id"`
	// StartedAt is when the wait's first message reached TikMan and
	// CustomerSentAt when the customer sent it. The gap between them is how the
	// report tells a system delay from a slow answer.
	StartedAt      time.Time `gorm:"not null" json:"started_at"`
	CustomerSentAt time.Time `gorm:"not null" json:"customer_sent_at"`
	// LastCustomerSentAt is the newest message in the wait by the customer's
	// clock, which is what the silence rule measures from.
	LastCustomerSentAt time.Time      `gorm:"not null" json:"last_customer_sent_at"`
	EndedAt            *time.Time     `json:"ended_at,omitempty"`
	EndReason          *WaitEndReason `gorm:"type:varchar(20)" json:"end_reason,omitempty"`
	// EndedBy is the CS who replied or closed. It is nil for a phone answer and
	// an abandoned wait.
	EndedBy        *uuid.UUID `gorm:"type:uuid" json:"ended_by,omitempty"`
	ReplyMessageID *uuid.UUID `gorm:"type:uuid" json:"reply_message_id,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

func (w *CSWait) BeforeCreate(tx *gorm.DB) error {
	if w.ID == uuid.Nil {
		w.ID = uuid.New()
	}
	return nil
}

// TableName specifies the table name for GORM.
func (CSWait) TableName() string { return "cs_waits" }
```

Di `backend/internal/models/models.go`, tambahkan `&CSWait{},` tepat setelah `&PushSubscription{},` di daftar `db.AutoMigrate(`.

- [ ] **Step 4: Tulis migrasi 52**

Buat `backend/migrations/52_cs_waits.sql`:

```sql
-- cs_waits: one row per customer waiting for an answer, recorded as it happens.
--
-- AutoMigrate creates the table from models.CSWait before this runs; this file
-- adds what model tags cannot say. There is deliberately no foreign key to
-- cs_conversations or wa_accounts: deleting a number deletes its threads, and a
-- wait must outlive that, both so the figures cannot be changed by deleting and
-- so the delete is not refused, as the broadcast history once refused it.

-- A thread waits on at most one thing at a time. A second open wait would be
-- answered twice and counted twice.
CREATE UNIQUE INDEX IF NOT EXISTS uq_cs_waits_one_open_per_thread
    ON cs_waits (conversation_id) WHERE ended_at IS NULL;

ALTER TABLE cs_waits DROP CONSTRAINT IF EXISTS cs_waits_end_reason_valid;
ALTER TABLE cs_waits ADD CONSTRAINT cs_waits_end_reason_valid
    CHECK (end_reason IS NULL OR end_reason IN ('replied', 'phone', 'closed', 'abandoned'));

ALTER TABLE cs_waits DROP CONSTRAINT IF EXISTS cs_waits_ended_together;
ALTER TABLE cs_waits ADD CONSTRAINT cs_waits_ended_together
    CHECK ((ended_at IS NULL) = (end_reason IS NULL));

-- Who ended a wait, and with which message, follows from how it ended. A CASE
-- rather than ORed conditions: with end_reason NULL an OR chain evaluates to
-- NULL, and a CHECK passes on NULL.
ALTER TABLE cs_waits DROP CONSTRAINT IF EXISTS cs_waits_actor_matches_reason;
ALTER TABLE cs_waits ADD CONSTRAINT cs_waits_actor_matches_reason CHECK (
    CASE COALESCE(end_reason, 'open')
        WHEN 'replied' THEN ended_by IS NOT NULL AND reply_message_id IS NOT NULL
        WHEN 'phone' THEN ended_by IS NULL AND reply_message_id IS NOT NULL
        WHEN 'closed' THEN ended_by IS NOT NULL AND reply_message_id IS NULL
        ELSE ended_by IS NULL AND reply_message_id IS NULL
    END
);

-- The report reads waits by when they ended, and one CS's waits by who ended
-- them.
CREATE INDEX IF NOT EXISTS idx_cs_waits_ended_at ON cs_waits (ended_at);
CREATE INDEX IF NOT EXISTS idx_cs_waits_ended_by_ended_at ON cs_waits (ended_by, ended_at);
```

- [ ] **Step 5: Jalankan tes, pastikan lulus**

Jalankan kontainer TimescaleDB sementara (lihat Global Constraints), lalu:

Run: `cd backend && TEST_POSTGRES_DSN="host=localhost port=5439 user=postgres password=test dbname=tikman_test sslmode=disable" go test ./internal/database/ -v`
Expected: PASS, termasuk keenam tes `TestDatabase...Wait...` baru dan `TestEveryMigrationAppliesToAFreshSchema`. Hentikan kontainer sesudahnya.

Run: `cd backend && go build ./... && go test ./internal/models/ ./internal/services/ -run 'CS|Inbound|Awaiting|Claim' 2>&1 | tail -5`
Expected: build bersih; tes yang dipilih lulus (AutoMigrate SQLite kini ikut membuat `cs_waits`).

Run: `cd backend && gofmt -s -l internal/models internal/database`
Expected: kosong.

- [ ] **Step 6: Commit**

```bash
cd /Users/rohadimraja/Documents/tikman
git add backend/internal/models/cs_wait.go backend/internal/models/models.go \
        backend/migrations/52_cs_waits.sql backend/internal/database/migrations_cs_waits_postgres_test.go
git commit -m "$(cat <<'EOF'
feat(cs): add the table customer waits are recorded in

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 2: Aturan pencatatan giliran

**Files:**
- Create: `backend/internal/services/cs_wait.go`
- Test: `backend/internal/services/cs_wait_test.go`

**Interfaces:**
- Consumes: `models.CSWait`, `models.WaitEndReason` dan konstantanya (Task 1); `models.CSMessage`; helper tes `setupTestDB(t)` (`user_service_test.go`), `csAccount(t, db)` dan `peer(accountID)` (`cs_conversation_service_test.go`).
- Produces (package `services`, unexported, dipakai Task 3 dan 4):
  - `const csCustomerSilence = 24 * time.Hour`
  - `type csWaits struct { logger *zap.Logger }` dengan `func (w csWaits) record(tx *gorm.DB, conversationID uuid.UUID, step string, fn func(*gorm.DB) error)`
  - `func customerWrote(tx *gorm.DB, msg *models.CSMessage) error`
  - `func replied(tx *gorm.DB, msg *models.CSMessage) error` (memakai `msg.SenderUserID` dan `msg.ID`)
  - `func answeredFromPhone(tx *gorm.DB, msg *models.CSMessage) error`
  - `func closedWithoutReply(tx *gorm.DB, conversationID, closedBy uuid.UUID, at time.Time) error`
  - `func openWait(tx *gorm.DB, conversationID uuid.UUID, startedAt, sentAt, lastSentAt time.Time) error`

- [ ] **Step 1: Tulis tes yang gagal**

Buat `backend/internal/services/cs_wait_test.go`:

```go
package services

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// waitFixture is one thread whose messages are stored at chosen moments. Every
// time is UTC: SQLite compares stored times as text, so they must share a zone.
type waitFixture struct {
	t    *testing.T
	db   *gorm.DB
	conv *models.CSConversation
}

func newWaitFixture(t *testing.T) *waitFixture {
	t.Helper()
	db := setupTestDB(t)
	conv, err := NewCSConversationService(db).FindOrCreate(peer(csAccount(t, db).ID))
	require.NoError(t, err)
	return &waitFixture{t: t, db: db, conv: conv}
}

// waitClock is a moment a number of minutes after 01:00 UTC on 10 September.
func waitClock(minutes int) time.Time {
	return time.Date(2026, 9, 10, 1, 0, 0, 0, time.UTC).Add(time.Duration(minutes) * time.Minute)
}

// customer stores a customer's message sent at sentMin that reached TikMan at
// arrivedMin.
func (f *waitFixture) customer(sentMin, arrivedMin int) *models.CSMessage {
	return f.store(models.MessageIn, nil, sentMin, arrivedMin)
}

// reply stores an outbound message: queued by sender through TikMan, or typed
// on the phone when sender is nil.
func (f *waitFixture) reply(sender *uuid.UUID, sentMin, arrivedMin int) *models.CSMessage {
	return f.store(models.MessageOut, sender, sentMin, arrivedMin)
}

func (f *waitFixture) store(dir models.MessageDirection, sender *uuid.UUID, sentMin, arrivedMin int) *models.CSMessage {
	f.t.Helper()
	msg := &models.CSMessage{
		ConversationID: f.conv.ID, Direction: dir, SenderUserID: sender,
		Kind: models.MessageKindText, Body: "pesan", Status: models.MessageDelivered,
		WATimestamp: waitClock(sentMin), CreatedAt: waitClock(arrivedMin),
	}
	require.NoError(f.t, f.db.Create(msg).Error)
	return msg
}

// step runs one bookkeeping function in a transaction of its own.
func (f *waitFixture) step(fn func(*gorm.DB) error) {
	f.t.Helper()
	require.NoError(f.t, f.db.Transaction(fn))
}

func (f *waitFixture) wrote(msg *models.CSMessage) {
	f.t.Helper()
	f.step(func(tx *gorm.DB) error { return customerWrote(tx, msg) })
}

func (f *waitFixture) waits() []models.CSWait {
	f.t.Helper()
	var waits []models.CSWait
	require.NoError(f.t, f.db.Order("started_at ASC").Find(&waits).Error)
	return waits
}

func TestAFirstCustomerMessageStartsAWait(t *testing.T) {
	f := newWaitFixture(t)

	f.wrote(f.customer(0, 1))

	waits := f.waits()
	require.Len(t, waits, 1)
	assert.WithinDuration(t, waitClock(1), waits[0].StartedAt, 0, "a wait starts when TikMan held the message")
	assert.WithinDuration(t, waitClock(0), waits[0].CustomerSentAt, 0)
	assert.Equal(t, f.conv.WAAccountID, waits[0].WAAccountID)
	assert.Nil(t, waits[0].EndedAt)
}

func TestMessagesBeforeAnAnswerStayInOneWait(t *testing.T) {
	f := newWaitFixture(t)
	f.wrote(f.customer(0, 0))

	f.wrote(f.customer(10, 10))

	waits := f.waits()
	require.Len(t, waits, 1)
	assert.WithinDuration(t, waitClock(0), waits[0].StartedAt, 0)
	assert.WithinDuration(t, waitClock(10), waits[0].LastCustomerSentAt, 0)
}

// "ok makasih" and a new question days later are two waits, not one long one.
func TestWritingAgainAfterADayOfSilenceAbandonsTheOldWait(t *testing.T) {
	f := newWaitFixture(t)
	f.wrote(f.customer(0, 0))
	later := 24*60 + 1

	f.wrote(f.customer(later, later))

	waits := f.waits()
	require.Len(t, waits, 2)
	require.NotNil(t, waits[0].EndReason)
	assert.Equal(t, models.WaitAbandoned, *waits[0].EndReason)
	assert.WithinDuration(t, waitClock(later), *waits[0].EndedAt, 0)
	assert.Nil(t, waits[1].EndedAt)
	assert.WithinDuration(t, waitClock(later), waits[1].StartedAt, 0)
}

func TestAnOlderMessageArrivingLateDoesNotMoveTheWaitBack(t *testing.T) {
	f := newWaitFixture(t)
	f.wrote(f.customer(10, 10))

	f.wrote(f.customer(5, 12))

	assert.WithinDuration(t, waitClock(10), f.waits()[0].LastCustomerSentAt, 0)
}

func TestAReplyEndsTheWaitAndNamesTheCS(t *testing.T) {
	f := newWaitFixture(t)
	f.wrote(f.customer(0, 0))
	cs := uuid.New()
	answer := f.reply(&cs, 7, 7)

	f.step(func(tx *gorm.DB) error { return replied(tx, answer) })

	w := f.waits()[0]
	require.NotNil(t, w.EndReason)
	assert.Equal(t, models.WaitReplied, *w.EndReason)
	assert.WithinDuration(t, waitClock(7), *w.EndedAt, 0)
	assert.Equal(t, &cs, w.EndedBy)
	assert.Equal(t, &answer.ID, w.ReplyMessageID)
}

func TestAReplyWithNoOneWaitingChangesNothing(t *testing.T) {
	f := newWaitFixture(t)
	cs := uuid.New()
	answer := f.reply(&cs, 7, 7)

	f.step(func(tx *gorm.DB) error { return replied(tx, answer) })

	assert.Empty(t, f.waits())
}

func TestAPhoneReplyEndsTheWaitWithNobodyNamed(t *testing.T) {
	f := newWaitFixture(t)
	f.wrote(f.customer(0, 0))
	answer := f.reply(nil, 5, 5)

	f.step(func(tx *gorm.DB) error { return answeredFromPhone(tx, answer) })

	w := f.waits()[0]
	require.NotNil(t, w.EndReason)
	assert.Equal(t, models.WaitPhone, *w.EndReason)
	assert.WithinDuration(t, waitClock(5), *w.EndedAt, 0)
	assert.Nil(t, w.EndedBy)
	assert.Equal(t, &answer.ID, w.ReplyMessageID)
}

// A phone reply sent before this wait began answered an earlier one.
func TestAPhoneReplyOlderThanTheWaitChangesNothing(t *testing.T) {
	f := newWaitFixture(t)
	f.wrote(f.customer(10, 10))
	answer := f.reply(nil, 5, 11)

	f.step(func(tx *gorm.DB) error { return answeredFromPhone(tx, answer) })

	assert.Nil(t, f.waits()[0].EndedAt)
}

// The phone answered at minute 10, but WhatsApp handed that over only at
// minute 30, after the customer had written again at minute 20. The message
// at 20 is still unanswered.
func TestALatePhoneReplyLeavesTheCustomersLaterMessagesWaiting(t *testing.T) {
	f := newWaitFixture(t)
	f.wrote(f.customer(0, 0))
	f.wrote(f.customer(20, 20))
	answer := f.reply(nil, 10, 30)

	f.step(func(tx *gorm.DB) error { return answeredFromPhone(tx, answer) })

	waits := f.waits()
	require.Len(t, waits, 2)
	require.NotNil(t, waits[0].EndReason)
	assert.Equal(t, models.WaitPhone, *waits[0].EndReason)
	assert.WithinDuration(t, waitClock(10), *waits[0].EndedAt, 0)
	assert.Nil(t, waits[1].EndedAt)
	assert.WithinDuration(t, waitClock(20), waits[1].StartedAt, 0)
}

// The customer's message reached TikMan at minute 40; the phone had answered
// at minute 5. Nobody in TikMan waited, so the wait ends the moment it starts.
func TestAPhoneReplySentBeforeTheMessageArrivedEndsTheWaitAtOnce(t *testing.T) {
	f := newWaitFixture(t)
	f.wrote(f.customer(0, 40))
	answer := f.reply(nil, 5, 41)

	f.step(func(tx *gorm.DB) error { return answeredFromPhone(tx, answer) })

	w := f.waits()[0]
	require.NotNil(t, w.EndedAt)
	assert.WithinDuration(t, w.StartedAt, *w.EndedAt, 0)
}

func TestClosingEndsTheWaitAsClosedByTheCS(t *testing.T) {
	f := newWaitFixture(t)
	f.wrote(f.customer(0, 0))
	cs := uuid.New()

	f.step(func(tx *gorm.DB) error { return closedWithoutReply(tx, f.conv.ID, cs, waitClock(15)) })

	w := f.waits()[0]
	require.NotNil(t, w.EndReason)
	assert.Equal(t, models.WaitClosed, *w.EndReason)
	assert.Equal(t, &cs, w.EndedBy)
	assert.Nil(t, w.ReplyMessageID)
}

// openThenFail writes a wait and then fails, the way a step can fail partway.
func openThenFail(conversationID uuid.UUID) func(*gorm.DB) error {
	return func(tx *gorm.DB) error {
		if err := openWait(tx, conversationID, waitClock(0), waitClock(0), waitClock(0)); err != nil {
			return err
		}
		return errors.New("the step fails after writing")
	}
}

// A step that fails is undone on its own; what the transaction wrote before it
// still commits.
func TestAFailedStepRollsBackAloneAndTheTransactionCommits(t *testing.T) {
	f := newWaitFixture(t)
	waits := csWaits{logger: zap.NewNop()}

	f.step(func(tx *gorm.DB) error {
		msg := &models.CSMessage{
			ConversationID: f.conv.ID, Direction: models.MessageIn, Kind: models.MessageKindText,
			Body: "tetap tersimpan", Status: models.MessageDelivered, WATimestamp: waitClock(0),
		}
		if err := tx.Create(msg).Error; err != nil {
			return err
		}
		waits.record(tx, f.conv.ID, "test", openThenFail(f.conv.ID))
		return nil
	})

	assert.Empty(t, f.waits(), "the failed step's wait is rolled back")
	var stored int64
	require.NoError(t, f.db.Model(&models.CSMessage{}).Count(&stored).Error)
	assert.Equal(t, int64(1), stored, "the message written before it commits")
}
```

- [ ] **Step 2: Jalankan tes, pastikan gagal**

Run: `cd backend && go test ./internal/services/ -run 'Wait|PhoneReply|Closing|Reply' -v 2>&1 | head -20`
Expected: FAIL saat kompilasi: `undefined: customerWrote`, `undefined: csWaits`, dan fungsi lain dari file yang belum ada.

- [ ] **Step 3: Tulis `cs_wait.go`**

Buat `backend/internal/services/cs_wait.go`:

```go
package services

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// csCustomerSilence is how long a customer can go quiet before writing again
// starts a new wait instead of continuing the old one. Without it, "ok makasih"
// followed by a new question days later reads as days of waiting.
const csCustomerSilence = 24 * time.Hour

// csWaits records how long customers wait for an answer, inside the
// transactions that store what the waits are made of.
type csWaits struct {
	logger *zap.Logger
}

// record runs one step of wait bookkeeping under a savepoint of the caller's
// transaction. A failing step is rolled back alone and logged, and the caller
// carries on: storing the message, the reply or the close is the inbox's job,
// and a figure must never be why one of those is lost.
func (w csWaits) record(tx *gorm.DB, conversationID uuid.UUID, step string, fn func(*gorm.DB) error) {
	if err := tx.Transaction(fn); err != nil {
		w.logger.Error("Could not record a CS wait",
			zap.String("step", step),
			zap.String("conversation_id", conversationID.String()),
			zap.Error(err))
	}
}

// customerWrote folds a customer's message into the thread's open wait, or
// starts one. An open wait whose customer went quiet for csCustomerSilence ends
// as abandoned, and a new one starts from this message.
func customerWrote(tx *gorm.DB, msg *models.CSMessage) error {
	extended, err := extendOpenWait(tx, msg)
	if err != nil || extended {
		return err
	}
	if _, err := endOpenWait(tx, msg.ConversationID, models.WaitAbandoned, msg.CreatedAt, nil, nil); err != nil {
		return err
	}
	return openWait(tx, msg.ConversationID, msg.CreatedAt, msg.WATimestamp, msg.WATimestamp)
}

// extendOpenWait adds a message to an open wait the customer has not gone quiet
// on. The newest send time is kept, so a message WhatsApp hands over late
// cannot move the wait backwards.
func extendOpenWait(tx *gorm.DB, msg *models.CSMessage) (bool, error) {
	res := tx.Model(&models.CSWait{}).
		Where("conversation_id = ? AND ended_at IS NULL AND last_customer_sent_at >= ?",
			msg.ConversationID, msg.WATimestamp.Add(-csCustomerSilence)).
		Update("last_customer_sent_at", gorm.Expr(
			"CASE WHEN last_customer_sent_at < ? THEN ? ELSE last_customer_sent_at END",
			msg.WATimestamp, msg.WATimestamp))
	if res.Error != nil {
		return false, fmt.Errorf("extend open wait: %w", res.Error)
	}
	return res.RowsAffected > 0, nil
}

// endOpenWait ends the thread's open wait, answering whether there was one. The
// condition on ended_at is what makes two enders safe: whichever commits second
// finds nothing left to end.
func endOpenWait(tx *gorm.DB, conversationID uuid.UUID, reason models.WaitEndReason, at time.Time,
	endedBy, replyMessageID *uuid.UUID) (bool, error) {
	res := tx.Model(&models.CSWait{}).
		Where("conversation_id = ? AND ended_at IS NULL", conversationID).
		Updates(map[string]any{
			"ended_at":         at,
			"end_reason":       reason,
			"ended_by":         endedBy,
			"reply_message_id": replyMessageID,
		})
	if res.Error != nil {
		return false, fmt.Errorf("end open wait: %w", res.Error)
	}
	return res.RowsAffected > 0, nil
}

// openWait starts a wait on a thread.
func openWait(tx *gorm.DB, conversationID uuid.UUID, startedAt, sentAt, lastSentAt time.Time) error {
	var conv models.CSConversation
	if err := tx.Select("wa_account_id").First(&conv, "id = ?", conversationID).Error; err != nil {
		return fmt.Errorf("load thread for wait: %w", err)
	}
	wait := models.CSWait{
		ConversationID:     conversationID,
		WAAccountID:        conv.WAAccountID,
		StartedAt:          startedAt,
		CustomerSentAt:     sentAt,
		LastCustomerSentAt: lastSentAt,
	}
	if err := tx.Create(&wait).Error; err != nil {
		return fmt.Errorf("open wait: %w", err)
	}
	return nil
}

// replied ends the thread's open wait as answered by the CS who queued msg.
func replied(tx *gorm.DB, msg *models.CSMessage) error {
	_, err := endOpenWait(tx, msg.ConversationID, models.WaitReplied, msg.WATimestamp, msg.SenderUserID, &msg.ID)
	return err
}

// answeredFromPhone ends the open wait a phone reply answers. A reply sent
// before the wait's first message answered an earlier wait and changes nothing.
// When WhatsApp hands the reply over late, the customer's messages sent after it
// are still unanswered, so a new wait starts from the first of them.
func answeredFromPhone(tx *gorm.DB, msg *models.CSMessage) error {
	var open models.CSWait
	res := tx.Where("conversation_id = ? AND ended_at IS NULL", msg.ConversationID).Limit(1).Find(&open)
	if res.Error != nil {
		return fmt.Errorf("load open wait: %w", res.Error)
	}
	if res.RowsAffected == 0 || msg.WATimestamp.Before(open.CustomerSentAt) {
		return nil
	}
	endedAt := msg.WATimestamp
	if endedAt.Before(open.StartedAt) {
		endedAt = open.StartedAt
	}
	endedWait, err := endOpenWait(tx, msg.ConversationID, models.WaitPhone, endedAt, nil, &msg.ID)
	if err != nil || !endedWait {
		return err
	}
	return reopenAfter(tx, msg.ConversationID, msg.WATimestamp)
}

// reopenAfter starts a wait from the customer messages sent after a moment.
func reopenAfter(tx *gorm.DB, conversationID uuid.UUID, after time.Time) error {
	var later []models.CSMessage
	err := tx.Where("conversation_id = ? AND direction = ? AND wa_timestamp > ?",
		conversationID, models.MessageIn, after).
		Order("wa_timestamp ASC").Find(&later).Error
	if err != nil {
		return fmt.Errorf("load messages sent after a phone reply: %w", err)
	}
	if len(later) == 0 {
		return nil
	}
	first, last := later[0], later[len(later)-1]
	return openWait(tx, conversationID, first.CreatedAt, first.WATimestamp, last.WATimestamp)
}

// closedWithoutReply ends the thread's open wait as closed by a CS.
func closedWithoutReply(tx *gorm.DB, conversationID, closedBy uuid.UUID, at time.Time) error {
	_, err := endOpenWait(tx, conversationID, models.WaitClosed, at, &closedBy, nil)
	return err
}
```

- [ ] **Step 4: Jalankan tes, pastikan lulus**

Run: `cd backend && go test ./internal/services/ -run 'TestAFirstCustomerMessageStartsAWait|TestMessagesBeforeAnAnswerStayInOneWait|TestWritingAgainAfterADayOfSilenceAbandonsTheOldWait|TestAnOlderMessageArrivingLateDoesNotMoveTheWaitBack|TestAReplyEndsTheWaitAndNamesTheCS|TestAReplyWithNoOneWaitingChangesNothing|TestAPhoneReplyEndsTheWaitWithNobodyNamed|TestAPhoneReplyOlderThanTheWaitChangesNothing|TestALatePhoneReplyLeavesTheCustomersLaterMessagesWaiting|TestAPhoneReplySentBeforeTheMessageArrivedEndsTheWaitAtOnce|TestClosingEndsTheWaitAsClosedByTheCS|TestAFailedStepRollsBackAloneAndTheTransactionCommits' -v`
Expected: 12 tes PASS.

Run: `cd backend && gofmt -s -l internal/services && go vet ./internal/services/`
Expected: kosong dan bersih.

- [ ] **Step 5: Commit**

```bash
cd /Users/rohadimraja/Documents/tikman
git add backend/internal/services/cs_wait.go backend/internal/services/cs_wait_test.go
git commit -m "$(cat <<'EOF'
feat(cs): define how a customer's wait starts and ends

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 3: Catat giliran dari pesan pelanggan dan balasan HP

**Files:**
- Modify: `backend/internal/services/cs_conversation_service.go` (field `waits`, konstruktor, `SetWaitLogger`)
- Modify: `backend/internal/services/cs_message_service.go` (`saveArrived`, `SaveInbound`, `customerMessageStored`)
- Modify: `backend/internal/services/cs_message_phone.go` (`SaveFromPhone`, `phoneMessageStored`)
- Modify: `backend/cmd/wa/main.go` (pasang logger)
- Test: `backend/internal/services/cs_wait_recording_test.go`

**Interfaces:**
- Consumes: `csWaits.record`, `customerWrote`, `answeredFromPhone` (Task 2); `bumpConversation(tx, conversationID, at)` (`cs_message_fields.go`); `markAnsweredFromPhone(tx, conversationID, at)` (`cs_message_phone.go`).
- Produces:
  - `func (s *CSConversationService) SetWaitLogger(logger *zap.Logger)` — dipakai `cmd/wa/main.go` (task ini) dan `newCSStack` (Task 4).
  - Tipe parameter `touch` pada `saveArrived` berubah menjadi `func(*gorm.DB, *models.CSMessage) error`.
  - Helper tes `recordingSetup(t)`, `customerSent(conversationID, waID, sent)`, `storedWaits(t, db)` yang dipakai Task 4.

- [ ] **Step 1: Tulis tes yang gagal**

Buat `backend/internal/services/cs_wait_recording_test.go`:

```go
package services

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

func recordingSetup(t *testing.T) (*gorm.DB, *CSMessageService, *CSConversationService, *models.CSConversation) {
	t.Helper()
	db := setupTestDB(t)
	conversations := NewCSConversationService(db)
	conv, err := conversations.FindOrCreate(peer(csAccount(t, db).ID))
	require.NoError(t, err)
	return db, NewCSMessageService(db, conversations), conversations, conv
}

func customerSent(conversationID uuid.UUID, waID string, sent time.Time) InboundMessage {
	return InboundMessage{
		ConversationID: conversationID, WAMessageID: waID,
		Kind: models.MessageKindText, Body: "internet mati", At: sent,
	}
}

func storedWaits(t *testing.T, db *gorm.DB) []models.CSWait {
	t.Helper()
	var waits []models.CSWait
	require.NoError(t, db.Order("started_at ASC").Find(&waits).Error)
	return waits
}

func TestStoringACustomerMessageStartsAWait(t *testing.T) {
	db, messages, _, conv := recordingSetup(t)

	_, _, err := messages.SaveInbound(customerSent(conv.ID, "3EB0A", time.Now()))
	require.NoError(t, err)

	waits := storedWaits(t, db)
	require.Len(t, waits, 1)
	assert.Equal(t, conv.ID, waits[0].ConversationID)
	assert.Nil(t, waits[0].EndedAt)
}

// WhatsApp re-delivers on every reconnect, and a duplicate is not more waiting.
func TestACustomerMessageDeliveredTwiceTouchesTheWaitOnce(t *testing.T) {
	db, messages, _, conv := recordingSetup(t)
	in := customerSent(conv.ID, "3EB0A", time.Now())
	_, _, err := messages.SaveInbound(in)
	require.NoError(t, err)

	_, created, err := messages.SaveInbound(in)
	require.NoError(t, err)

	assert.False(t, created)
	assert.Len(t, storedWaits(t, db), 1)
}

func TestStoringAPhoneReplyEndsTheWait(t *testing.T) {
	db, messages, _, conv := recordingSetup(t)
	sent := time.Now()
	_, _, err := messages.SaveInbound(customerSent(conv.ID, "3EB0A", sent))
	require.NoError(t, err)

	_, _, err = messages.SaveFromPhone(InboundMessage{
		ConversationID: conv.ID, WAMessageID: "3EB0PHONE",
		Kind: models.MessageKindText, Body: "sudah kami cek", At: sent.Add(time.Minute),
	})
	require.NoError(t, err)

	waits := storedWaits(t, db)
	require.Len(t, waits, 1)
	require.NotNil(t, waits[0].EndReason)
	assert.Equal(t, models.WaitPhone, *waits[0].EndReason)
}

// The figures must never be why a customer's message is lost: with the waits
// table gone, the message and the thread still land as before.
func TestAMessageIsStoredEvenWhenItsWaitCannotBeRecorded(t *testing.T) {
	db, messages, conversations, conv := recordingSetup(t)
	require.NoError(t, db.Migrator().DropTable(&models.CSWait{}))

	_, created, err := messages.SaveInbound(customerSent(conv.ID, "3EB0A", time.Now()))

	require.NoError(t, err)
	assert.True(t, created)
	waiting, err := conversations.List(ConversationFilter{AwaitingReply: true})
	require.NoError(t, err)
	assert.Len(t, waiting, 1)
}

func TestAPhoneReplyIsStoredEvenWhenItsWaitCannotBeRecorded(t *testing.T) {
	db, messages, conversations, conv := recordingSetup(t)
	sent := time.Now()
	_, _, err := messages.SaveInbound(customerSent(conv.ID, "3EB0A", sent))
	require.NoError(t, err)
	require.NoError(t, db.Migrator().DropTable(&models.CSWait{}))

	_, created, err := messages.SaveFromPhone(InboundMessage{
		ConversationID: conv.ID, WAMessageID: "3EB0PHONE",
		Kind: models.MessageKindText, Body: "sudah", At: sent.Add(time.Minute),
	})

	require.NoError(t, err)
	assert.True(t, created)
	waiting, err := conversations.List(ConversationFilter{AwaitingReply: true})
	require.NoError(t, err)
	assert.Empty(t, waiting)
}
```

- [ ] **Step 2: Jalankan tes, pastikan gagal**

Run: `cd backend && go test ./internal/services/ -run 'Storing|DeliveredTwiceTouches|EvenWhenItsWait' -v`
Expected:
- FAIL: `TestStoringACustomerMessageStartsAWait` dan `TestACustomerMessageDeliveredTwiceTouchesTheWaitOnce` (tidak ada baris giliran sama sekali), serta `TestStoringAPhoneReplyEndsTheWait`.
- PASS: kedua tes "EvenWhenItsWait..." — keduanya penjaga perilaku yang harus tetap benar setelah perubahan.

- [ ] **Step 3: Beri `CSConversationService` tempat melaporkan kegagalan**

Di `backend/internal/services/cs_conversation_service.go`, tambahkan `"go.uber.org/zap"` ke blok impor, lalu ganti struct dan konstruktornya:

```go
// CSConversationService owns a customer's thread: who they are, which ONT is
// theirs, who is holding the thread, and when it is done.
type CSConversationService struct {
	db    *gorm.DB
	waits csWaits
}

// NewCSConversationService constructs a CSConversationService.
func NewCSConversationService(db *gorm.DB) *CSConversationService {
	return &CSConversationService{db: db, waits: csWaits{logger: zap.NewNop()}}
}

// SetWaitLogger names where a failure to record a CS wait is reported. cmd/api
// and cmd/wa set it once after construction, the way SetSender is wired on
// PushNotifierService; until then those failures are dropped, which is what
// tests want.
func (s *CSConversationService) SetWaitLogger(logger *zap.Logger) {
	s.waits.logger = logger
}
```

- [ ] **Step 4: Catat giliran saat pesan disimpan**

Di `backend/internal/services/cs_message_service.go`, ubah tipe parameter `touch` pada `saveArrived` dan pemanggilannya:

```go
func (s *CSMessageService) saveArrived(
	in InboundMessage,
	row func(*gorm.DB, InboundMessage) models.CSMessage,
	touch func(*gorm.DB, *models.CSMessage) error,
) (*models.CSMessage, bool, error) {
```

dan di dalam transaksinya ganti `if err := touch(tx, in.ConversationID, in.At); err != nil {` menjadi:

```go
		if err := touch(tx, &stored); err != nil {
```

Lalu ganti `SaveInbound` dengan:

```go
// SaveInbound stores an incoming message, answering false when this WhatsApp
// message was already stored. WhatsApp re-delivers events it is unsure about,
// and the duplicate would otherwise be shown to the CS and counted as unread.
func (s *CSMessageService) SaveInbound(in InboundMessage) (*models.CSMessage, bool, error) {
	return s.saveArrived(in, inboundRow, s.customerMessageStored)
}

// customerMessageStored brings the thread up to date and records that the
// customer is waiting. The wait is bookkeeping: it rides a savepoint, so a
// failure there costs the figure and not the message.
func (s *CSMessageService) customerMessageStored(tx *gorm.DB, msg *models.CSMessage) error {
	if err := bumpConversation(tx, msg.ConversationID, msg.WATimestamp); err != nil {
		return err
	}
	s.conversations.waits.record(tx, msg.ConversationID, "customer message", func(tx *gorm.DB) error {
		return customerWrote(tx, msg)
	})
	return nil
}
```

Di `backend/internal/services/cs_message_phone.go`, ganti `SaveFromPhone` dengan:

```go
func (s *CSMessageService) SaveFromPhone(in InboundMessage) (*models.CSMessage, bool, error) {
	return s.saveArrived(in, phoneRow, s.phoneMessageStored)
}

// phoneMessageStored brings the thread up to date and records the wait this
// answer ended, under the same savepoint rule as a customer's message.
func (s *CSMessageService) phoneMessageStored(tx *gorm.DB, msg *models.CSMessage) error {
	if err := markAnsweredFromPhone(tx, msg.ConversationID, msg.WATimestamp); err != nil {
		return err
	}
	s.conversations.waits.record(tx, msg.ConversationID, "phone reply", func(tx *gorm.DB) error {
		return answeredFromPhone(tx, msg)
	})
	return nil
}
```

- [ ] **Step 5: Pasang logger di proses `wa`**

Di `backend/cmd/wa/main.go`, tepat setelah baris `conversations := services.NewCSConversationService(db)`, tambahkan:

```go
	conversations.SetWaitLogger(logger)
```

- [ ] **Step 6: Jalankan tes, pastikan lulus**

Run: `cd backend && go test ./internal/services/ -run 'Storing|DeliveredTwiceTouches|EvenWhenItsWait' -v`
Expected: 5 tes PASS.

Run: `cd backend && go test ./internal/services/ ./internal/wa/ 2>&1 | grep -E '^(ok|FAIL|--- FAIL)'`
Expected: `internal/wa` ok; `internal/services` gagal hanya pada tes WireGuard baseline (lihat Global Constraints).

Run: `cd backend && go build ./... && gofmt -s -l internal cmd`
Expected: build bersih, gofmt kosong.

- [ ] **Step 7: Commit**

```bash
cd /Users/rohadimraja/Documents/tikman
git add backend/internal/services/cs_conversation_service.go backend/internal/services/cs_message_service.go \
        backend/internal/services/cs_message_phone.go backend/internal/services/cs_wait_recording_test.go \
        backend/cmd/wa/main.go
git commit -m "$(cat <<'EOF'
feat(cs): record a customer's wait as their message lands

A wait now starts when a customer's message reaches TikMan and ends when
an answer typed outside TikMan arrives. The bookkeeping rides a savepoint
of the transaction that stores the message, so a failure costs the figure
and never the message.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 4: Catat giliran dari balasan TikMan dan penutupan thread

**Files:**
- Modify: `backend/internal/services/cs_message_service.go` (`Queue`)
- Modify: `backend/internal/services/cs_conversation_service.go` (`Close` menerima penutupnya)
- Modify: `backend/internal/api/cs_handler_conversations.go` (`SetStatus`)
- Modify: `backend/internal/api/router_handlers.go` (pasang logger)
- Modify (pemanggil `Close` di tes): `backend/internal/services/cs_conversation_service_test.go:88`, `backend/internal/services/cs_claim_test.go:75`, `backend/internal/services/cs_awaiting_reply_test.go:80` dan `:130`, `backend/internal/services/cs_message_phone_test.go:82`, `backend/internal/services/cs_claim_postgres_test.go:106`
- Test: `backend/internal/services/cs_wait_reply_test.go`
- Test: `backend/internal/services/cs_wait_postgres_test.go`
- Test: `backend/internal/api/cs_handler_close_wait_test.go`

**Interfaces:**
- Consumes: `replied`, `closedWithoutReply`, `csWaits.record` (Task 2); `recordingSetup`, `customerSent`, `storedWaits` (Task 3); `setupPostgresTestDB` (`cs_message_postgres_test.go`), `postgresAgent` (`cs_claim_postgres_test.go`), `setupCSHandler`/`asUser`/`csTestUser` (`internal/api/cs_handler_test.go`).
- Produces: `func (s *CSConversationService) Close(conversationID, closedBy uuid.UUID) error`.

- [ ] **Step 1: Tulis tes yang gagal**

Buat `backend/internal/services/cs_wait_reply_test.go`:

```go
package services

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

func TestQueuingAReplyEndsTheWaitAsRepliedByItsSender(t *testing.T) {
	db, messages, _, conv := recordingSetup(t)
	_, _, err := messages.SaveInbound(customerSent(conv.ID, "3EB0A", time.Now()))
	require.NoError(t, err)
	cs := uuid.New()

	reply, err := messages.Queue(conv.ID, cs, models.MessageKindText, "sudah kami cek", nil, nil)
	require.NoError(t, err)

	waits := storedWaits(t, db)
	require.Len(t, waits, 1)
	require.NotNil(t, waits[0].EndReason)
	assert.Equal(t, models.WaitReplied, *waits[0].EndReason)
	assert.Equal(t, &cs, waits[0].EndedBy)
	assert.Equal(t, &reply.ID, waits[0].ReplyMessageID)
}

// Closing a hard thread instead of answering it is the one way out that leaves
// no reply, so the report has to name whoever took it.
func TestClosingAThreadEndsTheWaitAsClosedByWhoeverClosedIt(t *testing.T) {
	db, messages, conversations, conv := recordingSetup(t)
	_, _, err := messages.SaveInbound(customerSent(conv.ID, "3EB0A", time.Now()))
	require.NoError(t, err)
	cs := uuid.New()

	require.NoError(t, conversations.Close(conv.ID, cs))

	waits := storedWaits(t, db)
	require.Len(t, waits, 1)
	require.NotNil(t, waits[0].EndReason)
	assert.Equal(t, models.WaitClosed, *waits[0].EndReason)
	assert.Equal(t, &cs, waits[0].EndedBy)
}

func TestAReplyAndACloseStillHappenWhenTheirWaitCannotBeRecorded(t *testing.T) {
	db, messages, conversations, conv := recordingSetup(t)
	_, _, err := messages.SaveInbound(customerSent(conv.ID, "3EB0A", time.Now()))
	require.NoError(t, err)
	require.NoError(t, db.Migrator().DropTable(&models.CSWait{}))

	_, err = messages.Queue(conv.ID, uuid.New(), models.MessageKindText, "sudah kami cek", nil, nil)
	require.NoError(t, err)
	require.NoError(t, conversations.Close(conv.ID, uuid.New()))

	got, err := conversations.Get(conv.ID)
	require.NoError(t, err)
	assert.Equal(t, models.ConversationClosed, got.Status)
}

func TestClosingAThreadThatIsNotThereStillSaysSo(t *testing.T) {
	_, _, conversations, _ := recordingSetup(t)

	err := conversations.Close(uuid.New(), uuid.New())

	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

// The figures must survive the deletions the inbox allows, or a slow answer
// could be erased by deleting what it was measured from — and removing a
// number must not be refused by rows that outlive it, the way the broadcast
// history once refused it.
func TestDeletingMessagesAThreadOrANumberLeavesTheWaitsAlone(t *testing.T) {
	db, messages, _, conv := recordingSetup(t)
	_, _, err := messages.SaveInbound(customerSent(conv.ID, "3EB0A", time.Now()))
	require.NoError(t, err)
	_, err = messages.Queue(conv.ID, uuid.New(), models.MessageKindText, "sudah kami cek", nil, nil)
	require.NoError(t, err)
	purge := NewCSPurgeService(db, t.TempDir())

	_, err = purge.Conversation(conv.ID)
	require.NoError(t, err)
	require.NoError(t, purge.DeleteAccount(conv.WAAccountID))

	assert.Len(t, storedWaits(t, db), 1, "the wait outlives the messages and the number")
	var threads int64
	require.NoError(t, db.Model(&models.CSConversation{}).Count(&threads).Error)
	assert.Zero(t, threads, "the number and its threads really are gone")
}
```

Buat `backend/internal/services/cs_wait_postgres_test.go`:

```go
package services

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// assertWaitMatchesThread checks the rule the recording keeps: a thread whose
// last word is the customer's has exactly one open wait, and a thread we
// answered last has none. A message folded into a wait somebody closed at the
// same moment would break it, and nobody would be recorded as waiting.
func assertWaitMatchesThread(t *testing.T, db *gorm.DB, conversationID uuid.UUID) {
	t.Helper()
	var conv models.CSConversation
	require.NoError(t, db.First(&conv, "id = ?", conversationID).Error)
	var open int64
	require.NoError(t, db.Model(&models.CSWait{}).
		Where("conversation_id = ? AND ended_at IS NULL", conversationID).Count(&open).Error)

	if conv.LastMessageDirection == models.MessageIn {
		assert.Equal(t, int64(1), open, "a customer with the last word is waiting")
		return
	}
	assert.Equal(t, int64(0), open, "a thread we answered last waits on nobody")
}

// SQLite serialises every write, so only Postgres can show a customer's message
// and a CS's reply landing together.
func TestAMessageAndAReplyArrivingTogetherNeverLoseAWaitOnPostgres(t *testing.T) {
	db := setupPostgresTestDB(t)
	conversations := NewCSConversationService(db)
	messages := NewCSMessageService(db, conversations)
	account := csAccount(t, db)
	agent := postgresAgent(t, db, "waitagent")

	for i := 0; i < 20; i++ {
		number := fmt.Sprintf("62811100%04d", i)
		conv, err := conversations.FindOrCreate(IncomingPeer{
			WAAccountID: account.ID, JID: number + "@s.whatsapp.net", Phone: number, Name: "Pelanggan",
		})
		require.NoError(t, err)
		_, _, err = messages.SaveInbound(InboundMessage{
			ConversationID: conv.ID, WAMessageID: fmt.Sprintf("3EB0FIRST%d", i),
			Kind: models.MessageKindText, Body: "halo", At: time.Now(),
		})
		require.NoError(t, err)

		start := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			_, _, err := messages.SaveInbound(InboundMessage{
				ConversationID: conv.ID, WAMessageID: fmt.Sprintf("3EB0AGAIN%d", i),
				Kind: models.MessageKindText, Body: "masih mati", At: time.Now(),
			})
			assert.NoError(t, err)
		}()
		go func() {
			defer wg.Done()
			<-start
			_, err := messages.Queue(conv.ID, agent, models.MessageKindText, "sudah kami cek", nil, nil)
			assert.NoError(t, err)
		}()
		close(start)
		wg.Wait()

		assertWaitMatchesThread(t, db, conv.ID)
	}
}
```

Buat `backend/internal/api/cs_handler_close_wait_test.go`:

```go
package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
)

// Closing through the inbox is how a thread ends without a reply, and the
// report names whoever did it.
func TestClosingAThreadThroughTheInboxRecordsWhoClosedIt(t *testing.T) {
	env := setupCSHandler(t)
	conv, err := env.conversations.FindOrCreate(services.IncomingPeer{
		WAAccountID: env.account.ID, JID: "628123456789@s.whatsapp.net",
		Phone: "628123456789", Name: "Pak Budi",
	})
	require.NoError(t, err)
	_, _, err = env.messages.SaveInbound(services.InboundMessage{
		ConversationID: conv.ID, WAMessageID: "3EB0CLOSE",
		Kind: models.MessageKindText, Body: "halo", At: time.Now(),
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPut,
		"/api/v1/cs/conversations/"+conv.ID.String()+"/status",
		strings.NewReader(`{"status":"closed"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.asUser(env.cs, models.UserRoleCS).ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var wait models.CSWait
	require.NoError(t, env.db.First(&wait, "conversation_id = ?", conv.ID).Error)
	require.NotNil(t, wait.EndReason)
	assert.Equal(t, models.WaitClosed, *wait.EndReason)
	assert.Equal(t, &env.cs, wait.EndedBy)
}
```

- [ ] **Step 2: Jalankan tes, pastikan gagal**

Run: `cd backend && go test ./internal/services/ ./internal/api/ -run 'Queuing|Closing' -v 2>&1 | head -20`
Expected: FAIL saat kompilasi: `not enough arguments in call to conversations.Close`.

- [ ] **Step 3: Catat giliran saat balasan diantrekan**

Di `backend/internal/services/cs_message_service.go`, di dalam transaksi `Queue`, ganti

```go
		return s.conversations.touchTx(tx, conversationID, msg.WATimestamp)
```

dengan

```go
		if err := s.conversations.touchTx(tx, conversationID, msg.WATimestamp); err != nil {
			return err
		}
		s.conversations.waits.record(tx, conversationID, "reply", func(tx *gorm.DB) error {
			return replied(tx, &msg)
		})
		return nil
```

- [ ] **Step 4: Catat giliran saat thread ditutup**

Di `backend/internal/services/cs_conversation_service.go`, ganti `Close` dengan:

```go
// Close marks a conversation finished and ends the customer's wait, if there is
// one, as closed by closedBy. The holder stays on the row, so the history still
// says who dealt with it.
func (s *CSConversationService) Close(conversationID, closedBy uuid.UUID) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := updateConversation(tx, conversationID, map[string]any{"status": models.ConversationClosed}); err != nil {
			return err
		}
		s.waits.record(tx, conversationID, "close", func(tx *gorm.DB) error {
			return closedWithoutReply(tx, conversationID, closedBy, time.Now())
		})
		return nil
	})
}
```

Di `backend/internal/api/cs_handler_conversations.go` (`SetStatus`), ganti

```go
	if err := h.conversations.Close(convID); err != nil {
```

dengan

```go
	userID, _ := middleware.GetUserID(c)
	if err := h.conversations.Close(convID, userID); err != nil {
```

Perbarui keenam pemanggil di tes menjadi `Close(<id>, uuid.New())`:
- `cs_conversation_service_test.go:88` — `require.NoError(t, svc.Close(conv.ID, uuid.New()))`
- `cs_claim_test.go:75`, `cs_awaiting_reply_test.go:80`, `cs_awaiting_reply_test.go:130`, `cs_message_phone_test.go:82` — `require.NoError(t, conversations.Close(conv.ID, uuid.New()))`
- `cs_claim_postgres_test.go:106` — `require.NoError(t, conversations.Close(closed, uuid.New()))`

Bila salah satu file itu belum mengimpor `github.com/google/uuid`, tambahkan.

- [ ] **Step 5: Pasang logger di proses `api`**

Di `backend/internal/api/router_handlers.go` (fungsi `newCSStack`), tepat setelah baris `csConversationService := services.NewCSConversationService(db)`, tambahkan:

```go
	csConversationService.SetWaitLogger(logger)
```

- [ ] **Step 6: Jalankan tes, pastikan lulus**

Run: `cd backend && go test ./internal/services/ -run 'Queuing|Closing|StillHappenWhen|DeletingMessagesAThread' -v`
Expected: 5 tes PASS.

Run: `cd backend && go test ./internal/api/ -run 'ClosingAThreadThroughTheInbox' -v`
Expected: PASS.

Jalankan kontainer TimescaleDB sementara, lalu:

Run: `cd backend && TEST_POSTGRES_DSN="host=localhost port=5439 user=postgres password=test dbname=tikman_test sslmode=disable" go test ./internal/services/ -run 'OnPostgres' -v`
Expected: PASS, termasuk `TestAMessageAndAReplyArrivingTogetherNeverLoseAWaitOnPostgres`. Hentikan kontainer sesudahnya.

Run: `cd backend && go test ./internal/... 2>&1 | grep -E '^(ok|FAIL)'`
Expected: hanya `internal/api` dan `internal/services` yang FAIL, dan hanya karena tes WireGuard baseline.

- [ ] **Step 7: Commit**

```bash
cd /Users/rohadimraja/Documents/tikman
git add backend/internal/services/cs_message_service.go backend/internal/services/cs_conversation_service.go \
        backend/internal/services/cs_wait_reply_test.go backend/internal/services/cs_wait_postgres_test.go \
        backend/internal/services/cs_conversation_service_test.go backend/internal/services/cs_claim_test.go \
        backend/internal/services/cs_awaiting_reply_test.go backend/internal/services/cs_message_phone_test.go \
        backend/internal/services/cs_claim_postgres_test.go \
        backend/internal/api/cs_handler_conversations.go backend/internal/api/router_handlers.go \
        backend/internal/api/cs_handler_close_wait_test.go
git commit -m "$(cat <<'EOF'
feat(cs): record who answered a wait and who closed one

Queuing a reply ends the customer's wait in the same transaction, naming
the CS who sent it, and closing a thread ends it as closed by whoever
closed it — which is what keeps closing a hard thread from being invisible.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 5: Perhitungan laporan

**Files:**
- Create: `backend/internal/services/cs_performance_calc.go`
- Test: `backend/internal/services/cs_performance_calc_test.go`

**Interfaces:**
- Consumes: `models.CSWait` (Task 1); `wib` (`worker_alert.go`); `ErrValidation` (`wireguard_validate.go`).
- Produces (dipakai Task 6 dan 7):
  - `const csReplyTargetMinutes = 15`, `csSessionGap = 3 * time.Hour`, `csSystemDelay = 30 * time.Minute`, `csSessionLookback = 24 * time.Hour`, `csReportMaxDays = 366`
  - `type WaitStats struct { Count int; MedianMinutes, P90Minutes, WithinTargetPct *float64 }` (JSON: `count`, `median_minutes`, `p90_minutes`, `within_target_pct`)
  - `type ReportRange struct { From, To time.Time }` dengan `func ReportRangeFromDates(from, to string) (ReportRange, error)` dan metode `dates() []string`
  - `func summarizeMinutes(minutes []float64) WaitStats`
  - `func percentile(sorted []float64, p float64) float64`
  - `func systemDelayed(w models.CSWait) bool`, `func answered(w models.CSWait) bool`
  - `func teamMinutes(w models.CSWait) float64`, `func countedMinutes(w models.CSWait, sessionStart time.Time) float64`
  - `func sessionStarts(replies []time.Time) []time.Time`
  - `func reportDate(t time.Time) string`

- [ ] **Step 1: Tulis tes yang gagal**

Buat `backend/internal/services/cs_performance_calc_test.go`:

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

// endedWait is a wait that started and ended at chosen moments, sent whenever
// the customer sent it.
func endedWait(sent, started, ended time.Time) models.CSWait {
	reason := models.WaitReplied
	return models.CSWait{
		ID: uuid.New(), CustomerSentAt: sent, StartedAt: started,
		EndedAt: &ended, EndReason: &reason,
	}
}

// The figures must be checkable by hand against the same query in Postgres.
// These are what percentile_cont gives for these four numbers.
func TestPercentileInterpolatesLikePercentileCont(t *testing.T) {
	sorted := []float64{1.5, 10, 30, 600}

	assert.InDelta(t, 20.0, percentile(sorted, 0.5), 1e-9)
	assert.InDelta(t, 172.5, percentile(sorted, 0.75), 1e-9)
	assert.InDelta(t, 429.0, percentile(sorted, 0.9), 1e-9)
	assert.InDelta(t, 5.0, percentile([]float64{5}, 0.9), 1e-9)
}

func TestSummarizeMinutesCountsTheTargetInclusively(t *testing.T) {
	stats := summarizeMinutes([]float64{3, 15, 15.5})

	assert.Equal(t, 3, stats.Count)
	require.NotNil(t, stats.MedianMinutes)
	assert.InDelta(t, 15.0, *stats.MedianMinutes, 1e-9)
	require.NotNil(t, stats.WithinTargetPct)
	assert.InDelta(t, 200.0/3, *stats.WithinTargetPct, 1e-9, "15 minutes exactly is on time")
}

// A day nobody was answered is not a day everyone was answered at once.
func TestSummarizeMinutesOfNothingHasNoFigures(t *testing.T) {
	stats := summarizeMinutes(nil)

	assert.Equal(t, 0, stats.Count)
	assert.Nil(t, stats.MedianMinutes)
	assert.Nil(t, stats.P90Minutes)
	assert.Nil(t, stats.WithinTargetPct)
}

func TestSystemDelayedIsMoreThanHalfAnHourLate(t *testing.T) {
	sent := time.Date(2026, 9, 10, 1, 0, 0, 0, time.UTC)

	assert.False(t, systemDelayed(endedWait(sent, sent.Add(30*time.Minute), sent.Add(time.Hour))))
	assert.True(t, systemDelayed(endedWait(sent, sent.Add(31*time.Minute), sent.Add(time.Hour))))
}

// A phone reply sent before the message reached TikMan would otherwise count as
// a negative wait.
func TestTeamMinutesNeverGoNegative(t *testing.T) {
	started := time.Date(2026, 9, 10, 1, 0, 0, 0, time.UTC)

	assert.InDelta(t, 0.0, teamMinutes(endedWait(started, started, started.Add(-time.Minute))), 1e-9)
	assert.InDelta(t, 5.0, teamMinutes(endedWait(started, started, started.Add(5*time.Minute))), 1e-9)
}

func TestSessionStartsBreakAtAPauseOfThreeHoursOrMore(t *testing.T) {
	base := time.Date(2026, 9, 10, 1, 0, 0, 0, time.UTC)
	replies := []time.Time{
		base,                             // 08:00 WIB, the first of the day
		base.Add(30 * time.Minute),       // half an hour later, same stretch
		base.Add(3*time.Hour + 29*time.Minute), // 2h59m after that, still the same
		base.Add(6*time.Hour + 29*time.Minute), // exactly three hours later: a new one
	}

	starts := sessionStarts(replies)

	require.Len(t, starts, 4)
	assert.True(t, starts[0].Equal(base))
	assert.True(t, starts[1].Equal(base))
	assert.True(t, starts[2].Equal(base))
	assert.True(t, starts[3].Equal(replies[3]))
}

func TestCountedMinutesStartAtTheLaterOfTheWaitAndTheStretch(t *testing.T) {
	night := time.Date(2026, 9, 9, 16, 0, 0, 0, time.UTC)
	stretch := time.Date(2026, 9, 10, 1, 0, 0, 0, time.UTC)

	overnight := endedWait(night, night, stretch.Add(10*time.Minute))
	assert.InDelta(t, 10.0, countedMinutes(overnight, stretch), 1e-9, "the night is not the morning CS's")

	later := endedWait(stretch.Add(5*time.Minute), stretch.Add(5*time.Minute), stretch.Add(10*time.Minute))
	assert.InDelta(t, 5.0, countedMinutes(later, stretch), 1e-9)
}

func TestAReportRangeCoversWholeWIBDays(t *testing.T) {
	r, err := ReportRangeFromDates("2026-09-10", "2026-09-11")
	require.NoError(t, err)

	assert.True(t, r.From.Equal(time.Date(2026, 9, 9, 17, 0, 0, 0, time.UTC)), "00:00 WIB on the 10th")
	assert.True(t, r.To.Equal(time.Date(2026, 9, 11, 17, 0, 0, 0, time.UTC)), "00:00 WIB on the 12th")
	assert.Equal(t, []string{"2026-09-10", "2026-09-11"}, r.dates())
}

func TestAReportRangeRefusesWhatItCannotRead(t *testing.T) {
	cases := []struct {
		name     string
		from, to string
	}{
		{"not a date", "2026-9-10", "2026-09-10"},
		{"nothing at all", "", ""},
		{"ends before it starts", "2026-09-11", "2026-09-10"},
		{"longer than a year", "2026-01-01", "2027-01-02"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := ReportRangeFromDates(c.from, c.to)

			assert.ErrorIs(t, err, ErrValidation)
		})
	}

	_, err := ReportRangeFromDates("2026-01-01", "2027-01-01")
	assert.NoError(t, err, "366 days is the longest report allowed")
}

func TestReportDateIsTheWIBCalendarDay(t *testing.T) {
	assert.Equal(t, "2026-09-10", reportDate(time.Date(2026, 9, 10, 16, 59, 0, 0, time.UTC)), "23:59 WIB")
	assert.Equal(t, "2026-09-11", reportDate(time.Date(2026, 9, 10, 17, 1, 0, 0, time.UTC)), "00:01 WIB")
}
```

- [ ] **Step 2: Jalankan tes, pastikan gagal**

Run: `cd backend && go test ./internal/services/ -run 'Percentile|SummarizeMinutes|SystemDelayed|TeamMinutes|SessionStarts|CountedMinutes|ReportRange|ReportDate' -v 2>&1 | head -20`
Expected: FAIL saat kompilasi: `undefined: percentile`, `undefined: summarizeMinutes`, dan seterusnya.

- [ ] **Step 3: Tulis perhitungannya**

Buat `backend/internal/services/cs_performance_calc.go`:

```go
package services

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/tikman/olt-provisioning/internal/models"
)

const (
	// csReplyTargetMinutes is the benchmark the team is measured against: an
	// answer within this many minutes counts as on time.
	csReplyTargetMinutes = 15
	// csSessionGap separates two stretches of one CS's work. Measured on
	// production, the pauses between a CS's own replies fall either under two
	// hours or over six, so three hours splits them cleanly.
	csSessionGap = 3 * time.Hour
	// csSystemDelay is how late a message may reach TikMan before the waiting is
	// the system's doing rather than anyone's slowness.
	csSystemDelay = 30 * time.Minute
	// csSessionLookback is how far before a report's first reply a CS's earlier
	// replies are read, to find where that stretch of work began. Nobody works a
	// whole day without a three-hour pause.
	csSessionLookback = 24 * time.Hour
	// csReportMaxDays bounds one report to a year.
	csReportMaxDays = 366
)

// WaitStats is how fast a set of waits was answered, in minutes. The figures are
// pointers because a set with nothing in it has no median, and reporting zero
// would read as "answered instantly".
type WaitStats struct {
	Count           int      `json:"count"`
	MedianMinutes   *float64 `json:"median_minutes"`
	P90Minutes      *float64 `json:"p90_minutes"`
	WithinTargetPct *float64 `json:"within_target_pct"`
}

// summarizeMinutes reduces a set of waits to the figures the report shows.
func summarizeMinutes(minutes []float64) WaitStats {
	stats := WaitStats{Count: len(minutes)}
	if len(minutes) == 0 {
		return stats
	}
	sorted := append([]float64(nil), minutes...)
	sort.Float64s(sorted)

	within := 0
	for _, m := range sorted {
		if m <= csReplyTargetMinutes {
			within++
		}
	}
	median := percentile(sorted, 0.5)
	p90 := percentile(sorted, 0.9)
	pct := 100 * float64(within) / float64(len(sorted))
	stats.MedianMinutes, stats.P90Minutes, stats.WithinTargetPct = &median, &p90, &pct
	return stats
}

// percentile interpolates linearly between the two nearest ranks, the way
// Postgres's percentile_cont does, so a figure here can be checked against a
// query run by hand. sorted must be ascending and hold at least one value.
func percentile(sorted []float64, p float64) float64 {
	position := p * float64(len(sorted)-1)
	lower := int(math.Floor(position))
	if lower+1 >= len(sorted) {
		return sorted[len(sorted)-1]
	}
	return sorted[lower] + (position-float64(lower))*(sorted[lower+1]-sorted[lower])
}

// systemDelayed marks a wait whose first message reached TikMan long after the
// customer sent it: nobody could answer what had not arrived.
func systemDelayed(w models.CSWait) bool {
	return w.StartedAt.Sub(w.CustomerSentAt) > csSystemDelay
}

// answered says whether a wait ended with the customer getting an answer.
func answered(w models.CSWait) bool {
	return w.EndReason != nil && (*w.EndReason == models.WaitReplied || *w.EndReason == models.WaitPhone)
}

// teamMinutes is how long the customer waited once TikMan held their message.
func teamMinutes(w models.CSWait) float64 {
	return minutesBetween(w.StartedAt, *w.EndedAt)
}

// countedMinutes is the part of a wait charged to the CS who answered it: from
// the later of the wait's start and the start of their stretch of work.
func countedMinutes(w models.CSWait, sessionStart time.Time) float64 {
	from := w.StartedAt
	if sessionStart.After(from) {
		from = sessionStart
	}
	return minutesBetween(from, *w.EndedAt)
}

func minutesBetween(from, to time.Time) float64 {
	return math.Max(0, to.Sub(from).Minutes())
}

// sessionStarts answers, for each of one CS's replies in ascending order, when
// the stretch of work it belongs to began: the first reply after a pause of
// csSessionGap or more.
func sessionStarts(replies []time.Time) []time.Time {
	starts := make([]time.Time, len(replies))
	for i, at := range replies {
		if i == 0 || at.Sub(replies[i-1]) >= csSessionGap {
			starts[i] = at
			continue
		}
		starts[i] = starts[i-1]
	}
	return starts
}

// reportDate is the WIB calendar day a moment falls on. The report's days are
// the days the team works, not UTC days.
func reportDate(t time.Time) string {
	return t.In(wib).Format(time.DateOnly)
}

// ReportRange is the span one report covers, half open: [From, To).
type ReportRange struct {
	From time.Time
	To   time.Time
}

// ReportRangeFromDates reads the two inclusive WIB dates the page sends into the
// span they cover.
func ReportRangeFromDates(from, to string) (ReportRange, error) {
	start, err := time.ParseInLocation(time.DateOnly, from, wib)
	if err != nil {
		return ReportRange{}, fmt.Errorf("%w: tanggal awal harus berformat YYYY-MM-DD", ErrValidation)
	}
	end, err := time.ParseInLocation(time.DateOnly, to, wib)
	if err != nil {
		return ReportRange{}, fmt.Errorf("%w: tanggal akhir harus berformat YYYY-MM-DD", ErrValidation)
	}
	if end.Before(start) {
		return ReportRange{}, fmt.Errorf("%w: tanggal akhir tidak boleh sebelum tanggal awal", ErrValidation)
	}
	r := ReportRange{From: start, To: end.AddDate(0, 0, 1)}
	if r.days() > csReportMaxDays {
		return ReportRange{}, fmt.Errorf("%w: rentang laporan paling panjang %d hari", ErrValidation, csReportMaxDays)
	}
	return r, nil
}

func (r ReportRange) days() int {
	return int(r.To.Sub(r.From).Hours() / 24)
}

// dates lists every WIB day the range covers, so a chart can show a day nothing
// happened on as a gap rather than leaving it out.
func (r ReportRange) dates() []string {
	var days []string
	for day := r.From; day.Before(r.To); day = day.AddDate(0, 0, 1) {
		days = append(days, reportDate(day))
	}
	return days
}
```

- [ ] **Step 4: Jalankan tes, pastikan lulus**

Run: `cd backend && go test ./internal/services/ -run 'Percentile|SummarizeMinutes|SystemDelayed|TeamMinutes|SessionStarts|CountedMinutes|ReportRange|ReportDate' -v`
Expected: semua PASS.

Run: `cd backend && gofmt -s -l internal/services && go vet ./internal/services/`
Expected: kosong dan bersih.

- [ ] **Step 5: Commit**

```bash
cd /Users/rohadimraja/Documents/tikman
git add backend/internal/services/cs_performance_calc.go backend/internal/services/cs_performance_calc_test.go
git commit -m "$(cat <<'EOF'
feat(cs): work out the figures a wait report is made of

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 6: Query laporan

**Files:**
- Create: `backend/internal/services/cs_performance_service.go` (ringkasan)
- Create: `backend/internal/services/cs_performance_waits.go` (daftar giliran)
- Test: `backend/internal/services/cs_performance_service_test.go`

**Interfaces:**
- Consumes: semua yang dihasilkan Task 5; `models.CSWait`, `models.CSConversation`, `models.User`.
- Produces (dipakai Task 7):
  - `func NewCSPerformanceService(db *gorm.DB) *CSPerformanceService`
  - `func (s *CSPerformanceService) Summary(r ReportRange, onlyAgent *uuid.UUID, now time.Time) (*PerformanceSummary, error)`
  - `func (s *CSPerformanceService) Waits(r ReportRange, endedBy *uuid.UUID, limit, offset int) (*WaitList, error)`
  - `type PerformanceSummary struct { TargetMinutes int; Team TeamPerformance; Agents []AgentPerformance; Days []DayPerformance; Waiting WaitingNow }`
  - `type TeamPerformance struct { Replies WaitStats; ClosedWithoutReply, Abandoned, SystemDelayed int }`
  - `type AgentPerformance struct { UserID uuid.UUID; Username string; Replies WaitStats; ClosedWithoutReply int }`
  - `type DayPerformance struct { Date string; Replies WaitStats }`
  - `type WaitingNow struct { Count int; LongestMinutes *float64 }`
  - `type WaitListItem struct { ... }` dan `type WaitList struct { Items []WaitListItem; Total int64 }`

- [ ] **Step 1: Tulis tes yang gagal**

Buat `backend/internal/services/cs_performance_service_test.go`:

```go
package services

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// reportFixture plants waits the way the recording would have left them. Every
// time is UTC: SQLite compares stored times as text, so they must share a zone.
type reportFixture struct {
	t    *testing.T
	db   *gorm.DB
	svc  *CSPerformanceService
	conv *models.CSConversation
	ani  uuid.UUID
	budi uuid.UUID
}

func newReportFixture(t *testing.T) *reportFixture {
	t.Helper()
	db := setupTestDB(t)
	conv, err := NewCSConversationService(db).FindOrCreate(peer(csAccount(t, db).ID))
	require.NoError(t, err)
	return &reportFixture{
		t: t, db: db, svc: NewCSPerformanceService(db), conv: conv,
		ani: reportUser(t, db, "ani"), budi: reportUser(t, db, "budi"),
	}
}

func reportUser(t *testing.T, db *gorm.DB, username string) uuid.UUID {
	t.Helper()
	user := models.User{
		ID: uuid.New(), Username: username, Email: username + "@example.test", Role: models.UserRoleCS,
	}
	require.NoError(t, db.Create(&user).Error)
	return user.ID
}

// sept is a moment in September 2026, in UTC. WIB is seven hours ahead, so
// sept(10, 1, 0) is 08:00 WIB on the 10th.
func sept(day, hour, minute int) time.Time {
	return time.Date(2026, 9, day, hour, minute, 0, 0, time.UTC)
}

// planted describes one wait to plant. An empty thread means the fixture's own.
type planted struct {
	started, sent, ended time.Time
	reason               models.WaitEndReason
	by                   *uuid.UUID
	thread               uuid.UUID
}

func (f *reportFixture) plant(p planted) {
	f.t.Helper()
	if p.thread == uuid.Nil {
		p.thread = f.conv.ID
	}
	wait := models.CSWait{
		ConversationID: p.thread, WAAccountID: f.conv.WAAccountID,
		StartedAt: p.started, CustomerSentAt: p.sent, LastCustomerSentAt: p.sent,
		EndedAt: &p.ended, EndReason: &p.reason, EndedBy: p.by,
	}
	if p.reason == models.WaitReplied || p.reason == models.WaitPhone {
		message := uuid.New()
		wait.ReplyMessageID = &message
	}
	require.NoError(f.t, f.db.Create(&wait).Error)
}

func (f *reportFixture) plantOpen(thread uuid.UUID, started time.Time) {
	f.t.Helper()
	require.NoError(f.t, f.db.Create(&models.CSWait{
		ConversationID: thread, WAAccountID: f.conv.WAAccountID,
		StartedAt: started, CustomerSentAt: started, LastCustomerSentAt: started,
	}).Error)
}

func reportRangeFor(t *testing.T, from, to string) ReportRange {
	t.Helper()
	r, err := ReportRangeFromDates(from, to)
	require.NoError(t, err)
	return r
}

func TestTheTeamFiguresCountOnlyAnswersThatWereNotTheSystemsFault(t *testing.T) {
	f := newReportFixture(t)
	f.plant(planted{started: sept(10, 1, 0), sent: sept(10, 1, 0), ended: sept(10, 1, 5), reason: models.WaitReplied, by: &f.ani})
	f.plant(planted{started: sept(10, 2, 0), sent: sept(10, 2, 0), ended: sept(10, 2, 3), reason: models.WaitPhone})
	f.plant(planted{started: sept(10, 3, 0), sent: sept(10, 2, 0), ended: sept(10, 3, 1), reason: models.WaitReplied, by: &f.budi})
	f.plant(planted{started: sept(10, 4, 0), sent: sept(10, 4, 0), ended: sept(10, 4, 30), reason: models.WaitClosed, by: &f.budi})
	f.plant(planted{started: sept(10, 5, 0), sent: sept(10, 5, 0), ended: sept(10, 6, 0), reason: models.WaitAbandoned})
	// 01:00 WIB on the 11th, outside the report for the 10th.
	f.plant(planted{started: sept(10, 17, 55), sent: sept(10, 17, 55), ended: sept(10, 18, 0), reason: models.WaitReplied, by: &f.ani})

	summary, err := f.svc.Summary(reportRangeFor(t, "2026-09-10", "2026-09-10"), nil, sept(10, 12, 0))
	require.NoError(t, err)

	assert.Equal(t, csReplyTargetMinutes, summary.TargetMinutes)
	assert.Equal(t, 2, summary.Team.Replies.Count, "one reply and one phone answer")
	require.NotNil(t, summary.Team.Replies.MedianMinutes)
	assert.InDelta(t, 4.0, *summary.Team.Replies.MedianMinutes, 1e-9)
	assert.Equal(t, 1, summary.Team.SystemDelayed)
	assert.Equal(t, 1, summary.Team.ClosedWithoutReply)
	assert.Equal(t, 1, summary.Team.Abandoned)
}

// Ani starts work at 08:00 WIB with a customer who wrote at 01:00 WIB. That
// first answer costs her nothing; the customer who wrote at 07:30 WIB is
// counted from 08:00, when she started.
func TestACSIsChargedFromWhenTheirStretchOfWorkBegan(t *testing.T) {
	f := newReportFixture(t)
	f.plant(planted{started: sept(9, 18, 0), sent: sept(9, 18, 0), ended: sept(10, 1, 0), reason: models.WaitReplied, by: &f.ani})
	f.plant(planted{started: sept(10, 0, 30), sent: sept(10, 0, 30), ended: sept(10, 1, 10), reason: models.WaitReplied, by: &f.ani})

	summary, err := f.svc.Summary(reportRangeFor(t, "2026-09-10", "2026-09-10"), nil, sept(10, 12, 0))
	require.NoError(t, err)

	require.Len(t, summary.Agents, 1)
	assert.Equal(t, "ani", summary.Agents[0].Username)
	assert.Equal(t, 2, summary.Agents[0].Replies.Count)
	require.NotNil(t, summary.Agents[0].Replies.MedianMinutes)
	assert.InDelta(t, 5.0, *summary.Agents[0].Replies.MedianMinutes, 1e-9, "0 and 10 minutes")
	require.NotNil(t, summary.Team.Replies.MedianMinutes)
	assert.InDelta(t, 230.0, *summary.Team.Replies.MedianMinutes, 1e-9, "the team still waited 420 and 40 minutes")
}

// Ani's stretch began at 23:30 WIB on the 9th, before the report for the 10th
// starts. Her answer at 00:10 WIB still counts from when the customer wrote.
func TestAStretchOfWorkThatBeganBeforeTheReportIsStillRecognised(t *testing.T) {
	f := newReportFixture(t)
	f.plant(planted{started: sept(9, 16, 20), sent: sept(9, 16, 20), ended: sept(9, 16, 30), reason: models.WaitReplied, by: &f.ani})
	f.plant(planted{started: sept(9, 16, 40), sent: sept(9, 16, 40), ended: sept(9, 17, 10), reason: models.WaitReplied, by: &f.ani})

	summary, err := f.svc.Summary(reportRangeFor(t, "2026-09-10", "2026-09-10"), nil, sept(10, 12, 0))
	require.NoError(t, err)

	require.Len(t, summary.Agents, 1)
	require.NotNil(t, summary.Agents[0].Replies.MedianMinutes)
	assert.InDelta(t, 30.0, *summary.Agents[0].Replies.MedianMinutes, 1e-9,
		"without the lookback her stretch would start at this very reply, and the answer would cost nothing")
}

func TestACSSeesTheWholeTeamButOnlyTheirOwnRow(t *testing.T) {
	f := newReportFixture(t)
	f.plant(planted{started: sept(10, 1, 0), sent: sept(10, 1, 0), ended: sept(10, 1, 5), reason: models.WaitReplied, by: &f.ani})
	f.plant(planted{started: sept(10, 2, 0), sent: sept(10, 2, 0), ended: sept(10, 2, 5), reason: models.WaitReplied, by: &f.budi})

	summary, err := f.svc.Summary(reportRangeFor(t, "2026-09-10", "2026-09-10"), &f.ani, sept(10, 12, 0))
	require.NoError(t, err)

	require.Len(t, summary.Agents, 1)
	assert.Equal(t, f.ani, summary.Agents[0].UserID)
	assert.Equal(t, 2, summary.Team.Replies.Count)
}

// Closing hard threads instead of answering them must show up somewhere.
func TestACSWhoOnlyClosedThreadsStillHasARow(t *testing.T) {
	f := newReportFixture(t)
	f.plant(planted{started: sept(10, 4, 0), sent: sept(10, 4, 0), ended: sept(10, 4, 30), reason: models.WaitClosed, by: &f.budi})

	summary, err := f.svc.Summary(reportRangeFor(t, "2026-09-10", "2026-09-10"), nil, sept(10, 12, 0))
	require.NoError(t, err)

	require.Len(t, summary.Agents, 1)
	assert.Equal(t, "budi", summary.Agents[0].Username)
	assert.Equal(t, 1, summary.Agents[0].ClosedWithoutReply)
	assert.Equal(t, 0, summary.Agents[0].Replies.Count)
	assert.Nil(t, summary.Agents[0].Replies.MedianMinutes)
}

func TestEveryDayOfTheReportHasARowEvenWithNothingInIt(t *testing.T) {
	f := newReportFixture(t)
	f.plant(planted{started: sept(10, 1, 0), sent: sept(10, 1, 0), ended: sept(10, 1, 5), reason: models.WaitReplied, by: &f.ani})

	summary, err := f.svc.Summary(reportRangeFor(t, "2026-09-09", "2026-09-11"), nil, sept(10, 12, 0))
	require.NoError(t, err)

	require.Len(t, summary.Days, 3)
	assert.Equal(t, "2026-09-09", summary.Days[0].Date)
	assert.Equal(t, 0, summary.Days[0].Replies.Count)
	assert.Nil(t, summary.Days[0].Replies.WithinTargetPct)
	assert.Equal(t, "2026-09-10", summary.Days[1].Date)
	assert.Equal(t, 1, summary.Days[1].Replies.Count)
}

// A wait on a thread that was deleted can never be answered, so it is not
// somebody still waiting.
func TestWaitingNowCountsOpenWaitsOnThreadsThatStillExist(t *testing.T) {
	f := newReportFixture(t)
	f.plantOpen(f.conv.ID, sept(10, 11, 40))
	f.plantOpen(uuid.New(), sept(10, 10, 0))

	summary, err := f.svc.Summary(reportRangeFor(t, "2026-09-10", "2026-09-10"), nil, sept(10, 12, 0))
	require.NoError(t, err)

	assert.Equal(t, 1, summary.Waiting.Count)
	require.NotNil(t, summary.Waiting.LongestMinutes)
	assert.InDelta(t, 20.0, *summary.Waiting.LongestMinutes, 1e-9)
}

func TestTheWaitListShowsNewestFirstWithItsThreadAndMinutes(t *testing.T) {
	f := newReportFixture(t)
	f.plant(planted{started: sept(10, 1, 0), sent: sept(10, 1, 0), ended: sept(10, 1, 5), reason: models.WaitReplied, by: &f.ani})
	f.plant(planted{started: sept(10, 2, 0), sent: sept(10, 2, 0), ended: sept(10, 2, 3), reason: models.WaitPhone})
	f.plant(planted{started: sept(10, 4, 0), sent: sept(10, 3, 0), ended: sept(10, 4, 30), reason: models.WaitClosed, by: &f.budi, thread: uuid.New()})

	list, err := f.svc.Waits(reportRangeFor(t, "2026-09-10", "2026-09-10"), nil, 10, 0)
	require.NoError(t, err)

	assert.EqualValues(t, 3, list.Total)
	require.Len(t, list.Items, 3)
	closed, phone, replied := list.Items[0], list.Items[1], list.Items[2]

	assert.True(t, closed.ConversationDeleted)
	assert.Equal(t, "budi", closed.EndedByUsername)
	assert.Nil(t, closed.TeamMinutes, "a thread closed without a reply has no answer to time")
	assert.True(t, closed.SystemDelayed)

	assert.False(t, phone.ConversationDeleted)
	assert.Equal(t, f.conv.CustomerName, phone.CustomerName)
	require.NotNil(t, phone.TeamMinutes)
	assert.InDelta(t, 3.0, *phone.TeamMinutes, 1e-9)
	assert.Nil(t, phone.CountedMinutes, "nobody in TikMan is charged for an answer from the phone")

	require.NotNil(t, replied.TeamMinutes)
	assert.InDelta(t, 5.0, *replied.TeamMinutes, 1e-9)
	require.NotNil(t, replied.CountedMinutes)
	assert.InDelta(t, 0.0, *replied.CountedMinutes, 1e-9, "her first answer of the stretch")
}

func TestTheWaitListForOneCSHoldsOnlyWhatTheyEnded(t *testing.T) {
	f := newReportFixture(t)
	f.plant(planted{started: sept(10, 1, 0), sent: sept(10, 1, 0), ended: sept(10, 1, 5), reason: models.WaitReplied, by: &f.ani})
	f.plant(planted{started: sept(10, 4, 0), sent: sept(10, 4, 0), ended: sept(10, 4, 30), reason: models.WaitClosed, by: &f.budi})

	list, err := f.svc.Waits(reportRangeFor(t, "2026-09-10", "2026-09-10"), &f.budi, 10, 0)
	require.NoError(t, err)

	assert.EqualValues(t, 1, list.Total)
	require.Len(t, list.Items, 1)
	assert.Equal(t, models.WaitClosed, list.Items[0].EndReason)
}

func TestTheWaitListPages(t *testing.T) {
	f := newReportFixture(t)
	for minute := 0; minute < 3; minute++ {
		f.plant(planted{
			started: sept(10, 1, minute), sent: sept(10, 1, minute), ended: sept(10, 2, minute),
			reason: models.WaitPhone,
		})
	}

	list, err := f.svc.Waits(reportRangeFor(t, "2026-09-10", "2026-09-10"), nil, 2, 2)
	require.NoError(t, err)

	assert.EqualValues(t, 3, list.Total)
	assert.Len(t, list.Items, 1)
}
```

- [ ] **Step 2: Jalankan tes, pastikan gagal**

Run: `cd backend && go test ./internal/services/ -run 'TheTeamFigures|ChargedFrom|StretchOfWorkThatBegan|SeesTheWholeTeam|OnlyClosedThreads|EveryDayOfTheReport|WaitingNowCounts|TheWaitList' -v 2>&1 | head -20`
Expected: FAIL saat kompilasi: `undefined: NewCSPerformanceService`.

- [ ] **Step 3: Tulis ringkasan laporan**

Buat `backend/internal/services/cs_performance_service.go`:

```go
package services

import (
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// CSPerformanceService answers how fast the team answers customers, from the
// waits recorded as they happened (see cs_wait.go). It computes on read: the
// stored rows are facts, and the target, the session gap and the system-delay
// threshold are how the report reads them today.
type CSPerformanceService struct {
	db *gorm.DB
}

// NewCSPerformanceService constructs a CSPerformanceService.
func NewCSPerformanceService(db *gorm.DB) *CSPerformanceService {
	return &CSPerformanceService{db: db}
}

// TeamPerformance is what the customers of one period got.
type TeamPerformance struct {
	Replies            WaitStats `json:"replies"`
	ClosedWithoutReply int       `json:"closed_without_reply"`
	Abandoned          int       `json:"abandoned"`
	SystemDelayed      int       `json:"system_delayed"`
}

// AgentPerformance is one CS's row: the replies they sent, and the threads they
// closed without sending one.
type AgentPerformance struct {
	UserID             uuid.UUID `json:"user_id"`
	Username           string    `json:"username"`
	Replies            WaitStats `json:"replies"`
	ClosedWithoutReply int       `json:"closed_without_reply"`
}

// DayPerformance is one WIB day of the period.
type DayPerformance struct {
	Date    string    `json:"date"`
	Replies WaitStats `json:"replies"`
}

// WaitingNow is how many customers are waiting at this moment, and how long the
// one at the front has waited.
type WaitingNow struct {
	Count          int      `json:"count"`
	LongestMinutes *float64 `json:"longest_minutes"`
}

// PerformanceSummary is one report.
type PerformanceSummary struct {
	TargetMinutes int                `json:"target_minutes"`
	Team          TeamPerformance    `json:"team"`
	Agents        []AgentPerformance `json:"agents"`
	Days          []DayPerformance   `json:"days"`
	Waiting       WaitingNow         `json:"waiting"`
}

// Summary reports the waits that ended within r. onlyAgent narrows the per-CS
// rows to one person — what a CS sees of the others — and leaves the team's
// figures whole.
func (s *CSPerformanceService) Summary(r ReportRange, onlyAgent *uuid.UUID, now time.Time) (*PerformanceSummary, error) {
	waits, err := s.waitsEndedIn(r)
	if err != nil {
		return nil, err
	}
	counted, err := s.countedByWait(waits)
	if err != nil {
		return nil, err
	}
	agents, err := s.agentRows(waits, counted, onlyAgent)
	if err != nil {
		return nil, err
	}
	waiting, err := s.waitingNow(now)
	if err != nil {
		return nil, err
	}
	return &PerformanceSummary{
		TargetMinutes: csReplyTargetMinutes,
		Team:          teamRow(waits),
		Agents:        agents,
		Days:          dayRows(r, waits),
		Waiting:       waiting,
	}, nil
}

// waitsEndedIn loads the waits of one period, oldest first. A wait belongs to
// the period it ended in: an answer given this morning is this morning's work,
// whenever the customer wrote.
func (s *CSPerformanceService) waitsEndedIn(r ReportRange) ([]models.CSWait, error) {
	var waits []models.CSWait
	err := s.db.Where("ended_at >= ? AND ended_at < ?", r.From.UTC(), r.To.UTC()).
		Order("ended_at ASC").Find(&waits).Error
	if err != nil {
		return nil, fmt.Errorf("load waits: %w", err)
	}
	return waits, nil
}

// teamRow sums the period as the customers experienced it.
func teamRow(waits []models.CSWait) TeamPerformance {
	var row TeamPerformance
	var minutes []float64
	for _, w := range waits {
		switch {
		case answered(w) && systemDelayed(w):
			row.SystemDelayed++
		case answered(w):
			minutes = append(minutes, teamMinutes(w))
		case *w.EndReason == models.WaitClosed:
			row.ClosedWithoutReply++
		default:
			row.Abandoned++
		}
	}
	row.Replies = summarizeMinutes(minutes)
	return row
}

// dayRows gives every WIB day of the period a row, so a day nothing was
// answered on reads as a gap in the chart rather than as a missing day.
func dayRows(r ReportRange, waits []models.CSWait) []DayPerformance {
	byDate := map[string][]float64{}
	for _, w := range waits {
		if answered(w) && !systemDelayed(w) {
			byDate[reportDate(*w.EndedAt)] = append(byDate[reportDate(*w.EndedAt)], teamMinutes(w))
		}
	}
	dates := r.dates()
	rows := make([]DayPerformance, len(dates))
	for i, date := range dates {
		rows[i] = DayPerformance{Date: date, Replies: summarizeMinutes(byDate[date])}
	}
	return rows
}

// countedByWait works out, for every TikMan reply among the waits, the minutes
// charged to the CS who sent it.
func (s *CSPerformanceService) countedByWait(waits []models.CSWait) (map[uuid.UUID]float64, error) {
	byAgent := map[uuid.UUID][]models.CSWait{}
	for _, w := range waits {
		if w.EndedBy != nil && *w.EndReason == models.WaitReplied {
			byAgent[*w.EndedBy] = append(byAgent[*w.EndedBy], w)
		}
	}
	counted := map[uuid.UUID]float64{}
	for agent, replies := range byAgent {
		if err := s.chargeAgent(agent, replies, counted); err != nil {
			return nil, err
		}
	}
	return counted, nil
}

// chargeAgent charges one CS's replies. Their replies are read back
// csSessionLookback before the earliest of them, so a stretch of work that began
// before the report still starts where it really started — otherwise the first
// answer of every report would cost nothing.
func (s *CSPerformanceService) chargeAgent(agent uuid.UUID, replies []models.CSWait, counted map[uuid.UUID]float64) error {
	first, last := *replies[0].EndedAt, *replies[0].EndedAt
	for _, w := range replies {
		if w.EndedAt.Before(first) {
			first = *w.EndedAt
		}
		if w.EndedAt.After(last) {
			last = *w.EndedAt
		}
	}

	var times []time.Time
	err := s.db.Model(&models.CSWait{}).
		Where("end_reason = ? AND ended_by = ? AND ended_at >= ? AND ended_at <= ?",
			models.WaitReplied, agent, first.Add(-csSessionLookback).UTC(), last.UTC()).
		Order("ended_at ASC").Pluck("ended_at", &times).Error
	if err != nil {
		return fmt.Errorf("load replies for sessions: %w", err)
	}

	starts := sessionStarts(times)
	for _, w := range replies {
		i := sort.Search(len(times), func(i int) bool { return !times[i].Before(*w.EndedAt) })
		if i < len(times) {
			counted[w.ID] = countedMinutes(w, starts[i])
		}
	}
	return nil
}

// agentRows builds a row for every CS who replied or closed within the waits.
func (s *CSPerformanceService) agentRows(waits []models.CSWait, counted map[uuid.UUID]float64,
	onlyAgent *uuid.UUID) ([]AgentPerformance, error) {
	minutes := map[uuid.UUID][]float64{}
	closed := map[uuid.UUID]int{}
	for _, w := range waits {
		if w.EndedBy == nil || (onlyAgent != nil && *w.EndedBy != *onlyAgent) {
			continue
		}
		agent := *w.EndedBy
		if _, seen := minutes[agent]; !seen {
			minutes[agent] = nil
		}
		if *w.EndReason == models.WaitClosed {
			closed[agent]++
		} else if !systemDelayed(w) {
			minutes[agent] = append(minutes[agent], counted[w.ID])
		}
	}
	return s.namedAgentRows(minutes, closed)
}

// namedAgentRows puts the CS's name on each row and sorts them by it, so the
// table reads the same way twice running.
func (s *CSPerformanceService) namedAgentRows(minutes map[uuid.UUID][]float64,
	closed map[uuid.UUID]int) ([]AgentPerformance, error) {
	ids := make([]uuid.UUID, 0, len(minutes))
	for id := range minutes {
		ids = append(ids, id)
	}
	names, err := s.usernames(ids)
	if err != nil {
		return nil, err
	}
	rows := make([]AgentPerformance, 0, len(ids))
	for _, id := range ids {
		rows = append(rows, AgentPerformance{
			UserID: id, Username: names[id],
			Replies: summarizeMinutes(minutes[id]), ClosedWithoutReply: closed[id],
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Username < rows[j].Username })
	return rows, nil
}

// usernames answers the names of the CS ids given. A name missing here is a
// user who has been deleted; the row still stands, because the work did.
func (s *CSPerformanceService) usernames(ids []uuid.UUID) (map[uuid.UUID]string, error) {
	names := make(map[uuid.UUID]string, len(ids))
	if len(ids) == 0 {
		return names, nil
	}
	var users []models.User
	if err := s.db.Select("id", "username").Where("id IN ?", ids).Find(&users).Error; err != nil {
		return nil, fmt.Errorf("load CS names: %w", err)
	}
	for _, user := range users {
		names[user.ID] = user.Username
	}
	return names, nil
}

// waitingNow counts the customers still waiting on threads that still exist. A
// wait left open on a deleted thread can never be answered, so counting it
// would leave a number nobody can ever bring down.
func (s *CSPerformanceService) waitingNow(now time.Time) (WaitingNow, error) {
	var started []time.Time
	err := s.db.Model(&models.CSWait{}).
		Joins("JOIN cs_conversations ON cs_conversations.id = cs_waits.conversation_id").
		Where("cs_waits.ended_at IS NULL").
		Order("cs_waits.started_at ASC").
		Pluck("cs_waits.started_at", &started).Error
	if err != nil {
		return WaitingNow{}, fmt.Errorf("load open waits: %w", err)
	}
	waiting := WaitingNow{Count: len(started)}
	if len(started) > 0 {
		longest := minutesBetween(started[0], now)
		waiting.LongestMinutes = &longest
	}
	return waiting, nil
}
```

- [ ] **Step 4: Tulis daftar giliran**

Buat `backend/internal/services/cs_performance_waits.go`:

```go
package services

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// WaitListItem is one wait as the report's list shows it: enough to find the
// thread it came from and to see how its figures were reached.
type WaitListItem struct {
	ID                  uuid.UUID            `json:"id"`
	ConversationID      uuid.UUID            `json:"conversation_id"`
	CustomerName        string               `json:"customer_name"`
	CustomerPhone       string               `json:"customer_phone"`
	ConversationDeleted bool                 `json:"conversation_deleted"`
	StartedAt           time.Time            `json:"started_at"`
	CustomerSentAt      time.Time            `json:"customer_sent_at"`
	EndedAt             time.Time            `json:"ended_at"`
	EndReason           models.WaitEndReason `json:"end_reason"`
	EndedBy             *uuid.UUID           `json:"ended_by,omitempty"`
	EndedByUsername     string               `json:"ended_by_username,omitempty"`
	TeamMinutes         *float64             `json:"team_minutes"`
	CountedMinutes      *float64             `json:"counted_minutes"`
	SystemDelayed       bool                 `json:"system_delayed"`
}

// WaitList is one page of waits, with how many there are in all.
type WaitList struct {
	Items []WaitListItem
	Total int64
}

// Waits lists the waits that ended within r, newest first. endedBy narrows the
// list to the waits one CS replied to or closed, which is all a CS may see.
func (s *CSPerformanceService) Waits(r ReportRange, endedBy *uuid.UUID, limit, offset int) (*WaitList, error) {
	within := func(db *gorm.DB) *gorm.DB {
		db = db.Where("ended_at >= ? AND ended_at < ?", r.From.UTC(), r.To.UTC())
		if endedBy != nil {
			db = db.Where("ended_by = ?", *endedBy)
		}
		return db
	}

	var total int64
	if err := s.db.Model(&models.CSWait{}).Scopes(within).Count(&total).Error; err != nil {
		return nil, fmt.Errorf("count waits: %w", err)
	}
	var page []models.CSWait
	err := s.db.Model(&models.CSWait{}).Scopes(within).
		Order("ended_at DESC").Limit(limit).Offset(offset).Find(&page).Error
	if err != nil {
		return nil, fmt.Errorf("load a page of waits: %w", err)
	}

	items, err := s.describeWaits(page)
	if err != nil {
		return nil, err
	}
	return &WaitList{Items: items, Total: total}, nil
}

// describeWaits turns stored waits into list items: the thread they belong to,
// who ended them, and their minutes.
func (s *CSPerformanceService) describeWaits(page []models.CSWait) ([]WaitListItem, error) {
	threads, err := s.threads(page)
	if err != nil {
		return nil, err
	}
	counted, err := s.countedByWait(page)
	if err != nil {
		return nil, err
	}
	names, err := s.usernames(enders(page))
	if err != nil {
		return nil, err
	}
	items := make([]WaitListItem, len(page))
	for i, w := range page {
		items[i] = waitListItem(w, threads, counted, names)
	}
	return items, nil
}

// threads loads the threads the waits point at. One that is missing was deleted
// with its number, and the wait outlives it on purpose.
func (s *CSPerformanceService) threads(page []models.CSWait) (map[uuid.UUID]models.CSConversation, error) {
	found := make(map[uuid.UUID]models.CSConversation, len(page))
	if len(page) == 0 {
		return found, nil
	}
	ids := make([]uuid.UUID, 0, len(page))
	for _, w := range page {
		ids = append(ids, w.ConversationID)
	}
	var threads []models.CSConversation
	err := s.db.Select("id", "customer_name", "customer_phone").Where("id IN ?", ids).Find(&threads).Error
	if err != nil {
		return nil, fmt.Errorf("load threads for waits: %w", err)
	}
	for _, thread := range threads {
		found[thread.ID] = thread
	}
	return found, nil
}

func enders(page []models.CSWait) []uuid.UUID {
	var ids []uuid.UUID
	for _, w := range page {
		if w.EndedBy != nil {
			ids = append(ids, *w.EndedBy)
		}
	}
	return ids
}

func waitListItem(w models.CSWait, threads map[uuid.UUID]models.CSConversation,
	counted map[uuid.UUID]float64, names map[uuid.UUID]string) WaitListItem {
	thread, exists := threads[w.ConversationID]
	item := WaitListItem{
		ID: w.ID, ConversationID: w.ConversationID,
		CustomerName: thread.CustomerName, CustomerPhone: thread.CustomerPhone,
		ConversationDeleted: !exists,
		StartedAt:           w.StartedAt, CustomerSentAt: w.CustomerSentAt,
		EndedAt: *w.EndedAt, EndReason: *w.EndReason, EndedBy: w.EndedBy,
		SystemDelayed: systemDelayed(w),
	}
	if w.EndedBy != nil {
		item.EndedByUsername = names[*w.EndedBy]
	}
	if answered(w) {
		team := teamMinutes(w)
		item.TeamMinutes = &team
	}
	if minutes, charged := counted[w.ID]; charged && !item.SystemDelayed {
		item.CountedMinutes = &minutes
	}
	return item
}
```

- [ ] **Step 5: Jalankan tes, pastikan lulus**

Run: `cd backend && go test ./internal/services/ -run 'TheTeamFigures|ChargedFrom|StretchOfWorkThatBegan|SeesTheWholeTeam|OnlyClosedThreads|EveryDayOfTheReport|WaitingNowCounts|TheWaitList' -v`
Expected: 9 tes PASS.

Run: `cd backend && gofmt -s -l internal/services && go vet ./internal/services/ && wc -l internal/services/cs_performance_service.go internal/services/cs_performance_waits.go`
Expected: gofmt kosong, vet bersih, kedua file di bawah 350 baris.

- [ ] **Step 6: Commit**

```bash
cd /Users/rohadimraja/Documents/tikman
git add backend/internal/services/cs_performance_service.go backend/internal/services/cs_performance_waits.go \
        backend/internal/services/cs_performance_service_test.go
git commit -m "$(cat <<'EOF'
feat(cs): read the recorded waits as a report

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 7: API laporan

**Files:**
- Create: `backend/internal/api/cs_handler_performance.go`
- Modify: `backend/internal/api/router_handlers.go` (struct `handlers`, struct `csStack`, `newCSStack`, literal `newHandlers`)
- Modify: `backend/internal/api/router.go` (dua rute baru di `registerCSRoutes`)
- Modify: `docs/superpowers/specs/2026-09-15-cs-response-time-design.md` (catatan paginasi)
- Test: `backend/internal/api/cs_handler_performance_test.go`

**Interfaces:**
- Consumes: `services.NewCSPerformanceService`, `services.ReportRangeFromDates`, `Summary`, `Waits` (Task 6); `paginationParams` (`provision_handler.go`), `mapCSError` (`cs_handler_conversations.go`), `ErrorResponse` (`dto.go`), `middleware.GetUserID`, `middleware.GetUserRole`, `TestDB(t)` dan `csTestUser(t, db, username, initials)` (`internal/api`).
- Produces: `func NewCSPerformanceHandler(performance *services.CSPerformanceService) *CSPerformanceHandler` dengan metode `Summary` dan `Waits`; rute `GET /api/v1/cs/performance/summary` dan `GET /api/v1/cs/performance/waits`.

- [ ] **Step 1: Tulis tes yang gagal**

Buat `backend/internal/api/cs_handler_performance_test.go`:

```go
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/middleware"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"gorm.io/gorm"
)

// reportDay is the report for 10 September 2026 in WIB.
const reportDay = "from=2026-09-10&to=2026-09-10"

// performanceRouter builds the report routes as one authenticated request sees
// them: a stand-in for AuthMiddleware, then the real RequireRole.
func performanceRouter(db *gorm.DB, userID uuid.UUID, role models.UserRole) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Set("user_role", role)
		c.Next()
	})
	handler := NewCSPerformanceHandler(services.NewCSPerformanceService(db))
	cs := router.Group("/api/v1/cs")
	cs.Use(middleware.RequireRole(models.UserRoleAdmin, models.UserRoleCS, models.UserRoleTechnician))
	cs.GET("/performance/summary", handler.Summary)
	cs.GET("/performance/waits", handler.Waits)
	return router
}

// plantReply leaves a wait answered by one CS at 10:00 WIB on 10 September.
func plantReply(t *testing.T, db *gorm.DB, by uuid.UUID) {
	t.Helper()
	started := time.Date(2026, 9, 10, 2, 55, 0, 0, time.UTC)
	ended := time.Date(2026, 9, 10, 3, 0, 0, 0, time.UTC)
	reason, message := models.WaitReplied, uuid.New()
	require.NoError(t, db.Create(&models.CSWait{
		ConversationID: uuid.New(), WAAccountID: uuid.New(),
		StartedAt: started, CustomerSentAt: started, LastCustomerSentAt: started,
		EndedAt: &ended, EndReason: &reason, EndedBy: &by, ReplyMessageID: &message,
	}).Error)
}

func getJSON(t *testing.T, router *gin.Engine, path string, into any) int {
	t.Helper()
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	if rec.Code == http.StatusOK && into != nil {
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), into))
	}
	return rec.Code
}

type summaryBody struct {
	Data struct {
		TargetMinutes int `json:"target_minutes"`
		Team          struct {
			Replies struct {
				Count int `json:"count"`
			} `json:"replies"`
		} `json:"team"`
		Agents []struct {
			UserID uuid.UUID `json:"user_id"`
		} `json:"agents"`
	} `json:"data"`
}

type waitsBody struct {
	Data []struct {
		EndedBy *uuid.UUID `json:"ended_by"`
	} `json:"data"`
	Total int64 `json:"total"`
}

func TestAnAdminSeesEveryCSInTheSummary(t *testing.T) {
	db := TestDB(t)
	ani, budi := csTestUser(t, db, "ani", "AN"), csTestUser(t, db, "budi", "BU")
	plantReply(t, db, ani)
	plantReply(t, db, budi)

	var body summaryBody
	code := getJSON(t, performanceRouter(db, uuid.New(), models.UserRoleAdmin),
		"/api/v1/cs/performance/summary?"+reportDay, &body)

	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, 15, body.Data.TargetMinutes)
	assert.Equal(t, 2, body.Data.Team.Replies.Count)
	assert.Len(t, body.Data.Agents, 2)
}

// A CS is shown the team's figures, because that is what they are part of, and
// their own row, because the others' are not theirs to read.
func TestACSSeesTheTeamButOnlyTheirOwnRow(t *testing.T) {
	db := TestDB(t)
	ani, budi := csTestUser(t, db, "ani", "AN"), csTestUser(t, db, "budi", "BU")
	plantReply(t, db, ani)
	plantReply(t, db, budi)

	var body summaryBody
	code := getJSON(t, performanceRouter(db, ani, models.UserRoleCS),
		"/api/v1/cs/performance/summary?"+reportDay, &body)

	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, 2, body.Data.Team.Replies.Count)
	require.Len(t, body.Data.Agents, 1)
	assert.Equal(t, ani, body.Data.Agents[0].UserID)
}

// Naming somebody else in the query must not hand their waits over.
func TestACSCannotListAnotherCSsWaits(t *testing.T) {
	db := TestDB(t)
	ani, budi := csTestUser(t, db, "ani", "AN"), csTestUser(t, db, "budi", "BU")
	plantReply(t, db, ani)
	plantReply(t, db, budi)

	var body waitsBody
	code := getJSON(t, performanceRouter(db, ani, models.UserRoleCS),
		"/api/v1/cs/performance/waits?"+reportDay+"&user_id="+budi.String(), &body)

	require.Equal(t, http.StatusOK, code)
	assert.EqualValues(t, 1, body.Total)
	require.Len(t, body.Data, 1)
	assert.Equal(t, &ani, body.Data[0].EndedBy)
}

func TestAnAdminCanListOneCSsWaits(t *testing.T) {
	db := TestDB(t)
	ani, budi := csTestUser(t, db, "ani", "AN"), csTestUser(t, db, "budi", "BU")
	plantReply(t, db, ani)
	plantReply(t, db, budi)

	var body waitsBody
	code := getJSON(t, performanceRouter(db, uuid.New(), models.UserRoleAdmin),
		"/api/v1/cs/performance/waits?"+reportDay+"&user_id="+budi.String(), &body)

	require.Equal(t, http.StatusOK, code)
	require.Len(t, body.Data, 1)
	assert.Equal(t, &budi, body.Data[0].EndedBy)
}

func TestAViewerIsTurnedAwayFromTheReport(t *testing.T) {
	db := TestDB(t)
	router := performanceRouter(db, uuid.New(), models.UserRoleViewer)

	assert.Equal(t, http.StatusForbidden, getJSON(t, router, "/api/v1/cs/performance/summary?"+reportDay, nil))
	assert.Equal(t, http.StatusForbidden, getJSON(t, router, "/api/v1/cs/performance/waits?"+reportDay, nil))
}

func TestTheReportRefusesARangeItCannotRead(t *testing.T) {
	db := TestDB(t)
	router := performanceRouter(db, uuid.New(), models.UserRoleAdmin)
	cases := map[string]string{
		"not a date":            "from=2026-9-10&to=2026-09-10",
		"nothing at all":        "",
		"ends before it starts": "from=2026-09-11&to=2026-09-10",
		"longer than a year":    "from=2026-01-01&to=2027-01-02",
	}
	for name, query := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, http.StatusBadRequest,
				getJSON(t, router, "/api/v1/cs/performance/summary?"+query, nil))
		})
	}
}

func TestTheWaitListRefusesAUserIDItCannotRead(t *testing.T) {
	db := TestDB(t)

	code := getJSON(t, performanceRouter(db, uuid.New(), models.UserRoleAdmin),
		"/api/v1/cs/performance/waits?"+reportDay+"&user_id=bukan-uuid", nil)

	assert.Equal(t, http.StatusBadRequest, code)
}
```

- [ ] **Step 2: Jalankan tes, pastikan gagal**

Run: `cd backend && go test ./internal/api/ -run 'Summary|Report|WaitList|SeesTheTeam|CannotList|TurnedAway' -v 2>&1 | head -20`
Expected: FAIL saat kompilasi: `undefined: NewCSPerformanceHandler`.

- [ ] **Step 3: Tulis handler**

Buat `backend/internal/api/cs_handler_performance.go`:

```go
package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/middleware"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
)

// CSPerformanceHandler serves the CS response-time report: the figures, and the
// waits they are made of.
type CSPerformanceHandler struct {
	performance *services.CSPerformanceService
}

// NewCSPerformanceHandler constructs a CSPerformanceHandler.
func NewCSPerformanceHandler(performance *services.CSPerformanceService) *CSPerformanceHandler {
	return &CSPerformanceHandler{performance: performance}
}

// Summary answers the report for the dates asked for. An admin sees every CS's
// row; anyone else sees the team's figures and their own row.
func (h *CSPerformanceHandler) Summary(c *gin.Context) {
	period, ok := reportRange(c)
	if !ok {
		return
	}
	summary, err := h.performance.Summary(period, reportViewer(c), time.Now())
	if err != nil {
		mapCSError(c, err, "CS_PERFORMANCE_FAILED")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": summary})
}

// Waits answers one page of the waits behind the report, so a CS can check the
// figure they were given and open the thread it came from.
func (h *CSPerformanceHandler) Waits(c *gin.Context) {
	period, ok := reportRange(c)
	if !ok {
		return
	}
	endedBy, ok := waitListOwner(c)
	if !ok {
		return
	}
	limit, offset := paginationParams(c)
	list, err := h.performance.Waits(period, endedBy, limit, offset)
	if err != nil {
		mapCSError(c, err, "CS_PERFORMANCE_WAITS_FAILED")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": list.Items, "total": list.Total})
}

// reportRange reads the two dates a report covers, answering the request itself
// when they cannot be read.
func reportRange(c *gin.Context) (services.ReportRange, bool) {
	period, err := services.ReportRangeFromDates(c.Query("from"), c.Query("to"))
	if err != nil {
		mapCSError(c, err, "INVALID_REPORT_RANGE")
		return services.ReportRange{}, false
	}
	return period, true
}

// reportViewer is nil for an admin, who sees every CS, and the caller's own id
// for anyone else.
func reportViewer(c *gin.Context) *uuid.UUID {
	if role, _ := middleware.GetUserRole(c); role == models.UserRoleAdmin {
		return nil
	}
	id, _ := middleware.GetUserID(c)
	return &id
}

// waitListOwner decides whose waits a list may show. An admin may name anyone,
// or nobody and see them all; everyone else is pinned to themselves, whatever
// the query says.
func waitListOwner(c *gin.Context) (*uuid.UUID, bool) {
	if viewer := reportViewer(c); viewer != nil {
		return viewer, true
	}
	named := c.Query("user_id")
	if named == "" {
		return nil, true
	}
	id, err := uuid.Parse(named)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid ID format", Code: "INVALID_USER_ID"})
		return nil, false
	}
	return &id, true
}
```

- [ ] **Step 4: Daftarkan rutenya**

Di `backend/internal/api/router.go`, di dalam blok `registerCSRoutes`, tepat setelah `cs.GET("/link-preview", h.csHandler.LinkPreview)`, tambahkan:

```go
		cs.GET("/performance/summary", h.csPerformanceHandler.Summary)
		cs.GET("/performance/waits", h.csPerformanceHandler.Waits)
```

Di `backend/internal/api/router_handlers.go`:

1. Tambahkan field ke struct `handlers`, tepat setelah `csHandler *CSHandler`:

```go
	csPerformanceHandler   *CSPerformanceHandler
```

2. Tambahkan ke literal `&handlers{...}` di `newHandlers`, tepat setelah `csHandler: cs.handler,`:

```go
		csPerformanceHandler:   cs.performance,
```

3. Tambahkan field ke struct `csStack`, tepat setelah `handler  *CSHandler`:

```go
	performance *CSPerformanceHandler
```

4. Di `newCSStack`, tepat sebelum `pushService := services.NewPushService(db)`, tambahkan:

```go
	performanceHandler := NewCSPerformanceHandler(services.NewCSPerformanceService(db))
```

   dan ganti baris `return` terakhirnya dengan:

```go
	return csStack{handler: csHandler, performance: performanceHandler, push: pushHandler, notifier: pushNotifier, listener: pushListener}
```

Jalankan `gofmt -s -w backend/internal/api/router_handlers.go` supaya field-field itu kembali rata.

- [ ] **Step 5: Luruskan spec soal paginasi**

Di `docs/superpowers/specs/2026-09-15-cs-response-time-design.md`, ganti `(default 50, maksimum 100)` menjadi:

```
(`paginationParams` yang sudah ada: default 20, maksimum 100)
```

- [ ] **Step 6: Jalankan tes, pastikan lulus**

Run: `cd backend && go test ./internal/api/ -run 'Summary|Report|WaitList|SeesTheTeam|CannotList|TurnedAway' -v`
Expected: semua PASS (tujuh fungsi tes, satu di antaranya dengan empat subtes).

Run: `cd backend && go build ./... && gofmt -s -l internal && go vet ./internal/api/`
Expected: bersih.

- [ ] **Step 7: Commit**

```bash
cd /Users/rohadimraja/Documents/tikman
git add backend/internal/api/cs_handler_performance.go backend/internal/api/cs_handler_performance_test.go \
        backend/internal/api/router.go backend/internal/api/router_handlers.go \
        docs/superpowers/specs/2026-09-15-cs-response-time-design.md
git commit -m "$(cat <<'EOF'
feat(cs): serve the response-time report, with each role's share of it

An admin reads every CS's row; a CS reads the team's figures and their
own, and cannot list anyone else's waits by naming them in the query.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 8: Frontend — entitas, endpoint, repository, hook

**Files:**
- Create: `frontend/src/domain/entities/CsPerformance.ts`
- Modify: `frontend/src/domain/entities/index.ts`
- Create: `frontend/src/domain/repositories/ICsPerformanceRepository.ts`
- Modify: `frontend/src/domain/repositories/index.ts`
- Modify: `frontend/src/infrastructure/http/endpoints.ts`
- Create: `frontend/src/infrastructure/repositories/CsPerformanceRepository.ts`
- Modify: `frontend/src/infrastructure/repositories/index.ts`
- Create: `frontend/src/application/hooks/useCsPerformance.ts`
- Modify: `frontend/src/application/hooks/index.ts`
- Test: `frontend/src/infrastructure/repositories/CsPerformanceRepository.test.ts`

**Interfaces:**
- Consumes: bentuk JSON dari Task 7 (`{ data: summary }` dan `{ data: [...], total }`), diubah ke camelCase oleh interceptor `apiClient`.
- Produces (dipakai Task 9 dan 10): tipe `WaitStats`, `TeamPerformance`, `AgentPerformance`, `DayPerformance`, `PerformanceSummary`, `PerformanceWait`, `PerformanceWaitPage`, `PerformancePeriod`, `PerformanceWaitQuery`, `WaitEndReason`; kelas `CsPerformanceRepository` dengan `getSummary(period)` dan `getWaits(query)`; hook `useCsPerformanceSummary(period)` dan `useCsPerformanceWaits(query | undefined)`.

- [ ] **Step 1: Tulis tes repository yang gagal**

Buat `frontend/src/infrastructure/repositories/CsPerformanceRepository.test.ts`:

```ts
import { beforeEach, describe, expect, it, vi } from "vitest";
import { CsPerformanceRepository } from "./CsPerformanceRepository";

const get = vi.fn();

vi.mock("../http/apiClient", () => ({
  apiClient: {
    get: (...args: unknown[]) => get(...args),
  },
}));

describe("CsPerformanceRepository", () => {
  beforeEach(() => {
    get.mockReset();
  });

  it("asks for the summary of the dates given and unwraps the envelope", async () => {
    get.mockResolvedValue({ data: { data: { targetMinutes: 15 } } });

    const summary = await new CsPerformanceRepository().getSummary({
      from: "2026-09-01",
      to: "2026-09-15",
    });

    expect(get).toHaveBeenCalledWith("/api/v1/cs/performance/summary", {
      params: { from: "2026-09-01", to: "2026-09-15" },
    });
    expect(summary).toEqual({ targetMinutes: 15 });
  });

  // The client decamelizes request bodies, never query params, so user_id has
  // to be spelled the way the server reads it.
  it("sends the wait query in snake_case and keeps the total", async () => {
    get.mockResolvedValue({ data: { data: [{ id: "w1" }], total: 7 } });

    const page = await new CsPerformanceRepository().getWaits({
      from: "2026-09-01",
      to: "2026-09-15",
      userId: "u1",
      limit: 20,
      offset: 40,
    });

    expect(get).toHaveBeenCalledWith("/api/v1/cs/performance/waits", {
      params: {
        from: "2026-09-01",
        to: "2026-09-15",
        user_id: "u1",
        limit: 20,
        offset: 40,
      },
    });
    expect(page).toEqual({ items: [{ id: "w1" }], total: 7 });
  });
});
```

- [ ] **Step 2: Jalankan tes, pastikan gagal**

Run: `cd frontend && npm test -- --run src/infrastructure/repositories/CsPerformanceRepository.test.ts`
Expected: FAIL: `Failed to resolve import "./CsPerformanceRepository"`.

- [ ] **Step 3: Tulis entitas**

Buat `frontend/src/domain/entities/CsPerformance.ts`:

```ts
export type WaitEndReason = "replied" | "phone" | "closed" | "abandoned";

/** How fast a set of waits was answered, in minutes. A null figure means there
 * was nothing to measure, which is not the same as zero. */
export interface WaitStats {
  count: number;
  medianMinutes: number | null;
  p90Minutes: number | null;
  withinTargetPct: number | null;
}

export interface TeamPerformance {
  replies: WaitStats;
  closedWithoutReply: number;
  abandoned: number;
  systemDelayed: number;
}

export interface AgentPerformance {
  userId: string;
  username: string;
  replies: WaitStats;
  closedWithoutReply: number;
}

export interface DayPerformance {
  /** A WIB calendar day, YYYY-MM-DD. */
  date: string;
  replies: WaitStats;
}

export interface PerformanceSummary {
  targetMinutes: number;
  team: TeamPerformance;
  agents: AgentPerformance[];
  days: DayPerformance[];
  waiting: { count: number; longestMinutes: number | null };
}

/** One customer's wait, as the list behind the figures shows it. */
export interface PerformanceWait {
  id: string;
  conversationId: string;
  customerName: string;
  customerPhone: string;
  /** The thread was deleted with its number; the wait outlives it. */
  conversationDeleted: boolean;
  startedAt: string;
  customerSentAt: string;
  endedAt: string;
  endReason: WaitEndReason;
  endedBy?: string;
  endedByUsername?: string;
  teamMinutes: number | null;
  countedMinutes: number | null;
  systemDelayed: boolean;
}

export interface PerformanceWaitPage {
  items: PerformanceWait[];
  total: number;
}

/** The inclusive WIB dates a report covers, as YYYY-MM-DD. */
export interface PerformancePeriod {
  from: string;
  to: string;
}

/** One page of the waits behind a report. userId narrows it to one CS; the
 * server ignores it for anyone but an admin. */
export interface PerformanceWaitQuery extends PerformancePeriod {
  userId?: string;
  limit: number;
  offset: number;
}
```

Di `frontend/src/domain/entities/index.ts`, tambahkan setelah `export * from "./CsMessage";`:

```ts
export * from "./CsPerformance";
```

- [ ] **Step 4: Tulis interface dan repository**

Buat `frontend/src/domain/repositories/ICsPerformanceRepository.ts`:

```ts
import type {
  PerformancePeriod,
  PerformanceSummary,
  PerformanceWaitPage,
  PerformanceWaitQuery,
} from "@/domain/entities";

/** Reads the CS response-time report. */
export interface ICsPerformanceRepository {
  getSummary(period: PerformancePeriod): Promise<PerformanceSummary>;
  getWaits(query: PerformanceWaitQuery): Promise<PerformanceWaitPage>;
}
```

Di `frontend/src/domain/repositories/index.ts`, tambahkan setelah `export * from "./ICsRepository";`:

```ts
export * from "./ICsPerformanceRepository";
```

Di `frontend/src/infrastructure/http/endpoints.ts`, tambahkan setelah `CS_BROADCASTS_MEDIA: "/api/v1/cs/broadcasts/media",`:

```ts
  CS_PERFORMANCE_SUMMARY: "/api/v1/cs/performance/summary",
  CS_PERFORMANCE_WAITS: "/api/v1/cs/performance/waits",
```

Buat `frontend/src/infrastructure/repositories/CsPerformanceRepository.ts`:

```ts
import { apiClient } from "../http/apiClient";
import { API_ENDPOINTS } from "../http/endpoints";
import type { ICsPerformanceRepository } from "@/domain/repositories";
import type {
  PerformancePeriod,
  PerformanceSummary,
  PerformanceWaitPage,
  PerformanceWaitQuery,
} from "@/domain/entities";

/**
 * CsPerformanceRepository reads the CS response-time report. Query params are
 * written in snake_case by hand: the client only decamelizes request bodies.
 */
export class CsPerformanceRepository implements ICsPerformanceRepository {
  async getSummary(period: PerformancePeriod): Promise<PerformanceSummary> {
    const response = await apiClient.get(API_ENDPOINTS.CS_PERFORMANCE_SUMMARY, {
      params: { from: period.from, to: period.to },
    });
    return response.data.data;
  }

  async getWaits(query: PerformanceWaitQuery): Promise<PerformanceWaitPage> {
    const response = await apiClient.get(API_ENDPOINTS.CS_PERFORMANCE_WAITS, {
      params: {
        from: query.from,
        to: query.to,
        user_id: query.userId,
        limit: query.limit,
        offset: query.offset,
      },
    });
    return { items: response.data.data ?? [], total: response.data.total ?? 0 };
  }
}
```

Di `frontend/src/infrastructure/repositories/index.ts`, tambahkan setelah `export * from "./CsRepository";`:

```ts
export * from "./CsPerformanceRepository";
```

- [ ] **Step 5: Tulis hook**

Buat `frontend/src/application/hooks/useCsPerformance.ts`:

```ts
import { useQuery } from "@tanstack/react-query";
import { CsPerformanceRepository } from "@/infrastructure/repositories";
import type { PerformancePeriod, PerformanceWaitQuery } from "@/domain/entities";

const performanceRepository = new CsPerformanceRepository();

/** The team's, each CS's and each day's figures for one period. */
export function useCsPerformanceSummary(period: PerformancePeriod) {
  return useQuery({
    queryKey: ["cs", "performance", "summary", period],
    queryFn: () => performanceRepository.getSummary(period),
  });
}

/** One page of the waits behind the figures. Pass undefined while the list is
 * closed, so nothing is asked for until somebody opens it. */
export function useCsPerformanceWaits(query: PerformanceWaitQuery | undefined) {
  return useQuery({
    queryKey: ["cs", "performance", "waits", query],
    queryFn: () => performanceRepository.getWaits(query!),
    enabled: query !== undefined,
  });
}
```

Di `frontend/src/application/hooks/index.ts`, tambahkan setelah `export * from "./useCsQuickReplies";`:

```ts
export * from "./useCsPerformance";
```

- [ ] **Step 6: Jalankan tes, pastikan lulus**

Run: `cd frontend && npm test -- --run src/infrastructure/repositories/CsPerformanceRepository.test.ts`
Expected: 2 tes PASS.

Run: `cd frontend && npx tsc --noEmit && npm run lint`
Expected: bersih.

- [ ] **Step 7: Commit**

```bash
cd /Users/rohadimraja/Documents/tikman
git add frontend/src/domain/entities/CsPerformance.ts frontend/src/domain/entities/index.ts \
        frontend/src/domain/repositories/ICsPerformanceRepository.ts frontend/src/domain/repositories/index.ts \
        frontend/src/infrastructure/http/endpoints.ts \
        frontend/src/infrastructure/repositories/CsPerformanceRepository.ts \
        frontend/src/infrastructure/repositories/index.ts \
        frontend/src/application/hooks/useCsPerformance.ts frontend/src/application/hooks/index.ts \
        frontend/src/infrastructure/repositories/CsPerformanceRepository.test.ts
git commit -m "$(cat <<'EOF'
feat(cs): read the response-time report from the browser

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 9: Frontend — periode dan format

**Files:**
- Create: `frontend/src/presentation/components/cs/performance/performancePeriod.ts`
- Create: `frontend/src/presentation/components/cs/performance/performanceFormat.ts`
- Test: `frontend/src/presentation/components/cs/performance/performancePeriod.test.ts`
- Test: `frontend/src/presentation/components/cs/performance/performanceFormat.test.ts`

**Interfaces:**
- Consumes: `PerformancePeriod`, `WaitEndReason` (Task 8).
- Produces (dipakai Task 10): `type PeriodPreset = "hari-ini" | "7-hari" | "bulan-ini" | "bulan-lalu"`, `presetPeriod(preset, now)`, `formatMinutes(minutes)`, `formatPercent(pct)`, `END_REASON_LABELS`.

- [ ] **Step 1: Tulis tes yang gagal**

Buat `frontend/src/presentation/components/cs/performance/performancePeriod.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { presetPeriod } from "./performancePeriod";

// 03:00 WIB on 16 September is still the 15th in UTC. The report's days are WIB
// days, so "today" here has to be the 16th whatever zone the browser is in.
const earlyMorningWIB = new Date("2026-09-15T20:00:00Z");

describe("presetPeriod", () => {
  it("takes today as the WIB day", () => {
    expect(presetPeriod("hari-ini", earlyMorningWIB)).toEqual({
      from: "2026-09-16",
      to: "2026-09-16",
    });
  });

  it("counts seven days including today", () => {
    expect(presetPeriod("7-hari", earlyMorningWIB)).toEqual({
      from: "2026-09-10",
      to: "2026-09-16",
    });
  });

  it("runs this month from its first day to today", () => {
    expect(presetPeriod("bulan-ini", earlyMorningWIB)).toEqual({
      from: "2026-09-01",
      to: "2026-09-16",
    });
  });

  it("covers the whole of last month", () => {
    expect(presetPeriod("bulan-lalu", earlyMorningWIB)).toEqual({
      from: "2026-08-01",
      to: "2026-08-31",
    });
  });

  it("reaches into last year in January", () => {
    expect(presetPeriod("bulan-lalu", new Date("2026-01-10T01:00:00Z"))).toEqual({
      from: "2025-12-01",
      to: "2025-12-31",
    });
  });
});
```

Buat `frontend/src/presentation/components/cs/performance/performanceFormat.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { END_REASON_LABELS, formatMinutes, formatPercent } from "./performanceFormat";

describe("formatMinutes", () => {
  it("says nothing was measured rather than showing zero", () => {
    expect(formatMinutes(null)).toBe("—");
  });

  it("keeps one decimal under an hour", () => {
    expect(formatMinutes(4.2)).toBe("4,2 mnt");
    expect(formatMinutes(0)).toBe("0 mnt");
  });

  it("switches to hours before it would ever print 60 minutes", () => {
    expect(formatMinutes(59.96)).toBe("1 j");
    expect(formatMinutes(125)).toBe("2 j 5 mnt");
  });
});

describe("formatPercent", () => {
  it("rounds to a whole percent", () => {
    expect(formatPercent(86.4)).toBe("86%");
  });

  it("says nothing was measured rather than showing zero", () => {
    expect(formatPercent(null)).toBe("—");
  });
});

describe("END_REASON_LABELS", () => {
  it("names every way a wait can end", () => {
    expect(END_REASON_LABELS.replied).toBe("Dibalas");
    expect(END_REASON_LABELS.phone).toBe("Dibalas dari HP");
    expect(END_REASON_LABELS.closed).toBe("Ditutup tanpa balasan");
    expect(END_REASON_LABELS.abandoned).toBe("Ditinggal");
  });
});
```

- [ ] **Step 2: Jalankan tes, pastikan gagal**

Run: `cd frontend && npm test -- --run src/presentation/components/cs/performance`
Expected: FAIL: `Failed to resolve import "./performancePeriod"` dan `"./performanceFormat"`.

- [ ] **Step 3: Tulis helper periode**

Buat `frontend/src/presentation/components/cs/performance/performancePeriod.ts`:

```ts
import type { PerformancePeriod } from "@/domain/entities";

export type PeriodPreset = "hari-ini" | "7-hari" | "bulan-ini" | "bulan-lalu";

const WIB_OFFSET_MS = 7 * 60 * 60 * 1000;
const DAY_MS = 24 * 60 * 60 * 1000;

/** The WIB calendar day a moment falls on, as a UTC midnight so the arithmetic
 * below has no zone of its own. The report's days are WIB days on the server,
 * and a browser somewhere else must still ask for the same ones. */
function wibDay(moment: Date): Date {
  const shifted = new Date(moment.getTime() + WIB_OFFSET_MS);
  return new Date(
    Date.UTC(shifted.getUTCFullYear(), shifted.getUTCMonth(), shifted.getUTCDate()),
  );
}

function iso(day: Date): string {
  return day.toISOString().slice(0, 10);
}

function addDays(day: Date, days: number): Date {
  return new Date(day.getTime() + days * DAY_MS);
}

/** The dates a preset covers, counted from now. */
export function presetPeriod(preset: PeriodPreset, now: Date): PerformancePeriod {
  const today = wibDay(now);
  const firstOfThisMonth = Date.UTC(today.getUTCFullYear(), today.getUTCMonth(), 1);
  switch (preset) {
    case "hari-ini":
      return { from: iso(today), to: iso(today) };
    case "7-hari":
      return { from: iso(addDays(today, -6)), to: iso(today) };
    case "bulan-ini":
      return { from: iso(new Date(firstOfThisMonth)), to: iso(today) };
    case "bulan-lalu":
      return {
        from: iso(new Date(Date.UTC(today.getUTCFullYear(), today.getUTCMonth() - 1, 1))),
        to: iso(addDays(new Date(firstOfThisMonth), -1)),
      };
  }
}
```

- [ ] **Step 4: Tulis helper format**

Buat `frontend/src/presentation/components/cs/performance/performanceFormat.ts`:

```ts
import type { WaitEndReason } from "@/domain/entities";

/** Minutes as the page shows them: "4,2 mnt", "2 j 5 mnt", or "—" when there
 * was nothing to measure. The switch to hours happens before rounding could
 * print "60 mnt". */
export function formatMinutes(minutes: number | null | undefined): string {
  if (minutes === null || minutes === undefined) {
    return "—";
  }
  const tenths = Math.round(minutes * 10) / 10;
  if (tenths < 60) {
    return `${tenths.toLocaleString("id-ID")} mnt`;
  }
  const whole = Math.round(minutes);
  const hours = Math.floor(whole / 60);
  const rest = whole % 60;
  return rest === 0 ? `${hours} j` : `${hours} j ${rest} mnt`;
}

export function formatPercent(pct: number | null | undefined): string {
  if (pct === null || pct === undefined) {
    return "—";
  }
  return `${Math.round(pct)}%`;
}

/** How a wait ended, in the words the inbox already uses. */
export const END_REASON_LABELS: Record<WaitEndReason, string> = {
  replied: "Dibalas",
  phone: "Dibalas dari HP",
  closed: "Ditutup tanpa balasan",
  abandoned: "Ditinggal",
};
```

- [ ] **Step 5: Jalankan tes, pastikan lulus**

Run: `cd frontend && npm test -- --run src/presentation/components/cs/performance`
Expected: semua PASS.

Run: `cd frontend && npx prettier --check "src/presentation/components/cs/performance/**" && npm run lint`
Expected: bersih.

- [ ] **Step 6: Commit**

```bash
cd /Users/rohadimraja/Documents/tikman
git add frontend/src/presentation/components/cs/performance/performancePeriod.ts \
        frontend/src/presentation/components/cs/performance/performanceFormat.ts \
        frontend/src/presentation/components/cs/performance/performancePeriod.test.ts \
        frontend/src/presentation/components/cs/performance/performanceFormat.test.ts
git commit -m "$(cat <<'EOF'
feat(cs): pick report periods in WIB and read the figures in Indonesian

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 10: Halaman "Kinerja CS"

**Files:**
- Create: `frontend/src/presentation/components/cs/performance/PerformanceTiles.tsx`
- Create: `frontend/src/presentation/components/cs/performance/AgentPerformanceTable.tsx`
- Create: `frontend/src/presentation/components/cs/performance/DailyPerformanceChart.tsx`
- Create: `frontend/src/presentation/components/cs/performance/WaitListDrawer.tsx`
- Create: `frontend/src/presentation/pages/CsPerformancePage.tsx`
- Modify: `frontend/src/presentation/routes/index.tsx`
- Modify: `frontend/src/presentation/components/layout/navigationRoutes.tsx`
- Modify: `frontend/src/presentation/components/layout/navigationRoutes.test.tsx`
- Modify: `docs/superpowers/specs/2026-09-15-cs-response-time-design.md` (path halaman)
- Test: `frontend/src/presentation/pages/__tests__/CsPerformancePage.test.tsx`

**Interfaces:**
- Consumes: `useCsPerformanceSummary`, `useCsPerformanceWaits` (Task 8); `presetPeriod`, `PeriodPreset`, `formatMinutes`, `formatPercent`, `END_REASON_LABELS` (Task 9); `PageHeader` (`presentation/components/common/PageHeader`); `colors` (`@/shared/theme/colors`); tautan inbox `/cs?view=belum-dibalas` dan `/cs?conversation=<id>`.
- Produces: rute `/cs-performance` dan entri menu "Kinerja CS".

- [ ] **Step 1: Tulis tes halaman yang gagal**

Buat `frontend/src/presentation/pages/__tests__/CsPerformancePage.test.tsx`:

```tsx
import { describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";

const fixtures = vi.hoisted(() => {
  const stats = (
    count: number,
    median: number | null,
    pct: number | null,
    p90: number | null,
  ) => ({ count, medianMinutes: median, withinTargetPct: pct, p90Minutes: p90 });

  return {
    queries: [] as unknown[],
    summary: {
      targetMinutes: 15,
      team: {
        replies: stats(12, 3.2, 83, 30.8),
        closedWithoutReply: 2,
        abandoned: 1,
        systemDelayed: 4,
      },
      agents: [
        { userId: "u-ani", username: "ani", replies: stats(7, 2.3, 89, 17.9), closedWithoutReply: 0 },
        { userId: "u-budi", username: "budi", replies: stats(0, null, null, null), closedWithoutReply: 2 },
      ],
      days: [{ date: "2026-09-16", replies: stats(12, 3.2, 83, 30.8) }],
      waiting: { count: 3, longestMinutes: 18 },
    },
    waits: {
      items: [
        {
          id: "w1", conversationId: "c1", customerName: "Pak Budi", customerPhone: "628123456789",
          conversationDeleted: false, startedAt: "2026-09-16T01:00:00Z", customerSentAt: "2026-09-16T01:00:00Z",
          endedAt: "2026-09-16T01:05:00Z", endReason: "replied", endedBy: "u-ani", endedByUsername: "ani",
          teamMinutes: 5, countedMinutes: 5, systemDelayed: false,
        },
        {
          id: "w2", conversationId: "c2", customerName: "", customerPhone: "",
          conversationDeleted: true, startedAt: "2026-09-16T02:00:00Z", customerSentAt: "2026-09-16T01:00:00Z",
          endedAt: "2026-09-16T02:10:00Z", endReason: "closed", endedBy: "u-ani", endedByUsername: "ani",
          teamMinutes: null, countedMinutes: null, systemDelayed: true,
        },
      ],
      total: 2,
    },
  };
});

vi.mock("@/application/hooks/useCsPerformance", () => ({
  useCsPerformanceSummary: () => ({ data: fixtures.summary, isLoading: false }),
  useCsPerformanceWaits: (query?: unknown) => {
    fixtures.queries.push(query);
    return { data: query ? fixtures.waits : undefined, isLoading: false };
  },
}));

// jsdom has no layout, so recharts warns about a zero-sized chart on every
// render. The chart is not what these tests are about.
vi.mock("../../components/cs/performance/DailyPerformanceChart", () => ({
  DailyPerformanceChart: () => null,
}));

import { CsPerformancePage } from "../CsPerformancePage";

function draw() {
  return render(
    <MemoryRouter>
      <CsPerformancePage />
    </MemoryRouter>,
  );
}

const lastQuery = () => fixtures.queries[fixtures.queries.length - 1] as Record<string, unknown>;

describe("the CS performance page", () => {
  it("shows the team's figures and who is waiting now", () => {
    draw();

    expect(screen.getByText("83%")).toBeInTheDocument();
    expect(screen.getByText("3,2 mnt")).toBeInTheDocument();
    expect(screen.getByText(/Ditutup tanpa balasan: 2/)).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: /Sedang menunggu: 3 pelanggan, terlama 18 mnt/ }),
    ).toHaveAttribute("href", "/cs?view=belum-dibalas");
  });

  it("gives every CS a row, with dashes where there is nothing to measure", () => {
    draw();

    const budi = screen.getByText("budi").closest("tr");
    expect(budi).not.toBeNull();
    expect(within(budi!).getByText("2")).toBeInTheDocument();
    expect(within(budi!).getAllByText("—").length).toBeGreaterThanOrEqual(3);
    expect(screen.getByText("2,3 mnt")).toBeInTheDocument();
  });

  it("opens one CS's waits, marking a deleted thread and a system delay", async () => {
    draw();

    await userEvent.click(screen.getByText("ani"));

    expect(screen.getByText("Giliran ani")).toBeInTheDocument();
    expect(screen.getByText("Pak Budi")).toBeInTheDocument();
    expect(screen.getByText("Thread dihapus")).toBeInTheDocument();
    expect(screen.getByText("Tertunda sistem")).toBeInTheDocument();
    expect(lastQuery()).toMatchObject({ userId: "u-ani", limit: 20, offset: 0 });
  });

  it("lists the whole team's waits when no CS is named", async () => {
    draw();

    await userEvent.click(screen.getByRole("button", { name: "Daftar giliran" }));

    expect(screen.getByText("Daftar giliran")).toBeInTheDocument();
    expect(lastQuery().userId).toBeUndefined();
  });
});
```

- [ ] **Step 2: Jalankan tes, pastikan gagal**

Run: `cd frontend && npm test -- --run src/presentation/pages/__tests__/CsPerformancePage.test.tsx`
Expected: FAIL: `Failed to resolve import "../CsPerformancePage"`.

- [ ] **Step 3: Tulis kartu angka tim**

Buat `frontend/src/presentation/components/cs/performance/PerformanceTiles.tsx`:

```tsx
import { Button, Card, Col, Row, Space, Statistic, Typography } from "antd";
import { Link } from "react-router-dom";
import type { PerformanceSummary } from "@/domain/entities";
import { formatMinutes, formatPercent } from "./performanceFormat";

const { Text } = Typography;

interface PerformanceTilesProps {
  summary: PerformanceSummary;
  onShowWaits: () => void;
}

/** What the customers of this period got, and who is waiting right now. */
export function PerformanceTiles({ summary, onShowWaits }: PerformanceTilesProps) {
  const { team, waiting, targetMinutes } = summary;
  return (
    <Card title="Tim" extra={<Button onClick={onShowWaits}>Daftar giliran</Button>}>
      <Row gutter={[16, 16]}>
        <Col xs={12} md={6}>
          <Statistic title="Giliran dibalas" value={team.replies.count} />
        </Col>
        <Col xs={12} md={6}>
          <Statistic title="Median" value={formatMinutes(team.replies.medianMinutes)} />
        </Col>
        <Col xs={12} md={6}>
          <Statistic
            title={`≤ ${targetMinutes} menit`}
            value={formatPercent(team.replies.withinTargetPct)}
          />
        </Col>
        <Col xs={12} md={6}>
          <Statistic title="90% tercepat" value={formatMinutes(team.replies.p90Minutes)} />
        </Col>
      </Row>
      <Space wrap size="large" style={{ marginTop: 16 }}>
        <Text type="secondary">Ditutup tanpa balasan: {team.closedWithoutReply}</Text>
        <Text type="secondary">Ditinggal: {team.abandoned}</Text>
        <Text type="secondary">Tertunda sistem: {team.systemDelayed}</Text>
        <Link to="/cs?view=belum-dibalas">
          Sedang menunggu: {waiting.count} pelanggan
          {waiting.longestMinutes !== null && `, terlama ${formatMinutes(waiting.longestMinutes)}`}
        </Link>
      </Space>
    </Card>
  );
}
```

- [ ] **Step 4: Tulis tabel per CS**

Buat `frontend/src/presentation/components/cs/performance/AgentPerformanceTable.tsx`:

```tsx
import { Card, Table } from "antd";
import type { ColumnsType } from "antd/es/table";
import type { AgentPerformance } from "@/domain/entities";
import { formatMinutes, formatPercent } from "./performanceFormat";

interface AgentPerformanceTableProps {
  agents: AgentPerformance[];
  targetMinutes: number;
  loading: boolean;
  onSelect: (agent: AgentPerformance) => void;
}

/** One row per CS: how fast the replies they sent were, and how many threads
 * they closed without sending one. Anyone but an admin is served their own row
 * alone, by the API. */
export function AgentPerformanceTable({
  agents,
  targetMinutes,
  loading,
  onSelect,
}: AgentPerformanceTableProps) {
  const columns: ColumnsType<AgentPerformance> = [
    {
      title: "CS",
      dataIndex: "username",
      render: (username: string) => username || "Pengguna terhapus",
    },
    { title: "Balasan", render: (_, agent) => agent.replies.count },
    { title: "Median", render: (_, agent) => formatMinutes(agent.replies.medianMinutes) },
    {
      title: `≤ ${targetMinutes} menit`,
      render: (_, agent) => formatPercent(agent.replies.withinTargetPct),
    },
    { title: "90% tercepat", render: (_, agent) => formatMinutes(agent.replies.p90Minutes) },
    { title: "Ditutup tanpa balasan", dataIndex: "closedWithoutReply" },
  ];

  return (
    <Card title="Per CS">
      <Table
        rowKey="userId"
        size="small"
        scroll={{ x: 720 }}
        loading={loading}
        dataSource={agents}
        columns={columns}
        pagination={false}
        onRow={(agent) => ({
          onClick: () => onSelect(agent),
          style: { cursor: "pointer" },
        })}
      />
    </Card>
  );
}
```

- [ ] **Step 5: Tulis grafik harian**

Buat `frontend/src/presentation/components/cs/performance/DailyPerformanceChart.tsx`:

```tsx
import { Card, Empty } from "antd";
import {
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import type { DayPerformance } from "@/domain/entities";
import { colors } from "@/shared/theme/colors";

interface DailyPerformanceChartProps {
  days: DayPerformance[];
  targetMinutes: number;
}

/** The share of answers within target, day by day. A day nobody was answered
 * leaves a gap rather than a drop to zero, which would read as a bad day. */
export function DailyPerformanceChart({ days, targetMinutes }: DailyPerformanceChartProps) {
  const data = days.map((day) => ({ date: day.date.slice(5), pct: day.replies.withinTargetPct }));
  const measured = days.some((day) => day.replies.count > 0);

  return (
    <Card title={`Dibalas ≤ ${targetMinutes} menit per hari`}>
      {measured ? (
        <ResponsiveContainer width="100%" height={220}>
          <LineChart data={data}>
            <CartesianGrid strokeDasharray="3 3" opacity={0.2} />
            <XAxis dataKey="date" tick={{ fontSize: 10 }} />
            <YAxis domain={[0, 100]} unit="%" tick={{ fontSize: 10 }} />
            <Tooltip
              formatter={(value: unknown) =>
                typeof value === "number" ? `${Math.round(value)}%` : "—"
              }
            />
            <Line type="monotone" dataKey="pct" stroke={colors.success} connectNulls={false} />
          </LineChart>
        </ResponsiveContainer>
      ) : (
        <Empty description="Belum ada giliran yang dibalas di periode ini" />
      )}
    </Card>
  );
}
```

- [ ] **Step 6: Tulis daftar giliran**

Buat `frontend/src/presentation/components/cs/performance/WaitListDrawer.tsx`:

```tsx
import { useState } from "react";
import { Drawer, Table, Tag } from "antd";
import type { ColumnsType } from "antd/es/table";
import { useNavigate } from "react-router-dom";
import { useCsPerformanceWaits } from "@/application/hooks/useCsPerformance";
import type { PerformancePeriod, PerformanceWait } from "@/domain/entities";
import { END_REASON_LABELS, formatMinutes } from "./performanceFormat";

const PAGE_SIZE = 20;

interface WaitListDrawerProps {
  title: string;
  period: PerformancePeriod;
  userId?: string;
  onClose: () => void;
}

/** Times are read in WIB, the way the report counts its days. */
function wibTime(iso: string): string {
  return new Date(iso).toLocaleString("id-ID", {
    timeZone: "Asia/Jakarta",
    day: "2-digit",
    month: "short",
    hour: "2-digit",
    minute: "2-digit",
  });
}

/** The waits behind the figures, newest first, so a CS can check the number
 * they were given and open the thread it came from. */
export function WaitListDrawer({ title, period, userId, onClose }: WaitListDrawerProps) {
  const navigate = useNavigate();
  const [page, setPage] = useState(1);
  const { data, isLoading } = useCsPerformanceWaits({
    ...period,
    userId,
    limit: PAGE_SIZE,
    offset: (page - 1) * PAGE_SIZE,
  });

  const columns: ColumnsType<PerformanceWait> = [
    {
      title: "Pelanggan",
      render: (_, wait) =>
        wait.conversationDeleted ? <Tag>Thread dihapus</Tag> : wait.customerName || wait.customerPhone,
    },
    { title: "Mulai", render: (_, wait) => wibTime(wait.startedAt) },
    { title: "Selesai", render: (_, wait) => wibTime(wait.endedAt) },
    { title: "Cara selesai", render: (_, wait) => END_REASON_LABELS[wait.endReason] },
    { title: "Oleh", render: (_, wait) => wait.endedByUsername ?? "—" },
    { title: "Menit tim", render: (_, wait) => formatMinutes(wait.teamMinutes) },
    { title: "Menit dihitung", render: (_, wait) => formatMinutes(wait.countedMinutes) },
    {
      title: "",
      render: (_, wait) => (wait.systemDelayed ? <Tag color="warning">Tertunda sistem</Tag> : null),
    },
  ];

  return (
    <Drawer open title={title} width={960} onClose={onClose}>
      <Table
        rowKey="id"
        size="small"
        scroll={{ x: 900 }}
        loading={isLoading}
        dataSource={data?.items}
        columns={columns}
        pagination={{
          current: page,
          pageSize: PAGE_SIZE,
          total: data?.total ?? 0,
          onChange: setPage,
          showSizeChanger: false,
        }}
        onRow={(wait) => ({
          onClick: () => {
            if (!wait.conversationDeleted) {
              navigate(`/cs?conversation=${wait.conversationId}`);
            }
          },
          style: { cursor: wait.conversationDeleted ? "default" : "pointer" },
        })}
      />
    </Drawer>
  );
}
```

- [ ] **Step 7: Tulis halamannya**

Buat `frontend/src/presentation/pages/CsPerformancePage.tsx`:

```tsx
import { useState } from "react";
import { DatePicker, Segmented, Space, Spin } from "antd";
import { useCsPerformanceSummary } from "@/application/hooks/useCsPerformance";
import type { PerformancePeriod } from "@/domain/entities";
import { PageHeader } from "../components/common/PageHeader";
import { AgentPerformanceTable } from "../components/cs/performance/AgentPerformanceTable";
import { DailyPerformanceChart } from "../components/cs/performance/DailyPerformanceChart";
import { PerformanceTiles } from "../components/cs/performance/PerformanceTiles";
import { WaitListDrawer } from "../components/cs/performance/WaitListDrawer";
import { presetPeriod, type PeriodPreset } from "../components/cs/performance/performancePeriod";

const { RangePicker } = DatePicker;

type PeriodChoice = PeriodPreset | "pilih-tanggal";

const PERIOD_OPTIONS: { label: string; value: PeriodChoice }[] = [
  { label: "Hari ini", value: "hari-ini" },
  { label: "7 hari", value: "7-hari" },
  { label: "Bulan ini", value: "bulan-ini" },
  { label: "Bulan lalu", value: "bulan-lalu" },
  { label: "Pilih tanggal", value: "pilih-tanggal" },
];

/** Which waits the list drawer shows: one CS's, or the whole team's. */
interface WaitListTarget {
  title: string;
  userId?: string;
}

/** How fast the team answers customers: the team's figures, each CS's, each
 * day's, and the waits behind them. */
export function CsPerformancePage() {
  const [choice, setChoice] = useState<PeriodChoice>("hari-ini");
  const [period, setPeriod] = useState<PerformancePeriod>(() =>
    presetPeriod("hari-ini", new Date()),
  );
  const [waitList, setWaitList] = useState<WaitListTarget | null>(null);
  const { data: summary, isLoading } = useCsPerformanceSummary(period);

  const choose = (value: PeriodChoice) => {
    setChoice(value);
    if (value !== "pilih-tanggal") {
      setPeriod(presetPeriod(value, new Date()));
    }
  };

  return (
    <Space direction="vertical" size="large" style={{ width: "100%" }}>
      <PageHeader title="Kinerja CS" description="Seberapa cepat pelanggan dibalas" />
      <Space wrap>
        <Segmented
          options={PERIOD_OPTIONS}
          value={choice}
          onChange={(value) => choose(value as PeriodChoice)}
        />
        {choice === "pilih-tanggal" && (
          <RangePicker
            allowEmpty={[false, false]}
            onChange={(values) => {
              if (values?.[0] && values[1]) {
                setPeriod({
                  from: values[0].format("YYYY-MM-DD"),
                  to: values[1].format("YYYY-MM-DD"),
                });
              }
            }}
          />
        )}
      </Space>
      {!summary && isLoading && <Spin />}
      {summary && (
        <>
          <PerformanceTiles
            summary={summary}
            onShowWaits={() => setWaitList({ title: "Daftar giliran" })}
          />
          <AgentPerformanceTable
            agents={summary.agents}
            targetMinutes={summary.targetMinutes}
            loading={isLoading}
            onSelect={(agent) =>
              setWaitList({ title: `Giliran ${agent.username}`, userId: agent.userId })
            }
          />
          <DailyPerformanceChart days={summary.days} targetMinutes={summary.targetMinutes} />
        </>
      )}
      {waitList && (
        <WaitListDrawer
          title={waitList.title}
          period={period}
          userId={waitList.userId}
          onClose={() => setWaitList(null)}
        />
      )}
    </Space>
  );
}
```

- [ ] **Step 8: Daftarkan rute dan menunya**

Di `frontend/src/presentation/routes/index.tsx`:

1. Ganti komentar dan deklarasi lazy `GraphsPage` dengan:

```tsx
// Graphs and Kinerja CS are the routes that pull in recharts, so they load on
// demand to keep the charting library out of the initial bundle.
const GraphsPage = lazy(() =>
  import("../pages/GraphsPage").then((m) => ({ default: m.GraphsPage })),
);
const CsPerformancePage = lazy(() =>
  import("../pages/CsPerformancePage").then((m) => ({ default: m.CsPerformancePage })),
);
```

2. Tambahkan rute berikut tepat setelah objek rute `cs`:

```tsx
          {
            path: "cs-performance",
            element: (
              <Suspense
                fallback={
                  <div style={{ padding: 24, textAlign: "center" }}>
                    <Spin />
                  </div>
                }
              >
                <CsPerformancePage />
              </Suspense>
            ),
          },
```

Di `frontend/src/presentation/components/layout/navigationRoutes.tsx`:

1. Tambahkan `LineChartOutlined` ke impor `@ant-design/icons`.
2. Ganti blok untuk peran bukan viewer dengan:

```tsx
    // Every route under /api/v1/cs turns a viewer away, so showing them the
    // inbox only offers a page that answers 403 on every request it makes. The
    // report sits beside it rather than under /cs: ProLayout matches a menu
    // path by prefix, so a page at /cs/... would light up "CS Inbox" too.
    ...(role === UserRole.VIEWER
      ? []
      : [
          { path: "/cs", name: "CS Inbox", icon: <MessageOutlined /> },
          { path: "/cs-performance", name: "Kinerja CS", icon: <LineChartOutlined /> },
        ]),
```

Di `frontend/src/presentation/components/layout/navigationRoutes.test.tsx`, tambahkan tes berikut setelah tes "keeps the CS inbox away from a viewer":

```tsx
  it("lists Kinerja CS for everyone who can open the inbox", () => {
    expect(paths(UserRole.CS)).toContain("/cs-performance");
    expect(paths(UserRole.TECHNICIAN)).toContain("/cs-performance");
    expect(paths(UserRole.ADMIN)).toContain("/cs-performance");
    expect(paths(UserRole.VIEWER)).not.toContain("/cs-performance");
  });
```

- [ ] **Step 9: Luruskan spec soal path halaman**

Di `docs/superpowers/specs/2026-09-15-cs-response-time-design.md`, ganti potongan ini:

```
Halaman **"Kinerja CS"** di `/cs/performance`.
```

dengan:

```
Halaman **"Kinerja CS"** di `/cs-performance`. Bukan `/cs/performance`:
ProLayout mencocokkan path menu berdasarkan awalannya, jadi halaman di bawah
`/cs` membuat "CS Inbox" ikut tersorot.
```

- [ ] **Step 10: Jalankan tes, pastikan lulus**

Run: `cd frontend && npm test -- --run src/presentation/pages/__tests__/CsPerformancePage.test.tsx src/presentation/components/layout/navigationRoutes.test.tsx`
Expected: semua PASS, tanpa peringatan recharts di output.

Run: `cd frontend && npm test -- --run src/presentation/components/cs`
Expected: PASS, termasuk tes komponen CS yang sudah ada.

Run: `cd frontend && npx prettier --write "src/presentation/components/cs/performance/**" src/presentation/pages/CsPerformancePage.tsx src/presentation/pages/__tests__/CsPerformancePage.test.tsx src/presentation/routes/index.tsx src/presentation/components/layout/navigationRoutes.tsx src/presentation/components/layout/navigationRoutes.test.tsx && npm run lint && npx tsc --noEmit`
Expected: bersih.

- [ ] **Step 11: Commit**

```bash
cd /Users/rohadimraja/Documents/tikman
git add frontend/src/presentation/components/cs/performance/ \
        frontend/src/presentation/pages/CsPerformancePage.tsx \
        frontend/src/presentation/pages/__tests__/CsPerformancePage.test.tsx \
        frontend/src/presentation/routes/index.tsx \
        frontend/src/presentation/components/layout/navigationRoutes.tsx \
        frontend/src/presentation/components/layout/navigationRoutes.test.tsx \
        docs/superpowers/specs/2026-09-15-cs-response-time-design.md
git commit -m "$(cat <<'EOF'
feat(cs): show how fast the team answers, and the waits behind it

The page carries the team's figures, a row per CS, the days as a line, and
a list of the waits each figure is made of — a CS opens their own row and
reads the threads it came from.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 11: Verifikasi menyeluruh

**Files:** tidak ada perubahan kode, kecuali perbaikan bila ada gerbang yang gagal.

**Interfaces:**
- Consumes: hasil Task 1-10.
- Produces: branch yang lulus semua gerbang CI.

- [ ] **Step 1: Gerbang backend, dengan Postgres**

```bash
docker run -d --rm --name tikman-test-pg -e POSTGRES_PASSWORD=test -e POSTGRES_DB=tikman_test \
  -p 5439:5432 timescale/timescaledb:latest-pg15
docker exec tikman-test-pg sh -c 'until pg_isready -h 127.0.0.1 -U postgres -q; do sleep 1; done'
cd backend && TEST_POSTGRES_DSN="host=localhost port=5439 user=postgres password=test dbname=tikman_test sslmode=disable" \
  go test ./... -race
docker stop tikman-test-pg
```

Expected: yang gagal hanya 30 tes WireGuard baseline. Bandingkan nama tes yang gagal dengan daftar di Global Constraints, dan laporkan bila ada yang di luar daftar itu. Tidak boleh ada tanda `DATA RACE`.

- [ ] **Step 2: Gerbang backend lainnya**

```bash
cd backend
gofmt -s -l .
go vet ./...
go build ./...
go mod verify
```

Expected: `gofmt` tidak mencetak apa pun; sisanya bersih.

- [ ] **Step 3: Gerbang frontend**

```bash
cd frontend
npm test -- --run
npm run lint
npm run format:check
npm run build
```

Expected: semua lulus. Jumlah tes bertambah dari 639 menjadi lebih banyak karena tes baru di Task 8-10.

- [ ] **Step 4: Perbarui graphify**

Run: `cd /Users/rohadimraja/Documents/tikman && graphify update .`
Expected: selesai tanpa error. `graphify-out/` di-ignore git, jadi tidak ada yang perlu di-commit.

---

### Task 12: Deploy ke radpro (hanya dengan persetujuan user)

**Files:** tidak ada.

**Interfaces:**
- Consumes: branch yang lulus Task 11, sudah di-merge ke `main` dan di-push atas persetujuan user.

- [ ] **Step 1: Tanya user dulu**

Deploy ini membuat ulang `api` (menjalankan migrasi 52), lalu `worker`, `trapd`, `wa`, dan `frontend`. Membuat ulang `api` memutus polling dan trap sebentar, dan membuat ulang `wa` memutus sesi WhatsApp sebentar, jadi kerjakan di jam sepi. Jangan lanjut tanpa persetujuan eksplisit; merge dan push juga butuh persetujuan.

- [ ] **Step 2: Tarik kode di radpro**

```bash
ssh radpro 'cd /opt/tikman/src && git fetch && git checkout main && git pull --ff-only && git log -1 --oneline'
```

Jalankan `git` sebagai `radpro`, jangan dengan `sudo`: git di bawah sudo meninggalkan objek milik root yang memblokir fetch berikutnya.

- [ ] **Step 3: Build**

```bash
ssh radpro 'C="sudo -n docker compose --env-file /opt/tikman/.env -f docker-compose.yml -f docker-compose.vps.yml"; cd /opt/tikman/src && $C build api wa frontend'
```

Selalu pakai `--env-file` dan kedua file compose: tanpa `--env-file`, redis gagal start dan seluruh stack ikut mati.

- [ ] **Step 4: Buat ulang `api`, lalu `worker` dan `trapd`**

`worker` dan `trapd` berbagi network namespace `api`, jadi keduanya harus berhenti sebelum `api` dibuat ulang dan dijalankan lagi sesudahnya.

```bash
ssh radpro 'C="sudo -n docker compose --env-file /opt/tikman/.env -f docker-compose.yml -f docker-compose.vps.yml"; cd /opt/tikman/src && $C stop worker trapd && $C up -d --no-deps --force-recreate api'
ssh radpro 'until [ "$(sudo -n docker inspect -f "{{.State.Health.Status}}" tikman-api)" = healthy ]; do sleep 3; done; echo api healthy'
ssh radpro 'C="sudo -n docker compose --env-file /opt/tikman/.env -f docker-compose.yml -f docker-compose.vps.yml"; cd /opt/tikman/src && $C up -d --no-deps --force-recreate --scale worker=3 worker trapd'
```

- [ ] **Step 5: Buat ulang `wa` dan `frontend`**

```bash
ssh radpro 'C="sudo -n docker compose --env-file /opt/tikman/.env -f docker-compose.yml -f docker-compose.vps.yml"; cd /opt/tikman/src && $C up -d --no-deps --force-recreate wa && $C up -d --no-deps --force-recreate frontend'
```

`--no-deps` di sini penting: `wa` bergantung pada `api`, `postgres`, dan `redis`, dan tanpa itu Compose bisa ikut membuat ulang `api` dan membuat `worker`/`trapd` tertinggal di namespace lama.

- [ ] **Step 6: Verifikasi**

```bash
ssh radpro 'sudo -n docker ps --format "{{.Names}}\t{{.Status}}" | grep -E "tikman-(api|wa|frontend|trapd)|src-worker" | sort'
```
Expected: kelimanya berjalan, `api` dan `frontend` healthy.

```bash
ssh radpro 'sudo -n docker exec tikman-postgres psql -U tikman -d tikman -c "SELECT indexname FROM pg_indexes WHERE tablename = '"'"'cs_waits'"'"' ORDER BY indexname"'
```
Expected: memuat `uq_cs_waits_one_open_per_thread`, `idx_cs_waits_ended_at`, dan `idx_cs_waits_ended_by_ended_at`. Bila perintah ini diblokir untuk Anda, minta user menjalankannya dengan awalan `!`.

```bash
ssh radpro 'sudo -n docker exec tikman-api sh -c "curl -s -o /dev/null -w %{http_code} http://127.0.0.1:8080/api/v1/cs/performance/summary"'
```
Expected: `401` — rutenya terdaftar. `404` berarti build lama.

```bash
ssh radpro 'sudo -n docker exec tikman-wa sh -c '"'"'grep -c "Could not record a CS wait" $(readlink -f /proc/1/exe)'"'"''
```
Expected: angka ≥ 1.

```bash
bundle=$(curl -s https://noc.radpro.id | grep -oE 'assets/index-[A-Za-z0-9_-]+\.js' | head -1)
curl -s "https://noc.radpro.id/$bundle" | grep -c "Kinerja CS"
```
Expected: angka ≥ 1.

```bash
ssh radpro 'sudo -n docker logs --since 5m tikman-wa 2>&1 | grep -ciE "error|panic" ; sudo -n docker logs --since 5m tikman-api 2>&1 | grep -ci "Could not record a CS wait"'
```
Expected: keduanya `0`.

- [ ] **Step 7: Periksa dengan pelanggan sungguhan**

Minta user mengirim satu pesan uji dari nomor pelanggan, lalu membalasnya dari TikMan. Buka halaman "Kinerja CS" dengan periode "Hari ini". Expected: giliran itu muncul di angka tim dan di baris CS yang membalas, dan daftar gilirannya menautkan ke thread yang benar.
