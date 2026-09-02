package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

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
	// defaultAccountLabel names the row a fresh install pairs against.
	defaultAccountLabel = "CS Utama"
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

	account, err := firstAccount(db)
	if err != nil {
		logger.Fatal("Failed to load the WhatsApp account", zap.Error(err))
	}

	conversations := services.NewCSConversationService(db)
	messages := services.NewCSMessageService(db, conversations)
	assignment := services.NewCSAssignmentService(db, conversations, services.NewRedisPresence(redisClient))
	retention := services.NewCSMediaRetention(db, cfg.WAMediaDir, cfg.WAMediaRetentionDays)

	client, err := wa.NewClient(ctx, wa.Options{
		Container:     sqlstore.NewWithDB(sqlDB, "postgres", waLog.Noop),
		AccountID:     account.ID,
		DB:            db,
		Publisher:     wa.NewPublisher(redisClient),
		Logger:        logger,
		Conversations: conversations,
		Messages:      messages,
		Assignment:    assignment,
		MediaRoot:     cfg.WAMediaDir,
	})
	if err != nil {
		logger.Fatal("Failed to open the WhatsApp session store", zap.Error(err))
	}

	if client.NeedsPairing() {
		codes, err := client.QRChannel(ctx)
		if err != nil {
			logger.Fatal("Failed to start WhatsApp pairing", zap.Error(err))
		}
		go showPairingCodes(codes, logger)
	}

	if err := client.Connect(ctx); err != nil {
		logger.Fatal("Failed to connect to WhatsApp", zap.Error(err))
	}
	defer client.Disconnect()

	// One Drainer, shared by both callers. The lock that stops two drains from
	// handing WhatsApp the same reply twice lives inside this instance, so a
	// second Drainer would be a second lock guarding nothing.
	drainer := wa.NewDrainer(messages, conversations, client, cfg.WAMediaDir,
		time.Duration(cfg.WASendIntervalMS)*time.Millisecond)

	go drainOnAnnouncement(ctx, redisClient, drainer, logger)
	go every(ctx, max(time.Duration(cfg.WADrainIntervalSeconds)*time.Second, time.Second), func() {
		drainOutbox(ctx, drainer, logger)
	})
	go every(ctx, assignSweep, func() { assignWaiting(ctx, assignment, logger) })
	go every(ctx, mediaSweep, func() { sweepMedia(retention, logger) })

	logger.Info("Starting WhatsApp service",
		zap.String("account", account.Label),
		zap.Bool("needs_pairing", client.NeedsPairing()))

	<-ctx.Done()
	logger.Info("Received shutdown signal")
}

// firstAccount returns the number this process answers from, creating the row
// on first start so a fresh install has something to pair against.
func firstAccount(db *gorm.DB) (models.WAAccount, error) {
	var account models.WAAccount
	err := db.Order("created_at").First(&account).Error
	if err == nil {
		return account, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return account, err
	}

	account = models.WAAccount{Label: defaultAccountLabel, Status: models.WAAccountDisconnected}
	return account, db.Create(&account).Error
}

// showPairingCodes writes each pairing code to the log. Drawing a scannable
// square would mean another dependency, so an operator pastes the string into
// any QR generator instead.
func showPairingCodes(codes <-chan string, logger *zap.Logger) {
	for code := range codes {
		logger.Info("Scan this WhatsApp pairing code", zap.String("code", code))
	}
}

// drainOnAnnouncement empties the outbox as soon as a CS hits send. The
// periodic sweep is the safety net behind it: a lost announcement costs the
// customer a few seconds, not the reply.
func drainOnAnnouncement(ctx context.Context, client *redis.Client, drainer *wa.Drainer, logger *zap.Logger) {
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
			drainOutbox(ctx, drainer, logger)
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
		logger.Error("Failed to sweep expired CS media", zap.Error(err))
		return
	}
	if cleared > 0 {
		logger.Info("Removed expired CS media", zap.Int("count", cleared))
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
