package services

import (
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
)

// defaultONTBatch is how many ONTs one page carries when the caller names no
// size. Large enough that a populated chassis costs few round trips to the
// database, small enough that a page stays cheap to hold in memory.
const defaultONTBatch = 1000

// EachONTOfOLT hands every ONT of one OLT to fn, a page at a time.
//
// The worker used to read ONTs with a single List call capped at 1000 rows and
// treat what came back as the whole network. On this installation that happened
// to be enough; on a populated chassis it silently stopped monitoring
// everything past the first page, and no count anywhere disagreed.
//
// Paging walks by id rather than by offset. An OFFSET moves relative to a
// result set that discovery is inserting into at the same time, which makes a
// page skip or repeat rows; a cursor on an ordered key does not.
func (s *ONTService) EachONTOfOLT(oltID uuid.UUID, batch int, fn func([]models.ONT) error) error {
	if batch < 1 {
		batch = defaultONTBatch
	}

	var cursor uuid.UUID
	for {
		var rows []models.ONT
		query := s.db.Where("olt_id = ?", oltID)
		if cursor != uuid.Nil {
			query = query.Where("id > ?", cursor)
		}
		if err := query.Order("id").Limit(batch).Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		if err := fn(rows); err != nil {
			return err
		}
		cursor = rows[len(rows)-1].ID
	}
}
