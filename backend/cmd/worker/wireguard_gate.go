package main

import (
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// oltsBehindDownTunnel names the OLTs that cannot be reached this cycle because
// their site's tunnel is down. Polling them anyway would mark every ONT in the
// site offline at once, which is the false alarm commit 9d8c9ee removed.
func oltsBehindDownTunnel(db *gorm.DB, now time.Time, logger *zap.Logger) map[uuid.UUID]bool {
	blocked := make(map[uuid.UUID]bool)

	var peers []models.WireGuardPeer
	if err := db.Where("enabled = ?", true).Find(&peers).Error; err != nil {
		logger.Error("Failed to read WireGuard peers", zap.Error(err))
		return blocked
	}

	downSites := make([]uuid.UUID, 0, len(peers))
	for _, peer := range peers {
		if !services.PeerConnected(peer.LastHandshakeAt, now) {
			downSites = append(downSites, peer.SiteID)
		}
	}
	if len(downSites) == 0 {
		return blocked
	}

	var olts []models.OLT
	if err := db.Where("site_id IN ?", downSites).Find(&olts).Error; err != nil {
		logger.Error("Failed to read OLTs behind down tunnels", zap.Error(err))
		return blocked
	}
	for _, olt := range olts {
		blocked[olt.ID] = true
		logger.Info("Skipping OLT this cycle: site tunnel is down", zap.String("olt", olt.Name))
	}
	return blocked
}
