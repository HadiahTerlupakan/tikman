package services

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/utils"
	"gorm.io/gorm"
)

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
		s.db.Delete(&models.WireGuardPeer{}, "id = ?", peer.ID)
		return nil, err
	}
	return peer, nil
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
