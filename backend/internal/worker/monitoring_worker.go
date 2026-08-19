package worker

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/tikman/olt-provisioning/internal/services"
	"gorm.io/gorm"
)

// Polling intervals for ONT status and metrics collection
const (
)

// MonitoringWorker handles periodic ONT status and metrics polling
type MonitoringWorker struct {
	db              *gorm.DB
	oltService      *services.OLTService
	ontService      *services.ONTService
	metricsService  *services.MetricsService
	eventService    *services.EventService
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
	eventService *services.EventService,
	statusInterval time.Duration,
	metricsInterval time.Duration,
) *MonitoringWorker {
	ctx, cancel := context.WithCancel(context.Background())
	return &MonitoringWorker{
		db:              db,
		oltService:      oltService,
		ontService:      ontService,
		metricsService:  metricsService,
		eventService:    eventService,
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

// pollONTStatus polls ONT status via SNMP ZXGPON-MIB phase state
// OID: 1.3.6.1.4.1.3902.1012.3.28.2.1.4.{ifIndex}.{onuIndex}
// Status mapping per NetManeger verified values (C300 V2.1.0):
//   3 = working/online - ONU registered and passing traffic
//   4 = dying_gasp     - ONU just lost power
//   6 = offline        - ONU powered off or cable disconnected
//   1 = los            - Loss of Signal
//   other = unknown    - unrecognized value
// pollONTStatus is retained for targeted single-ONT checks.
// Note: Slot parameter is now 0 (discovered dynamically from ifIndex).
// UNUSED: func (w *MonitoringWorker) pollONTStatus(olt *models.OLT, ont *models.ONT) error {
// UNUSED: 	phaseState, err := connectivity.PollOntStatus(
// UNUSED: 		olt.IPAddress,
// UNUSED: 		olt.SNMPCommunity,
// UNUSED: 		olt.SNMPPort,
// UNUSED: 		0, // Slot - no longer used, discovered from device via SNMP walk
// UNUSED: 		ont.PortID,
// UNUSED: 		ont.ONTID,
// UNUSED: 	)
// UNUSED: 	if err != nil {
// UNUSED: 		return fmt.Errorf("SNMP poll failed: %w", err)
// UNUSED: 	}
// UNUSED: 
// UNUSED: 	newStatus := models.ONTStatus(utils.StatusMap(phaseState))
// UNUSED: 
// UNUSED: 	if ont.Status != newStatus {
// UNUSED: 		log.Printf("[Worker] ONT %s status changed: %s -> %s (phase state: %d)",
// UNUSED: 			ont.SerialNumber, ont.Status, newStatus, phaseState)
// UNUSED: 		if err := w.ontService.UpdateStatus(ont.ID, newStatus); err != nil {
// UNUSED: 			return fmt.Errorf("failed to update status: %w", err)
// UNUSED: 		}
// UNUSED: 	}
// UNUSED: 
// UNUSED: 	return nil
// UNUSED: }

// formatPower renders an optical power reading for logs, showing "no-signal"
// when the ONT returned the sentinel value instead of a real measurement.
func formatPower(dbm *float64) string {
	if dbm == nil {
		return "no-signal"
	}
	return fmt.Sprintf("%.2f", *dbm)
}
