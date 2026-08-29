package services

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
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
}

// NewWireGuardService constructs a WireGuardService backed by db and device,
// encrypting and decrypting private keys with encryptionKey.
func NewWireGuardService(db *gorm.DB, encryptionKey string, device connectivity.TunnelDevice) *WireGuardService {
	return &WireGuardService{db: db, encryptionKey: encryptionKey, device: device}
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

	key, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate server key: %w", err)
	}
	encrypted, err := utils.Encrypt(key.String(), s.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt server key: %w", err)
	}

	created := &models.WireGuardServer{
		InterfaceName: defaultInterfaceName,
		ListenPort:    defaultListenPort,
		PrivateKey:    encrypted,
		PublicKey:     key.PublicKey().String(),
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
// device so a listen port change takes effect immediately.
func (s *WireGuardService) UpdateServer(endpointHost string, listenPort int) (*models.WireGuardServer, error) {
	server, err := s.GetServer()
	if err != nil {
		return nil, err
	}

	server.EndpointHost = endpointHost
	server.ListenPort = listenPort
	if err := s.db.Save(server).Error; err != nil {
		return nil, fmt.Errorf("failed to update wireguard server: %w", err)
	}
	if err := s.Reconcile(); err != nil {
		return nil, err
	}
	return server, nil
}

// ListPeers returns every peer, ordered by name.
func (s *WireGuardService) ListPeers() ([]models.WireGuardPeer, error) {
	var peers []models.WireGuardPeer
	if err := s.db.Order("name").Find(&peers).Error; err != nil {
		return nil, fmt.Errorf("failed to list wireguard peers: %w", err)
	}
	return peers, nil
}

// GetPeer loads a single peer by ID.
func (s *WireGuardService) GetPeer(id uuid.UUID) (*models.WireGuardPeer, error) {
	var peer models.WireGuardPeer
	if err := s.db.First(&peer, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("peer not found: %w", err)
		}
		return nil, fmt.Errorf("database error: %w", err)
	}
	return &peer, nil
}

// SuggestAllowedIPsForSite proposes /24 subnets from the OLTs already
// registered for siteID, for the operator to confirm or edit.
func (s *WireGuardService) SuggestAllowedIPsForSite(siteID uuid.UUID) ([]string, error) {
	var addresses []string
	if err := s.db.Model(&models.OLT{}).Where("site_id = ?", siteID).Pluck("ip_address", &addresses).Error; err != nil {
		return nil, fmt.Errorf("failed to read OLT addresses: %w", err)
	}
	return SuggestAllowedIPs(addresses), nil
}

// CreatePeer registers a new site tunnel: it validates the subnet against
// every existing peer, allocates a tunnel address when tunnelAddress is
// empty, generates the peer's keypair, and reconciles the device. If the
// device rejects the resulting configuration, the peer row is removed so a
// later reconcile does not keep retrying it.
func (s *WireGuardService) CreatePeer(siteID uuid.UUID, name string, allowedIPs []string, tunnelAddress string) (*models.WireGuardPeer, error) {
	server, err := s.GetServer()
	if err != nil {
		return nil, err
	}

	peers, err := s.ListPeers()
	if err != nil {
		return nil, err
	}
	for _, existing := range peers {
		if existing.SiteID == siteID {
			return nil, fmt.Errorf("this site already has a tunnel")
		}
	}

	networks, err := s.peerNetworks(peers, uuid.Nil)
	if err != nil {
		return nil, err
	}
	if err := ValidateAllowedIPs(allowedIPs, networks, server.TunnelSubnet, DefaultReservedSubnets); err != nil {
		return nil, err
	}

	if tunnelAddress == "" {
		tunnelAddress, err = AllocateTunnelAddress(server.TunnelSubnet, server.Address, takenAddresses(peers))
		if err != nil {
			return nil, err
		}
	}
	if err := ValidateTunnelAddress(tunnelAddress, server.TunnelSubnet, server.Address, takenAddresses(peers)); err != nil {
		return nil, err
	}

	key, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate peer key: %w", err)
	}
	encrypted, err := utils.Encrypt(key.String(), s.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt peer key: %w", err)
	}

	peer := &models.WireGuardPeer{
		SiteID:              siteID,
		Name:                name,
		PublicKey:           key.PublicKey().String(),
		PrivateKey:          encrypted,
		TunnelAddress:       tunnelAddress,
		PersistentKeepalive: defaultKeepalive,
		Enabled:             true,
	}
	if err := peer.SetAllowedIPs(allowedIPs); err != nil {
		return nil, err
	}
	if err := s.db.Create(peer).Error; err != nil {
		return nil, fmt.Errorf("failed to create wireguard peer: %w", err)
	}

	// A peer the kernel refuses must not linger in the database, or the next
	// reconcile would keep trying to apply it.
	if err := s.Reconcile(); err != nil {
		s.db.Delete(&models.WireGuardPeer{}, "id = ?", peer.ID)
		return nil, err
	}
	return peer, nil
}

// UpdatePeer changes the fields that are non-nil and reconciles the device.
// Passing allowedIPs re-validates it against every other peer's subnets.
func (s *WireGuardService) UpdatePeer(id uuid.UUID, name *string, allowedIPs []string, enabled *bool) (*models.WireGuardPeer, error) {
	server, err := s.GetServer()
	if err != nil {
		return nil, err
	}
	peer, err := s.GetPeer(id)
	if err != nil {
		return nil, err
	}

	if allowedIPs != nil {
		peers, err := s.ListPeers()
		if err != nil {
			return nil, err
		}
		networks, err := s.peerNetworks(peers, id)
		if err != nil {
			return nil, err
		}
		if err := ValidateAllowedIPs(allowedIPs, networks, server.TunnelSubnet, DefaultReservedSubnets); err != nil {
			return nil, err
		}
		if err := peer.SetAllowedIPs(allowedIPs); err != nil {
			return nil, err
		}
	}
	if name != nil {
		peer.Name = *name
	}
	if enabled != nil {
		peer.Enabled = *enabled
	}

	if err := s.db.Save(peer).Error; err != nil {
		return nil, fmt.Errorf("failed to update wireguard peer: %w", err)
	}
	if err := s.Reconcile(); err != nil {
		return nil, err
	}
	return peer, nil
}

// DeletePeer removes a peer and reconciles the device so its key stops being
// accepted immediately.
func (s *WireGuardService) DeletePeer(id uuid.UUID) error {
	if err := s.db.Delete(&models.WireGuardPeer{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("failed to delete wireguard peer: %w", err)
	}
	return s.Reconcile()
}

// PeerConfig renders the config a site imports, in the given format
// (ConfigFormatWGQuick or ConfigFormatMikroTik). This is the only path that
// decrypts a peer's private key.
func (s *WireGuardService) PeerConfig(id uuid.UUID, format string) (string, error) {
	server, err := s.GetServer()
	if err != nil {
		return "", err
	}
	peer, err := s.GetPeer(id)
	if err != nil {
		return "", err
	}

	privateKey, err := utils.Decrypt(peer.PrivateKey, s.encryptionKey)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt peer key: %w", err)
	}
	allowedIPs, err := peer.AllowedIPsList()
	if err != nil {
		return "", err
	}

	input := PeerConfigInput{
		PeerPrivateKey:  privateKey,
		PeerAddress:     peer.TunnelAddress,
		ServerPublicKey: server.PublicKey,
		EndpointHost:    server.EndpointHost,
		TunnelSubnet:    server.TunnelSubnet,
		ListenPort:      server.ListenPort,
		Keepalive:       peer.PersistentKeepalive,
		AllowedIPs:      allowedIPs,
	}

	switch format {
	case ConfigFormatWGQuick:
		return RenderWGQuickConfig(input), nil
	case ConfigFormatMikroTik:
		return RenderMikroTikConfig(input), nil
	default:
		return "", fmt.Errorf("unsupported format %q", format)
	}
}

// Reconcile applies the whole database to the interface in one call. There is
// no incremental path, so kernel state cannot drift from what is stored. If
// the server has not been configured yet, it is a no-op.
func (s *WireGuardService) Reconcile() error {
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
		return fmt.Errorf("failed to apply wireguard configuration: %w", err)
	}
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

// peerNetworks converts peers into the PeerNetwork slice ValidateAllowedIPs
// expects, skipping the peer identified by exclude (used when validating an
// update against every *other* peer).
func (s *WireGuardService) peerNetworks(peers []models.WireGuardPeer, exclude uuid.UUID) ([]PeerNetwork, error) {
	networks := make([]PeerNetwork, 0, len(peers))
	for _, peer := range peers {
		if peer.ID == exclude {
			continue
		}
		allowedIPs, err := peer.AllowedIPsList()
		if err != nil {
			return nil, err
		}
		networks = append(networks, PeerNetwork{
			PeerID:     peer.ID,
			SiteName:   peer.Name,
			AllowedIPs: allowedIPs,
		})
	}
	return networks, nil
}

func takenAddresses(peers []models.WireGuardPeer) []string {
	addresses := make([]string, 0, len(peers))
	for _, peer := range peers {
		addresses = append(addresses, peer.TunnelAddress)
	}
	return addresses
}
