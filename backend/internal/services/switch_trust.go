package services

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
)

// ErrHostKeyNotPresented means the fingerprint an admin confirmed is not the
// one the switch last presented: the poller saw another key after the page
// loaded, and that key is one nobody has looked at.
var ErrHostKeyNotPresented = errors.New("fingerprint is not the one the switch presents")

// sshFingerprint is what ssh-keygen -l prints for a key: SHA256: and the
// unpadded base64 of 32 bytes.
var sshFingerprint = regexp.MustCompile(`^SHA256:[A-Za-z0-9+/]{43}$`)

func validateFingerprint(fingerprint string) error {
	if !sshFingerprint.MatchString(fingerprint) {
		return fmt.Errorf("%w: fingerprint must be SHA256: followed by 43 base64 characters", ErrValidation)
	}
	return nil
}

// TrustHostKey pins fingerprint, the key the admin was shown, and clears the
// changed flag, so the next SSH session may log in. The pin is made only if
// the switch still presents that key; the admin is trusting that the address
// still reaches their switch.
func (s *SwitchService) TrustHostKey(id uuid.UUID, fingerprint string) (*models.Switch, error) {
	sw, err := s.Get(id)
	if err != nil {
		return nil, err
	}
	if sw.Protocol != models.SwitchProtocolSSH {
		return nil, fmt.Errorf("%w: hanya SSH yang punya host key", ErrValidation)
	}
	if err := validateFingerprint(fingerprint); err != nil {
		return nil, err
	}
	result := s.db.Model(&models.Switch{}).Where("id = ? AND presented_fingerprint = ?", id, fingerprint).
		Updates(map[string]interface{}{"host_key_fingerprint": fingerprint, "host_key_changed": false})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, ErrHostKeyNotPresented
	}
	return s.Get(id)
}
