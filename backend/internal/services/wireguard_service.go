package services

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/utils"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"gorm.io/gorm"
)

const (
	defaultInterfaceName = "wg0"
	defaultListenPort    = 51820
	defaultTunnelSubnet  = "10.88.0.0/24"
	defaultServerAddress = "10.88.0.1"
	defaultKeepalive     = 25

	// ConfigFormatWGQuick selects the wg-quick config format for Linux sites.
	ConfigFormatWGQuick = "wg-quick"
	// ConfigFormatMikroTik selects the RouterOS command format for MikroTik sites.
	ConfigFormatMikroTik = "mikrotik"
)

// ErrServerNotConfigured is returned before the operator has completed the
// one-time server setup.
var ErrServerNotConfigured = errors.New("wireguard server is not configured")

// WireGuardService owns the WireGuard server and its peers. The database is
// the sole source of truth: every mutation ends by reconciling the whole
// desired state onto the tunnel device rather than applying a delta.
type WireGuardService struct {
	db            *gorm.DB
	encryptionKey string
	device        connectivity.TunnelDevice
	// pingHost is a field so the reachability test can be exercised without a
	// live network; production always gets the real prober.
	pingHost func(address string, timeout time.Duration) error
	// localSubnets is a field for the same reason as pingHost: the reachability
	// of the host is not something a unit test can have.
	localSubnets func(excludeInterface string) []string

	// mu makes a reconcile a single read-then-apply step. Without it a
	// reconcile running alongside a mutation can apply a peer set read before
	// that mutation, reinstating a peer a concurrent delete had just removed.
	mu               sync.Mutex
	reconcilePending bool
}

// NewWireGuardService constructs a WireGuardService backed by db and device,
// encrypting and decrypting private keys with encryptionKey.
func NewWireGuardService(db *gorm.DB, encryptionKey string, device connectivity.TunnelDevice) *WireGuardService {
	return &WireGuardService{
		db:            db,
		encryptionKey: encryptionKey,
		device:        device,
		pingHost:      connectivity.PingTest,
		localSubnets:  connectivity.LocalSubnets,
	}
}

// GetServer loads the single server row, or ErrServerNotConfigured if the
// operator has not run EnsureServer yet.
func (s *WireGuardService) GetServer() (*models.WireGuardServer, error) {
	var server models.WireGuardServer
	if err := s.db.First(&server).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrServerNotConfigured
		}
		return nil, fmt.Errorf("failed to load wireguard server: %w", err)
	}
	return &server, nil
}

// EnsureServer creates the single server row on first use, generating the
// keypair itself so no private key ever arrives from user input. Later calls
// return the existing row unchanged.
func (s *WireGuardService) EnsureServer(endpointHost string) (*models.WireGuardServer, error) {
	server, err := s.GetServer()
	if err == nil {
		return server, nil
	}
	if !errors.Is(err, ErrServerNotConfigured) {
		return nil, err
	}

	publicKey, encryptedPrivateKey, err := s.generateEncryptedKeypair()
	if err != nil {
		return nil, err
	}

	created := &models.WireGuardServer{
		InterfaceName: defaultInterfaceName,
		ListenPort:    defaultListenPort,
		PrivateKey:    encryptedPrivateKey,
		PublicKey:     publicKey,
		EndpointHost:  endpointHost,
		TunnelSubnet:  defaultTunnelSubnet,
		Address:       defaultServerAddress,
	}
	if err := s.db.Create(created).Error; err != nil {
		return nil, fmt.Errorf("failed to create wireguard server: %w", err)
	}
	if err := s.Reconcile(); err != nil {
		return nil, err
	}
	return created, nil
}

// UpdateServer changes the operator-facing server settings and reconciles the
// device so a listen port change takes effect immediately. If the device
// rejects the result, the settings are rolled back so a later reconcile does
// not keep failing on the same row.
func (s *WireGuardService) UpdateServer(endpointHost string, listenPort int) (*models.WireGuardServer, error) {
	server, err := s.GetServer()
	if err != nil {
		return nil, err
	}
	original := *server

	if err := ValidateEndpointHost(endpointHost); err != nil {
		return nil, err
	}

	server.EndpointHost = endpointHost
	server.ListenPort = listenPort
	if err := s.db.Save(server).Error; err != nil {
		return nil, fmt.Errorf("failed to update wireguard server: %w", err)
	}
	if err := s.Reconcile(); err != nil {
		if restoreErr := s.db.Save(&original).Error; restoreErr != nil {
			return nil, errors.Join(err, fmt.Errorf("failed to restore server settings after a rejected update: %w", restoreErr))
		}
		return nil, s.reconcileAfterRollback(err)
	}
	return server, nil
}

// reconcileAfterRollback puts the device back in step with the restored
// database. Apply is a sequence of kernel steps and is not atomic, so a failure
// part-way through can leave the interface holding a configuration the database
// no longer describes. The original refusal stays the cause: a failed recovery
// is joined onto it, never substituted for it.
func (s *WireGuardService) reconcileAfterRollback(cause error) error {
	if err := s.Reconcile(); err != nil {
		return errors.Join(cause, fmt.Errorf("failed to reconcile the device after rolling back: %w", err))
	}
	return cause
}

// Reconcile applies the whole database to the interface in one call. There is
// no incremental path, so kernel state cannot drift from what is stored. If
// the server has not been configured yet, it is a no-op.
func (s *WireGuardService) Reconcile() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reconcileLocked()
}

// ReconcileIfPending re-applies only after an earlier Apply failed. Applying on
// a schedule would be worse than the drift it repairs: Apply replaces the
// device's peers, which discards every site's learned endpoint, and a site
// behind NAT is the only side that can establish a new session.
func (s *WireGuardService) ReconcileIfPending() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.reconcilePending {
		return nil
	}
	return s.reconcileLocked()
}

func (s *WireGuardService) reconcileLocked() error {
	server, err := s.GetServer()
	if err != nil {
		if errors.Is(err, ErrServerNotConfigured) {
			return nil
		}
		return err
	}

	privateKey, err := utils.Decrypt(server.PrivateKey, s.encryptionKey)
	if err != nil {
		return fmt.Errorf("failed to decrypt server key: %w", err)
	}
	peers, err := s.ListPeers()
	if err != nil {
		return err
	}

	cfg := connectivity.TunnelConfig{
		InterfaceName: server.InterfaceName,
		PrivateKey:    privateKey,
		Address:       addressWithSubnetPrefix(server.Address, server.TunnelSubnet),
		ListenPort:    server.ListenPort,
	}
	for _, peer := range peers {
		if !peer.Enabled {
			continue
		}
		allowedIPs, err := peer.AllowedIPsList()
		if err != nil {
			return err
		}
		cfg.Peers = append(cfg.Peers, connectivity.TunnelPeerConfig{
			PublicKey:  peer.PublicKey,
			AllowedIPs: allowedIPs,
			Keepalive:  time.Duration(peer.PersistentKeepalive) * time.Second,
		})
	}

	if err := s.device.Apply(cfg); err != nil {
		// Only a refused Apply can leave the kernel holding what the database
		// no longer describes, so that is the one case worth retrying later.
		s.reconcilePending = true
		return fmt.Errorf("failed to apply wireguard configuration: %w", err)
	}
	s.reconcilePending = false
	return nil
}

// RefreshStatus reads live peer state from the device and stores the last
// handshake time and traffic counters against the matching peer row. If the
// server has not been configured yet, it is a no-op.
func (s *WireGuardService) RefreshStatus() error {
	server, err := s.GetServer()
	if err != nil {
		if errors.Is(err, ErrServerNotConfigured) {
			return nil
		}
		return err
	}

	statuses, err := s.device.Status(server.InterfaceName)
	if err != nil {
		return fmt.Errorf("failed to read wireguard status: %w", err)
	}

	for _, status := range statuses {
		updates := map[string]interface{}{
			"last_handshake_at": status.LastHandshakeAt,
			"rx_bytes":          status.RxBytes,
			"tx_bytes":          status.TxBytes,
		}
		if err := s.db.Model(&models.WireGuardPeer{}).
			Where("public_key = ?", status.PublicKey).
			Updates(updates).Error; err != nil {
			return fmt.Errorf("failed to store peer status: %w", err)
		}
	}
	return nil
}

// generateEncryptedKeypair returns a fresh public key and the matching private
// key encrypted at rest, so a private key never exists in plaintext outside
// the moment a config is rendered.
func (s *WireGuardService) generateEncryptedKeypair() (publicKey, encryptedPrivateKey string, err error) {
	key, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return "", "", fmt.Errorf("failed to generate key: %w", err)
	}
	encrypted, err := utils.Encrypt(key.String(), s.encryptionKey)
	if err != nil {
		return "", "", fmt.Errorf("failed to encrypt key: %w", err)
	}
	return key.PublicKey().String(), encrypted, nil
}
