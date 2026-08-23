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
const ()

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

// formatPower renders an optical power reading for logs, showing "no-signal"
// when the ONT returned the sentinel value instead of a real measurement.
func formatPower(dbm *float64) string {
	if dbm == nil {
		return "no-signal"
	}
	return fmt.Sprintf("%.2f", *dbm)
}
