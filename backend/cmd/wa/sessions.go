package main

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/tikman/olt-provisioning/internal/config"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"github.com/tikman/olt-provisioning/internal/wa"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// sessionDeps is everything a session needs that is shared between all of them.
type sessionDeps struct {
	cfg           *config.Config
	db            *gorm.DB
	container     *sqlstore.Container
	redis         *redis.Client
	conversations *services.CSConversationService
	messages      *services.CSMessageService
	assignment    *services.CSAssignmentService
	logger        *zap.Logger
}

// session is one live WhatsApp connection and the loops that feed it.
type session struct {
	client  *wa.Client
	drainer *wa.Drainer
	stop    context.CancelFunc
}

// sessions holds one live connection per CS number.
//
// One process rather than one container per number. At five or ten numbers the
// container-apiece shape means that many near-identical Compose blocks and a
// redeploy every time a number is added; here adding one is a row plus pairing
// from the admin screen. The cost is a shared blast radius, which is why
// wa.Client recovers from a panic in handling any single event: one number's
// bad message must not silence the other nine.
type sessions struct {
	mu      sync.Mutex
	running map[uuid.UUID]*session
	deps    sessionDeps
}

func newSessions(deps sessionDeps) *sessions {
	return &sessions{running: map[uuid.UUID]*session{}, deps: deps}
}

// sync starts a session for every account that does not have one yet.
//
// It runs on a ticker as well as at startup, so a number added from the admin
// screen is picked up without restarting the process — which is the whole
// reason for holding the numbers in one process.
func (s *sessions) sync(ctx context.Context) {
	var accounts []models.WAAccount
	if err := s.deps.db.Find(&accounts).Error; err != nil {
		s.deps.logger.Error("Could not read the WhatsApp accounts", zap.Error(err))
		return
	}
	for _, account := range accounts {
		s.ensure(ctx, account)
	}
}

// ensure starts one account's session unless it is already running.
func (s *sessions) ensure(ctx context.Context, account models.WAAccount) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, live := s.running[account.ID]; live {
		return
	}

	// Its own context, so one number can be taken down without touching the
	// others — a logout used to exit the process, which with one number was
	// harmless and with ten is an outage.
	sessionCtx, stop := context.WithCancel(ctx)
	client, err := wa.NewClient(sessionCtx, wa.Options{
		Container:     s.deps.container,
		AccountID:     account.ID,
		DeviceJID:     account.JID,
		DB:            s.deps.db,
		Publisher:     wa.NewPublisher(s.deps.redis),
		Logger:        s.deps.logger.With(zap.String("wa_account", account.Label)),
		Conversations: s.deps.conversations,
		Messages:      s.deps.messages,
		Assignment:    s.deps.assignment,
		MediaRoot:     s.deps.cfg.WAMediaDir,
	})
	if err != nil {
		stop()
		s.deps.logger.Error("Could not open a WhatsApp session",
			zap.String("wa_account", account.Label), zap.Error(err))
		return
	}

	client.Connect(sessionCtx)
	drainer := wa.NewDrainer(account.ID, s.deps.messages, s.deps.conversations, client,
		s.deps.cfg.WAMediaDir, time.Duration(s.deps.cfg.WASendIntervalMS)*time.Millisecond)

	s.running[account.ID] = &session{client: client, drainer: drainer, stop: stop}
	s.deps.logger.Info("Started a WhatsApp session",
		zap.String("wa_account", account.Label),
		zap.Bool("needs_pairing", client.NeedsPairing()))

	go s.feed(sessionCtx, account, drainer, client)
}

// feed runs the loops that belong to one number: its outbox and its customers'
// profile photos. Both are scoped to the account, so they never touch another
// number's threads.
func (s *sessions) feed(ctx context.Context, account models.WAAccount, drainer *wa.Drainer, client *wa.Client) {
	logger := s.deps.logger.With(zap.String("wa_account", account.Label))

	go every(ctx, max(time.Duration(s.deps.cfg.WADrainIntervalSeconds)*time.Second, time.Second),
		func() { drainOutbox(ctx, drainer, logger) })

	avatars := wa.NewAvatarSweeper(account.ID, s.deps.conversations, client,
		s.deps.cfg.WAMediaDir, avatarPace, avatarRefresh)
	sweepAvatars(ctx, avatars, logger)
	every(ctx, avatarSweep, func() { sweepAvatars(ctx, avatars, logger) })
}

// drainAll empties every number's outbox. The announcement that a reply is
// waiting does not say which number it belongs to, and it does not need to:
// each session's claim is scoped to its own threads, so the others find
// nothing and cost one indexed query.
func (s *sessions) drainAll(ctx context.Context) {
	for _, live := range s.snapshot() {
		drainOutbox(ctx, live.drainer, s.deps.logger)
	}
}

// client answers the live session for one account, nil when it has none.
func (s *sessions) client(accountID uuid.UUID) *wa.Client {
	s.mu.Lock()
	defer s.mu.Unlock()
	if live, ok := s.running[accountID]; ok {
		return live.client
	}
	return nil
}

// restart drops one number's session so the next sync opens it fresh.
//
// whatsmeow's Logout deletes the device, and every later Connect on a deleted
// device fails with store.ErrDeviceDeleted — there is no way back to a pairable
// state in place. Dropping the session and letting sync build a new one gives
// it a new device, which is what pairing needs.
func (s *sessions) restart(accountID uuid.UUID) {
	s.mu.Lock()
	live, ok := s.running[accountID]
	delete(s.running, accountID)
	s.mu.Unlock()

	if ok {
		live.stop()
		live.client.Disconnect()
	}
}

func (s *sessions) snapshot() []*session {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*session, 0, len(s.running))
	for _, live := range s.running {
		out = append(out, live)
	}
	return out
}
