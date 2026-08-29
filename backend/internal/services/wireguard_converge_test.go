package services

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCreatePeerReconcilesAfterRollingBackRejectedPeer(t *testing.T) {
	service, device, db := newWireGuardService(t)
	_, err := service.EnsureServer("vpn.contoh.id")
	require.NoError(t, err)
	first := createTestSite(t, db, "Site A")
	second := createTestSite(t, db, "Site B")

	_, err = service.CreatePeer(first.ID, "Site A", []string{"10.10.10.0/24"}, "")
	require.NoError(t, err)

	device.Applied = connectivity.TunnelConfig{}
	device.ApplyErr = errTunnelApplyForTest
	device.ApplyErrOnce = true

	_, err = service.CreatePeer(second.ID, "Site B", []string{"10.20.20.0/24"}, "")
	require.Error(t, err)

	require.Equal(t, "wg0", device.Applied.InterfaceName,
		"deleting the rejected row is not enough: the kernel may already hold it, so the rollback must reconcile")
	require.Len(t, device.Applied.Peers, 1,
		"the reconcile after the rollback must reinstate exactly the peers the database still holds")
}

func TestUpdatePeerReconcilesAfterRestoringRejectedEdit(t *testing.T) {
	service, device, db := newWireGuardService(t)
	_, err := service.EnsureServer("vpn.contoh.id")
	require.NoError(t, err)
	site := createTestSite(t, db, "Site A")

	peer, err := service.CreatePeer(site.ID, "Site A", []string{"10.10.10.0/24"}, "")
	require.NoError(t, err)

	device.Applied = connectivity.TunnelConfig{}
	device.ApplyErr = errTunnelApplyForTest
	device.ApplyErrOnce = true

	_, err = service.UpdatePeer(peer.ID, nil, []string{"10.20.20.0/24"}, nil)
	require.Error(t, err)

	require.Len(t, device.Applied.Peers, 1)
	require.Equal(t, []string{"10.10.10.0/24"}, device.Applied.Peers[0].AllowedIPs,
		"the kernel must be put back onto the restored row, not left holding the rejected subnet")
}

func TestUpdateServerReconcilesAfterRestoringRejectedEdit(t *testing.T) {
	service, device, _ := newWireGuardService(t)
	_, err := service.EnsureServer("vpn.contoh.id")
	require.NoError(t, err)

	device.Applied = connectivity.TunnelConfig{}
	device.ApplyErr = errTunnelApplyForTest
	device.ApplyErrOnce = true

	_, err = service.UpdateServer("vpn.lain.id", 51821)
	require.Error(t, err)

	require.Equal(t, 51820, device.Applied.ListenPort,
		"the kernel must be put back onto the restored port, not left listening on the rejected one")
}

func TestRecoveryReconcileFailureDoesNotMaskTheOriginalError(t *testing.T) {
	service, device, db := newWireGuardService(t)
	_, err := service.EnsureServer("vpn.contoh.id")
	require.NoError(t, err)
	site := createTestSite(t, db, "Site A")

	device.ApplyErr = errTunnelApplyForTest

	_, err = service.CreatePeer(site.ID, "Site A", []string{"10.10.10.0/24"}, "")
	require.ErrorIs(t, err, errTunnelApplyForTest,
		"the operator must still see why the configuration was refused")
	require.Contains(t, err.Error(), "failed to reconcile the device after rolling back")
}

// tickDevice counts Apply calls. RunStatusRefresher drives it from its own
// goroutine while the test reads the count, so the counter is guarded.
type tickDevice struct {
	mu      sync.Mutex
	applies int
}

func (d *tickDevice) Apply(connectivity.TunnelConfig) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.applies++
	return nil
}

func (d *tickDevice) Status(string) ([]connectivity.TunnelPeerStatus, error) {
	return nil, nil
}

func (d *tickDevice) applyCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.applies
}

func TestStatusRefresherAlsoReconciles(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, models.AutoMigrate(db))

	device := &tickDevice{}
	service := NewWireGuardService(db, wgTestEncryptionKey, device)
	_, err = service.EnsureServer("vpn.contoh.id")
	require.NoError(t, err)
	applied := device.applyCount()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go service.RunStatusRefresher(ctx, time.Millisecond, zap.NewNop())

	require.Eventually(t, func() bool { return device.applyCount() > applied },
		5*time.Second, 5*time.Millisecond,
		"a peer removed while Apply was failing stays in the kernel until something reconciles again")
}
