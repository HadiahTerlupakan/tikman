package services

import (
	"errors"
	"fmt"
	"time"

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

// RecordPhoneIfUnclaimed writes a number onto an ONT that has none yet, and
// leaves an ONT that already carries one untouched — that value was put there
// deliberately and may be the more correct one. It never fails a caller's
// larger operation over a phone number: a collision with another ONT's number
// is reported back as "not recorded", not as an error, and a genuine failure
// (the ONT does not exist, the database is unreachable) is still returned so
// the caller can decide whether to log it, but is never itself a defect in
// the number — the wrapping ErrValidation for a duplicate is the only case
// callers are expected to treat as "fine, just didn't record."
//
// This is the one place that fills the phone column outside manual ONT
// creation: a CS linking a conversation to an ONT by hand (see LinkONT in
// cs_handler_conversations.go) is exactly the moment the spec wants the
// number captured, rather than a separate data-entry project.
func (s *ONTService) RecordPhoneIfUnclaimed(ontID uuid.UUID, phone string) (bool, error) {
	ont, err := s.GetByID(ontID)
	if err != nil {
		return false, err
	}
	if ont.Phone != "" {
		return false, nil
	}

	resolved, err := s.resolvePhone(phone, ontID)
	if err != nil {
		if errors.Is(err, ErrValidation) {
			return false, nil
		}
		return false, err
	}
	if resolved == "" {
		return false, nil
	}

	updates := map[string]interface{}{"phone": resolved, "updated_at": time.Now()}
	if err := s.db.Model(&models.ONT{}).Where("id = ?", ontID).Updates(updates).Error; err != nil {
		return false, fmt.Errorf("failed to record ONT phone: %w", err)
	}
	return true, nil
}

// ReleasePhone clears an ONT's subscriber number, but only when it is the one
// given, and answers whether it did.
//
// It is the undo of RecordPhoneIfUnclaimed. A CS who links a thread to the
// wrong ONT writes the customer's number onto it, and unlinking without taking
// that number back would leave the next chat from that customer matching the
// same wrong ONT — the correction would look done and not be.
//
// A number the operator entered for somebody else is left alone. Answering
// false is an ordinary outcome, not a failure.
func (s *ONTService) ReleasePhone(ontID uuid.UUID, phone string) (bool, error) {
	ont, err := s.GetByID(ontID)
	if err != nil {
		return false, err
	}
	if ont.Phone == "" {
		return false, nil
	}

	// Compared in the stored form, not as typed: the number on the ONT was
	// normalized on the way in, and "0812…" must still match "62812…".
	wanted, err := utils.NormalizePhone(phone)
	if err != nil || wanted != ont.Phone {
		return false, nil
	}

	err = s.db.Model(&models.ONT{}).Where("id = ?", ontID).
		Updates(map[string]interface{}{"phone": "", "updated_at": time.Now()}).Error
	if err != nil {
		return false, fmt.Errorf("failed to release ONT phone: %w", err)
	}
	return true, nil
}
