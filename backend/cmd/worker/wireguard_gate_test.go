package main

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newGateTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, models.AutoMigrate(db))
	return db
}

func createGateSiteWithOLT(t *testing.T, db *gorm.DB) (models.Site, models.OLT) {
	site := models.Site{Name: "Site A"}
	require.NoError(t, db.Create(&site).Error)

	olt := models.OLT{
		SiteID: site.ID, Name: "OLT 1", IPAddress: "10.10.10.5",
		Username: "admin", Password: "enc", Model: models.OLTModelZTEC300,
	}
	require.NoError(t, db.Create(&olt).Error)
	return site, olt
}

func createGatePeer(t *testing.T, db *gorm.DB, siteID uuid.UUID, handshake *time.Time, enabled bool) {
	createGatePop(t, db, siteID, "10.88.0.2", "10.10.10.0/24", handshake, enabled)
}

func createGatePop(t *testing.T, db *gorm.DB, siteID uuid.UUID, tunnelAddress, allowedIP string, handshake *time.Time, enabled bool) {
	peer := models.WireGuardPeer{
		SiteID:          siteID,
		Name:            "POP " + tunnelAddress,
		PublicKey:       "pub-" + tunnelAddress,
		PrivateKey:      "enc",
		TunnelAddress:   tunnelAddress,
		Enabled:         enabled,
		LastHandshakeAt: handshake,
	}
	require.NoError(t, peer.SetAllowedIPs([]string{allowedIP}))
	require.NoError(t, db.Create(&peer).Error)
}

func createGateOLT(t *testing.T, db *gorm.DB, siteID uuid.UUID, name, address string) models.OLT {
	olt := models.OLT{
		SiteID: siteID, Name: name, IPAddress: address,
		Username: "admin", Password: "enc", Model: models.OLTModelZTEC300,
	}
	require.NoError(t, db.Create(&olt).Error)
	return olt
}

func TestOLTsBehindDownTunnelBlocksStalePeer(t *testing.T) {
	db := newGateTestDB(t)
	now := time.Now()
	site, olt := createGateSiteWithOLT(t, db)

	stale := now.Add(-30 * time.Minute)
	createGatePeer(t, db, site.ID, &stale, true)

	blocked := oltsBehindDownTunnel(db, now, zap.NewNop())
	require.True(t, blocked[olt.ID])
}

func TestOLTsBehindDownTunnelAllowsLivePeer(t *testing.T) {
	db := newGateTestDB(t)
	now := time.Now()
	site, olt := createGateSiteWithOLT(t, db)

	recent := now.Add(-20 * time.Second)
	createGatePeer(t, db, site.ID, &recent, true)

	blocked := oltsBehindDownTunnel(db, now, zap.NewNop())
	require.False(t, blocked[olt.ID])
}

func TestOLTsBehindDownTunnelIgnoresSitesWithoutPeer(t *testing.T) {
	db := newGateTestDB(t)
	now := time.Now()
	_, olt := createGateSiteWithOLT(t, db)

	blocked := oltsBehindDownTunnel(db, now, zap.NewNop())
	require.False(t, blocked[olt.ID],
		"a site reachable without a tunnel must keep being polled")
}

func TestOLTsBehindDownTunnelIgnoresDisabledPeer(t *testing.T) {
	db := newGateTestDB(t)
	now := time.Now()
	site, olt := createGateSiteWithOLT(t, db)

	stale := now.Add(-30 * time.Minute)
	createGatePeer(t, db, site.ID, &stale, false)

	blocked := oltsBehindDownTunnel(db, now, zap.NewNop())
	require.False(t, blocked[olt.ID],
		"a disabled peer means the operator turned the tunnel off, not that the site is unreachable")
}

func TestOLTsBehindDownTunnelKeepsPollingThePopThatIsUp(t *testing.T) {
	db := newGateTestDB(t)
	now := time.Now()
	site, _ := createGateSiteWithOLT(t, db)
	behindDown := createGateOLT(t, db, site.ID, "OLT POP 1", "10.10.10.9")
	behindUp := createGateOLT(t, db, site.ID, "OLT POP 2", "10.20.20.9")

	stale, recent := now.Add(-30*time.Minute), now.Add(-20*time.Second)
	createGatePop(t, db, site.ID, "10.88.0.2", "10.10.10.0/24", &stale, true)
	createGatePop(t, db, site.ID, "10.88.0.3", "10.20.20.0/24", &recent, true)

	blocked := oltsBehindDownTunnel(db, now, zap.NewNop())

	require.True(t, blocked[behindDown.ID])
	// One site, two POPs: a router down at one of them says nothing about the
	// OLTs reached through the other, and pausing those invents an outage.
	require.False(t, blocked[behindUp.ID],
		"an OLT behind a healthy POP must keep being polled when another POP is down")
}

func TestOLTsBehindDownTunnelFallsBackToTheSiteWhenNoPeerCoversTheOLT(t *testing.T) {
	db := newGateTestDB(t)
	now := time.Now()
	site, _ := createGateSiteWithOLT(t, db)
	uncovered := createGateOLT(t, db, site.ID, "OLT baru", "172.16.4.9")

	stale := now.Add(-30 * time.Minute)
	createGatePop(t, db, site.ID, "10.88.0.2", "10.10.10.0/24", &stale, true)

	blocked := oltsBehindDownTunnel(db, now, zap.NewNop())

	// An OLT registered after the tunnel, whose address nobody added to
	// allowed_ips, is still reached through that tunnel. Matching on subnets
	// alone would quietly stop gating it and bring back the mass false alarm.
	require.True(t, blocked[uncovered.ID])
}
