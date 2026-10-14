package services

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestOwnPrefixesRefuseARangeBeyondRetention(t *testing.T) {
	_, err := NewFlowQueryService(setupTestDB(t)).OwnPrefixes(uuid.New(),
		FlowFilter{From: time.Now().Add(-40 * 24 * time.Hour), To: time.Now()}, time.Now())

	assert.ErrorIs(t, err, ErrOwnPrefixRange)
}
