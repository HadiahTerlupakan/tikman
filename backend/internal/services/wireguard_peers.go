package services

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/utils"
	"gorm.io/gorm"
)

// ErrPeerNotFound lets the HTTP layer answer 404 without reaching for GORM's
// error types or matching on message text.
var ErrPeerNotFound = errors.New("peer not found")

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
			return nil, fmt.Errorf("%w: %s", ErrPeerNotFound, id)
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

	if err := s.requireSite(siteID); err != nil {
		return nil, err
	}

	peers, err := s.ListPeers()
	if err != nil {
		return nil, err
	}
	for _, existing := range peers {
		if existing.SiteID == siteID {
			return nil, fmt.Errorf("%w: this site already has a tunnel", ErrValidation)
		}
	}

	tunnelAddress, err = s.resolveNewPeerNetwork(server, peers, allowedIPs, tunnelAddress)
	if err != nil {
		return nil, err
	}

	publicKey, encryptedPrivateKey, err := s.generateEncryptedKeypair()
	if err != nil {
		return nil, err
	}

	peer := &models.WireGuardPeer{
		SiteID:              siteID,
		Name:                name,
		PublicKey:           publicKey,
		PrivateKey:          encryptedPrivateKey,
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
		return nil, s.rollbackRejectedPeer(peer.ID, err)
	}
	return peer, nil
}

// requireSite refuses a peer for a site that does not exist. Without it any
// UUID mints a tunnel that holds a subnet and shows a name no site answers to.
func (s *WireGuardService) requireSite(siteID uuid.UUID) error {
	var count int64
	if err := s.db.Model(&models.Site{}).Where("id = ?", siteID).Count(&count).Error; err != nil {
		return fmt.Errorf("failed to look up site: %w", err)
	}
	if count == 0 {
		return fmt.Errorf("%w: site %s does not exist", ErrValidation, siteID)
	}
	return nil
}

// rollbackRejectedPeer removes a peer the device refused, so a later reconcile
// does not keep retrying it. A failure to clean up is joined onto the original
// error rather than hidden, because it means the invariant no longer holds.
func (s *WireGuardService) rollbackRejectedPeer(peerID uuid.UUID, cause error) error {
	if err := s.db.Delete(&models.WireGuardPeer{}, "id = ?", peerID).Error; err != nil {
		return errors.Join(cause, fmt.Errorf("failed to roll back rejected peer %s: %w", peerID, err))
	}
	return s.reconcileAfterRollback(cause)
}

// resolveNewPeerNetwork validates the requested subnets against every other
// peer and returns the tunnel address to use, allocating one when the caller
// did not choose it.
func (s *WireGuardService) resolveNewPeerNetwork(server *models.WireGuardServer, peers []models.WireGuardPeer, allowedIPs []string, tunnelAddress string) (string, error) {
	networks, err := s.peerNetworks(peers, uuid.Nil)
	if err != nil {
		return "", err
	}
	if err := ValidateAllowedIPs(allowedIPs, networks, server.TunnelSubnet, DefaultReservedSubnets); err != nil {
		return "", err
	}

	taken := takenAddresses(peers)
	if tunnelAddress == "" {
		tunnelAddress, err = AllocateTunnelAddress(server.TunnelSubnet, server.Address, taken)
		if err != nil {
			return "", err
		}
	}
	if err := ValidateTunnelAddress(tunnelAddress, server.TunnelSubnet, server.Address, taken); err != nil {
		return "", err
	}
	return tunnelAddress, nil
}

// UpdatePeer changes the fields that are non-nil and reconciles the device.
// Passing allowedIPs re-validates it against every other peer's subnets. If
// the device rejects the result, the edit is rolled back so a later reconcile
// does not keep failing on the same row.
func (s *WireGuardService) UpdatePeer(id uuid.UUID, name *string, allowedIPs []string, enabled *bool) (*models.WireGuardPeer, error) {
	server, err := s.GetServer()
	if err != nil {
		return nil, err
	}
	peer, err := s.GetPeer(id)
	if err != nil {
		return nil, err
	}
	original := *peer

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
		return nil, s.restorePeer(original, err)
	}
	return peer, nil
}

// restorePeer puts a rejected edit back the way it was, so a configuration the
// device refused cannot make every later reconcile fail on the same row.
func (s *WireGuardService) restorePeer(original models.WireGuardPeer, cause error) error {
	if err := s.db.Save(&original).Error; err != nil {
		return errors.Join(cause, fmt.Errorf("failed to restore peer %s after a rejected update: %w", original.ID, err))
	}
	return s.reconcileAfterRollback(cause)
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
		return "", fmt.Errorf("%w: unsupported format %q", ErrValidation, format)
	}
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
