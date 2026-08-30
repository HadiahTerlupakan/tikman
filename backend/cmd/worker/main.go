package main

import (
	"context"
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

const metricsInterval = 1 * time.Minute

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

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

	zapLogger.Info("Starting Worker service")
	zapLogger.Info("Starting metrics collection", zap.Duration("interval", metricsInterval))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	ticker := time.NewTicker(metricsInterval)
	defer ticker.Stop()

	collectMetrics(db, ontService, oltService, metricsService, eventService, zapLogger)

	for {
		select {
		case <-ticker.C:
			collectMetrics(db, ontService, oltService, metricsService, eventService, zapLogger)
		case <-sigCh:
			zapLogger.Info("Received shutdown signal")
			return
		case <-ctx.Done():
			return
		}
	}
}

func collectMetrics(db *gorm.DB, ontService *services.ONTService, oltService *services.OLTService, metricsService *services.MetricsService, eventService *services.EventService, logger *zap.Logger) {
	logger.Info("Starting metrics collection cycle")

	blockedOLTs := oltsBehindDownTunnel(db, time.Now(), logger)

	// Discover and register ONTs for every configured OLT before querying
	// metrics. This makes a newly added OLT self-populate without requiring
	// an operator to press Discover manually.
	var olts []models.OLT
	if err := db.Find(&olts).Error; err != nil {
		logger.Error("Failed to list OLTs for discovery", zap.Error(err))
	} else {
		for i := range olts {
			if blockedOLTs[olts[i].ID] {
				continue
			}
			go oltService.AutoDiscoverONTMetrics(&olts[i])
		}
	}

	onts, _, err := ontService.List(nil, nil, 1000, 0)
	if err != nil {
		logger.Error("Failed to list ONTs", zap.Error(err))
		return
	}

	logger.Info(fmt.Sprintf("Found %d ONTs to collect metrics", len(onts)))

	oltMetricsCache := make(map[string]map[connectivity.ONTLocation]connectivity.ONTMetrics)
	oltStatusCache := make(map[string]map[connectivity.ONTLocation]int)
	oltStatusWalkOK := make(map[string]bool)
	oltPruned := make(map[string]bool)
	oltRatesCache := make(map[string]map[connectivity.ONTLocation]connectivity.ONUTrafficRates)

	for _, ont := range onts {
		if blockedOLTs[ont.OLTID] {
			continue
		}

		oltKey, olt, ok := getOrInitOLT(db, ont, &oltMetricsCache, &oltStatusCache, &oltStatusWalkOK, &oltRatesCache, logger)
		if !ok {
			continue
		}

		if oltStatusWalkOK[oltKey] && !oltPruned[oltKey] {
			if !syncOntsWithDiscovery(*olt, ontService, logger) {
				continue
			}
			oltPruned[oltKey] = true
		}

		if oltStatusWalkOK[oltKey] {
			if _, found := lookupByPortAndONT(oltStatusCache[oltKey], ont.PortID, ont.ONTID); !found {
				continue
			}
		}

		processOnt(db, olt.Slot, oltMetricsCache[oltKey], oltStatusCache[oltKey], oltRatesCache[oltKey], ont, metricsService, eventService, logger)
	}

	recordHeartbeat(db, logger)
	logger.Info("Metrics collection cycle completed")
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

func lookupByPortAndONT[T any](entries map[connectivity.ONTLocation]T, portID, ontID int) (T, bool) {
	for loc, value := range entries {
		if loc.Port == portID && loc.ONTID == ontID {
			return value, true
		}
	}
	var zero T
	return zero, false
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
