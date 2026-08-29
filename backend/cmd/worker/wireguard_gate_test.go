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
	peer := models.WireGuardPeer{
		SiteID:          siteID,
		Name:            "Site A",
		PublicKey:       "pub-" + siteID.String(),
		PrivateKey:      "enc",
		TunnelAddress:   "10.88.0.2",
		Enabled:         enabled,
		LastHandshakeAt: handshake,
	}
	require.NoError(t, peer.SetAllowedIPs([]string{"10.10.10.0/24"}))
	require.NoError(t, db.Create(&peer).Error)

	// Enabled carries a gorm "default:true" tag, so Create silently drops the
	// column and lets the DB default win whenever the Go zero value (false) is
	// passed in. A follow-up column update writes the exact value instead.
	if !enabled {
		require.NoError(t, db.Model(&peer).Update("enabled", false).Error)
	}
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
