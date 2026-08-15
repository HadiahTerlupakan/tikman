package worker

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"gorm.io/gorm"
)

// MonitoringWorker handles periodic ONT status and metrics polling
type MonitoringWorker struct {
	db              *gorm.DB
	oltService      *services.OLTService
	ontService      *services.ONTService
	metricsService  *services.MetricsService
	statusInterval  time.Duration
	metricsInterval time.Duration
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
}

// NewMonitoringWorker creates a new monitoring worker
func NewMonitoringWorker(
	db *gorm.DB,
	oltService *services.OLTService,
	ontService *services.ONTService,
	metricsService *services.MetricsService,
	statusInterval time.Duration,
	metricsInterval time.Duration,
) *MonitoringWorker {
	ctx, cancel := context.WithCancel(context.Background())
	return &MonitoringWorker{
		db:              db,
		oltService:      oltService,
		ontService:      ontService,
		metricsService:  metricsService,
		statusInterval:  statusInterval,
		metricsInterval: metricsInterval,
		ctx:             ctx,
		cancel:          cancel,
	}
}

// Start begins the monitoring worker
func (w *MonitoringWorker) Start() {
	log.Printf("[Worker] Monitoring worker starting (status: %v, metrics: %v)",
		w.statusInterval, w.metricsInterval)

	w.wg.Add(2)
	go w.statusPollLoop()
	go w.metricsPollLoop()
}

// Stop gracefully stops the worker
func (w *MonitoringWorker) Stop() {
	log.Printf("[Worker] Stopping monitoring worker...")
	w.cancel()
	w.wg.Wait()
	log.Printf("[Worker] Monitoring worker stopped")
}

// statusPollLoop runs the status polling cycle
func (w *MonitoringWorker) statusPollLoop() {
	defer w.wg.Done()

	ticker := time.NewTicker(w.statusInterval)
	defer ticker.Stop()

	// Run first poll immediately
	w.pollAllONTsStatus()

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			w.pollAllONTsStatus()
		}
	}
}

// metricsPollLoop runs the metrics polling cycle
func (w *MonitoringWorker) metricsPollLoop() {
	defer w.wg.Done()

	ticker := time.NewTicker(w.metricsInterval)
	defer ticker.Stop()

	// Run first poll after 10 seconds (let status poll complete first)
	time.Sleep(10 * time.Second)
	w.pollAllONTsMetrics()

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			w.pollAllONTsMetrics()
		}
	}
}

// pollAllONTsStatus polls status for all ONTs
func (w *MonitoringWorker) pollAllONTsStatus() {
	start := time.Now()

	// Get all OLTs
	olts, err := w.oltService.List()
	if err != nil {
		log.Printf("[Worker] Failed to list OLTs: %v", err)
		return
	}

	if len(olts) == 0 {
		return
	}

	log.Printf("[Worker] Polling %d OLTs...", len(olts))

	var totalONTs int
	var successCount int

	for _, olt := range olts {
		// Get all ONTs for this OLT
		onts, _, err := w.ontService.List(&olt.ID, nil, 1000, 0)
		if err != nil {
			log.Printf("[Worker] Failed to list ONTs for OLT %s: %v", olt.Name, err)
			continue
		}

		totalONTs += len(onts)

		for _, ont := range onts {
			if err := w.pollONTStatus(&olt, &ont); err != nil {
				log.Printf("[Worker] Failed to poll ONT %s: %v", ont.SerialNumber, err)
			} else {
				successCount++
			}
		}
	}

	duration := time.Since(start)
	log.Printf("[Worker] Poll completed: %d/%d ONTs successful (duration: %v)", successCount, totalONTs, duration)
}

// pollONTStatus polls a single ONT status via SNMP
func (w *MonitoringWorker) pollONTStatus(olt *models.OLT, ont *models.ONT) error {
	// Build SNMP OID for ONT status
	// ZTE C300 OID format: .1.3.6.1.4.1.3902.1012.3.28.1.1.3.{rack}.{shelf}.{slot}.{port}.{ont_id}
	// Simplified: assuming rack=1, shelf=1, slot=1
	// TODO: Use gosnmp library to query this OID
	// oid := fmt.Sprintf(".1.3.6.1.4.1.3902.1012.3.28.1.1.3.1.1.1.%d.%d", ont.PortID, ont.ONTID)

	// For now, simulate with ping test as basic connectivity check
	err := connectivity.PingTest(olt.IPAddress, 2*time.Second)

	var newStatus models.ONTStatus
	if err != nil {
		// Ping failed - ONT likely offline
		newStatus = models.ONTStatusOffline
	} else {
		// Ping success - ONT online (simplified, real implementation would parse SNMP response)
		newStatus = models.ONTStatusOnline
	}

	// Only update if status changed
	if ont.Status != newStatus {
		log.Printf("[Worker] ONT %s status changed: %s -> %s", ont.SerialNumber, ont.Status, newStatus)
		if err := w.ontService.UpdateStatus(ont.ID, newStatus); err != nil {
			return fmt.Errorf("failed to update status: %w", err)
		}
	}

	return nil
}

// pollAllONTsMetrics collects metrics for all online ONTs
func (w *MonitoringWorker) pollAllONTsMetrics() {
	start := time.Now()

	olts, err := w.oltService.List()
	if err != nil {
		log.Printf("[Worker] Failed to list OLTs for metrics: %v", err)
		return
	}

	if len(olts) == 0 {
		return
	}

	log.Printf("[Worker] Collecting metrics from %d OLTs...", len(olts))

	var totalONTs int
	var successCount int

	for _, olt := range olts {
		onts, _, err := w.ontService.List(&olt.ID, nil, 1000, 0)
		if err != nil {
			log.Printf("[Worker] Failed to list ONTs for metrics on OLT %s: %v", olt.Name, err)
			continue
		}

		totalONTs += len(onts)

		for _, ont := range onts {
			// Only collect metrics from online ONTs
			if ont.Status != models.ONTStatusOnline {
				continue
			}

			if err := w.collectONTMetrics(&olt, &ont); err != nil {
				log.Printf("[Worker] Failed to collect metrics for ONT %s: %v", ont.SerialNumber, err)
			} else {
				successCount++
			}
		}
	}

	duration := time.Since(start)
	log.Printf("[Worker] Metrics collection completed: %d/%d ONTs successful (duration: %v)",
		successCount, totalONTs, duration)
}

// collectONTMetrics collects and stores metrics for a single ONT
func (w *MonitoringWorker) collectONTMetrics(olt *models.OLT, ont *models.ONT) error {
	metrics, err := connectivity.QueryONTMetrics(
		olt.IPAddress,
		olt.SNMPCommunity,
		ont.PortID,
		ont.ONTID,
	)
	if err != nil {
		return fmt.Errorf("SNMP query failed: %w", err)
	}

	if err := w.metricsService.StoreMetrics(ont.ID, metrics); err != nil {
		return fmt.Errorf("store metrics failed: %w", err)
	}

	return nil
}
