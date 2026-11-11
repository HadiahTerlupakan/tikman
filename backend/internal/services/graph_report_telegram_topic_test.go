package services

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

// forumTopics turns the fixture's chat into a forum with the given topics.
func (f graphReportFixture) forumTopics(t *testing.T, chat *models.TelegramChat, topics map[int64]string) {
	t.Helper()
	require.NoError(t, f.db.Model(chat).Update("is_forum", true).Error)
	for threadID, name := range topics {
		require.NoError(t, f.db.Create(&models.TelegramTopic{TelegramChatID: chat.ID, ThreadID: threadID, Name: name}).Error)
	}
}

func topicInput(chatID uuid.UUID, threadID *int64) GraphReportInput {
	in := reportInput(uuid.Nil, "Uplink")
	in.TelegramChatID, in.TelegramThreadID = &chatID, threadID
	return in
}

func threadOf(id int64) *int64 { return &id }

func TestTelegramReportPostsIntoItsTopic(t *testing.T) {
	f := newGraphReportFixture(t)
	fake, chat := f.useTelegram(t)
	f.forumTopics(t, &chat, map[int64]string{9: "PT Contoh"})
	in := topicInput(chat.ID, threadOf(9))
	in.Ports = reportInput(uuid.Nil, "A", "B", "C").Ports
	album, err := f.reports.Create(in, uuid.New())
	require.NoError(t, err)
	photo, err := f.reports.Create(topicInput(chat.ID, nil), uuid.New())
	require.NoError(t, err)
	f.sender.now = func() time.Time { return atWIB(11, 6, 0) }

	require.NoError(t, f.sender.SendNow(context.Background(), album.ID))
	require.NoError(t, f.sender.SendNow(context.Background(), photo.ID))

	posts := fake.sent()
	require.Len(t, posts, 2)
	assert.Equal(t, "sendMediaGroup", posts[0].method)
	assert.Equal(t, "9", posts[0].threadID)
	assert.Equal(t, "sendPhoto", posts[1].method)
	assert.Empty(t, posts[1].threadID, "General is the chat itself")
}

func TestATelegramForumTakesOneReportPerTopic(t *testing.T) {
	f := newGraphReportFixture(t)
	_, chat := f.useTelegram(t)
	f.forumTopics(t, &chat, map[int64]string{9: "PT Contoh", 12: "PT Lain"})
	_, err := f.reports.Create(topicInput(chat.ID, threadOf(9)), uuid.New())
	require.NoError(t, err)
	_, err = f.reports.Create(topicInput(chat.ID, threadOf(12)), uuid.New())
	require.NoError(t, err, "another topic of the same forum")
	_, err = f.reports.Create(topicInput(chat.ID, nil), uuid.New())
	require.NoError(t, err, "General is a topic of its own")

	_, err = f.reports.Create(topicInput(chat.ID, threadOf(9)), uuid.New())
	assert.True(t, errors.Is(err, ErrValidation))
	assert.ErrorContains(t, err, "topik Telegram ini sudah memiliki laporan grafik")

	_, err = f.reports.Create(topicInput(chat.ID, nil), uuid.New())
	assert.True(t, errors.Is(err, ErrValidation))
	assert.ErrorContains(t, err, "grup Telegram ini sudah memiliki laporan grafik")
}

func TestATelegramTopicMustBeOneTheBotHasSeen(t *testing.T) {
	f := newGraphReportFixture(t)
	_, chat := f.useTelegram(t)
	plain := models.TelegramChat{ChatID: -100456, Title: "Biasa", Type: chat.Type, Active: true}
	require.NoError(t, f.db.Create(&plain).Error)
	f.forumTopics(t, &chat, map[int64]string{9: "PT Contoh"})
	require.NoError(t, f.db.Create(&models.TelegramTopic{TelegramChatID: plain.ID, ThreadID: 3}).Error)
	noChat := reportInput(f.group.ID, "Uplink")
	noChat.TelegramThreadID = threadOf(9)

	cases := map[string]struct {
		in   GraphReportInput
		want string
	}{
		"unknown topic":     {topicInput(chat.ID, threadOf(10)), "topik Telegram tidak ditemukan"},
		"another's topic":   {topicInput(chat.ID, threadOf(3)), "topik Telegram tidak ditemukan"},
		"chat is no forum":  {topicInput(plain.ID, threadOf(3)), "grup Telegram ini tidak memakai topik"},
		"topic but no chat": {noChat, "topik Telegram dipilih tanpa grup Telegram"},
	}
	for name, tc := range cases {
		_, err := f.reports.Create(tc.in, uuid.New())
		assert.True(t, errors.Is(err, ErrValidation), name)
		assert.ErrorContains(t, err, tc.want, name)
	}
}

// The table's on/off switch resends the whole report; a forum that has since
// turned topics off must not stop the admin from switching it off.
func TestAReportKeepsItsTopicAfterTheForumLosesTopics(t *testing.T) {
	f := newGraphReportFixture(t)
	_, chat := f.useTelegram(t)
	f.forumTopics(t, &chat, map[int64]string{9: "PT Contoh"})
	report, err := f.reports.Create(topicInput(chat.ID, threadOf(9)), uuid.New())
	require.NoError(t, err)
	require.NoError(t, f.db.Model(&chat).Update("is_forum", false).Error)
	in := topicInput(chat.ID, threadOf(9))
	in.Enabled = false

	updated, err := f.reports.Update(report.ID, in)

	require.NoError(t, err)
	assert.False(t, updated.Enabled)
	assert.Equal(t, threadOf(9), updated.TelegramThreadID)
}

func TestGraphReportViewNamesTheTopic(t *testing.T) {
	f := newGraphReportFixture(t)
	_, chat := f.useTelegram(t)
	f.forumTopics(t, &chat, map[int64]string{9: "PT Contoh", 12: ""})
	_, err := f.reports.Create(topicInput(chat.ID, threadOf(9)), uuid.New())
	require.NoError(t, err)
	unnamed := topicInput(chat.ID, threadOf(12))
	unnamed.CustomerName = "PT Tanpa Nama"
	_, err = f.reports.Create(unnamed, uuid.New())
	require.NoError(t, err)

	views, err := f.reports.List()

	require.NoError(t, err)
	require.Len(t, views, 2)
	assert.Equal(t, "PT Contoh", views[0].TelegramTopicName)
	assert.Equal(t, threadOf(9), views[0].TelegramThreadID)
	assert.Equal(t, "", views[1].TelegramTopicName)
	assert.Equal(t, threadOf(12), views[1].TelegramThreadID)
}

// A deleted topic is a refusal shown as Telegram words it; the chat still
// takes posts in its other topics, so it is not marked as one the bot left.
func TestADeletedTopicDoesNotRetireTheChat(t *testing.T) {
	f := newGraphReportFixture(t)
	fake, chat := f.useTelegram(t)
	f.forumTopics(t, &chat, map[int64]string{9: "PT Contoh"})
	report, err := f.reports.Create(topicInput(chat.ID, threadOf(9)), uuid.New())
	require.NoError(t, err)
	fake.refuse(http.StatusBadRequest, `{"ok":false,"error_code":400,"description":"Bad Request: message thread not found"}`)

	err = f.sender.SendNow(context.Background(), report.ID)

	assert.True(t, errors.Is(err, ErrTelegramSend))
	assert.EqualError(t, err, "Bad Request: message thread not found")
	var stored models.TelegramChat
	require.NoError(t, f.db.First(&stored, "id = ?", chat.ID).Error)
	assert.True(t, stored.Active)
}
