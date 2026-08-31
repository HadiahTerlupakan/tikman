// Command trapd receives SNMP traps from the OLTs.
//
// Polling a chassis for status costs seconds and cannot run faster than its
// agent answers, so a subscriber going down is noticed on the next pass rather
// than when it happens. A trap arrives when it happens.
//
// It is a notification, never the truth. Traps travel over UDP and are lost
// without trace, so the status poll keeps running and remains what the system
// believes; a trap only makes it look sooner.
//
// This build records what arrives and acts on nothing. The notification OIDs a
// ZTE C300 sends for ONU state changes are not documented in this repository,
// and mapping them from a guess would write subscribers' statuses from a guess.
// They get learned from the device first.
package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/gosnmp/gosnmp"
	"github.com/tikman/olt-provisioning/internal/config"
	"github.com/tikman/olt-provisioning/internal/database"
	"github.com/tikman/olt-provisioning/internal/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// listenAddress is the standard SNMP trap port on every interface. The OLTs
// reach this host over WireGuard, so the address it answers on is the tunnel's.
const listenAddress = "0.0.0.0:162"

// oltRefreshInterval is how often the source-address table is rebuilt, so an
// OLT added while this is running starts being recognised without a restart.
const oltRefreshInterval = time.Minute

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer func() { _ = logger.Sync() }()

	db, err := database.Connect(cfg)
	if err != nil {
		logger.Fatal(fmt.Sprintf("Failed to connect to database: %v", err))
	}

	directory := newOLTDirectory(db, logger)
	directory.refresh()

	store := &trapStore{db: db, logger: logger}

	listener := gosnmp.NewTrapListener()
	listener.Params = gosnmp.Default
	listener.OnNewTrap = func(packet *gosnmp.SnmpPacket, addr *net.UDPAddr) {
		trap, err := parseTrap(packet, addr, directory.find)
		if err != nil {
			// Logged rather than dropped: on this hardware the first traps to
			// arrive are the evidence of what it sends at all.
			logger.Warn("Ignoring trap", zap.Error(err))
			return
		}
		store.record(trap)

		identity := identify(trap.Varbinds)
		logger.Info("Trap received",
			zap.String("olt_id", trap.OLTID.String()),
			zap.String("source", trap.Source),
			zap.String("oid", trap.OID),
			zap.String("serial", identity.SerialNumber),
			zap.String("onu", identity.Label))
	}

	stopSweep := make(chan struct{})
	defer close(stopSweep)
	go store.sweepExpired(stopSweep)

	go func() {
		ticker := time.NewTicker(oltRefreshInterval)
		defer ticker.Stop()
		for range ticker.C {
			directory.refresh()
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		logger.Info("Received shutdown signal")
		listener.Close()
	}()

	logger.Info("Listening for SNMP traps", zap.String("address", listenAddress))
	if err := listener.Listen(listenAddress); err != nil {
		logger.Fatal(fmt.Sprintf("Trap listener stopped: %v", err))
	}
}

// oltDirectory maps a source address to the OLT that owns it.
type oltDirectory struct {
	db     *gorm.DB
	logger *zap.Logger
	byIP   atomic.Pointer[map[string]uuid.UUID]
}

func newOLTDirectory(db *gorm.DB, logger *zap.Logger) *oltDirectory {
	directory := &oltDirectory{db: db, logger: logger}
	empty := map[string]uuid.UUID{}
	directory.byIP.Store(&empty)
	return directory
}

// refresh rebuilds the table. A failure leaves the previous one in place: an
// unreachable database should not turn every OLT into an unknown sender.
func (d *oltDirectory) refresh() {
	var olts []models.OLT
	if err := d.db.Find(&olts).Error; err != nil {
		d.logger.Error("Failed to refresh the OLT directory", zap.Error(err))
		return
	}

	byIP := make(map[string]uuid.UUID, len(olts))
	for _, olt := range olts {
		byIP[olt.IPAddress] = olt.ID
	}
	d.byIP.Store(&byIP)
}

func (d *oltDirectory) find(ip string) (uuid.UUID, bool) {
	byIP := *d.byIP.Load()
	id, known := byIP[ip]
	return id, known
}
