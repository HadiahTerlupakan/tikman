package api

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

// An ONU's address reads card/port:id. Leaving the card out of the response
// had the configure form fall back to card 1 for an ONU on card 3, which would
// have provisioned against the wrong line card.
func TestToONTResponseCarriesTheSlot(t *testing.T) {
	slot := 3
	ont := &models.ONT{ID: uuid.New(), OLTID: uuid.New(), Slot: &slot, PortID: 1, ONTID: 15}

	response := ToONTResponse(ont)

	require.NotNil(t, response.Slot)
	assert.Equal(t, 3, *response.Slot)
}

// An ONT the poll has not placed yet answers without one, rather than with a
// number the form would treat as real.
func TestToONTResponseLeavesAnUnknownSlotOut(t *testing.T) {
	ont := &models.ONT{ID: uuid.New(), OLTID: uuid.New(), PortID: 1, ONTID: 15}

	assert.Nil(t, ToONTResponse(ont).Slot)
}
