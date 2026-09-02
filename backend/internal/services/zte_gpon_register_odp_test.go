package services

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// odpForRegistration is a box hanging straight off the OLT under test, which is
// the shape a newly registered ONU lands in.
func odpForRegistration(t *testing.T, db *gorm.DB, oltID uuid.UUID, ports int) uuid.UUID {
	t.Helper()
	slot, port := 1, 1
	odp, err := NewDistributionService(db).CreateODP(ODPInput{
		Code: "ODP-" + uuid.NewString()[:8], PortCount: ports,
		OLTID: &oltID, Slot: &slot, PortID: &port,
	})
	require.NoError(t, err)
	return odp.ID
}

func TestRegisterLandsTheDropOnTheChosenODPPort(t *testing.T) {
	commander := &zteIntegrationCommander{}
	svc, olt := newZTEGPONIntegrationService(t, commander,
		&zteIntegrationSnapshotFake{after: &ConfigSnapshot{ZTE: &ZTESnapshot{SerialNumber: "HWTCB403E8A0"}}},
		&zteIntegrationRollbackFake{})
	odpID := odpForRegistration(t, svc.db, olt.ID, 8)
	port := 5

	req := validZTERequest(olt.ID)
	req.ODPID, req.ODPPort = &odpID, &port
	_, err := svc.RegisterAndConfigure(context.Background(), req, uuid.New())
	require.NoError(t, err)

	subscribers, err := NewDistributionService(svc.db).SubscribersOn(odpID)
	require.NoError(t, err)
	require.Len(t, subscribers, 1)
	assert.Equal(t, "HWTCB403E8A0", subscribers[0].SerialNumber)
	require.NotNil(t, subscribers[0].ODPPort)
	assert.Equal(t, 5, *subscribers[0].ODPPort)
}

func TestRegisterRefusesAPortAnotherSubscriberHolds(t *testing.T) {
	commander := &zteIntegrationCommander{}
	svc, olt := newZTEGPONIntegrationService(t, commander,
		&zteIntegrationSnapshotFake{after: &ConfigSnapshot{ZTE: &ZTESnapshot{SerialNumber: "HWTCB403E8A0"}}},
		&zteIntegrationRollbackFake{})
	odpID := odpForRegistration(t, svc.db, olt.ID, 8)
	port := 5
	sitting := seedDistributionONT(t, svc.db, olt.ID, uniqueSerial())
	require.NoError(t, NewDistributionService(svc.db).AssignONT(sitting.ID, odpID, port))

	req := validZTERequest(olt.ID)
	req.ODPID, req.ODPPort = &odpID, &port
	_, err := svc.RegisterAndConfigure(context.Background(), req, uuid.New())

	require.ErrorIs(t, err, ErrValidation)
	// The refusal has to come before the OLT is touched: a half-registered ONU
	// on live hardware is far worse than a rejected form.
	assert.Empty(t, commander.commands)
}

func TestRegisterRefusesAnODPWithoutAPort(t *testing.T) {
	commander := &zteIntegrationCommander{}
	svc, olt := newZTEGPONIntegrationService(t, commander,
		&zteIntegrationSnapshotFake{after: &ConfigSnapshot{ZTE: &ZTESnapshot{SerialNumber: "HWTCB403E8A0"}}},
		&zteIntegrationRollbackFake{})
	odpID := odpForRegistration(t, svc.db, olt.ID, 8)

	req := validZTERequest(olt.ID)
	req.ODPID = &odpID
	_, err := svc.RegisterAndConfigure(context.Background(), req, uuid.New())

	require.ErrorIs(t, err, ErrValidation)
	assert.Empty(t, commander.commands)
}
