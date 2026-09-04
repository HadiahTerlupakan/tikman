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
	channels      *services.CSChannelService
	channelPosts  *services.CSChannelPostService
	logger        *zap.Logger
}

// session is one live WhatsApp connection and the loops that feed it.
type session struct {
	client         *wa.Client
	drainer        *wa.Drainer
	channelDrainer *wa.ChannelDrainer
	stop           context.CancelFunc
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
	s.closeOrphans(accounts)
}

// closeOrphans shuts down the sessions whose account row has been deleted.
//
// This is the safety net under deleting a number. The control message asking
// this process to give the pairing up is fire-and-forget, so a process that
// was down when an admin deleted a number would otherwise come back up still
// holding its session — writing inbound messages against threads that no
// longer exist, from a number the inbox has stopped showing.
func (s *sessions) closeOrphans(accounts []models.WAAccount) {
	s.mu.Lock()
	running := make([]uuid.UUID, 0, len(s.running))
	for id := range s.running {
		running = append(running, id)
	}
	s.mu.Unlock()

	// The lock is released first: restart takes it too.
	for _, id := range orphaned(running, accounts) {
		s.deps.logger.Info("Closing the session of a WhatsApp number that was deleted",
			zap.String("account_id", id.String()))
		s.restart(id)
	}
}

// orphaned answers which running sessions no longer have an account row.
func orphaned(running []uuid.UUID, accounts []models.WAAccount) []uuid.UUID {
	known := make(map[uuid.UUID]struct{}, len(accounts))
	for _, account := range accounts {
		known[account.ID] = struct{}{}
	}

	var gone []uuid.UUID
	for _, id := range running {
		if _, ok := known[id]; !ok {
			gone = append(gone, id)
		}
	}
	return gone
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
	// One publisher for the session: the client announces what arrives, the
	// drainer announces what leaves, and both speak on the same channel.
	publisher := wa.NewPublisher(s.deps.redis)
	client, err := wa.NewClient(sessionCtx, wa.Options{
		Container:     s.deps.container,
		AccountID:     account.ID,
		DeviceJID:     account.JID,
		DB:            s.deps.db,
		Publisher:     publisher,
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
		publisher, s.deps.cfg.WAMediaDir,
		time.Duration(s.deps.cfg.WASendIntervalMS)*time.Millisecond)
	channelDrainer := wa.NewChannelDrainer(account.ID, s.deps.channelPosts, client,
		publisher, s.deps.cfg.WAMediaDir,
		time.Duration(s.deps.cfg.WASendIntervalMS)*time.Millisecond)

	live := &session{
		client: client, drainer: drainer, channelDrainer: channelDrainer, stop: stop,
	}
	s.running[account.ID] = live
	s.deps.logger.Info("Started a WhatsApp session",
		zap.String("wa_account", account.Label),
		zap.Bool("needs_pairing", client.NeedsPairing()))

	go s.feed(sessionCtx, account, live)
}

// feed runs the loops that belong to one number: its outboxes, its channel
// list, and its customers' profile photos. All are scoped to the account, so
// they never touch another number's threads.
func (s *sessions) feed(ctx context.Context, account models.WAAccount, live *session) {
	logger := s.deps.logger.With(zap.String("wa_account", account.Label))

	go every(ctx, max(time.Duration(s.deps.cfg.WADrainIntervalSeconds)*time.Second, time.Second),
		func() {
			drainOutbox(ctx, live.drainer, logger)
			// The channel outbox rides the same ticker for the same reason the
			// message one needs it: Redis pub/sub is not durable, so a post
			// queued while this process was down has no announcement left to
			// wake it and would sit at "Antre" until somebody happened to send
			// something else.
			drainChannelOutbox(ctx, live.channelDrainer, logger)
		})

	// Off the connection being established rather than off Connect returning:
	// Connect returns once the noise handshake is sent, and asking an
	// unauthenticated socket for the channel list either errors or waits out
	// whatsmeow's 75-second request timeout.
	go syncChannelsOnConnect(ctx, live.client, s.deps.channels, account.ID, logger)
	go every(ctx, channelSync, func() {
		syncChannels(ctx, live.client, s.deps.channels, account.ID, logger)
	})

	avatars := wa.NewAvatarSweeper(account.ID, s.deps.conversations, live.client,
		s.deps.cfg.WAMediaDir, avatarPace, avatarRefresh)
	sweepAvatars(ctx, avatars, logger)
	every(ctx, avatarSweep, func() { sweepAvatars(ctx, avatars, logger) })
}

// drainAll empties both of every number's outboxes, the chat replies and the
// channel updates. The announcement that something is waiting does not say
// which number it belongs to, and it does not need to: each session's claim is
// scoped to its own rows, so the others find nothing and cost one indexed
// query.
func (s *sessions) drainAll(ctx context.Context) {
	for _, live := range s.snapshot() {
		drainOutbox(ctx, live.drainer, s.deps.logger)
		drainChannelOutbox(ctx, live.channelDrainer, s.deps.logger)
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

// restart drops one number's session. What happens next is decided by whether
// the account row is still there: sync opens a session for every row that has
// none, so a number that still exists comes back and a deleted one does not.
//
// After a logout the drop is what makes the number pairable again. whatsmeow's
// Logout deletes the device, and every later Connect on a deleted device fails
// with store.ErrDeviceDeleted — there is no way back to a pairable state in
// place, so the session has to be rebuilt around a new device.
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
