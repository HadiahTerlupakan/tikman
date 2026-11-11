// Package telegram is the part of the Telegram Bot API the graph reports use:
// reading which chats the bot is in, and posting photos to them.
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// DefaultBaseURL is the public Bot API server.
const DefaultBaseURL = "https://api.telegram.org"

// sendTimeout covers uploading a report's images, a few hundred kilobytes.
const sendTimeout = 60 * time.Second

// pollGrace is how much longer than the long poll itself a getUpdates request
// may take before it is given up on.
const pollGrace = 10 * time.Second

// lookupTimeout covers one getChat, a small answer.
const lookupTimeout = 10 * time.Second

// maxBodyBytes bounds an answer; a hundred updates are tens of kilobytes.
const maxBodyBytes = 4 << 20

// redactedToken stands in for the token wherever an error would print it.
const redactedToken = "<token>"

// allowedUpdates is what discovery needs: the bot joining or leaving a chat,
// and messages, which carry a group's upgrade to a supergroup.
const allowedUpdates = `["my_chat_member","message"]`

// The chat types Telegram names in an update.
const (
	ChatTypeGroup      = "group"
	ChatTypeSupergroup = "supergroup"
	ChatTypeChannel    = "channel"
)

// The statuses a chat member update gives the bot.
const (
	StatusCreator       = "creator"
	StatusAdministrator = "administrator"
	StatusMember        = "member"
	StatusLeft          = "left"
	StatusKicked        = "kicked"
)

// ErrInvalidToken is a token not shaped like one @BotFather issues.
var ErrInvalidToken = errors.New("token bot Telegram tidak valid")

// tokenPattern is the shape of a bot token, "<bot id>:<secret>". Anything
// else is refused before it goes into a URL: url.Error quotes the URL with
// %q, so a token holding a control character would print escaped, past the
// redaction that looks for it verbatim.
var tokenPattern = regexp.MustCompile(`^[0-9]+:[A-Za-z0-9_-]+$`)

// ValidToken says whether token is shaped like a bot token.
func ValidToken(token string) bool { return tokenPattern.MatchString(token) }

// Chat is a chat as an update names it.
type Chat struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
	Type  string `json:"type"`
	// IsForum is a supergroup with topics, where a post without a thread ID
	// lands in General.
	IsForum bool `json:"is_forum"`
}

// ChatMember is the bot's own membership after a change.
type ChatMember struct {
	Status string `json:"status"`
}

// ChatMemberUpdated is the bot being added to, promoted in or removed from a chat.
type ChatMemberUpdated struct {
	Chat          Chat       `json:"chat"`
	NewChatMember ChatMember `json:"new_chat_member"`
}

// ForumTopic is the part of a topic's creation or edit that names it.
type ForumTopic struct {
	Name string `json:"name"`
}

// Message is the part of a message discovery reads. The Bot API cannot list
// a forum's topics, so they too are learnt from messages: a topic's creation,
// its renaming, and any message posted inside it.
type Message struct {
	Chat              Chat  `json:"chat"`
	MigrateToChatID   int64 `json:"migrate_to_chat_id"`
	MigrateFromChatID int64 `json:"migrate_from_chat_id"`
	MessageThreadID   int64 `json:"message_thread_id"`
	// IsTopicMessage tells a message inside a topic from a reply in General,
	// which carries a thread ID as well.
	IsTopicMessage    bool        `json:"is_topic_message"`
	ForumTopicCreated *ForumTopic `json:"forum_topic_created"`
	ForumTopicEdited  *ForumTopic `json:"forum_topic_edited"`
	// ReplyToMessage of a message in a topic is the topic's creation, unless
	// it replies to something else.
	ReplyToMessage *Message `json:"reply_to_message"`
}

// Update is one event from getUpdates.
type Update struct {
	UpdateID     int64              `json:"update_id"`
	MyChatMember *ChatMemberUpdated `json:"my_chat_member"`
	Message      *Message           `json:"message"`
}

// Photo is one PNG to post.
type Photo struct {
	PNG      []byte
	Filename string
	Caption  string
}

// APIError is Telegram refusing a request.
type APIError struct {
	StatusCode  int
	Description string
	// MigrateToChatID is set when the chat was upgraded to a supergroup, which
	// has a new ID; the request must be repeated there.
	MigrateToChatID int64
}

func (e *APIError) Error() string {
	if e.Description != "" {
		return e.Description
	}
	return fmt.Sprintf("Telegram menjawab HTTP %d", e.StatusCode)
}

// Client talks to the Bot API as one bot.
type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

// New constructs a client for the public Bot API server.
func New(token string) *Client {
	return NewAt(DefaultBaseURL, token)
}

// NewAt constructs a client for the Bot API server at baseURL.
func NewAt(baseURL, token string) *Client {
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), token: token, http: &http.Client{}}
}

// GetUpdates long-polls for updates from offset on, waiting up to timeoutSeconds.
func (c *Client) GetUpdates(ctx context.Context, offset int64, timeoutSeconds int) ([]Update, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second+pollGrace)
	defer cancel()
	form := url.Values{
		"offset":          {strconv.FormatInt(offset, 10)},
		"timeout":         {strconv.Itoa(timeoutSeconds)},
		"allowed_updates": {allowedUpdates},
	}
	var updates []Update
	err := c.call(ctx, "getUpdates", "application/x-www-form-urlencoded", strings.NewReader(form.Encode()), &updates)
	return updates, err
}

// GetChat reads what Telegram says of a chat now, whether or not any update
// has carried it lately.
func (c *Client) GetChat(ctx context.Context, chatID int64) (Chat, error) {
	ctx, cancel := context.WithTimeout(ctx, lookupTimeout)
	defer cancel()
	form := url.Values{"chat_id": {strconv.FormatInt(chatID, 10)}}
	var chat Chat
	err := c.call(ctx, "getChat", "application/x-www-form-urlencoded", strings.NewReader(form.Encode()), &chat)
	return chat, err
}

// SendPhoto posts one photo into a forum's topic threadID, or with 0 into
// the chat itself, which in a forum is General.
func (c *Client) SendPhoto(ctx context.Context, chatID, threadID int64, photo Photo) error {
	return c.upload(ctx, "sendPhoto", func(w *multipart.Writer) error {
		if err := writeFields(w, chatID, threadID, map[string]string{"caption": photo.Caption}); err != nil {
			return err
		}
		return writePhoto(w, "photo", photo)
	})
}

// SendMediaGroup posts photos as one album, into threadID as SendPhoto does.
// Telegram takes 2 to 10 of them.
func (c *Client) SendMediaGroup(ctx context.Context, chatID, threadID int64, photos []Photo) error {
	type inputMedia struct {
		Type    string `json:"type"`
		Media   string `json:"media"`
		Caption string `json:"caption,omitempty"`
	}
	media := make([]inputMedia, len(photos))
	for i, photo := range photos {
		media[i] = inputMedia{Type: "photo", Media: "attach://" + partName(i), Caption: photo.Caption}
	}
	encoded, err := json.Marshal(media)
	if err != nil {
		return fmt.Errorf("encode media group: %w", err)
	}
	return c.upload(ctx, "sendMediaGroup", func(w *multipart.Writer) error {
		if err := writeFields(w, chatID, threadID, map[string]string{"media": string(encoded)}); err != nil {
			return err
		}
		for i, photo := range photos {
			if err := writePhoto(w, partName(i), photo); err != nil {
				return err
			}
		}
		return nil
	})
}

func partName(i int) string { return "p" + strconv.Itoa(i) }

func writeFields(w *multipart.Writer, chatID, threadID int64, fields map[string]string) error {
	fields["chat_id"] = strconv.FormatInt(chatID, 10)
	if threadID != 0 {
		fields["message_thread_id"] = strconv.FormatInt(threadID, 10)
	}
	for name, value := range fields {
		if err := w.WriteField(name, value); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
	}
	return nil
}

func writePhoto(w *multipart.Writer, field string, photo Photo) error {
	part, err := w.CreateFormFile(field, photo.Filename)
	if err != nil {
		return fmt.Errorf("write %s: %w", field, err)
	}
	_, err = part.Write(photo.PNG)
	return err
}

func (c *Client) upload(ctx context.Context, method string, write func(*multipart.Writer) error) error {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	if err := write(w); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("encode %s: %w", method, err)
	}
	ctx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()
	return c.call(ctx, method, w.FormDataContentType(), &body, nil)
}

// call posts one Bot API request and decodes its result into out.
func (c *Client) call(ctx context.Context, method, contentType string, body io.Reader, out any) error {
	if !ValidToken(c.token) {
		return ErrInvalidToken
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/bot"+c.token+"/"+method, body)
	if err != nil {
		return c.redact(fmt.Errorf("%s: %w", method, err))
	}
	req.Header.Set("Content-Type", contentType)
	resp, err := c.http.Do(req)
	if err != nil {
		return c.redact(fmt.Errorf("%s: %w", method, err))
	}
	defer func() { _ = resp.Body.Close() }()
	var answer struct {
		OK          bool            `json:"ok"`
		Description string          `json:"description"`
		Result      json.RawMessage `json:"result"`
		Parameters  struct {
			MigrateToChatID int64 `json:"migrate_to_chat_id"`
		} `json:"parameters"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxBodyBytes)).Decode(&answer); err != nil {
		return &APIError{StatusCode: resp.StatusCode}
	}
	if !answer.OK {
		return &APIError{
			StatusCode: resp.StatusCode, Description: answer.Description,
			MigrateToChatID: answer.Parameters.MigrateToChatID,
		}
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(answer.Result, out); err != nil {
		return fmt.Errorf("%s: decode result: %w", method, err)
	}
	return nil
}

// redact keeps the token out of an error. It sits in the URL path, and Go's
// url.Error prints the URL, so a network failure would otherwise carry the
// bot's credential into last_error, which the page shows, and into the logs.
// The original error is not wrapped, as unwrapping it would bring it back.
func (c *Client) redact(err error) error {
	msg := err.Error()
	if c.token != "" {
		msg = strings.ReplaceAll(msg, c.token, redactedToken)
		msg = strings.ReplaceAll(msg, url.PathEscape(c.token), redactedToken)
	}
	return fmt.Errorf("telegram %s", msg)
}
