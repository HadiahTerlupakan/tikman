package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tikman/olt-provisioning/internal/connectivity"
)

// A C300 answers the registration line with a bare ".[Failed]" when it is given
// a specific ONU type for an ONU it cannot see. Observed directly: HG8245H5 was
// refused for an offline ONU, and ALL for the same serial on the same port was
// accepted seconds later. The reply alone leaves an operator with nothing.
func TestFailedZTECommandExplainsARejectedONUType(t *testing.T) {
	err := failedZTECommand(
		[]string{"configure terminal", "interface gpon-olt_1/3/1", "onu 15 type HG8245H5 sn HWTCB403E8A0"},
		2,
		&connectivity.CommandResult{Error: ".[Failed]"},
	)

	assert.Contains(t, err.Error(), "onu 15 type HG8245H5 sn HWTCB403E8A0")
	assert.Contains(t, err.Error(), "type ALL")
}

// ALL is the fallback the hint recommends, so suggesting it when it was already
// used would be noise.
func TestFailedZTECommandDoesNotSuggestTypeALLWhenItWasUsed(t *testing.T) {
	err := failedZTECommand(
		[]string{"onu 15 type ALL sn HWTCB403E8A0"},
		0,
		&connectivity.CommandResult{Error: ".[Failed]"},
	)

	assert.NotContains(t, err.Error(), "register ALL as type ALL")
	assert.NotContains(t, err.Error(), "cannot see")
}

// Every other command keeps the OLT's reply on its own, with nothing added.
func TestFailedZTECommandLeavesOtherFailuresAlone(t *testing.T) {
	err := failedZTECommand(
		[]string{"service-port 1 vport 1 user-vlan 214 vlan 214"},
		0,
		&connectivity.CommandResult{Error: "%Error 20200: Invalid input"},
	)

	assert.Contains(t, err.Error(), "%Error 20200")
	assert.NotContains(t, err.Error(), "type ALL")
}
