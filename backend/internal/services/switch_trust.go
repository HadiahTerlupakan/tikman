package services

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
)

// TrustHostKey pins the fingerprint the switch last presented and clears the
// changed flag, so the next SSH session may log in. It trusts what the poller
// saw, so the admin is trusting that the address still reaches their switch.
func (s *SwitchService) TrustHostKey(id uuid.UUID) (*models.Switch, error) {
	sw, err := s.Get(id)
	if err != nil {
		return nil, err
	}
	if sw.Protocol != models.SwitchProtocolSSH {
		return nil, fmt.Errorf("%w: hanya SSH yang punya host key", ErrValidation)
	}
	if sw.PresentedFingerprint == nil || *sw.PresentedFingerprint == "" {
		return nil, fmt.Errorf("%w: belum ada fingerprint yang bisa dipercaya", ErrValidation)
	}
	err = s.db.Model(&models.Switch{}).Where("id = ?", id).Updates(map[string]interface{}{
		"host_key_fingerprint": *sw.PresentedFingerprint, "host_key_changed": false}).Error
	if err != nil {
		return nil, err
	}
	return s.Get(id)
}
