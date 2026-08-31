package main

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"go.uber.org/zap"
)

func TestRecordKeepsTheCommunityTheTrapArrivedWith(t *testing.T) {
	db := setupApplyTestDB(t)
	store := &trapStore{db: db, logger: zap.NewNop()}
	oltID := uuid.New()

	store.record(Trap{
		OLTID:     oltID,
		Source:    "172.30.30.3",
		OID:       oidONUOffline,
		Community: commMajor,
	})

	var stored models.ONTTrapEvent
	require.NoError(t, db.Where("olt_id = ?", oltID).First(&stored).Error)

	// The severity that decides whether a subscriber is down travels in the
	// community and nowhere else. It was read at runtime and thrown away, so
	// the archive could not answer afterwards what level a trap had carried —
	// which is how a day was spent inferring severity from OIDs instead.
	assert.Equal(t, commMajor, stored.Community)
}
