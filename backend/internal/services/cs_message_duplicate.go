package services

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/tikman/olt-provisioning/internal/models"
)

// pgUniqueViolation is the SQLSTATE Postgres raises for a refused duplicate.
const pgUniqueViolation = "23505"

// waMessageIDConstraint is the partial unique index from migration 41.
const waMessageIDConstraint = "uq_cs_messages_wa_id"

// errStoredByAnother is saveArrived losing the race its lookup cannot win: a
// concurrent write committed the same WhatsApp message between the lookup and
// the insert. It never leaves the service — the caller is told the same thing
// the lookup would have told it, that the message was already stored.
var errStoredByAnother = errors.New("whatsapp message stored by another write")

// storedByAnother tells that race from any other failed insert. The constraint
// is named, not just the SQLSTATE: a duplicate primary key is a bug, and
// reading it as "already stored" would drop a message and say nothing.
func storedByAnother(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) &&
		pgErr.Code == pgUniqueViolation &&
		pgErr.ConstraintName == waMessageIDConstraint
}

// messageByWAID reads the message the index refused a second copy of. It has
// to be read outside the transaction that tried to write it, because that one
// is aborted by the violation.
func (s *CSMessageService) messageByWAID(waMessageID string) (*models.CSMessage, error) {
	var stored models.CSMessage
	if err := s.db.Where("wa_message_id = ?", waMessageID).First(&stored).Error; err != nil {
		return nil, fmt.Errorf("look for the message another write stored: %w", err)
	}
	return &stored, nil
}
