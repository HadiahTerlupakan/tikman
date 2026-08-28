package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var registerBatch = []string{
	"configure terminal",
	"interface gpon-olt_1/3/1",
	"onu 15 type ALL sn HWTCB403E8A0",
	"exit",
	"interface gpon-onu_1/3/1:15",
	"tcont 1 name bridge profile 1G",
}

func TestZteRegistrationIndex(t *testing.T) {
	assert.Equal(t, 2, zteRegistrationIndex(registerBatch))
}

// A service-only job sends no registration line, so there is no index to find
// and nothing about the ONU's existence to infer from one.
func TestZteRegistrationIndexReportsAbsence(t *testing.T) {
	assert.Equal(t, -1, zteRegistrationIndex([]string{
		"configure terminal",
		"interface gpon-onu_1/3/1:15",
		"tcont 1 name bridge profile 1G",
	}))
}

// The rule the rollback guard reads. A registration refused because the serial
// was already on the OLT would otherwise answer by sending "no onu 15",
// deleting a working subscriber an earlier job had put there.
func TestOnlyCommandsAfterTheRegistrationMeanTheONUIsOurs(t *testing.T) {
	registrationAt := zteRegistrationIndex(registerBatch)

	for _, failedAt := range []int{0, 1, 2} {
		assert.False(t, failedAt > registrationAt,
			"a failure at command %d created no ONU", failedAt)
	}
	for _, failedAt := range []int{3, 4, 5} {
		assert.True(t, failedAt > registrationAt,
			"a failure at command %d follows a registration this job made", failedAt)
	}
}
