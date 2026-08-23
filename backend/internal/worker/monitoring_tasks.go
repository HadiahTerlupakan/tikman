package worker

import (
	"log"
	"time"

	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/utils"
)

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
		onts, _, err := w.ontService.List(&olt.ID, nil, 1000, 0)
		if err != nil {
			log.Printf("[Worker] Failed to list ONTs for OLT %s: %v", olt.Name, err)
			continue
		}
		totalONTs += len(onts)

		driver, err := connectivity.DriverFor(olt.Model)
		if err != nil {
			log.Printf("[Worker] Cannot poll OLT %s: %v", olt.Name, err)
			continue
		}

		// One walk per OLT covers every ONT: the ZXGPON ifIndex in each OID
		// carries the line-card slot as reported by the device, so we never
		// have to guess it from stored rack/shelf/slot values.
		statuses, err := driver.WalkStatuses(olt.IPAddress, olt.SNMPCommunity, olt.SNMPPort)
		if err != nil {
			log.Printf("[Worker] Failed to walk phase state on OLT %s: %v", olt.Name, err)
			continue
		}

		for _, ont := range onts {
			phaseState, found := lookupByPortAndONT(statuses, ont.PortID, ont.ONTID)
			if !found {
				if ont.Status != models.ONTStatusUnknown {
					log.Printf("[Worker] ONT %s not reported by OLT %s, marking unknown",
						ont.SerialNumber, olt.Name)
					if err := w.ontService.UpdateStatus(ont.ID, models.ONTStatusUnknown); err != nil {
						log.Printf("[Worker] Failed to update ONT %s: %v", ont.SerialNumber, err)
						continue
					}
					if err := w.eventService.LogStatusChange(ont.ID, models.EventTypeOffline, "Unknown"); err != nil {
						log.Printf("[Worker] Failed to log event for ONT %s: %v", ont.SerialNumber, err)
					}
				}
				if err := w.ontService.UpdateUptimeMetrics(ont.ID); err != nil {
					log.Printf("[Worker] Failed to update uptime for ONT %s: %v", ont.SerialNumber, err)
				}
				successCount++
				continue
			}

			newStatus := models.ONTStatus(utils.StatusMap(phaseState))
			if ont.Status != newStatus {
				log.Printf("[Worker] ONT %s status changed: %s -> %s (phase state: %d)",
					ont.SerialNumber, ont.Status, newStatus, phaseState)
				if err := w.ontService.UpdateStatus(ont.ID, newStatus); err != nil {
					log.Printf("[Worker] Failed to update ONT %s: %v", ont.SerialNumber, err)
					continue
				}
			} else if newStatus == models.ONTStatusOnline {
				if err := w.ontService.UpdateStatus(ont.ID, newStatus); err != nil {
					log.Printf("[Worker] Failed to refresh ONT %s: %v", ont.SerialNumber, err)
					continue
				}
			}

			// Recorded on every poll, not only on a transition. LogStatusChange is
			// already idempotent - it opens a baseline event when an ONT has none,
			// returns without writing when the state is unchanged, and closes out
			// the previous event's duration on a real transition.
			//
			// Calling it only inside the "changed" branch starved it: an ONT whose
			// stored status already matched the OLT from the moment it was
			// registered never got a first event, so its Events tab stayed empty
			// and availability had no interval to measure. That is every ONT on a
			// freshly added OLT.
			eventType := models.EventTypeOffline
			if newStatus == models.ONTStatusOnline {
				eventType = models.EventTypeOnline
			}
			if err := w.eventService.LogStatusChange(ont.ID, eventType, string(newStatus)); err != nil {
				log.Printf("[Worker] Failed to log event for ONT %s: %v", ont.SerialNumber, err)
			}

			if err := w.ontService.UpdateUptimeMetrics(ont.ID); err != nil {
				log.Printf("[Worker] Failed to update uptime for ONT %s: %v", ont.SerialNumber, err)
			}

			successCount++
		}
	}

	duration := time.Since(start)
	log.Printf("[Worker] Poll completed: %d/%d ONTs successful (duration: %v)", successCount, totalONTs, duration)
}

// lookupByPortAndONT finds an entry for a given PON port and ONT id regardless of
// which line-card slot the OLT reports it on. TikMan stores port/ONT id but not
// the slot, and a PON port number is unique per OLT in practice.
func lookupByPortAndONT[T any](entries map[connectivity.ONTLocation]T, portID, ontID int) (T, bool) {
	for loc, value := range entries {
		if loc.Port == portID && loc.ONTID == ontID {
			return value, true
		}
	}
	var zero T
	return zero, false
}

// pollAllONTsMetrics discovers, registers, and collects metrics for every ONT the
// OLT reports. The SNMP walk finds all connected ONTs (including those not yet
// stored in TikMan), registers them, then stores their optical metrics.
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
		driver, err := connectivity.DriverFor(olt.Model)
		if err != nil {
			log.Printf("[Worker] Cannot collect metrics from OLT %s: %v", olt.Name, err)
			continue
		}

		// Full topology discovery returns serial numbers and metrics for every ONT
		// the device reports, keyed by physical location.
		topology, err := connectivity.DiscoverOLTTopology(driver, olt.IPAddress, olt.SNMPCommunity, olt.SNMPPort)
		if err != nil {
			log.Printf("[Worker] Failed to discover topology on OLT %s: %v", olt.Name, err)
			continue
		}

		// Flatten the slot/port/ONT hierarchy into a single list.
		discovered := make([]connectivity.DiscoveredONT, 0)
		for _, slot := range topology {
			for _, port := range slot.Ports {
				discovered = append(discovered, port.ONTs...)
			}
		}

		log.Printf("[Worker] OLT %s reports %d ONTs via SNMP walk", olt.Name, len(discovered))

		// Register any ONTs we haven't seen before, then re-list so every ONT the
		// OLT reports is present in the database and can receive metrics.
		regResult := w.ontService.BulkRegisterFromDiscovery(olt.ID, discovered)
		if regResult.Registered > 0 {
			log.Printf("[Worker] Auto-registered %d new ONTs for OLT %s", regResult.Registered, olt.Name)
		}
		if regResult.Skipped > 0 {
			log.Printf("[Worker] %d ONTs already registered for OLT %s", regResult.Skipped, olt.Name)
		}
		for _, e := range regResult.Errors {
			log.Printf("[Worker] Registration error: %s", e)
		}

		// Reload ONTs now that new ones have been registered.
		onts, _, err := w.ontService.List(&olt.ID, nil, 1000, 0)
		if err != nil {
			log.Printf("[Worker] Failed to list ONTs for metrics on OLT %s: %v", olt.Name, err)
			continue
		}

		allMetrics, err := driver.WalkMetrics(olt.IPAddress, olt.SNMPCommunity, olt.SNMPPort)
		if err != nil {
			log.Printf("[Worker] Failed to walk metrics on OLT %s: %v", olt.Name, err)
			continue
		}

		for _, ont := range onts {
			totalONTs++

			metrics, found := lookupByPortAndONT(allMetrics, ont.PortID, ont.ONTID)
			if !found {
				continue
			}

			if err := w.metricsService.StoreMetrics(ont.ID, &metrics, nil); err != nil {
				log.Printf("[Worker] Failed to store metrics for ONT %s (port %d/%d): %v",
					ont.SerialNumber, ont.PortID, ont.ONTID, err)
				continue
			}

			latestMetricFields := make(map[string]interface{})
			if metrics.RxPower != nil {
				latestMetricFields["rx_power"] = *metrics.RxPower
			}
			if metrics.TxPower != nil {
				latestMetricFields["tx_power"] = *metrics.TxPower
			}
			if metrics.Distance > 0 {
				latestMetricFields["distance"] = metrics.Distance
			}

			if len(latestMetricFields) > 0 {
				if _, err := w.ontService.Update(ont.ID, latestMetricFields); err != nil {
					log.Printf("[Worker] Failed to update ONT %s metrics fields: %v", ont.SerialNumber, err)
				}
			}

			log.Printf("[Worker] Metrics collected: serial=%s port=%d/%d rx_power=%s tx_power=%s distance=%dm",
				ont.SerialNumber, ont.PortID, ont.ONTID,
				formatPower(metrics.RxPower), formatPower(metrics.TxPower), metrics.Distance)
			successCount++
		}
	}

	duration := time.Since(start)
	log.Printf("[Worker] Metrics collection completed: %d/%d ONTs successful (duration: %v)",
		successCount, totalONTs, duration)
}

// collectONTMetrics collects and stores metrics for a single ONT
// Note: This function is deprecated - pollAllONTsMetrics now uses WalkONTMetrics instead
