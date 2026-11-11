package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testToken = "123456:AAH-secret_token"

func serve(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return NewAt(server.URL, testToken)
}

func TestSendMediaGroupPostsOneAlbumWithEveryPhotoAttached(t *testing.T) {
	var media []map[string]string
	parts := map[string]string{}
	var chatID, threadID string
	client := serve(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/bot"+testToken+"/sendMediaGroup", r.URL.Path)
		require.NoError(t, r.ParseMultipartForm(1<<20))
		chatID, threadID = r.FormValue("chat_id"), r.FormValue("message_thread_id")
		require.NoError(t, json.Unmarshal([]byte(r.FormValue("media")), &media))
		for name, files := range r.MultipartForm.File {
			f, err := files[0].Open()
			require.NoError(t, err)
			raw, _ := io.ReadAll(f)
			parts[name] = files[0].Filename + "=" + string(raw)
		}
		_, _ = w.Write([]byte(`{"ok":true,"result":[]}`))
	})

	err := client.SendMediaGroup(context.Background(), -100123, 77, []Photo{
		{PNG: []byte("one"), Filename: "a.png", Caption: "first"},
		{PNG: []byte("two"), Filename: "b.png", Caption: "second"},
	})

	require.NoError(t, err)
	assert.Equal(t, "-100123", chatID)
	assert.Equal(t, "77", threadID, "the album goes into the forum topic")
	assert.Equal(t, []map[string]string{
		{"type": "photo", "media": "attach://p0", "caption": "first"},
		{"type": "photo", "media": "attach://p1", "caption": "second"},
	}, media)
	assert.Equal(t, map[string]string{"p0": "a.png=one", "p1": "b.png=two"}, parts)
}

func TestSendPhotoPostsThePhotoWithItsCaption(t *testing.T) {
	client := serve(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/bot"+testToken+"/sendPhoto", r.URL.Path)
		require.NoError(t, r.ParseMultipartForm(1<<20))
		assert.Equal(t, "42", r.FormValue("chat_id"))
		assert.Equal(t, "hello", r.FormValue("caption"))
		_, inThread := r.MultipartForm.Value["message_thread_id"]
		assert.False(t, inThread, "no thread ID means the chat itself, General in a forum")
		_, header, err := r.FormFile("photo")
		require.NoError(t, err)
		assert.Equal(t, "a.png", header.Filename)
		_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
	})

	assert.NoError(t, client.SendPhoto(context.Background(), 42, 0, Photo{PNG: []byte("x"), Filename: "a.png", Caption: "hello"}))
}

func TestSendPhotoNamesTheTopic(t *testing.T) {
	client := serve(t, func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseMultipartForm(1<<20))
		assert.Equal(t, "-100123", r.FormValue("chat_id"))
		assert.Equal(t, "9", r.FormValue("message_thread_id"))
		_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
	})

	assert.NoError(t, client.SendPhoto(context.Background(), -100123, 9, Photo{PNG: []byte("x"), Filename: "a.png"}))
}

func TestGetUpdatesReadsForumTopics(t *testing.T) {
	client := serve(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true,"result":[{"update_id":1,"message":{
			"chat":{"id":-100,"title":"Pelanggan","type":"supergroup","is_forum":true},
			"message_thread_id":5,"is_topic_message":true,
			"reply_to_message":{"chat":{"id":-100,"type":"supergroup","is_forum":true},"message_thread_id":5,
				"forum_topic_created":{"name":"PT Contoh","icon_color":7322096}}}}]}`))
	})

	updates, err := client.GetUpdates(context.Background(), 0, 0)

	require.NoError(t, err)
	msg := updates[0].Message
	assert.True(t, msg.Chat.IsForum)
	assert.Equal(t, int64(5), msg.MessageThreadID)
	assert.True(t, msg.IsTopicMessage)
	require.NotNil(t, msg.ReplyToMessage)
	assert.Equal(t, &ForumTopic{Name: "PT Contoh"}, msg.ReplyToMessage.ForumTopicCreated)
}

func TestARefusalCarriesTelegramsDescriptionAndTheNewChat(t *testing.T) {
	client := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"ok":false,"error_code":400,"description":"Bad Request: group chat was upgraded to a supergroup chat","parameters":{"migrate_to_chat_id":-100777}}`))
	})

	err := client.SendPhoto(context.Background(), -5, 0, Photo{PNG: []byte("x"), Filename: "a.png"})

	var apiErr *APIError
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, "Bad Request: group chat was upgraded to a supergroup chat", err.Error())
	assert.Equal(t, int64(-100777), apiErr.MigrateToChatID)
}

func TestANonJSONAnswerNamesTheStatus(t *testing.T) {
	client := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("<html>bad gateway</html>"))
	})

	err := client.SendPhoto(context.Background(), 1, 0, Photo{})

	assert.EqualError(t, err, "Telegram menjawab HTTP 502")
}

func TestANetworkErrorDoesNotCarryTheToken(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	base := server.URL
	server.Close()
	client := NewAt(base, testToken)

	err := client.SendPhoto(context.Background(), 1, 0, Photo{PNG: []byte("x"), Filename: "a.png"})
	require.Error(t, err)
	assert.NotContains(t, err.Error(), testToken)
	assert.NotContains(t, err.Error(), "AAH-secret_token")
	assert.Contains(t, err.Error(), redactedToken, "the error still says which request failed")

	_, err = client.GetUpdates(context.Background(), 0, 0)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "AAH-secret_token")
}

func TestGetUpdatesAsksForChatChangesFromTheOffset(t *testing.T) {
	client := serve(t, func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		assert.Equal(t, "7", r.PostForm.Get("offset"))
		assert.Equal(t, "50", r.PostForm.Get("timeout"))
		assert.JSONEq(t, `["my_chat_member","message"]`, r.PostForm.Get("allowed_updates"))
		_, _ = w.Write([]byte(`{"ok":true,"result":[
			{"update_id":7,"my_chat_member":{"chat":{"id":-100,"title":"NOC","type":"supergroup"},"new_chat_member":{"status":"administrator"}}},
			{"update_id":8,"message":{"chat":{"id":-5,"title":"Old","type":"group"},"migrate_to_chat_id":-100}}]}`))
	})

	updates, err := client.GetUpdates(context.Background(), 7, 50)

	require.NoError(t, err)
	require.Len(t, updates, 2)
	assert.Equal(t, Chat{ID: -100, Title: "NOC", Type: ChatTypeSupergroup}, updates[0].MyChatMember.Chat)
	assert.Equal(t, StatusAdministrator, updates[0].MyChatMember.NewChatMember.Status)
	assert.Equal(t, int64(-100), updates[1].Message.MigrateToChatID)
}

// A token with a control character reaches url.Error escaped by %q, where a
// verbatim search for it finds nothing; it must be refused before any request.
func TestAMalformedTokenIsRefusedWithoutBeingPrinted(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests++ }))
	t.Cleanup(server.Close)
	token := "123456:AAH-se\ncret"
	client := NewAt(server.URL, token)

	err := client.SendPhoto(context.Background(), 1, 0, Photo{PNG: []byte("x"), Filename: "a.png"})
	require.ErrorIs(t, err, ErrInvalidToken)
	assert.NotContains(t, err.Error(), "AAH-se")

	_, err = client.GetUpdates(context.Background(), 0, 0)
	require.ErrorIs(t, err, ErrInvalidToken)
	assert.Zero(t, requests)
}

func TestGetChatReadsWhetherTheChatIsAForum(t *testing.T) {
	client := serve(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/bot"+testToken+"/getChat", r.URL.Path)
		require.NoError(t, r.ParseForm())
		assert.Equal(t, "-100123", r.PostForm.Get("chat_id"))
		_, _ = w.Write([]byte(`{"ok":true,"result":{"id":-100123,"title":"Pelanggan","type":"supergroup","is_forum":true}}`))
	})

	chat, err := client.GetChat(context.Background(), -100123)

	require.NoError(t, err)
	assert.Equal(t, Chat{ID: -100123, Title: "Pelanggan", Type: ChatTypeSupergroup, IsForum: true}, chat)
}

func TestSendMessagePostsTheTextIntoTheTopic(t *testing.T) {
	var got map[string][]string
	client := serve(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/bot"+testToken+"/sendMessage", r.URL.Path)
		assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
		require.NoError(t, r.ParseForm())
		got = r.PostForm
		_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
	})

	require.NoError(t, client.SendMessage(context.Background(), -100123, 77, "🔴 DOWN — PT Contoh\nPort: Uplink"))

	assert.Equal(t, map[string][]string{
		"chat_id": {"-100123"}, "message_thread_id": {"77"}, "text": {"🔴 DOWN — PT Contoh\nPort: Uplink"},
	}, got)
}

func TestSendMessageToGeneralCarriesNoThread(t *testing.T) {
	var form map[string][]string
	client := serve(t, func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		form = r.PostForm
		_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
	})

	require.NoError(t, client.SendMessage(context.Background(), -100123, 0, "UP"))

	assert.NotContains(t, form, "message_thread_id")
}

func TestSendMessageAnswersTheRefusal(t *testing.T) {
	client := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"ok":false,"description":"Forbidden: bot was kicked from the supergroup chat"}`))
	})

	err := client.SendMessage(context.Background(), -100123, 0, "DOWN")

	var apiErr *APIError
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
}
