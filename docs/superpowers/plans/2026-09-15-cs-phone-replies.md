# Balasan CS dari HP — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Pesan yang diketik di HP nomor CS untuk pelanggan yang sudah punya thread tersimpan di thread itu sebagai pesan `out` tanpa pengirim, berlabel "dari HP", dan mengeluarkan thread dari "Belum dibalas".

**Architecture:** `inboundHandler` di proses `wa` berhenti membuang event `IsFromMe` untuk chat pribadi dan meneruskannya ke `handleFromPhone`, yang mencari thread lewat `Chat` lalu `RecipientAlt` tanpa pernah membuat thread. Penyimpanan memakai kerangka transaksi yang sama dengan `SaveInbound` (dipisah menjadi `saveArrived`), dengan baris `out` tanpa pengirim dan UPDATE thread bersyarat `last_message_at <= waktu pesan`. Frontend menampilkan label dari aturan "pesan `out` tanpa `senderUserId`".

**Tech Stack:** Go 1.25 (GORM, whatsmeow, testify, SQLite untuk tes unit, Postgres/TimescaleDB untuk tes `OnPostgres`), React 18 + TypeScript (Ant Design 5, Vitest 4, Testing Library).

**Spec:** `docs/superpowers/specs/2026-09-15-cs-phone-replies-design.md`

## Global Constraints

- Tanpa migrasi dan tanpa kolom baru. "Pesan `out` tanpa `sender_user_id`" adalah satu-satunya penanda pesan dari HP.
- Pesan dari HP tidak pernah membuat thread, tidak membuka ulang thread yang ditutup, tidak mengubah `assigned_user_id` atau `status`, dan tidak memicu push.
- Grup (termasuk status/broadcast) dan channel tetap dibuang.
- Tidak ada dependensi baru.
- File maksimal 350 baris, fungsi maksimal 50 baris, nesting maksimal 3 (CLAUDE.md).
- Komentar menjelaskan *kenapa*. Nama tes Go dan komentar kode dalam bahasa Inggris, mengikuti file di sekitarnya. Teks UI dalam bahasa Indonesia: label persis `dari HP`.
- Semua tes lama tetap lulus. `gofmt -s -l .` kosong, `go vet ./...` bersih, `npm run lint` dan `npm run format:check` bersih.
- Kerja di branch `feat/cs-phone-replies`. Merge, push, dan deploy hanya dengan persetujuan eksplisit user.
- Setiap commit diakhiri dengan baris `Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>`.

---

### Task 1: Simpan pesan dari HP di service

**Files:**
- Modify: `backend/internal/services/cs_message_service.go:66-110` (`SaveInbound` menjadi pemanggil `saveArrived`)
- Modify: `backend/internal/services/cs_message_fields.go:16-37` (`inboundRow` memakai `arrivedRow`)
- Modify: `backend/internal/models/cs_message.go:56` (komentar `SenderUserID`)
- Create: `backend/internal/services/cs_message_phone.go`
- Test: `backend/internal/services/cs_message_phone_test.go`
- Test: `backend/internal/services/cs_message_phone_postgres_test.go`

**Interfaces:**
- Consumes: `InboundMessage`, `quotedRow(tx, conversationID, waMessageID) *uuid.UUID`, `applyMedia`, `bumpConversation(tx, conversationID, at) error`, helper tes `awaitingSetup`, `thread`, `customerWrote` (`cs_awaiting_reply_test.go`), `setupPostgresTestDB` (`cs_message_postgres_test.go`).
- Produces: `func (s *CSMessageService) SaveFromPhone(in InboundMessage) (*models.CSMessage, bool, error)`. Nilai `bool` bernilai `false` bila `wa_message_id` sudah tersimpan. Task 2 memakainya sebagai nilai fungsi `h.messages.SaveFromPhone`.

- [ ] **Step 0: Siapkan branch dan commit dokumen**

```bash
cd /Users/rohadimraja/Documents/tikman
git switch -c feat/cs-phone-replies
git add docs/superpowers/specs/2026-09-15-cs-phone-replies-design.md \
        docs/superpowers/specs/2026-09-15-cs-response-time-design.md \
        docs/superpowers/plans/2026-09-15-cs-phone-replies.md
git commit -m "$(cat <<'EOF'
docs(cs): design how CS reply time is measured, starting with phone replies

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
EOF
)"
```

- [ ] **Step 1: Tulis tes yang gagal**

Buat `backend/internal/services/cs_message_phone_test.go`:

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

func phoneMessage(convID uuid.UUID, waID string, at time.Time) InboundMessage {
	return InboundMessage{
		ConversationID: convID, WAMessageID: waID,
		Kind: models.MessageKindText, Body: "sudah kami cek dari hp", At: at,
	}
}

// Nothing marks a phone reply but its shape: outbound, with no CS on it. Stored
// with a sender or as inbound, it would read as a TikMan reply or as the
// customer talking.
func TestAPhoneReplyIsStoredAsOutboundWithNoSender(t *testing.T) {
	messages, conversations, acc := awaitingSetup(t)
	conv := thread(t, conversations, acc, "628111222333")

	msg, created, err := messages.SaveFromPhone(phoneMessage(conv.ID, "3EB0PHONE", time.Now()))

	require.NoError(t, err)
	assert.True(t, created)
	assert.Equal(t, models.MessageOut, msg.Direction)
	assert.Nil(t, msg.SenderUserID)
	assert.Equal(t, models.MessageSent, msg.Status)
}

// The customer has been answered, so the thread must stop asking for an answer
// and stop badging messages somebody already read on the phone.
func TestAPhoneReplyTakesTheThreadOutOfAwaitingReply(t *testing.T) {
	messages, conversations, acc := awaitingSetup(t)
	conv := thread(t, conversations, acc, "628111222333")
	customerWrote(t, messages, conv.ID, "3EB0A")

	_, _, err := messages.SaveFromPhone(phoneMessage(conv.ID, "3EB0PHONE", time.Now()))
	require.NoError(t, err)

	waiting, err := conversations.List(ConversationFilter{AwaitingReply: true})
	require.NoError(t, err)
	assert.Empty(t, waiting)
	got, err := conversations.Get(conv.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, got.UnreadCount)
}

// A phone reply WhatsApp hands over late, after the customer wrote again, did
// not answer that newer message. Treating it as the last word would hide a
// customer who is still waiting.
func TestALatePhoneReplyLeavesANewerCustomerMessageWaiting(t *testing.T) {
	messages, conversations, acc := awaitingSetup(t)
	conv := thread(t, conversations, acc, "628111222333")
	customerWrote(t, messages, conv.ID, "3EB0A")

	earlier := time.Now().Add(-time.Minute)
	_, _, err := messages.SaveFromPhone(phoneMessage(conv.ID, "3EB0PHONE", earlier))
	require.NoError(t, err)

	waiting, err := conversations.List(ConversationFilter{AwaitingReply: true})
	require.NoError(t, err)
	require.Len(t, waiting, 1)
	got, err := conversations.Get(conv.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, got.UnreadCount)
}

// Nobody is known to have sent it, so it can neither claim the thread nor
// reopen one a CS decided was finished.
func TestAPhoneReplyLeavesTheHolderAndAClosedThreadAlone(t *testing.T) {
	messages, conversations, acc := awaitingSetup(t)
	conv := thread(t, conversations, acc, "628111222333")
	customerWrote(t, messages, conv.ID, "3EB0A")
	holder := uuid.New()
	require.NoError(t, conversations.Assign(conv.ID, holder))
	require.NoError(t, conversations.Close(conv.ID))

	_, _, err := messages.SaveFromPhone(phoneMessage(conv.ID, "3EB0PHONE", time.Now()))
	require.NoError(t, err)

	got, err := conversations.Get(conv.ID)
	require.NoError(t, err)
	assert.Equal(t, models.ConversationClosed, got.Status)
	require.NotNil(t, got.AssignedUserID)
	assert.Equal(t, holder, *got.AssignedUserID)
}

// WhatsApp re-delivers events on every reconnect. A second copy would show the
// reply twice in the thread.
func TestTheSamePhoneReplyDeliveredTwiceIsStoredOnce(t *testing.T) {
	messages, conversations, acc := awaitingSetup(t)
	conv := thread(t, conversations, acc, "628111222333")
	in := phoneMessage(conv.ID, "3EB0PHONE", time.Now())

	first, created, err := messages.SaveFromPhone(in)
	require.NoError(t, err)
	require.True(t, created)
	second, created, err := messages.SaveFromPhone(in)
	require.NoError(t, err)

	assert.False(t, created)
	assert.Equal(t, first.ID, second.ID)
	history, err := messages.History(conv.ID, 10, 0)
	require.NoError(t, err)
	assert.Len(t, history, 1)
}
```

Buat `backend/internal/services/cs_message_phone_postgres_test.go`:

```go
package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

// The ordering guard compares timestamps in SQL. SQLite compares them as text,
// so only Postgres says whether the guard holds where it actually runs.
func TestAPhoneReplyAnswersOnlyWhenItIsTheNewestMessageOnPostgres(t *testing.T) {
	db := setupPostgresTestDB(t)
	conversations := NewCSConversationService(db)
	messages := NewCSMessageService(db, conversations)
	account := models.WAAccount{Label: "CS Utama", Status: models.WAAccountConnected}
	require.NoError(t, db.Create(&account).Error)
	conv, err := conversations.FindOrCreate(IncomingPeer{
		WAAccountID: account.ID, JID: "628111@s.whatsapp.net", Phone: "628111222333", Name: "Budi",
	})
	require.NoError(t, err)

	customerAt := time.Date(2026, 9, 15, 3, 0, 0, 0, time.UTC)
	_, _, err = messages.SaveInbound(InboundMessage{
		ConversationID: conv.ID, WAMessageID: "3EB0CUSTOMER",
		Kind: models.MessageKindText, Body: "internet mati", At: customerAt,
	})
	require.NoError(t, err)

	_, _, err = messages.SaveFromPhone(phoneMessage(conv.ID, "3EB0LATE", customerAt.Add(-time.Minute)))
	require.NoError(t, err)
	waiting, err := conversations.List(ConversationFilter{AwaitingReply: true})
	require.NoError(t, err)
	require.Len(t, waiting, 1, "a reply older than the customer's message answers nothing")

	_, _, err = messages.SaveFromPhone(phoneMessage(conv.ID, "3EB0ANSWER", customerAt.Add(time.Minute)))
	require.NoError(t, err)
	waiting, err = conversations.List(ConversationFilter{AwaitingReply: true})
	require.NoError(t, err)
	assert.Empty(t, waiting)
}
```

- [ ] **Step 2: Jalankan tes, pastikan gagal**

Run: `cd backend && go test ./internal/services/ -run 'PhoneReply' -v`
Expected: FAIL saat kompilasi, dengan `messages.SaveFromPhone undefined (type *CSMessageService has no field or method SaveFromPhone)`.

- [ ] **Step 3: Pisahkan pembentuk baris di `cs_message_fields.go`**

Ganti fungsi `inboundRow` (baris 16-37) dengan:

```go
// inboundRow renders a customer's message as the row that will hold it.
func inboundRow(tx *gorm.DB, in InboundMessage) models.CSMessage {
	return arrivedRow(tx, in, models.MessageIn, models.MessageDelivered)
}

// arrivedRow renders a message WhatsApp delivered as the row that will hold
// it. It runs on the caller's transaction because the quoted message it points
// at must be looked up in the same one.
func arrivedRow(
	tx *gorm.DB, in InboundMessage, direction models.MessageDirection, status models.MessageStatus,
) models.CSMessage {
	waID := in.WAMessageID
	row := models.CSMessage{
		ConversationID:     in.ConversationID,
		WAMessageID:        &waID,
		Direction:          direction,
		Kind:               in.Kind,
		Body:               in.Body,
		PreviewURL:         previewURL(in.Preview),
		PreviewTitle:       previewTitle(in.Preview),
		PreviewDescription: previewDescription(in.Preview),
		PreviewThumbnail:   previewThumbnail(in.Preview),
		Status:             status,
		ReplyToID:          quotedRow(tx, in.ConversationID, in.ReplyToWAID),
		WATimestamp:        in.At,
	}
	applyMedia(&row, in.Media)
	return row
}
```

- [ ] **Step 4: Jadikan transaksi `SaveInbound` kerangka bersama**

Di `cs_message_service.go`, ganti seluruh `SaveInbound` (baris 66-110, dari komentar `// SaveInbound stores...` sampai `return &stored, created, nil }`) dengan:

```go
// SaveInbound stores an incoming message, answering false when this WhatsApp
// message was already stored. WhatsApp re-delivers events it is unsure about,
// and the duplicate would otherwise be shown to the CS and counted as unread.
func (s *CSMessageService) SaveInbound(in InboundMessage) (*models.CSMessage, bool, error) {
	return s.saveArrived(in, inboundRow, bumpConversation)
}

// saveArrived stores one message WhatsApp delivered, skipping one already
// stored. row renders it and touch updates its thread; the duplicate check and
// the transaction are shared so the two directions cannot drift apart on them.
//
// The lookup is done here as well as by the partial unique index in migration
// 41, because SQLite tests never get that index.
func (s *CSMessageService) saveArrived(
	in InboundMessage,
	row func(*gorm.DB, InboundMessage) models.CSMessage,
	touch func(*gorm.DB, uuid.UUID, time.Time) error,
) (*models.CSMessage, bool, error) {
	var (
		stored  models.CSMessage
		created bool
	)

	// One transaction, for two reasons. The caller in the wa process deletes the
	// attachment when this answers with an error, so a message row that survived
	// a failed thread update would have its file deleted out from under it —
	// and nothing repairs that, because Sweep tolerates a missing file. And a
	// message stored behind a stale last_message_at would sit in the inbox
	// without surfacing. Storing the message and the inbox knowing about it are
	// one fact, so they commit as one.
	err := s.db.Transaction(func(tx *gorm.DB) error {
		lookup := tx.Where("wa_message_id = ?", in.WAMessageID).First(&stored).Error
		if lookup == nil {
			return nil // WhatsApp re-delivered one it had already given us
		}
		if !errors.Is(lookup, gorm.ErrRecordNotFound) {
			return fmt.Errorf("look for existing message: %w", lookup)
		}

		stored = row(tx, in)
		if err := tx.Create(&stored).Error; err != nil {
			return fmt.Errorf("store message from whatsapp: %w", err)
		}
		if err := touch(tx, in.ConversationID, in.At); err != nil {
			return err
		}

		created = true
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return &stored, created, nil
}
```

`uuid`, `time`, `errors`, `fmt`, dan `gorm` sudah diimpor di file ini.

- [ ] **Step 5: Tulis `SaveFromPhone`**

Buat `backend/internal/services/cs_message_phone.go`:

```go
package services

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// SaveFromPhone stores a message the phone holding the number sent to a
// customer, answering false when WhatsApp had already delivered it.
//
// A CS sometimes answers from that phone instead of from TikMan. Left out, the
// thread stays in "Belum dibalas" with the customer already answered, and the
// next CS answers them again. No sender is recorded: the phone is shared, and
// naming the wrong CS would be worse than naming none.
func (s *CSMessageService) SaveFromPhone(in InboundMessage) (*models.CSMessage, bool, error) {
	return s.saveArrived(in, phoneRow, markAnsweredFromPhone)
}

// phoneRow renders a phone message as an outbound row with no sender, which is
// how every later reader tells it apart from a reply sent through TikMan.
func phoneRow(tx *gorm.DB, in InboundMessage) models.CSMessage {
	return arrivedRow(tx, in, models.MessageOut, models.MessageSent)
}

// markAnsweredFromPhone takes the thread out of "Belum dibalas" only when the
// phone's message is the newest thing in it. WhatsApp can deliver a phone reply
// late, after a reconnect, when the customer has already written again; that
// newer message must stay waiting, and its badge must stay with it.
func markAnsweredFromPhone(tx *gorm.DB, conversationID uuid.UUID, at time.Time) error {
	err := tx.Model(&models.CSConversation{}).
		Where("id = ? AND last_message_at <= ?", conversationID, at).
		Updates(map[string]any{
			"last_message_at":        at,
			"last_message_direction": models.MessageOut,
			"unread_count":           0,
		}).Error
	if err != nil {
		return fmt.Errorf("mark thread answered from phone: %w", err)
	}
	return nil
}
```

- [ ] **Step 6: Catat aturannya di model**

Di `backend/internal/models/cs_message.go`, tambahkan komentar tepat di atas field `SenderUserID` (baris 56):

```go
	// SenderUserID is nil on an outbound row typed on the phone holding the
	// number. Every reply TikMan sends carries the CS who wrote it (see
	// CSMessageService.Queue), and that is the only thing that tells the two
	// apart.
	SenderUserID   *uuid.UUID       `gorm:"type:uuid;index" json:"sender_user_id,omitempty"`
```

Lalu rapikan: `cd backend && gofmt -s -w internal/models/cs_message.go internal/services/cs_message_service.go internal/services/cs_message_fields.go internal/services/cs_message_phone.go internal/services/cs_message_phone_test.go internal/services/cs_message_phone_postgres_test.go`

- [ ] **Step 7: Jalankan tes, pastikan lulus**

Run: `cd backend && go test ./internal/services/ -run 'PhoneReply|Awaiting|Inbound|Quote|Purge|Retention' -v`
Expected: PASS semua. Tes `OnPostgres` di-skip bila `TEST_POSTGRES_DSN` tidak diset; langkah berikutnya menjalankannya.

Run: `cd backend && go test ./internal/services/ -v`
Expected: PASS (seluruh paket, termasuk tes `SaveInbound` lama yang kini lewat `saveArrived`).

- [ ] **Step 8: Jalankan tes Postgres**

```bash
docker run -d --rm --name tikman-test-pg -e POSTGRES_PASSWORD=test -e POSTGRES_DB=tikman_test \
  -p 5439:5432 timescale/timescaledb:latest-pg15
docker exec tikman-test-pg sh -c 'until pg_isready -h 127.0.0.1 -U postgres -q; do sleep 1; done'
cd backend && TEST_POSTGRES_DSN="host=localhost port=5439 user=postgres password=test dbname=tikman_test sslmode=disable" \
  go test ./internal/services/ -run 'OnPostgres' -v
docker stop tikman-test-pg
```

Expected: `TestAPhoneReplyAnswersOnlyWhenItIsTheNewestMessageOnPostgres` PASS, dan tes `OnPostgres` lain tetap PASS. Bila port 5439 terpakai, pakai port lain di `-p` dan DSN.

- [ ] **Step 9: Commit**

```bash
cd /Users/rohadimraja/Documents/tikman
git add backend/internal/models/cs_message.go backend/internal/services/cs_message_service.go \
        backend/internal/services/cs_message_fields.go backend/internal/services/cs_message_phone.go \
        backend/internal/services/cs_message_phone_test.go backend/internal/services/cs_message_phone_postgres_test.go
git commit -m "$(cat <<'EOF'
feat(cs): store what the number's phone sends to a customer

A reply typed on the phone left its thread in "Belum dibalas" with the
customer already answered. It is now stored as an outbound message with
no sender, and takes the thread out of the waiting view only when it is
the newest message there.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 2: Teruskan pesan dari HP di proses `wa`

**Files:**
- Modify: `backend/internal/wa/inbound.go:1-107` (impor `models`, `handle`, `store` baru, `attachmentFor`)
- Create: `backend/internal/wa/inbound_phone.go`
- Test: `backend/internal/wa/inbound_phone_test.go`

**Interfaces:**
- Consumes: `func (s *CSMessageService) SaveFromPhone(in services.InboundMessage) (*models.CSMessage, bool, error)` dari Task 1; `func (s *CSConversationService) FindByPeer(waAccountID uuid.UUID, jid string) (*models.CSConversation, error)` (nil, nil bila tidak ada); helper tes `inboundSetup(t) (*inboundHandler, *services.CSMessageService, *services.CSConversationService, uuid.UUID)` dari `inbound_quote_test.go`. Thread yang dibuat `inboundSetup` tersimpan dengan JID `628111@s.whatsapp.net`.
- Produces: `func (h *inboundHandler) handleFromPhone(ctx context.Context, evt *events.Message, att attachment) error`, dan `type saveFunc func(services.InboundMessage) (*models.CSMessage, bool, error)`.

- [ ] **Step 1: Tulis tes**

Buat `backend/internal/wa/inbound_phone_test.go`:

```go
package wa

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

// customerNumber is the address inboundSetup opens its thread under.
var customerNumber = types.JID{User: "628111", Server: types.DefaultUserServer}

// customerLID is the same customer as WhatsApp's privacy identifier names them.
var customerLID = types.JID{User: "213911014010978", Server: types.HiddenUserServer}

// phoneSends builds what whatsmeow hands over when the phone holding the number
// sends a message: IsFromMe, the recipient as the chat, and the recipient's
// other address in RecipientAlt.
func phoneSends(id, text string, chat, recipientAlt types.JID) *events.Message {
	return &events.Message{
		Info: types.MessageInfo{
			ID:        id,
			PushName:  "CS Utama",
			Timestamp: time.Now(),
			MessageSource: types.MessageSource{
				Chat:         chat,
				Sender:       types.JID{User: "628999000111", Server: types.DefaultUserServer},
				IsFromMe:     true,
				RecipientAlt: recipientAlt,
			},
		},
		Message: &waE2E.Message{Conversation: proto.String(text)},
	}
}

func TestAMessageTypedOnThePhoneLandsInTheCustomersThread(t *testing.T) {
	handler, messages, _, convID := inboundSetup(t)

	evt := phoneSends("3EB0PHONE", "sudah kami cek", customerNumber, types.EmptyJID)
	require.NoError(t, handler.handle(context.Background(), evt))

	history, err := messages.History(convID, 10, 0)
	require.NoError(t, err)
	require.Len(t, history, 1)
	assert.Equal(t, "sudah kami cek", history[0].Body)
	assert.Equal(t, models.MessageOut, history[0].Direction)
	assert.Nil(t, history[0].SenderUserID)
}

// The phone's copy may name the customer by LID while the thread was opened
// under their number. Looking only at Chat would drop the reply.
func TestAPhoneMessageAddressedByLIDFindsTheThreadOpenedUnderTheNumber(t *testing.T) {
	handler, messages, _, convID := inboundSetup(t)

	evt := phoneSends("3EB0LID", "sudah kami cek", customerLID, customerNumber)
	require.NoError(t, handler.handle(context.Background(), evt))

	history, err := messages.History(convID, 10, 0)
	require.NoError(t, err)
	assert.Len(t, history, 1)
}

// WhatsApp re-delivers on every reconnect; the reply must show once.
func TestAPhoneMessageDeliveredTwiceShowsOnce(t *testing.T) {
	handler, messages, _, convID := inboundSetup(t)
	evt := phoneSends("3EB0TWICE", "sudah kami cek", customerNumber, types.EmptyJID)

	require.NoError(t, handler.handle(context.Background(), evt))
	require.NoError(t, handler.handle(context.Background(), evt))

	history, err := messages.History(convID, 10, 0)
	require.NoError(t, err)
	assert.Len(t, history, 1)
}

// The number's phone also carries chats that are not customers'. One the phone
// starts with somebody who has no thread must not open one.
func TestAPhoneMessageToSomeoneWithNoThreadOpensNothing(t *testing.T) {
	handler, _, conversations, _ := inboundSetup(t)
	before, err := conversations.List(services.ConversationFilter{})
	require.NoError(t, err)

	stranger := types.JID{User: "628777000111", Server: types.DefaultUserServer}
	require.NoError(t, handler.handle(context.Background(),
		phoneSends("3EB0STRANGER", "nanti saya telepon", stranger, types.EmptyJID)))

	after, err := conversations.List(services.ConversationFilter{})
	require.NoError(t, err)
	assert.Len(t, after, len(before))
}

// Letting IsFromMe through must not let the phone's groups and channels in with
// it.
func TestAPhoneMessageToAGroupOrChannelIsStillIgnored(t *testing.T) {
	cases := []struct {
		name    string
		chat    types.JID
		isGroup bool
	}{
		{"group", types.JID{User: "120363000000000000", Server: types.GroupServer}, true},
		{"channel", types.JID{User: "120363399457624066", Server: types.NewsletterServer}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler, messages, conversations, convID := inboundSetup(t)
			before, err := conversations.List(services.ConversationFilter{})
			require.NoError(t, err)

			evt := phoneSends("3EB0"+tc.name, "rapat jam 3", tc.chat, types.EmptyJID)
			evt.Info.IsGroup = tc.isGroup
			require.NoError(t, handler.handle(context.Background(), evt))

			after, err := conversations.List(services.ConversationFilter{})
			require.NoError(t, err)
			assert.Len(t, after, len(before))
			history, err := messages.History(convID, 10, 0)
			require.NoError(t, err)
			assert.Empty(t, history)
		})
	}
}
```

- [ ] **Step 2: Jalankan tes, pastikan yang baru gagal**

Run: `cd backend && go test ./internal/wa/ -run 'Phone' -v`
Expected:
- FAIL: `TestAMessageTypedOnThePhoneLandsInTheCustomersThread`, `TestAPhoneMessageAddressedByLIDFindsTheThreadOpenedUnderTheNumber`, `TestAPhoneMessageDeliveredTwiceShowsOnce`, karena history kosong (`IsFromMe` masih dibuang).
- PASS: `TestAPhoneMessageToSomeoneWithNoThreadOpensNothing` dan `TestAPhoneMessageToAGroupOrChannelIsStillIgnored`. Keduanya penjaga perilaku yang harus bertahan setelah perubahan.

- [ ] **Step 3: Ubah `inbound.go`**

Tambahkan impor `"github.com/tikman/olt-provisioning/internal/models"` ke blok impor.

Ganti `handle` (baris 31-79) dengan:

```go
// handle turns one WhatsApp event into a stored message. The order matters: the
// thread must exist before the message lands in it, and the message must be
// stored before anyone is told to come and read it.
func (h *inboundHandler) handle(ctx context.Context, evt *events.Message) error {
	att, keep := h.attachmentFor(evt)
	if !keep {
		return nil
	}
	if evt.Info.IsFromMe {
		return h.handleFromPhone(ctx, evt, att)
	}

	conv, err := h.conversations.FindOrCreate(services.IncomingPeer{
		WAAccountID: h.accountID,
		JID:         evt.Info.Chat.ToNonAD().String(),
		Phone:       senderPhone(evt.Info.MessageSource),
		Name:        evt.Info.PushName,
	})
	if err != nil {
		return err
	}
	return h.store(ctx, evt, att, conv.ID, h.messages.SaveInbound)
}

// saveFunc stores one direction of what WhatsApp delivers: SaveInbound for the
// customer, SaveFromPhone for the phone holding the number.
type saveFunc func(services.InboundMessage) (*models.CSMessage, bool, error)

// store fetches what a message carries, saves it, and tells the browsers. Both
// directions share it, so a failed save or a re-delivered duplicate cleans up
// its attachment the same way whichever side sent it.
func (h *inboundHandler) store(
	ctx context.Context, evt *events.Message, att attachment, conversationID uuid.UUID, save saveFunc,
) error {
	body, media := h.fetch(ctx, evt, att)

	msg, created, err := save(services.InboundMessage{
		ConversationID: conversationID,
		WAMessageID:    evt.Info.ID,
		ReplyToWAID:    quotedStanzaID(evt.Message),
		Kind:           att.kind,
		Body:           body,
		Preview:        inboundPreview(evt.Message),
		Media:          media,
		At:             evt.Info.Timestamp,
	})
	if err != nil {
		// Safe to delete because each save is one transaction: an error here
		// means no row was committed, so nothing names this file.
		h.discard(evt, media)
		return err
	}
	if !created {
		// WhatsApp re-delivered one it had already given us, and it does that on
		// every reconnect. The row it belongs to already names its own copy, so
		// this second file is referenced by nothing — and CSMediaRetention
		// sweeps from rows, so nothing would ever collect it.
		h.discard(evt, media)
		return nil
	}

	h.announce(ctx, conversationID, msg.ID)
	return nil
}
```

Ganti komentar dan kondisi awal `attachmentFor` (baris 81-94) dengan:

```go
// attachmentFor decides whether a message belongs in this inbox at all, and
// what shape it has.
//
// This inbox holds one-to-one chats: a customer's messages, and what the phone
// holding the number sent them (see handleFromPhone). Groups and channels stay
// out. The newsletter check cannot be folded into IsGroup: whatsmeow sets that
// flag only for GroupServer and BroadcastServer, so a channel arrives looking
// like a one-to-one chat. Nor does IsFromMe catch our own updates coming back —
// WhatsApp delivers those to the number that posted them with the channel as
// the sender, and a CS answering that thread would publish their reply to the
// channel's subscribers.
func (h *inboundHandler) attachmentFor(evt *events.Message) (attachment, bool) {
	if evt.Info.IsGroup || evt.Info.Chat.Server == types.NewsletterServer {
		return attachment{}, false
	}
```

Sisa `attachmentFor` (mulai `att, readable := describe(evt.Message)`) tidak berubah.

- [ ] **Step 4: Tulis `inbound_phone.go`**

Buat `backend/internal/wa/inbound_phone.go`:

```go
package wa

import (
	"context"

	"github.com/tikman/olt-provisioning/internal/models"
	"go.mau.fi/whatsmeow/types/events"
	"go.uber.org/zap"
)

// handleFromPhone stores a message the phone holding this number sent to a
// customer who already has a thread here.
//
// Only that phone can produce these: whatsmeow never encrypts a copy of what
// this process sends for this process's own device, so nothing TikMan sent
// comes back as IsFromMe. A chat the phone starts with someone who has no
// thread stays out, so the phone's other chats do not fill the inbox.
func (h *inboundHandler) handleFromPhone(ctx context.Context, evt *events.Message, att attachment) error {
	conv, err := h.threadForPhoneMessage(evt)
	if err != nil {
		return err
	}
	if conv == nil {
		h.logger.Debug("Skipping a phone message to someone with no thread",
			zap.String("wa_message_id", evt.Info.ID))
		return nil
	}
	return h.store(ctx, evt, att, conv.ID, h.messages.SaveFromPhone)
}

// threadForPhoneMessage finds the customer's thread by the address the phone
// used, then by the other one. WhatsApp names the same person by phone number
// or by LID, and a thread opened under one can be answered under the other.
func (h *inboundHandler) threadForPhoneMessage(evt *events.Message) (*models.CSConversation, error) {
	conv, err := h.conversations.FindByPeer(h.accountID, evt.Info.Chat.ToNonAD().String())
	if err != nil || conv != nil || evt.Info.RecipientAlt.IsEmpty() {
		return conv, err
	}
	return h.conversations.FindByPeer(h.accountID, evt.Info.RecipientAlt.ToNonAD().String())
}
```

Lalu: `cd backend && gofmt -s -w internal/wa/inbound.go internal/wa/inbound_phone.go internal/wa/inbound_phone_test.go`

- [ ] **Step 5: Jalankan tes, pastikan lulus**

Run: `cd backend && go test ./internal/wa/ -run 'Phone' -v`
Expected: semua tes PASS (5 fungsi tes; yang group/channel punya 2 subtes).

Run: `cd backend && go test ./internal/wa/ -v`
Expected: PASS seluruh paket, termasuk `inbound_quote_test.go` dan `inbound_newsletter_test.go`.

- [ ] **Step 6: Commit**

```bash
cd /Users/rohadimraja/Documents/tikman
git add backend/internal/wa/inbound.go backend/internal/wa/inbound_phone.go backend/internal/wa/inbound_phone_test.go
git commit -m "$(cat <<'EOF'
feat(cs): let a reply typed on the phone reach its thread

The wa process dropped every IsFromMe message. A one-to-one message from
the phone holding the number now lands in the customer's existing thread,
found by the chat address or its LID/number alternate. It never opens a
thread, and groups and channels stay out.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 3: Label "dari HP" di thread

**Files:**
- Modify: `frontend/src/presentation/components/cs/MessageThread.tsx:30-35` (helper baru) dan `:175-188` (baris jam)
- Test: `frontend/src/presentation/components/cs/__tests__/MessageThreadPhoneReply.test.tsx`

**Interfaces:**
- Consumes: `CsMessage.senderUserId?: string` (`frontend/src/domain/entities/CsMessage.ts`). Dari API, field ini tidak ada untuk pesan dari HP karena `sender_user_id` ber-`omitempty`.
- Produces: tidak ada yang dipakai task lain.

- [ ] **Step 1: Tulis tes yang gagal**

Buat `frontend/src/presentation/components/cs/__tests__/MessageThreadPhoneReply.test.tsx`:

```tsx
import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { MessageThread } from "../MessageThread";
import type { CsMessage } from "@/domain/entities";

function reply(overrides: Partial<CsMessage> = {}): CsMessage {
  return {
    id: "m1",
    conversationId: "c1",
    direction: "out",
    kind: "text",
    body: "sudah kami cek",
    status: "sent",
    waTimestamp: "2026-09-15T10:00:00Z",
    ...overrides,
  };
}

function draw(message: CsMessage) {
  return render(<MessageThread messages={[message]} onRetry={vi.fn()} />);
}

describe("a reply typed on the phone", () => {
  // Without the label a CS reads an answer nobody in TikMan wrote and asks
  // around for who sent it.
  it("is labelled as sent from the phone", () => {
    draw(reply());
    expect(screen.getByText("dari HP")).toBeInTheDocument();
  });

  it("leaves a reply sent from TikMan unlabelled", () => {
    draw(reply({ senderUserId: "u1" }));
    expect(screen.queryByText("dari HP")).not.toBeInTheDocument();
  });

  it("never labels the customer's own message", () => {
    draw(reply({ direction: "in", status: "delivered" }));
    expect(screen.queryByText("dari HP")).not.toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Jalankan tes, pastikan gagal**

Run: `cd frontend && npm test -- --run src/presentation/components/cs/__tests__/MessageThreadPhoneReply.test.tsx`
Expected: FAIL pada `is labelled as sent from the phone` (`Unable to find an element with the text: dari HP`). Dua tes lain PASS.

- [ ] **Step 3: Tambahkan label**

Di `MessageThread.tsx`, tambahkan tepat setelah fungsi `quoteAuthor` (baris 33-35):

```tsx
/** A reply with no sender was typed on the phone holding the number: every
 * reply sent from TikMan carries the CS who wrote it. */
function sentFromPhone(message: CsMessage): boolean {
  return message.direction === "out" && !message.senderUserId;
}
```

Di baris jam bubble (blok `<div>` yang berisi `<span>{clock(message.waTimestamp)}</span>`), tambahkan label sebelum jam:

```tsx
          {sentFromPhone(message) && <span>dari HP</span>}
          <span>{clock(message.waTimestamp)}</span>
          {outgoing && <DeliveryMark status={message.status} />}
```

- [ ] **Step 4: Jalankan tes, pastikan lulus**

Run: `cd frontend && npm test -- --run src/presentation/components/cs/__tests__/MessageThreadPhoneReply.test.tsx`
Expected: 3 tes PASS.

Run: `cd frontend && npm test -- --run src/presentation/components/cs`
Expected: PASS semua tes komponen CS, termasuk `MessageThreadDelete.test.tsx`.

Run: `cd frontend && npx prettier --write src/presentation/components/cs/MessageThread.tsx src/presentation/components/cs/__tests__/MessageThreadPhoneReply.test.tsx && npm run lint`
Expected: lint tanpa error.

- [ ] **Step 5: Commit**

```bash
cd /Users/rohadimraja/Documents/tikman
git add frontend/src/presentation/components/cs/MessageThread.tsx \
        frontend/src/presentation/components/cs/__tests__/MessageThreadPhoneReply.test.tsx
git commit -m "$(cat <<'EOF'
feat(cs): mark a reply that was typed on the phone

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 4: Verifikasi menyeluruh

**Files:**
- Tidak ada perubahan kode, kecuali perbaikan bila ada gerbang yang gagal.

**Interfaces:**
- Consumes: hasil Task 1-3.
- Produces: branch yang lulus semua gerbang CI.

- [ ] **Step 1: Gerbang backend**

```bash
cd backend
go test ./... -race
gofmt -s -l .
go vet ./...
go build ./...
go mod verify
```

Expected: tes PASS, `gofmt -s -l .` tidak mencetak apa pun, `vet`, `build`, dan `mod verify` bersih. Di Mac ini, tes WireGuard `internal/api` bisa gagal dengan 400 saat antarmuka `ppp0` (10.0.0.0/8) aktif. Itu masalah lingkungan, bukan regresi. Pastikan kegagalannya hanya tes WireGuard itu, lalu jalankan ulang dengan `ppp0` mati atau andalkan CI.

- [ ] **Step 2: Gerbang Postgres**

```bash
docker run -d --rm --name tikman-test-pg -e POSTGRES_PASSWORD=test -e POSTGRES_DB=tikman_test \
  -p 5439:5432 timescale/timescaledb:latest-pg15
docker exec tikman-test-pg sh -c 'until pg_isready -h 127.0.0.1 -U postgres -q; do sleep 1; done'
cd backend && TEST_POSTGRES_DSN="host=localhost port=5439 user=postgres password=test dbname=tikman_test sslmode=disable" \
  go test ./... -race
docker stop tikman-test-pg
```

Expected: PASS, dengan tes `OnPostgres` benar-benar berjalan (tidak di-skip). Bila port 5439 terpakai, pakai port lain di `-p` dan DSN.

- [ ] **Step 3: Gerbang frontend**

```bash
cd frontend
npm test -- --run
npm run lint
npm run format:check
npm run build
```

Expected: semua lulus.

- [ ] **Step 4: Perbarui graphify**

Run: `cd /Users/rohadimraja/Documents/tikman && graphify update .`
Expected: selesai tanpa error. Jangan commit `graphify-out/` bila berkas itu tidak di-track git (`git status` memastikannya).

---

### Task 5: Deploy ke radpro (hanya dengan persetujuan user)

**Files:** tidak ada.

**Interfaces:**
- Consumes: branch yang lulus Task 4, sudah di-merge ke `main` dan di-push oleh atau atas persetujuan user.

- [ ] **Step 1: Tanya user dulu**

Bagian 2 (`2026-09-15-cs-response-time-design.md`) juga mengubah `wa`. Tanyakan: deploy bagian 1 sekarang (sesi WhatsApp terputus sekali sekarang, dan sekali lagi saat bagian 2), atau tunggu bagian 2 supaya `wa` hanya dibuat ulang sekali. Jangan lanjut tanpa jawaban. Merge ke `main` dan push juga butuh persetujuan eksplisit.

- [ ] **Step 2: Tarik kode di radpro**

```bash
ssh radpro 'cd /opt/tikman/src && git fetch && git checkout main && git pull --ff-only && git log -1 --oneline'
```

Jalankan `git` sebagai `radpro`, jangan pakai `sudo`: git dengan sudo meninggalkan objek milik root yang memblokir fetch berikutnya. Expected: commit terakhir sama dengan `main` di GitHub.

- [ ] **Step 3: Build dan buat ulang `wa` serta `frontend`, di jam sepi**

```bash
ssh radpro 'C="sudo docker compose --env-file /opt/tikman/.env -f docker-compose.yml -f docker-compose.vps.yml"; cd /opt/tikman/src \
  && $C build wa frontend \
  && $C up -d --force-recreate wa \
  && $C up -d --no-deps --force-recreate frontend'
```

Selalu pakai `--env-file` dan kedua file compose. Tanpa `--env-file`, redis gagal start dan seluruh stack ikut mati. `api`, `worker`, dan `trapd` tidak disentuh.

- [ ] **Step 4: Verifikasi**

```bash
ssh radpro 'sudo docker exec tikman-wa sh -c '"'"'grep -c "Skipping a phone message" $(readlink -f /proc/1/exe)'"'"''
```

Expected: angka ≥ 1.

```bash
bundle=$(curl -s https://noc.radpro.id | grep -o 'assets/index-[A-Za-z0-9_-]*\.js' | head -1)
curl -s "https://noc.radpro.id/$bundle" | grep -c "dari HP"
```

Expected: angka ≥ 1.

Minta user membalas satu thread uji dari HP nomor CS. Expected: pesan muncul di thread berlabel "dari HP", thread keluar dari "Belum dibalas", dan tidak ada thread baru untuk kontak yang belum punya thread.
