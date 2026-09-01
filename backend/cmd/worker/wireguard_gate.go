package main

import (
	"net"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// oltsBehindDownTunnel names the OLTs that cannot be reached this cycle because
// the tunnel they sit behind is down. Polling them anyway would mark every ONT
// in the site offline at once, which is the false alarm commit 9d8c9ee removed.
//
// A site can terminate more than one tunnel: Cariu is one site with two POPs,
// each behind its own router. So the peers that speak for an OLT are the ones
// whose allowed_ips carry its address, and a POP going down says nothing about
// the OLTs reached through the other one.
func oltsBehindDownTunnel(db *gorm.DB, now time.Time, logger *zap.Logger) map[uuid.UUID]bool {
	blocked := make(map[uuid.UUID]bool)

	var peers []models.WireGuardPeer
	if err := db.Where("enabled = ?", true).Find(&peers).Error; err != nil {
		logger.Error("Failed to read WireGuard peers", zap.Error(err))
		return blocked
	}
	if len(peers) == 0 {
		return blocked
	}
	bySite, siteIDs := groupPeersBySite(peers)

	var olts []models.OLT
	if err := db.Where("site_id IN ?", siteIDs).Find(&olts).Error; err != nil {
		logger.Error("Failed to read OLTs behind tunnels", zap.Error(err))
		return blocked
	}
	for _, olt := range olts {
		if allPeersDown(peersReaching(olt, bySite[olt.SiteID]), now) {
			blocked[olt.ID] = true
			logger.Info("Skipping OLT this cycle: its tunnel is down", zap.String("olt", olt.Name))
		}
	}
	return blocked
}

func groupPeersBySite(peers []models.WireGuardPeer) (map[uuid.UUID][]models.WireGuardPeer, []uuid.UUID) {
	bySite := make(map[uuid.UUID][]models.WireGuardPeer, len(peers))
	siteIDs := make([]uuid.UUID, 0, len(peers))
	for _, peer := range peers {
		if _, seen := bySite[peer.SiteID]; !seen {
			siteIDs = append(siteIDs, peer.SiteID)
		}
		bySite[peer.SiteID] = append(bySite[peer.SiteID], peer)
	}
	return bySite, siteIDs
}

// peersReaching narrows a site's tunnels to the ones carrying this OLT's
// address. When allowed_ips names none of them, every tunnel of the site
// answers for it: an OLT registered after the tunnel is still reached through
// it, and matching on subnets alone would quietly stop gating it.
func peersReaching(olt models.OLT, sitePeers []models.WireGuardPeer) []models.WireGuardPeer {
	address := net.ParseIP(olt.IPAddress)
	if address == nil {
		return sitePeers
	}
	covering := make([]models.WireGuardPeer, 0, len(sitePeers))
	for _, peer := range sitePeers {
		if peerCovers(peer, address) {
			covering = append(covering, peer)
		}
	}
	if len(covering) == 0 {
		return sitePeers
	}
	return covering
}

func peerCovers(peer models.WireGuardPeer, address net.IP) bool {
	subnets, err := peer.AllowedIPsList()
	if err != nil {
		return false
	}
	for _, subnet := range subnets {
		if _, network, err := net.ParseCIDR(subnet); err == nil && network.Contains(address) {
			return true
		}
	}
	return false
}

func allPeersDown(peers []models.WireGuardPeer, now time.Time) bool {
	for _, peer := range peers {
		if services.PeerConnected(peer.LastHandshakeAt, now) {
			return false
		}
	}
	return len(peers) > 0
}
