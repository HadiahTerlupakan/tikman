package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/config"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/database"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// heartbeatInterval is how often the worker stamps the row the API reads to
// decide whether polling is still happening.
const heartbeatInterval = 1 * time.Minute

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Every SNMP table read goes through GETBULK; this is how many values one
	// round trip asks the agent for.
	connectivity.SetMaxRepetitions(uint8(cfg.SNMPMaxRepetitions))

	zapLogger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer func() {
		_ = zapLogger.Sync()
	}()

	db, err := database.Connect(cfg)
	if err != nil {
		zapLogger.Fatal(fmt.Sprintf("Failed to connect to database: %v", err))
	}

	ontService := services.NewONTService(db)
	commanderFactory := connectivity.NewCommanderFactoryWithEncryption(5*time.Second, cfg.EncryptionKey)
	oltService := services.NewOLTServiceWithCommander(db, cfg.EncryptionKey, commanderFactory)
	metricsService := services.NewMetricsService(db)
	// Status history was never recorded by the running worker: the only writer of
	// ont_events was the /admin/seed-events endpoint, so an ONT's Events tab and
	// its availability figure were empty unless someone had seeded demo data.
	eventService := services.NewEventService(db)

	rt := &workerRuntime{
		db:      db,
		id:      newWorkerID(),
		jobs:    services.NewPollJobService(db),
		onts:    ontService,
		olts:    oltService,
		metrics: metricsService,
		events:  eventService,
		logger:  zapLogger,
	}

	zapLogger.Info("Starting Worker service", zap.String("worker", rt.id))
	zapLogger.Info("Poll tiers",
		zap.Duration("status", services.StatusInterval),
		zap.Duration("metrics", services.MetricsInterval),
		zap.Duration("discovery", services.DiscoveryInterval))

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	stopHeartbeat := make(chan struct{})
	defer close(stopHeartbeat)
	go beatWhileAlive(db, stopHeartbeat, zapLogger)

	for {
		select {
		case <-sigCh:
			zapLogger.Info("Received shutdown signal")
			return
		default:
		}

		// An OLT added while nothing was running would otherwise never be
		// polled, and nothing would report it missing.
		if err := rt.jobs.EnsureJobs(); err != nil {
			zapLogger.Error("Failed to ensure poll jobs", zap.Error(err))
		}

		if !runNextJob(rt) {
			// Nothing is due. Sleeping here rather than spinning is what keeps
			// an idle worker off the database.
			time.Sleep(idleWait)
		}
	}
}

// beatWhileAlive stamps the worker heartbeat on its own timer.
//
// It used to be stamped only when a cycle finished, so that a worker hung on an
// unreachable OLT could not report itself alive. There is no single cycle any
// more, and a discovery job legitimately runs for minutes — longer than the
// staleness threshold — so tying the two together would flap the worker to
// "down" every hour for doing its job.
//
// A worker stuck inside a job is caught better elsewhere: its claim's lease
// expires and another worker takes the work. Each signal now means one thing.
func beatWhileAlive(db *gorm.DB, stop <-chan struct{}, logger *zap.Logger) {
	recordHeartbeat(db, logger)

	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			recordHeartbeat(db, logger)
		}
	}
}

// recordHeartbeat stamps the row the API reads to decide whether polling is
// still happening. It runs after the cycle rather than before it, so the stamp
// means "a cycle finished" and not "the loop is still spinning" — a worker hung
// on an unreachable OLT would otherwise keep reporting itself alive.
//
// A failed stamp is logged and nothing else: the poll itself succeeded, and
// aborting the cycle over the bookkeeping would turn a reporting fault into a
// real outage.
func recordHeartbeat(db *gorm.DB, logger *zap.Logger) {
	beat := models.WorkerHeartbeat{
		Name:   models.WorkerHeartbeatPoller,
		BeatAt: time.Now(),
	}

	err := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "name"}},
		DoUpdates: clause.AssignmentColumns([]string{"beat_at"}),
	}).Create(&beat).Error
	if err != nil {
		logger.Warn("Could not record worker heartbeat", zap.Error(err))
	}
}

func syncOntsWithDiscovery(olt models.OLT, ontService *services.ONTService, logger *zap.Logger) bool {
	discovered, err := discoverONTsForSync(olt)
	if err != nil {
		logger.Error("Failed to discover ONTs for sync", zap.String("olt", olt.Name), zap.Error(err))
		return false
	}

	deleted, err := ontService.PruneMissingFromDiscovery(olt.ID, discovered)
	if err != nil {
		logger.Error("Failed to prune stale ONTs", zap.String("olt", olt.Name), zap.Error(err))
		return false
	}
	if deleted > 0 {
		logger.Info("Pruned stale ONTs", zap.String("olt", olt.Name), zap.Int64("deleted", deleted))
	}

	regResult := ontService.BulkRegisterFromDiscovery(olt.ID, discovered)
	if regResult.Registered > 0 {
		logger.Info("Synced ONTs from OLT", zap.String("olt", olt.Name), zap.Int("changed", regResult.Registered))
	}
	for _, e := range regResult.Errors {
		logger.Warn("ONT sync registration error", zap.String("olt", olt.Name), zap.String("error", e))
	}

	return true
}

func updateOLTConnectionStatus(db *gorm.DB, oltID uuid.UUID, status models.OLTStatus, logger *zap.Logger) error {
	updates := map[string]interface{}{"status": status}
	if status == models.OLTStatusOnline {
		updates["last_seen"] = time.Now()
	}

	result := db.Model(&models.OLT{}).Where("id = ?", oltID).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		logger.Info("Updated OLT connection status", zap.String("olt_id", oltID.String()), zap.String("status", string(status)))
	}
	return nil
}
func discoverONTsForSync(olt models.OLT) ([]connectivity.DiscoveredONT, error) {
	driver, err := connectivity.DriverFor(olt.Model)
	if err != nil {
		return nil, err
	}

	topology, err := connectivity.DiscoverOLTTopology(driver, olt.IPAddress, olt.SNMPCommunity, olt.SNMPPort)
	if err != nil {
		return nil, err
	}

	discovered := make([]connectivity.DiscoveredONT, 0)
	for _, slot := range topology {
		for _, port := range slot.Ports {
			discovered = append(discovered, port.ONTs...)
		}
	}
	return discovered, nil
}

func mapRunStateToStatus(runState int) models.ONTStatus {
	switch runState {
	case 3:
		return models.ONTStatusOnline
	case 6:
		return models.ONTStatusOffline
	case 1:
		return models.ONTStatusLOS
	case 4:
		return models.ONTStatusDyingGas
	default:
		return models.ONTStatusUnknown
	}
}
