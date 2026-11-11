package services

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/telegram"
)

// A half-sent manual send must say what got through, or the admin retries it
// and posts into the WhatsApp group twice.
func TestAHalfSentManualSendNamesWhatGotThrough(t *testing.T) {
	f := newGraphReportFixture(t)
	fake, chat := f.useTelegram(t)
	report := f.createDestinations(t, f.group.ID, &chat.ID, "Uplink")
	fake.refuse(http.StatusTooManyRequests, `{"ok":false,"description":"Too Many Requests: retry after 5"}`)

	err := f.sender.SendNow(context.Background(), report.ID)

	assert.True(t, errors.Is(err, ErrTelegramSend), "still a Telegram failure for the status it maps to")
	assert.EqualError(t, err, "Terkirim ke WhatsApp; Telegram gagal: Too Many Requests: retry after 5")
	assert.Equal(t, err.Error(), f.load(t, report.ID).LastError)
	assert.Equal(t, int64(1), f.queuedCount(t))
}

// A dropped connection may follow a send Telegram took; the retries every ten
// minutes must not post the album again and again.
func TestADailySendWithAnUnknownFateIsNotRetried(t *testing.T) {
	f := newGraphReportFixture(t)
	fake, chat := f.useTelegram(t)
	report := f.createDestinations(t, uuid.Nil, &chat.ID, "Uplink")
	fake.mu.Lock()
	fake.hangUp = true
	fake.mu.Unlock()

	f.tickAt(t, 11, 6, 0)
	f.tickAt(t, 11, 6, 10)
	f.tickAt(t, 11, 6, 20)

	assert.Equal(t, 1, fake.requestCount(), "asked once, not once per retry")
	stored := f.load(t, report.ID)
	today := wibDate(atWIB(11, 6, 0))
	assert.True(t, sameDate(stored.TelegramSentOn, today))
	assert.True(t, sameDate(stored.LastSentOn, today), "the day is done")
	assert.True(t, strings.HasPrefix(stored.LastError, "Telegram: status kiriman tidak pasti ("), stored.LastError)
	assert.True(t, strings.HasSuffix(stored.LastError, "), periksa grup; kirim manual bila belum masuk"), stored.LastError)
	assert.NotContains(t, stored.LastError, "1:bot")
}

func TestADailySendTelegramRefusedIsRetried(t *testing.T) {
	f := newGraphReportFixture(t)
	fake, chat := f.useTelegram(t)
	report := f.createDestinations(t, uuid.Nil, &chat.ID, "Uplink")
	fake.refuse(http.StatusTooManyRequests, `{"ok":false,"description":"Too Many Requests: retry after 5"}`)

	f.tickAt(t, 11, 6, 0)
	f.tickAt(t, 11, 6, 10)

	assert.Equal(t, 2, fake.requestCount(), "a refusal posted nothing, so the retry asks again")
	stored := f.load(t, report.ID)
	assert.Nil(t, stored.TelegramSentOn)
	assert.Nil(t, stored.LastSentOn)
}

func TestARefusalThatLosesTheChatMarksItInactive(t *testing.T) {
	cases := map[string]struct {
		status int
		body   string
	}{
		"kicked":         {http.StatusForbidden, `{"ok":false,"description":"Forbidden: bot was kicked from the supergroup chat"}`},
		"not a member":   {http.StatusForbidden, `{"ok":false,"description":"Forbidden: bot is not a member of the supergroup chat"}`},
		"chat not found": {http.StatusBadRequest, `{"ok":false,"description":"Bad Request: chat not found"}`},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := newGraphReportFixture(t)
			fake, chat := f.useTelegram(t)
			report := f.createDestinations(t, uuid.Nil, &chat.ID, "Uplink")
			fake.refuse(tc.status, tc.body)

			err := f.sender.SendNow(context.Background(), report.ID)
			assert.True(t, errors.Is(err, ErrTelegramSend))

			var stored models.TelegramChat
			require.NoError(t, f.db.First(&stored, "id = ?", chat.ID).Error)
			assert.False(t, stored.Active)
			err = f.sender.SendNow(context.Background(), report.ID)
			assert.True(t, errors.Is(err, ErrTelegramChatInactive), "the page now says the bot left")
		})
	}
}

func TestATransientRefusalLeavesTheChatActive(t *testing.T) {
	f := newGraphReportFixture(t)
	fake, chat := f.useTelegram(t)
	report := f.createDestinations(t, uuid.Nil, &chat.ID, "Uplink")
	fake.refuse(http.StatusBadRequest, `{"ok":false,"description":"Bad Request: PHOTO_INVALID_DIMENSIONS"}`)

	require.Error(t, f.sender.SendNow(context.Background(), report.ID))

	var stored models.TelegramChat
	require.NoError(t, f.db.First(&stored, "id = ?", chat.ID).Error)
	assert.True(t, stored.Active)
}

// The table's on/off switch resends the whole report; a chat the bot has left
// must not stop it from being switched off.
func TestAReportKeepsTheChatTheBotLeft(t *testing.T) {
	f := newGraphReportFixture(t)
	_, chat := f.useTelegram(t)
	report := f.createDestinations(t, uuid.Nil, &chat.ID, "Uplink")
	require.NoError(t, f.db.Model(&chat).Update("active", false).Error)
	in := reportInput(uuid.Nil, "Uplink")
	in.TelegramChatID = &chat.ID
	in.Enabled = false

	updated, err := f.reports.Update(report.ID, in)

	require.NoError(t, err)
	assert.False(t, updated.Enabled)
}

func TestAReportCannotMoveToAChatTheBotLeft(t *testing.T) {
	f := newGraphReportFixture(t)
	_, chat := f.useTelegram(t)
	report := f.createDestinations(t, uuid.Nil, &chat.ID, "Uplink")
	left := models.TelegramChat{ChatID: -100456, Title: "Lama", Type: telegram.ChatTypeGroup, Active: true}
	require.NoError(t, f.db.Create(&left).Error)
	require.NoError(t, f.db.Model(&left).Update("active", false).Error)
	in := reportInput(uuid.Nil, "Uplink")
	in.TelegramChatID = &left.ID

	_, err := f.reports.Update(report.ID, in)

	assert.True(t, errors.Is(err, ErrValidation))
	assert.ErrorContains(t, err, "bot sudah tidak ada di grup Telegram ini")
}

// Guards claimDestinations' self-exclusion: a report is not its own rival.
func TestAReportKeepingItsTelegramChatCanBeRenamed(t *testing.T) {
	f := newGraphReportFixture(t)
	_, chat := f.useTelegram(t)
	report := f.createDestinations(t, uuid.Nil, &chat.ID, "Uplink")
	in := reportInput(uuid.Nil, "Uplink")
	in.TelegramChatID = &chat.ID
	in.CustomerName = "PT Baru"

	updated, err := f.reports.Update(report.ID, in)

	require.NoError(t, err)
	assert.Equal(t, "PT Baru", updated.CustomerName)
	assert.Equal(t, &chat.ID, updated.TelegramChatID)
}

func TestDeletingANumberLeavesItsGroupsTelegramReports(t *testing.T) {
	f := newGraphReportFixture(t)
	_, chat := f.useTelegram(t)
	both := f.createDestinations(t, f.group.ID, &chat.ID, "Uplink")

	require.NoError(t, NewCSPurgeService(f.db, f.mediaRoot).DeleteAccount(f.group.WAAccountID))

	stored := f.load(t, both.ID)
	assert.Nil(t, stored.WAGroupID)
	assert.Equal(t, &chat.ID, stored.TelegramChatID)
}
