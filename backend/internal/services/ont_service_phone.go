package services

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/utils"
)

// resolvePhone normalizes a subscriber's number into the 628… form the CS
// inbox's chat lookup compares against, and rejects one already claimed by a
// different ONT. An empty number is left as-is: most ONTs have none, and that
// is a legal, non-colliding state, not a value to validate.
//
// excludeID is the ONT being saved, so it does not collide with its own
// stored number on an update. Pass uuid.Nil when creating, since a new row
// cannot already hold the number under its own ID.
func (s *ONTService) resolvePhone(raw string, excludeID uuid.UUID) (string, error) {
	if raw == "" {
		return "", nil
	}

	phone, err := utils.NormalizePhone(raw)
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrValidation, err.Error())
	}

	if err := s.checkPhoneUnclaimed(phone, excludeID); err != nil {
		return "", err
	}

	return phone, nil
}

// checkPhoneUnclaimed rejects a number already stored on a different ONT. Two
// ONTs sharing one number would route a subscriber's WhatsApp chat to the
// wrong house. The database also enforces this with a partial unique index,
// but that index is Postgres-only and the SQLite-backed tests never see it.
func (s *ONTService) checkPhoneUnclaimed(phone string, excludeID uuid.UUID) error {
	query := s.db.Model(&models.ONT{}).Where("phone = ?", phone)
	if excludeID != uuid.Nil {
		query = query.Where("id != ?", excludeID)
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return fmt.Errorf("failed to check duplicate phone number: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("%w: nomor %s sudah terdaftar pada ONT lain", ErrValidation, phone)
	}
	return nil
}
