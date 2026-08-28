package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tikman/olt-provisioning/internal/models"
)

// The registration lives under the PON interface as "onu N type X sn Y", so
// the removal has to be issued in that context. Sending "no onu N" from the
// top level is rejected.
func TestBuildZTEONURemovalCommands(t *testing.T) {
	assert.Equal(t, []string{
		"configure terminal",
		"interface gpon-olt_1/3/1",
		"no onu 15",
		"exit",
	}, BuildZTEONURemovalCommands(3, 1, 15))
}

// The command must address the card the ONT is on. Two ONUs can share a PON
// number on different cards, and the wrong card deletes another subscriber.
func TestBuildZTEONURemovalCommandsAddressesTheCard(t *testing.T) {
	onCardFour := BuildZTEONURemovalCommands(4, 1, 15)

	assert.Equal(t, "interface gpon-olt_1/4/1", onCardFour[1])
	assert.NotEqual(t, BuildZTEONURemovalCommands(3, 1, 15)[1], onCardFour[1])
}

// An ONT the poll has not placed on a card has no address on the OLT. Guessing
// one would delete a different subscriber's ONU, so the removal refuses.
func TestOntCardRefusesAnUnplacedONT(t *testing.T) {
	_, err := ontCard(&models.ONT{SerialNumber: "HWTCB403E8A0"})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no card recorded")
}

func TestOntCardReturnsTheRecordedCard(t *testing.T) {
	slot := 3
	card, err := ontCard(&models.ONT{SerialNumber: "HWTCB403E8A0", Slot: &slot})

	assert.NoError(t, err)
	assert.Equal(t, 3, card)
}

// From the exec prompt a C300 rejects "interface gpon-olt_1/3/1" outright, which
// is how the first removal attempt failed. Config mode has to be entered first,
// exactly as the registration builder does it.
func TestBuildZTEONURemovalCommandsEntersConfigMode(t *testing.T) {
	assert.Equal(t, "configure terminal", BuildZTEONURemovalCommands(3, 1, 15)[0])
}
