package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/telegram"
	"gorm.io/gorm"
)

var forumChat = telegram.Chat{ID: -100777, Title: "Pelanggan", Type: telegram.ChatTypeSupergroup, IsForum: true}

func topicMessage(id int64, msg telegram.Message) telegram.Update {
	msg.Chat = forumChat
	return telegram.Update{UpdateID: id, Message: &msg}
}

func topicRows(t *testing.T, db *gorm.DB) map[int64]string {
	t.Helper()
	var topics []models.TelegramTopic
	require.NoError(t, db.Find(&topics).Error)
	names := make(map[int64]string, len(topics))
	for _, topic := range topics {
		names[topic.ThreadID] = topic.Name
	}
	return names
}

func TestTelegramTopicIsLearntFromItsCreationAndRename(t *testing.T) {
	db := setupTestDB(t)
	chats := NewTelegramChatService(db)

	require.NoError(t, chats.Apply(topicMessage(1, telegram.Message{
		MessageThreadID: 5, IsTopicMessage: true, ForumTopicCreated: &telegram.ForumTopic{Name: "PT Contoh"},
	})))
	require.NoError(t, chats.Apply(topicMessage(1, telegram.Message{
		MessageThreadID: 5, IsTopicMessage: true, ForumTopicCreated: &telegram.ForumTopic{Name: "PT Contoh"},
	})), "a re-delivered update is harmless")
	assert.Equal(t, map[int64]string{5: "PT Contoh"}, topicRows(t, db))
	assert.True(t, chatRows(t, db)[0].IsForum)

	require.NoError(t, chats.Apply(topicMessage(2, telegram.Message{
		MessageThreadID: 5, IsTopicMessage: true, ForumTopicEdited: &telegram.ForumTopic{Name: "PT Contoh Baru"},
	})))
	assert.Equal(t, map[int64]string{5: "PT Contoh Baru"}, topicRows(t, db))

	require.NoError(t, chats.Apply(topicMessage(3, telegram.Message{
		MessageThreadID: 5, IsTopicMessage: true, ForumTopicEdited: &telegram.ForumTopic{},
	})))
	assert.Equal(t, map[int64]string{5: "PT Contoh Baru"}, topicRows(t, db), "a change of icon alone keeps the name")
}

// A topic made before the bot joined is learnt from a post inside it, such
// as /start, whose reply_to_message is the topic's creation.
func TestTelegramTopicIsLearntFromAPostInsideIt(t *testing.T) {
	db := setupTestDB(t)
	chats := NewTelegramChatService(db)
	creation := &telegram.Message{MessageThreadID: 8, ForumTopicCreated: &telegram.ForumTopic{Name: "PT Lama"}}

	require.NoError(t, chats.Apply(topicMessage(1, telegram.Message{MessageThreadID: 9, IsTopicMessage: true})))
	require.NoError(t, chats.Apply(topicMessage(2, telegram.Message{
		MessageThreadID: 8, IsTopicMessage: true, ReplyToMessage: creation,
	})))

	assert.Equal(t, map[int64]string{8: "PT Lama", 9: ""}, topicRows(t, db), "an unnamed topic is still listed")
}

func TestTelegramTopicKeepsItsKnownName(t *testing.T) {
	db := setupTestDB(t)
	chats := NewTelegramChatService(db)
	require.NoError(t, chats.Apply(topicMessage(1, telegram.Message{
		MessageThreadID: 5, ForumTopicEdited: &telegram.ForumTopic{Name: "Nama Baru"},
	})))

	require.NoError(t, chats.Apply(topicMessage(2, telegram.Message{MessageThreadID: 5, IsTopicMessage: true})))
	require.NoError(t, chats.Apply(topicMessage(3, telegram.Message{
		MessageThreadID: 5, IsTopicMessage: true,
		ReplyToMessage: &telegram.Message{MessageThreadID: 5, ForumTopicCreated: &telegram.ForumTopic{Name: "Nama Lama"}},
	})))

	assert.Equal(t, map[int64]string{5: "Nama Baru"}, topicRows(t, db),
		"neither a nameless post nor the creation's name from before a rename overwrites it")
}

// A reply in General carries the replied-to message's ID as its thread ID,
// and a group that is no forum has no topics at all.
func TestTelegramTopicIgnoresGeneralAndPlainGroups(t *testing.T) {
	db := setupTestDB(t)
	chats := NewTelegramChatService(db)

	require.NoError(t, chats.Apply(topicMessage(1, telegram.Message{MessageThreadID: 40})))
	plain := telegram.Chat{ID: -100888, Title: "Biasa", Type: telegram.ChatTypeSupergroup}
	require.NoError(t, chats.Apply(telegram.Update{UpdateID: 2, Message: &telegram.Message{
		Chat: plain, MessageThreadID: 41, IsTopicMessage: true,
	}}))

	assert.Empty(t, topicRows(t, db))
	require.Len(t, chatRows(t, db), 2)
}

func TestTelegramChatListCarriesTopicsByName(t *testing.T) {
	db := setupTestDB(t)
	chats := NewTelegramChatService(db)
	for i, name := range []string{"PT Zeta", "PT Alfa", ""} {
		require.NoError(t, chats.Apply(topicMessage(int64(i), telegram.Message{
			MessageThreadID: int64(10 - i), ForumTopicCreated: &telegram.ForumTopic{Name: name},
		})))
	}
	require.NoError(t, chats.Apply(memberUpdate(9, telegram.Chat{ID: -5, Title: "Biasa", Type: telegram.ChatTypeGroup}, telegram.StatusMember)))

	views, err := chats.List()

	require.NoError(t, err)
	require.Len(t, views, 2)
	assert.Empty(t, views[0].Topics)
	assert.NotNil(t, views[0].Topics, "an empty list, not null")
	assert.Equal(t, []TelegramTopicView{{8, ""}, {9, "PT Alfa"}, {10, "PT Zeta"}}, views[1].Topics)
}
