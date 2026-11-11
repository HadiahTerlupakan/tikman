package services

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/telegram"
	"go.uber.org/zap"
)

// pgIntegrityViolationClass is the SQLSTATE class of unique, foreign key,
// check and not-null violations.
const pgIntegrityViolationClass = "23"

// sqliteConstraintFailed is how SQLite words the same refusals.
const sqliteConstraintFailed = "constraint failed"

// telegramPollSeconds is how long one getUpdates waits for news. Long enough
// that an idle bot costs a request a minute, short of the proxies' idle cut.
const telegramPollSeconds = 50

// telegramTokenRecheck is how often a watcher with no token looks for one.
const telegramTokenRecheck = time.Minute

// telegramBackoff spaces the polls after a failure, so a Telegram that is
// unreachable, or a token another process is already polling with, is not
// asked again in a tight loop.
const telegramBackoff = 30 * time.Second

// TelegramChatWatcher learns which chats the bot is in from its updates; the
// Bot API has no call that lists them.
type TelegramChatWatcher struct {
	settings  *SettingService
	chats     *TelegramChatService
	newClient func(token string) *telegram.Client
	logger    *zap.Logger
	// offset and token are the watcher's only state. Losing them to a
	// restart only means re-reading updates, which Apply takes twice.
	offset int64
	token  string
}

// NewTelegramChatWatcher constructs a TelegramChatWatcher.
func NewTelegramChatWatcher(chats *TelegramChatService, settings *SettingService, logger *zap.Logger) *TelegramChatWatcher {
	return &TelegramChatWatcher{settings: settings, chats: chats, newClient: telegram.New, logger: logger}
}

// Run polls until ctx ends.
func (w *TelegramChatWatcher) Run(ctx context.Context) {
	for ctx.Err() == nil {
		wait := w.Poll(ctx)
		if wait == 0 {
			continue
		}
		select {
		case <-ctx.Done():
		case <-time.After(wait):
		}
	}
}

// Poll reads one batch of updates and answers how long to wait before the
// next. The token is read every round, so one set in Pengaturan is picked up
// without a restart.
func (w *TelegramChatWatcher) Poll(ctx context.Context) time.Duration {
	token, err := w.settings.Value(models.SettingTelegramBotToken)
	if errors.Is(err, ErrSettingNotConfigured) {
		return telegramTokenRecheck
	}
	if err != nil {
		w.logger.Warn("Could not read the Telegram bot token", zap.Error(err))
		return telegramBackoff
	}
	if token != w.token {
		if err := w.switchBot(token); err != nil {
			w.logger.Warn("Could not retire the previous bot's Telegram chats", zap.Error(err))
			return telegramBackoff
		}
	}
	updates, err := w.newClient(token).GetUpdates(ctx, w.offset, telegramPollSeconds)
	if err != nil {
		if ctx.Err() == nil {
			w.logger.Warn("Telegram getUpdates failed", zap.Error(err))
		}
		return telegramBackoff
	}
	for _, update := range updates {
		if err := w.chats.Apply(update); err != nil {
			w.logger.Warn("Could not record a Telegram chat", zap.Int64("update_id", update.UpdateID), zap.Error(err))
			// The offset stops at an update that could not be stored, so the
			// next poll is handed it again; but one the database refuses
			// outright would be refused forever and stall every chat behind it.
			if !constraintViolation(err) {
				return telegramBackoff
			}
		}
		w.offset = update.UpdateID + 1
	}
	return 0
}

// switchBot starts over with another token. Update IDs are the bot's own, so
// the old offset could skip all of the new bot's updates. The chats are the
// old bot's: the new one may be in none of them, and a report posting to one
// would fail daily while the page showed it as fine. The first token a
// process reads is not a switch; a change made while it was down surfaces as
// a refused send instead, which deactivates that chat.
func (w *TelegramChatWatcher) switchBot(token string) error {
	if w.token != "" {
		if err := w.chats.DeactivateAll(); err != nil {
			return err
		}
	}
	w.token, w.offset = token, 0
	return nil
}

// constraintViolation is the database refusing a write for what it holds,
// SQLSTATE class 23 on Postgres, which no retry of the same write fixes.
func constraintViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return strings.HasPrefix(pgErr.Code, pgIntegrityViolationClass)
	}
	return strings.Contains(err.Error(), sqliteConstraintFailed)
}
