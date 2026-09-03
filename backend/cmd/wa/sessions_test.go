package main

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/tikman/olt-provisioning/internal/models"
)

// A session whose number was deleted has to be closed, or it goes on receiving
// for an inbox that has stopped showing it — writing messages against threads
// that are gone.
func TestOrphanedFindsTheSessionWhoseNumberWasDeleted(t *testing.T) {
	kept, deleted := uuid.New(), uuid.New()
	accounts := []models.WAAccount{{ID: kept, Label: "CS Utama"}}

	gone := orphaned([]uuid.UUID{kept, deleted}, accounts)
	assert.Equal(t, []uuid.UUID{deleted}, gone)
}

// The common case by far: every rescan runs this, and closing a live session
// because a query came back in a different order would be an outage.
func TestOrphanedClosesNothingWhenEveryNumberStillExists(t *testing.T) {
	first, second := uuid.New(), uuid.New()
	accounts := []models.WAAccount{{ID: second}, {ID: first}}

	assert.Empty(t, orphaned([]uuid.UUID{first, second}, accounts))
}

// A number added from the admin screen has a row before it has a session. It
// must not be mistaken for the opposite case and acted on.
func TestOrphanedIgnoresANumberThatHasNoSessionYet(t *testing.T) {
	running := uuid.New()
	accounts := []models.WAAccount{{ID: running}, {ID: uuid.New()}}

	assert.Empty(t, orphaned([]uuid.UUID{running}, accounts))
}
