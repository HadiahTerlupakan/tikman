package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/telegram"
)

// telegramCaptionLimit is the longest caption Telegram takes on a photo,
// counted in UTF-16 code units.
const telegramCaptionLimit = 1024

// captionEllipsis marks a message cut to fit the caption.
const captionEllipsis = "…"

var (
	// ErrTelegramNotConfigured is a send with no bot token.
	ErrTelegramNotConfigured = errors.New("token bot Telegram belum diatur di Pengaturan")
	// ErrTelegramChatInactive is a send to a chat the bot is no longer in.
	ErrTelegramChatInactive = errors.New("bot sudah tidak ada di grup Telegram ini")
	// ErrTelegramSend is Telegram refusing or not answering a send.
	ErrTelegramSend = errors.New("pengiriman ke Telegram gagal")
)

// telegramSendError reads as Telegram's own answer while matching
// ErrTelegramSend, so the page shows the reason and the handler the status.
type telegramSendError struct{ err error }

func (e *telegramSendError) Error() string   { return e.err.Error() }
func (e *telegramSendError) Unwrap() []error { return []error{ErrTelegramSend, e.err} }

// telegramTarget is a Telegram chat checked to take the report.
type telegramTarget struct {
	chat   models.TelegramChat
	client *telegram.Client
}

func (s *GraphReportSender) prepareTelegram(report *models.GraphReport) (*telegramTarget, error) {
	var chat models.TelegramChat
	if err := s.db.First(&chat, "id = ?", *report.TelegramChatID).Error; err != nil {
		return nil, fmt.Errorf("load Telegram chat: %w", err)
	}
	if !chat.Active {
		return nil, ErrTelegramChatInactive
	}
	token, err := s.settings.Value(models.SettingTelegramBotToken)
	if errors.Is(err, ErrSettingNotConfigured) {
		return nil, ErrTelegramNotConfigured
	}
	if err != nil {
		return nil, err
	}
	// Checked here, before anything is rendered, so that a send refused for
	// its token is never mistaken for one whose fate is unknown.
	if !telegram.ValidToken(token) {
		return nil, telegram.ErrInvalidToken
	}
	return &telegramTarget{chat: chat, client: s.telegram(token)}, nil
}

// sendTelegram posts the images as one album, or as a photo when there is
// only one: Telegram refuses an album of one.
func (s *GraphReportSender) sendTelegram(
	ctx context.Context, report *models.GraphReport, tg *telegramTarget, images []ReportImage, now time.Time, daily bool,
) error {
	base := reportFileBase(report.CustomerName, now)
	photos := make([]telegram.Photo, len(images))
	for i, image := range images {
		photos[i] = telegram.Photo{
			PNG: image.PNG, Filename: imageFilename(base, i, len(images)),
			Caption: TelegramImageCaption(report.Message, report.CustomerName, image.Ports),
		}
	}
	var err error
	if len(photos) == 1 {
		err = tg.client.SendPhoto(ctx, tg.chat.ChatID, photos[0])
	} else {
		err = tg.client.SendMediaGroup(ctx, tg.chat.ChatID, photos)
	}
	if err != nil {
		return s.telegramFailure(report.ID, tg.chat, err, now, daily)
	}
	if daily {
		s.markTelegramSent(report.ID, now)
	}
	return nil
}

// markTelegramSent records that the chat has the day's report. The photos
// are out whatever this write does; failing the send over it would only post
// them again on the retry.
func (s *GraphReportSender) markTelegramSent(id uuid.UUID, now time.Time) {
	_ = s.db.Model(&models.GraphReport{}).Where("id = ?", id).Update("telegram_sent_on", wibDate(now)).Error
}

// telegramFailure tells a refusal, where nothing was posted, from a send
// whose fate is unknown. A timeout or a dropped connection says nothing of
// whether Telegram took the album, and it often has; retrying a daily send
// every ten minutes could post it a dozen times, so the day is counted and
// the admin asked to look. A manual send is the admin looking already.
func (s *GraphReportSender) telegramFailure(
	id uuid.UUID, chat models.TelegramChat, err error, now time.Time, daily bool,
) error {
	var apiErr *telegram.APIError
	if errors.As(err, &apiErr) {
		return s.telegramRefusal(chat, apiErr)
	}
	if !daily {
		return &telegramSendError{err: err}
	}
	s.markTelegramSent(id, now)
	return &telegramUncertainError{err: err}
}

// telegramRefusal acts on why Telegram refused, and answers the refusal all
// the same. A group upgraded to a supergroup is followed, so the retry goes
// to the chat's new ID; a chat the bot was removed from, or that is gone, is
// marked inactive, so the page says so instead of failing every morning.
func (s *GraphReportSender) telegramRefusal(chat models.TelegramChat, apiErr *telegram.APIError) error {
	var followErr error
	switch {
	case apiErr.MigrateToChatID != 0:
		followErr = s.chats.Migrate(chat.ChatID, apiErr.MigrateToChatID)
	case botLostChat(apiErr):
		followErr = s.chats.Deactivate(chat.ID)
	}
	if followErr != nil {
		return &telegramSendError{err: fmt.Errorf("%v; status grup tidak tersimpan: %v", apiErr, followErr)}
	}
	return &telegramSendError{err: apiErr}
}

// telegramChatNotFound is how Telegram words a chat that is gone, or that a
// new bot was never in.
const telegramChatNotFound = "chat not found"

// botLostChat says whether a refusal means the bot cannot post in the chat
// at all: a 403 is the bot kicked, not a member or blocked, and a 400 naming
// no chat is one deleted or never joined.
func botLostChat(apiErr *telegram.APIError) bool {
	if apiErr.StatusCode == http.StatusForbidden {
		return true
	}
	return apiErr.StatusCode == http.StatusBadRequest &&
		strings.Contains(strings.ToLower(apiErr.Description), telegramChatNotFound)
}

// telegramUncertainError is a daily send that may or may not have arrived.
// It names Telegram itself even on a report with one destination: what the
// admin must check is that group.
type telegramUncertainError struct{ err error }

func (e *telegramUncertainError) Error() string {
	return fmt.Sprintf("%s: status kiriman tidak pasti (%v), periksa grup; kirim manual bila belum masuk",
		destinationTelegram, e.err)
}

func (e *telegramUncertainError) Unwrap() error { return e.err }

// TelegramImageCaption is GraphImageCaption fitted to Telegram's caption
// limit. The ports line is what says what the image shows, so it is kept
// whole and the message is cut instead.
func TelegramImageCaption(message, customer string, ports []string) string {
	head := GraphReportCaption(message, customer)
	tail := graphPortsLinePrefix + strings.Join(ports, ", ")
	if utf16Len(head)+utf16Len(tail) <= telegramCaptionLimit {
		return head + tail
	}
	if room := telegramCaptionLimit - utf16Len(tail); room > utf16Len(captionEllipsis) {
		return cutUTF16(head, room) + tail
	}
	return cutUTF16(head+tail, telegramCaptionLimit)
}

// cutUTF16 cuts s to at most limit UTF-16 units, the ellipsis included,
// without splitting a character.
func cutUTF16(s string, limit int) string {
	room := limit - utf16Len(captionEllipsis)
	used := 0
	for i, r := range s {
		size := utf16.RuneLen(r)
		if used+size > room {
			return s[:i] + captionEllipsis
		}
		used += size
	}
	return s
}

func utf16Len(s string) int {
	return len(utf16.Encode([]rune(s)))
}
