package services

import (
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
)

// GetPollingStats returns current polling statistics
func (s *MetricsService) GetPollingStats() map[string]interface{} {
	stats := make(map[string]interface{})

	var totalONTs int64
	s.db.Model(&models.ONT{}).Count(&totalONTs)

	var ontsPolled int64
	tenMinutesAgo := time.Now().Add(-10 * time.Minute)
	s.db.Raw(`
		SELECT COUNT(DISTINCT om.ont_id)
		FROM ont_metrics om
		JOIN onts o ON om.ont_id = o.id
		WHERE om.time >= ?
	`, tenMinutesAgo).Scan(&ontsPolled)

	percentage := float64(0)
	if totalONTs > 0 {
		percentage = float64(ontsPolled) / float64(totalONTs) * 100
	}

	stats["total_onts"] = totalONTs
	stats["onts_with_metrics"] = ontsPolled
	stats["percentage"] = percentage
	stats["last_poll_time"] = time.Now()

	log.Printf("[Metrics Stats] Total ONTs=%d, Polled=%d (%.1f%%), Last Poll=%v",
		totalONTs, ontsPolled, percentage, stats["last_poll_time"])

	return stats
}

// GetOLTPollingStats returns polling statistics for a specific OLT
// recentMetricsWindow is how fresh a reading must be to count an ONT as
// polled. It sits above the ten-minute metrics cycle so a cycle that has just
// finished does not read as half the ONTs having gone quiet.
const recentMetricsWindow = 10 * time.Minute

func (s *MetricsService) GetOLTPollingStats(oltID uuid.UUID) map[string]interface{} {
	var totalONTs int64
	s.db.Model(&models.ONT{}).Where("olt_id = ?", oltID).Count(&totalONTs)

	var ontsPolled int64
	s.db.Raw(`
		SELECT COUNT(DISTINCT om.ont_id)
		FROM ont_metrics om
		JOIN onts o ON om.ont_id = o.id
		WHERE o.olt_id = ? AND om.time >= ?
	`, oltID, time.Now().Add(-recentMetricsWindow)).Scan(&ontsPolled)

	percentage := float64(0)
	if totalONTs > 0 {
		percentage = float64(ontsPolled) / float64(totalONTs) * 100
	}

	var olt models.OLT
	if err := s.db.First(&olt, "id = ?", oltID).Error; err != nil {
		olt = models.OLT{DiscoveryPhase: "idle"}
	}

	// While a discovery is running the bar shows its progress instead: how many
	// ONTs have metrics is not what the operator is waiting on.
	if (olt.DiscoveryPhase == "discovering" || olt.DiscoveryPhase == "polling") && olt.DiscoveryTotal > 0 {
		percentage = float64(olt.DiscoveryRegistered) / float64(olt.DiscoveryTotal) * 100
	}

	log.Printf("[OLT Stats] OLT=%s: Total ONTs=%d, Polled=%d (%.1f%%)",
		oltID.String(), totalONTs, ontsPolled, percentage)

	return map[string]interface{}{
		"total_onts":           totalONTs,
		"onts_with_metrics":    ontsPolled,
		"percentage":           percentage,
		"phase":                olt.DiscoveryPhase,
		"discovery_total":      int64(olt.DiscoveryTotal),
		"discovery_registered": int64(olt.DiscoveryRegistered),
		"discovery_polled":     int64(olt.DiscoveryPolled),
		"discovery_error":      olt.DiscoveryError,
		"last_poll_time":       time.Now(),
		"olt_id":               oltID.String(),
	}
}
