package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/tikman/olt-provisioning/internal/config"
	"github.com/tikman/olt-provisioning/internal/database"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"github.com/tikman/olt-provisioning/internal/wa"
	"go.mau.fi/whatsmeow/store/sqlstore"
	waLog "go.mau.fi/whatsmeow/util/log"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const (
	// outboxBatch is how many replies one drain takes. It is small because the
	// drain paces itself between sends, and a long batch would keep a later
	// announcement waiting behind it.
	outboxBatch = 20
	// assignSweep picks up the threads that arrived while nobody was online.
	assignSweep = 1 * time.Minute
	// mediaSweep is how often expired attachments are removed from disk.
	mediaSweep = 24 * time.Hour
	// avatarSweep is how often profile photos are looked at. Faces change
	// rarely and the queries go to WhatsApp, so this is deliberately unhurried.
	avatarSweep = 15 * time.Minute
	// avatarBatch is how many customers one sweep asks about, and avatarPace
	// the gap left between them. A burst of queries about a list of strangers
	// is the shape of traffic that gets an unofficial number flagged.
	avatarBatch = 10
	avatarPace  = 3 * time.Second
	// avatarRefresh is how long a photo already looked at is left alone. Most
	// customers hide theirs, so most of this work is asking again about people
	// who will still say no.
	avatarRefresh = 7 * 24 * time.Hour
	// defaultAccountLabel names the row a fresh install pairs against.
	defaultAccountLabel = "CS Utama"
	// accountRescan is how often a number added from the admin screen is
	// noticed. Adding one is a rare, human action, so this is unhurried.
	accountRescan = 30 * time.Second
	// channelSync is how often a number re-reads which channels it administers.
	// Hourly rather than minutes: admin rights on a channel change rarely, and the
	// refresh button covers the case where somebody cannot wait.
	channelSync = time.Hour
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer func() {
		_ = logger.Sync()
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// The API owns the schema, including whatever this process reads. Migrating
	// from here would race it.
	db, err := database.Connect(cfg)
	if err != nil {
		logger.Fatal("Failed to connect to database", zap.Error(err))
	}
	sqlDB, err := db.DB()
	if err != nil {
		logger.Fatal("Failed to get database instance", zap.Error(err))
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.RedisHost, cfg.RedisPort),
		Password: cfg.RedisPassword,
		DB:       0,
	})
	if err := redisClient.Ping(ctx).Err(); err != nil {
		logger.Fatal("Failed to connect to Redis", zap.Error(err))
	}

	if err := seedFirstAccount(db); err != nil {
		logger.Fatal("Failed to prepare the WhatsApp account", zap.Error(err))
	}

	conversations := services.NewCSConversationService(db)
	messages := services.NewCSMessageService(db, conversations)
	assignment := services.NewCSAssignmentService(db, conversations, services.NewRedisPresence(redisClient))
	retention := services.NewCSMediaRetention(db, cfg.WAMediaDir, cfg.WAMediaRetentionDays)
	channels := services.NewCSChannelService(db)
	channelPosts := services.NewCSChannelPostService(db)

	live := newSessions(sessionDeps{
		cfg:           cfg,
		db:            db,
		container:     sqlstore.NewWithDB(sqlDB, "postgres", waLog.Noop),
		redis:         redisClient,
		conversations: conversations,
		messages:      messages,
		assignment:    assignment,
		channels:      channels,
		channelPosts:  channelPosts,
		logger:        logger,
	})
	live.sync(ctx)

	// Numbers added from the admin screen are picked up here rather than by a
	// restart. Slow on purpose: adding a number is a rare, human action.
	go every(ctx, accountRescan, func() { live.sync(ctx) })

	go controlLoop(ctx, redisClient, live, logger)
	go drainOnAnnouncement(ctx, redisClient, live, logger)
	go presenceLoop(ctx, redisClient, live, conversations, logger)
	go every(ctx, assignSweep, func() { assignWaiting(ctx, assignment, logger) })
	go func() {
		// Once at startup as well as on the ticker: a process that is restarted
		// more often than once a day would otherwise never sweep at all.
		sweepMedia(retention, logger)
		every(ctx, mediaSweep, func() { sweepMedia(retention, logger) })
	}()

	logger.Info("Starting WhatsApp service")

	<-ctx.Done()
	logger.Info("Received shutdown signal")
}

// seedFirstAccount makes sure a fresh install has one number to pair against.
// Every account after that is added from the admin screen; this only exists so
// the very first one does not have to be.
//
// Seeded by label rather than "insert if the table is empty": two processes
// starting together would otherwise each insert their own row.
func seedFirstAccount(db *gorm.DB) error {
	var account models.WAAccount
	return db.Where(models.WAAccount{Label: defaultAccountLabel}).
		Attrs(models.WAAccount{Status: models.WAAccountDisconnected}).
		FirstOrCreate(&account).Error
}

// controlLoop applies admin actions to the number they name. The channel
// carries actions for every number, so each is routed by account id and one
// naming a number this process has no session for is dropped.
func controlLoop(ctx context.Context, redisClient *redis.Client, live *sessions, logger *zap.Logger) {
	sub := redisClient.Subscribe(ctx, wa.ControlChannel)
	defer func() {
		_ = sub.Close()
	}()

	messages := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, open := <-messages:
			if !open {
				return
			}
			applyControl(ctx, msg.Payload, live, logger)
		}
	}
}

// applyControl decodes and acts on one control message. A malformed message,
// or one naming a number this process holds no session for, is logged and
// dropped harmlessly.
func applyControl(ctx context.Context, payload string, live *sessions, logger *zap.Logger) {
	var msg wa.ControlMessage
	if err := json.Unmarshal([]byte(payload), &msg); err != nil {
		logger.Warn("Could not decode a WhatsApp control message", zap.Error(err))
		return
	}
	accountID, err := uuid.Parse(msg.AccountID)
	if err != nil {
		logger.Warn("WhatsApp control message named no valid account",
			zap.String("account_id", msg.AccountID))
		return
	}
	client := live.client(accountID)
	if client == nil {
		return
	}

	switch msg.Action {
	case wa.ControlConnect:
		if err := client.Pair(ctx, msg.Phone); err != nil {
			logger.Error("Could not pair the WhatsApp session", zap.Error(err))
		}
	case wa.ControlDisconnect:
		if err := client.Unpair(ctx); err != nil {
			logger.Error("Could not disconnect the WhatsApp session", zap.Error(err))
			return
		}
		// whatsmeow's Logout deletes the on-disk device, and every later
		// Connect on a deleted device fails with store.ErrDeviceDeleted —
		// there is no in-process way back to a pairable state. Dropping this
		// one session lets the next rescan open a fresh, pairable one.
		//
		// It used to exit the process instead, which was harmless when the
		// process held one number and would now disconnect every other number
		// because one admin unpaired one of them.
		logger.Info("WhatsApp session logged out; restarting it so the number can be paired again",
			zap.String("account_id", accountID.String()))
		live.restart(accountID)
	case wa.ControlDelete:
		// The row is already gone; this is only the pairing. A number that was
		// never paired has none to give up, and Logout on it would fail for a
		// reason nobody needs to read about.
		if !client.NeedsPairing() {
			if err := client.Logout(ctx); err != nil {
				logger.Error("Could not give up the pairing of a deleted WhatsApp number",
					zap.String("account_id", accountID.String()), zap.Error(err))
			}
		}
		// Nothing restarts it: sync opens sessions for account rows, and this
		// number no longer has one. The rescan would have closed this session
		// within the minute anyway — this only spares the inbox that wait.
		live.restart(accountID)
	case wa.ControlSyncChannels:
		syncChannels(ctx, client, live.deps.channels, accountID, logger)
	default:
		logger.Warn("Unknown WhatsApp control action", zap.String("action", msg.Action))
	}
}

// drainOnAnnouncement empties the outbox as soon as a CS hits send. The
// periodic sweep is the safety net behind it: a lost announcement costs the
// customer a few seconds, not the reply.
func drainOnAnnouncement(ctx context.Context, client *redis.Client, live *sessions, logger *zap.Logger) {
	sub := client.Subscribe(ctx, wa.OutboxChannel)
	defer func() {
		_ = sub.Close()
	}()

	announcements := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case <-announcements:
			live.drainAll(ctx)
		}
	}
}

func drainOutbox(ctx context.Context, drainer *wa.Drainer, logger *zap.Logger) {
	sent, err := drainer.Drain(ctx, outboxBatch)
	if err != nil {
		logger.Error("Failed to drain the CS outbox", zap.Error(err))
	}
	if sent > 0 {
		logger.Info("Sent queued CS replies", zap.Int("count", sent))
	}
}

// drainChannelOutbox posts what is waiting for the channels one number
// administers, logging how many went.
func drainChannelOutbox(ctx context.Context, drainer *wa.ChannelDrainer, logger *zap.Logger) {
	sent, err := drainer.Drain(ctx, outboxBatch)
	if err != nil {
		logger.Error("Could not post the queued channel updates", zap.Error(err))
		return
	}
	if sent > 0 {
		logger.Info("Posted queued channel updates", zap.Int("sent", sent))
	}
}

func assignWaiting(ctx context.Context, assignment *services.CSAssignmentService, logger *zap.Logger) {
	assigned, err := assignment.AssignWaiting(ctx)
	if err != nil {
		logger.Error("Failed to assign waiting conversations", zap.Error(err))
		return
	}
	if assigned > 0 {
		logger.Info("Assigned waiting conversations", zap.Int("count", assigned))
	}
}

func sweepMedia(retention *services.CSMediaRetention, logger *zap.Logger) {
	cleared, err := retention.Sweep()
	if err != nil {
		logger.Error("Some expired CS media could not be swept", zap.Error(err))
	}
	if cleared > 0 {
		logger.Info("Removed expired CS media", zap.Int("count", cleared))
	}
}

// sweepAvatars refreshes a few customers' profile photos. A failure costs
// faces and nothing else, so it is logged rather than acted on.
func sweepAvatars(ctx context.Context, avatars *wa.AvatarSweeper, logger *zap.Logger) {
	stored, err := avatars.Sweep(ctx, avatarBatch)
	if err != nil {
		logger.Warn("Could not refresh customer profile photos", zap.Error(err))
		return
	}
	if stored > 0 {
		logger.Info("Stored customer profile photos", zap.Int("count", stored))
	}
}

func every(ctx context.Context, interval time.Duration, run func()) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}
