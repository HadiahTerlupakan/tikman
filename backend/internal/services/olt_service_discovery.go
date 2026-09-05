package services

import (
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
)

// One discovery run against one chassis: claiming it, walking it, and
// storing what the walk found.

// staleDiscoveryClaim is how long a discovery may go without publishing
// progress before another run may take its claim over. The claim lives in the
// database but the run holding it does not: a restart mid-walk leaves the
// phase at "discovering" with nobody working on it, and without this the OLT
// would never be polled again.
//
// It is measured against the heartbeat, not the start, so it only has to
// exceed the longest quiet stretch inside a run rather than the whole run.
// Storing the metrics for a large OLT is that stretch, at well under a minute.
const staleDiscoveryClaim = 5 * time.Minute

// minDiscoveryInterval is the quiet period after a completed discovery before
// another one may start. The worker ticks every minute but a full walk of a
// populated OLT takes several, so without this the run that just finished was
// restarted on the next tick: the progress bar reset to zero over and over and
// never showed a completed poll, and the OLT was walked continuously. ONT
// status does not depend on this — the metrics cycle walks it separately every
// minute.
const minDiscoveryInterval = 5 * time.Minute

// TryClaimDiscovery atomically claims an OLT for one discovery run. The
// database claim prevents the API-triggered run and worker fallback from
// polling the same OLT concurrently, and holds off a run that would follow a
// completed one too closely. An OLT that has never been polled has no last
// poll time, so a newly created one is discovered immediately.
func (s *OLTService) TryClaimDiscovery(oltID uuid.UUID) (bool, error) {
	// A run that has never published progress is judged on when it started, so
	// a claim taken but abandoned before its first instalment still expires.
	liveness := "COALESCE(discovery_heartbeat_at, discovery_started_at)"

	result := s.db.Model(&models.OLT{}).
		Where("id = ? AND (discovery_phase NOT IN ? OR "+liveness+" IS NULL OR "+liveness+" < ?)",
			oltID, []string{"discovering", "polling"}, time.Now().Add(-staleDiscoveryClaim)).
		Where("discovery_last_poll_at IS NULL OR discovery_last_poll_at < ?",
			time.Now().Add(-minDiscoveryInterval)).
		Updates(map[string]interface{}{
			"discovery_phase": "discovering", "discovery_error": "",
			// Stamped here, not when the walk starts, so the claim itself carries
			// the age the takeover above tests against.
			"discovery_started_at":   time.Now(),
			"discovery_heartbeat_at": time.Now(),
		})
	if result.Error != nil {
		return false, fmt.Errorf("claim discovery: %w", result.Error)
	}
	return result.RowsAffected == 1, nil
}

// AutoDiscoverONTMetrics polls ONT metrics from this OLT via SNMP and stores
// them asynchronously. The handler and worker may both call it; the database
// claim ensures only one run proceeds.
func (s *OLTService) AutoDiscoverONTMetrics(olt *models.OLT) {
	release, started := s.beginDiscovery(olt)
	if !started {
		return
	}
	defer release()

	driver, err := connectivity.DriverFor(olt.Model)
	if err != nil {
		log.Printf("[AutoDiscovery] Cannot poll OLT %s: %v", olt.Name, err)
		return
	}

	s.refreshVLANCache(olt)
	s.refreshSystemCache(olt)
	s.refreshProfileCache(olt)

	statuses, allMetrics, locations := s.walkONTPositions(driver, olt)
	s.registerInventory(driver, olt, locations, statuses, allMetrics)
	polled := s.storeDiscoveredMetrics(olt, allMetrics)

	s.updateDiscoveryProgress(olt.ID, map[string]interface{}{
		"discovery_phase": "completed", "discovery_polled": polled,
		"discovery_last_poll_at": time.Now(),
	})
	log.Printf("[AutoDiscovery] Completed: polled metrics for %d ONTs from OLT %s", polled, olt.Name)
}

// beginDiscovery takes both claims a run needs — the database one that keeps a
// second worker off this chassis, and the in-process SNMP one it shares with
// the metrics cycle — and publishes the run as started. The returned release
// drops the SNMP lock, and is only valid when the run was actually claimed.
func (s *OLTService) beginDiscovery(olt *models.OLT) (func(), bool) {
	claimed, err := s.TryClaimDiscovery(olt.ID)
	if err != nil {
		log.Printf("[AutoDiscovery] Cannot claim OLT %s: %v", olt.Name, err)
		return nil, false
	}
	if !claimed {
		log.Printf("[AutoDiscovery] Skipping OLT %s: discovery already running", olt.Name)
		return nil, false
	}

	// The metrics cycle walks the same tables. Whoever gets here second stands
	// down rather than doubling the load on the OLT.
	release, free := TryLockOLTSNMP(olt.ID)
	if !free {
		log.Printf("[AutoDiscovery] Skipping OLT %s: another collector is reading it", olt.Name)
		s.updateDiscoveryProgress(olt.ID, map[string]interface{}{"discovery_phase": "idle"})
		return nil, false
	}

	log.Printf("[AutoDiscovery] Starting immediate ONT metrics polling for OLT %s (%s)", olt.Name, olt.IPAddress)
	s.updateDiscoveryProgress(olt.ID, map[string]interface{}{
		"discovery_phase": "discovering", "discovery_total": 0,
		"discovery_registered": 0, "discovery_polled": 0,
		"discovery_error": "", "discovery_started_at": time.Now(),
	})
	return release, true
}

// walkONTPositions enumerates the ONTs the chassis reports. The status table
// is read first because it answers quickly, which is what lets the UI show a
// discovery total before the slower optical walk finishes. Neither walk
// failing stops the run: an inventory built from status data alone is still
// worth registering.
func (s *OLTService) walkONTPositions(driver connectivity.Driver, olt *models.OLT) (
	map[connectivity.ONTLocation]int,
	map[connectivity.ONTLocation]connectivity.ONTMetrics,
	map[connectivity.ONTLocation]bool,
) {
	statuses, statusErr := driver.WalkStatuses(olt.IPAddress, olt.SNMPCommunity, olt.SNMPPort)
	if statusErr != nil {
		log.Printf("[AutoDiscovery] Status walk failed for OLT %s: %v", olt.Name, statusErr)
	}

	locations := make(map[connectivity.ONTLocation]bool, len(statuses))
	for loc := range statuses {
		locations[loc] = true
	}
	if len(locations) > 0 {
		s.updateDiscoveryProgress(olt.ID, map[string]interface{}{
			"discovery_phase": "discovering", "discovery_total": len(locations),
		})
	}

	// Read for the ONUs the status walk named, rather than swept: sweeping the
	// optical tables does not finish on a populated OLT, and returned 96 of 200
	// rows before timing out. A failure here must still not prevent registering
	// ONTs from status data.
	allMetrics, metricsErr := readONTMetrics(driver, olt, locations)
	if metricsErr != nil {
		log.Printf("[AutoDiscovery] Metrics read failed for OLT %s: %v; continuing with inventory", olt.Name, metricsErr)
		allMetrics = make(map[connectivity.ONTLocation]connectivity.ONTMetrics)
	}
	for loc := range allMetrics {
		locations[loc] = true
	}

	log.Printf("[AutoDiscovery] Found %d ONT positions via SNMP", len(locations))
	s.updateDiscoveryProgress(olt.ID, map[string]interface{}{
		"discovery_phase": "polling", "discovery_total": len(locations),
	})
	return statuses, allMetrics, locations
}

// registerInventory reads the chassis inventory and stores each instalment as
// the walk reports it, rather than after the whole inventory, which is what
// lets the progress bar move on a large OLT.
func (s *OLTService) registerInventory(
	driver connectivity.Driver,
	olt *models.OLT,
	locations map[connectivity.ONTLocation]bool,
	statuses map[connectivity.ONTLocation]int,
	allMetrics map[connectivity.ONTLocation]connectivity.ONTMetrics,
) {
	locationList := make([]connectivity.ONTLocation, 0, len(locations))
	for loc := range locations {
		locationList = append(locationList, loc)
	}

	processed := 0
	err := driver.InventoryByPort(olt.IPAddress, olt.SNMPCommunity, olt.SNMPPort, locationList,
		func(locs []connectivity.ONTLocation, inventory map[connectivity.ONTLocation]connectivity.ONTInventory) {
			processed += len(locs)
			s.registerDiscoveredONTs(olt, locs, inventory, statuses, allMetrics, processed)
		})
	if err != nil {
		log.Printf("[AutoDiscovery] Inventory walk failed for OLT %s: %v", olt.Name, err)
	}

	registered, _ := NewONTService(s.db).CountONTsByOLT(olt.ID)
	if registered == 0 && len(locationList) > 0 {
		s.updateDiscoveryProgress(olt.ID, map[string]interface{}{"discovery_error": "inventory unavailable"})
	}
}

// storeDiscoveredMetrics writes each optical reading onto the ONT that holds
// that position, and reports how many were stored.
//
// Matching without the card attached every reading at port 12 ONU 22 to
// whichever card's row the database returned first. On a chassis where three
// cards carry that position, two subscribers' optical readings were being
// written onto a third one's ONT.
func (s *OLTService) storeDiscoveredMetrics(olt *models.OLT, allMetrics map[connectivity.ONTLocation]connectivity.ONTMetrics) int {
	metricsService := NewMetricsService(s.db)
	owner := NewONTService(s.db)

	stored := 0
	for loc, metrics := range allMetrics {
		ont, err := owner.GetByOLTAndPosition(olt.ID, loc.Slot, loc.Port, loc.ONTID)
		if err != nil || ont == nil {
			log.Printf("[AutoDiscovery] Skipping unregistered ONT at slot=%d port=%d ont=%d", loc.Slot, loc.Port, loc.ONTID)
			continue
		}
		if metrics.RxPower == nil && metrics.TxPower == nil && metrics.Distance <= 0 {
			continue
		}
		if err := metricsService.StoreMetrics(ont.ID, &metrics, nil); err != nil {
			log.Printf("[AutoDiscovery] Failed to store metrics for ONT %s: %v", ont.SerialNumber, err)
			continue
		}
		logPolledMetrics(ont.SerialNumber, loc, metrics)
		stored++
	}
	return stored
}

func logPolledMetrics(serial string, loc connectivity.ONTLocation, metrics connectivity.ONTMetrics) {
	rx, tx := "-", "-"
	if metrics.RxPower != nil {
		rx = fmt.Sprintf("%.2f", *metrics.RxPower)
	}
	if metrics.TxPower != nil {
		tx = fmt.Sprintf("%.2f", *metrics.TxPower)
	}
	log.Printf("[AutoDiscovery] ✅ Polled metrics: serial=%s port=%d/%d rx_power=%s dBm tx_power=%s dBm distance=%dm",
		serial, loc.Port, loc.ONTID, rx, tx, metrics.Distance)
}

// registerDiscoveredONTs persists one instalment of an inventory walk and
// publishes how far the walk has got. Progress counts locations this walk has
// covered, not rows in the table: re-running discovery on an OLT that is
// already populated would otherwise show a full bar from the first instalment.
func (s *OLTService) registerDiscoveredONTs(
	olt *models.OLT,
	locs []connectivity.ONTLocation,
	inventory map[connectivity.ONTLocation]connectivity.ONTInventory,
	statuses map[connectivity.ONTLocation]int,
	allMetrics map[connectivity.ONTLocation]connectivity.ONTMetrics,
	processed int,
) {
	registerService := NewONTService(s.db)

	discovered := make([]connectivity.DiscoveredONT, 0, len(locs))
	for _, loc := range locs {
		item := connectivity.DiscoveredONT{Slot: loc.Slot, PortID: loc.Port, ONTID: loc.ONTID, RunState: statuses[loc]}
		if inv, ok := inventory[loc]; ok {
			item.SerialNumber = inv.SerialNumber
			item.Name = inv.Name
			item.DeviceType = inv.DeviceType
			item.HardwareVersion = inv.HardwareVersion
			item.IPAddress = inv.IPAddress
		}
		if metrics, ok := allMetrics[loc]; ok {
			item.RxPower = metrics.RxPower
			item.TxPower = metrics.TxPower
			item.Distance = metrics.Distance
		}
		discovered = append(discovered, item)
	}

	result := registerService.BulkRegisterFromDiscovery(olt.ID, discovered)
	s.updateDiscoveryProgress(olt.ID, map[string]interface{}{"discovery_registered": processed})

	// An ONU the walk found but could not store is a subscriber missing from the
	// system. Discarding these left the loss to be noticed by counting rows.
	for _, regErr := range result.Errors {
		log.Printf("[AutoDiscovery] OLT %s: %s", olt.Name, regErr)
	}

	slot, port := 0, 0
	if len(locs) > 0 {
		slot, port = locs[0].Slot, locs[0].Port
	}
	log.Printf("[AutoDiscovery] Instalment slot=%d port=%d: onts=%d touched=%d processed=%d", slot, port, len(locs), result.Registered, processed)
}

func (s *OLTService) updateDiscoveryProgress(oltID uuid.UUID, updates map[string]interface{}) {
	// Every progress publish doubles as the claim's heartbeat, so liveness costs
	// no extra write and cannot drift from the work actually being done.
	updates["discovery_heartbeat_at"] = time.Now()
	if err := s.db.Model(&models.OLT{}).Where("id = ?", oltID).Updates(updates).Error; err != nil {
		log.Printf("[AutoDiscovery] Failed to update progress for OLT %s: %v", oltID, err)
	}
}

// readONTMetrics prefers a driver that can read named ONUs, falling back to the
// table sweep for one that cannot.
func readONTMetrics(driver connectivity.Driver, olt *models.OLT, known map[connectivity.ONTLocation]bool) (map[connectivity.ONTLocation]connectivity.ONTMetrics, error) {
	querier, direct := driver.(connectivity.MetricsQuerier)
	if !direct || len(known) == 0 {
		return driver.WalkMetrics(olt.IPAddress, olt.SNMPCommunity, olt.SNMPPort)
	}

	locations := make([]connectivity.ONTLocation, 0, len(known))
	for loc := range known {
		locations = append(locations, loc)
	}
	return querier.QueryMetricsFor(olt.IPAddress, olt.SNMPCommunity, olt.SNMPPort, locations)
}
