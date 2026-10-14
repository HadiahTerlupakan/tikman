// Command flowd collects IPFIX from the MikroTik border routers and stores,
// per minute, how much each POP exchanged with each destination AS over each
// upstream.
//
// It is its own container, not in api's network namespace like trapd: the
// routers send to the VPS's public address, and restarting api — or the
// pollers, or WhatsApp — must not stop collection, nor collection them.
//
// It keeps summaries, never flows. A border router holds hundreds of
// thousands of connections; their raw records would be millions of rows a
// day on a VPS that also runs everything else.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/tikman/olt-provisioning/internal/config"
	"github.com/tikman/olt-provisioning/internal/database"
	"github.com/tikman/olt-provisioning/internal/netflow"
	"github.com/tikman/olt-provisioning/internal/services"
	"go.uber.org/zap"
)

const (
	listenAddress = ":2055"
	// mappingRefresh is how soon a block discovery found, a renumbered
	// ifIndex or a changed sampling ratio takes effect.
	mappingRefresh = 5 * time.Minute
	defaultASNDir  = "/data/asn"
	asnFileName    = "ip2asn-v4.tsv.gz"
	// geoFileName sits beside the ASN dataset in the same volume.
	geoFileName = "ipinfo_lite.csv.gz"
	// socketBuffer gives bursts room while a minute is being closed.
	socketBuffer    = 4 << 20
	markSeenTimeout = 10 * time.Second
	// minuteTickOffset puts each close a little past the boundary, so a sleep
	// that wakes a hair early by the wall clock still lands in the new minute.
	minuteTickOffset = 500 * time.Millisecond
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
	defer func() { _ = logger.Sync() }()
	db, err := database.Connect(cfg)
	if err != nil {
		logger.Fatal(fmt.Sprintf("Failed to connect to database: %v", err))
	}

	store := services.NewFlowStore(db)
	asns := &asnSource{
		fetcher: &netflow.ASNFetcher{URL: netflow.ASNDatasetURL,
			Path: filepath.Join(asnDir(), asnFileName), Client: &http.Client{}},
		names: store, logger: logger,
	}
	// The copy on disk is installed before the socket opens, so flows after a
	// restart get their AS. Collection does not wait for a download: with no
	// copy at all, every destination counts as ASN 0 until it lands.
	asns.preload()
	go func() {
		asns.start(time.Now())
		for {
			time.Sleep(asns.nextRefresh())
			asns.refresh()
		}
	}()

	geo := &geoSource{
		fetcher: &netflow.GeoFetcher{URL: netflow.GeoDatasetURL + os.Getenv("IPINFO_TOKEN"),
			Path: filepath.Join(asnDir(), geoFileName), Client: &http.Client{}},
		logger: logger,
	}
	// Until it loads, top-IP rows are written without a country, and the UI
	// falls back to the AS's registered one.
	if os.Getenv("IPINFO_TOKEN") == "" {
		logger.Warn("IPINFO_TOKEN is not set; top destination IPs are not located")
	} else {
		geo.installLocal()
		go func() {
			geo.start(time.Now())
			for {
				time.Sleep(geo.nextRefresh())
				geo.refresh()
			}
		}()
	}

	anon := newAnonymitySource(asnDir(), &http.Client{}, logger)
	anon.preload()
	go func() {
		anon.refresh(time.Now())
		every(mappingRefresh, func() { anon.refresh(time.Now()) })
	}()
	go keepASNTypes(&http.Client{}, netflow.ASNTypesURL, os.Getenv("PEERINGDB_API_KEY"), store, logger)

	coll := newCollector(asns, geo, anon, time.Now())
	mappings := services.NewFlowMappingService(db)
	refreshRouters(coll, mappings, logger)
	go every(mappingRefresh, func() { refreshRouters(coll, mappings, logger) })

	conn := listen(logger)
	go runMinutes(coll, &retryWriter{sink: store, logger: logger}, store, logger)
	go closeOnSignal(conn, logger)
	logger.Info("Listening for IPFIX", zap.String("address", listenAddress))
	readLoop(conn, coll, logger)
}

// asnDir is the compose volume, or FLOWD_ASN_DIR for a run outside it.
func asnDir() string {
	if dir := os.Getenv("FLOWD_ASN_DIR"); dir != "" {
		return dir
	}
	return defaultASNDir
}

func listen(logger *zap.Logger) *net.UDPConn {
	addr, err := net.ResolveUDPAddr("udp", listenAddress)
	if err != nil {
		logger.Fatal(fmt.Sprintf("Bad listen address: %v", err))
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		logger.Fatal(fmt.Sprintf("Failed to listen on %s: %v", listenAddress, err))
	}
	if err := conn.SetReadBuffer(socketBuffer); err != nil {
		logger.Warn("Could not enlarge the socket buffer", zap.Error(err))
	}
	logReceiveBuffer(conn, logger)
	return conn
}

// logReceiveBuffer reports what the kernel granted: net.core.rmem_max silently
// clamps SetReadBuffer, and a small buffer drops bursts without any error.
func logReceiveBuffer(conn *net.UDPConn, logger *zap.Logger) {
	raw, err := conn.SyscallConn()
	if err != nil {
		return
	}
	var size int
	var sockErr error
	if err := raw.Control(func(fd uintptr) {
		size, sockErr = syscall.GetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_RCVBUF)
	}); err != nil || sockErr != nil {
		return
	}
	logger.Info("UDP receive buffer",
		zap.Int("requested_bytes", socketBuffer), zap.Int("effective_bytes", size))
	if size < socketBuffer {
		logger.Warn("Receive buffer smaller than requested; raise net.core.rmem_max on the host")
	}
}

func every(interval time.Duration, run func()) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		run()
	}
}

// refreshRouters reloads the mapping. A failure keeps the previous one: an
// unreachable database should not turn every router into an unknown sender.
func refreshRouters(c *collector, mappings *services.FlowMappingService, logger *zap.Logger) {
	routers, skipped, err := mappings.Load()
	if err != nil {
		logger.Error("Failed to load router mappings; keeping the previous ones", zap.Error(err))
		return
	}
	// Load does not say which of the two reasons applies to which router.
	for _, name := range skipped {
		logger.Warn("Router not matched to flows: host is not an IP address or is shared by another flow-enabled router",
			zap.String("router", name))
	}
	c.setRouters(routers)
	logger.Info("Router mappings loaded", zap.Int("routers", len(routers)),
		zap.Uint64("unknown_source_packets", c.unknownSources.Swap(0)),
		zap.Uint64("malformed_packets", c.malformed.Swap(0)))
}

// runMinutes closes each minute just after the clock's boundary and writes
// it. The minute open at shutdown is lost; a restart costs at most that
// minute.
//
// Each wait is computed afresh from the wall clock rather than kept by a
// Ticker: a Ticker runs on the monotonic clock while rows are stamped from
// the wall clock, so once NTP slewed the two apart every close would land on
// the wrong side of its boundary and every row be stamped a minute early.
func runMinutes(c *collector, w *retryWriter, sink flowSink, logger *zap.Logger) {
	for {
		time.Sleep(time.Until(nextMinuteTick(time.Now())))
		finishMinute(c, w, sink, logger, time.Now())
	}
}

// nextMinuteTick is minuteTickOffset past the next minute boundary after now.
func nextMinuteTick(now time.Time) time.Time {
	return now.Truncate(time.Minute).Add(time.Minute + minuteTickOffset)
}

func finishMinute(c *collector, w *retryWriter, sink flowSink, logger *zap.Logger, now time.Time) {
	rows, seen, folded := c.closeMinute(now)
	for routerID, n := range folded {
		logger.Warn("Keys over the per-router cap folded into ASN 0",
			zap.String("router_id", routerID.String()), zap.Uint64("folded_flows", n))
	}
	if n := c.implausible.Swap(0); n > 0 {
		logger.Warn("Dropped flow records whose counts no IPv4 flow can have", zap.Uint64("dropped_flows", n))
	}
	window := c.closeWindowIfDue(now)
	w.flush(pendingBatch{minute: rows, topIPs: window.TopIPs, ownPrefixes: window.OwnPrefixes})
	ctx, cancel := context.WithTimeout(context.Background(), markSeenTimeout)
	defer cancel()
	if err := sink.MarkSeen(ctx, seen, now); err != nil {
		logger.Warn("Failed to record when flows last arrived", zap.Error(err))
	}
}

// readLoop holds one netflow.MaxPacketBytes buffer, reused for every read: no
// IPv4 UDP datagram is larger, and a sender can make it hold no more.
func readLoop(conn *net.UDPConn, c *collector, logger *zap.Logger) {
	buf := make([]byte, netflow.MaxPacketBytes)
	for {
		n, src, err := conn.ReadFromUDPAddrPort(buf)
		if errors.Is(err, net.ErrClosed) {
			return
		}
		if err != nil {
			logger.Warn("UDP read failed", zap.Error(err))
			continue
		}
		c.handleSafely(src.Addr(), buf[:n], logger)
	}
}

func closeOnSignal(conn *net.UDPConn, logger *zap.Logger) {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
	logger.Info("Received shutdown signal")
	_ = conn.Close()
}
