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
func (s *MetricsService) GetOLTPollingStats(oltID uuid.UUID) map[string]interface{} {
	stats := make(map[string]interface{})

	// Count total ONTs for this OLT
	var totalONTs int64
	s.db.Model(&models.ONT{}).Where("olt_id = ?", oltID).Count(&totalONTs)

	var ontsPolled int64
	tenMinutesAgo := time.Now().Add(-10 * time.Minute)
	s.db.Raw(`
		SELECT COUNT(DISTINCT om.ont_id)
		FROM ont_metrics om
		JOIN onts o ON om.ont_id = o.id
		WHERE o.olt_id = ? AND om.time >= ?
	`, oltID, tenMinutesAgo).Scan(&ontsPolled)

	percentage := float64(0)
	if totalONTs > 0 {
		percentage = float64(ontsPolled) / float64(totalONTs) * 100
	}

	stats["total_onts"] = totalONTs
	stats["onts_with_metrics"] = ontsPolled
	stats["percentage"] = percentage
	stats["last_poll_time"] = time.Now()
	stats["olt_id"] = oltID.String()

	log.Printf("[OLT Stats] OLT=%s: Total ONTs=%d, Polled=%d (%.1f%%)",
		oltID.String(), totalONTs, ontsPolled, percentage)

	return stats
}
