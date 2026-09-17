package services

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// Restored against the mapping-node model after Task 6 deleted the odps table
// and, with it, zte_gpon_register_odp_test.go's original two cases: occupancy
// refused before the transaction, and the field actually persisted. Both were
// the only end-to-end coverage of validateRegisterODP wired into a real
// RegisterAndConfigure call, as opposed to validateRegisterODP called directly
// (mapping_ont_test.go).

// odpForRegistration is a distribution box on the map, the shape a newly
// registered ONU's drop lands in.
func odpForRegistration(t *testing.T, db *gorm.DB, capacity int) uuid.UUID {
	t.Helper()
	node, err := NewMappingService(db).CreateNode(models.MappingNode{
		NodeID: "ODP-REG-" + uuid.NewString()[:8], Type: models.NodeODP, Name: "ODP Registrasi",
		Latitude: -6.2, Longitude: 106.8, Capacity: capacity,
	})
	require.NoError(t, err)
	return node.ID
}

func TestRegisterLandsTheDropOnTheChosenODPPort(t *testing.T) {
	commander := &zteIntegrationCommander{}
	svc, olt := newZTEGPONIntegrationService(t, commander,
		&zteIntegrationSnapshotFake{after: &ConfigSnapshot{ZTE: &ZTESnapshot{SerialNumber: "HWTCB403E8A0"}}},
		&zteIntegrationRollbackFake{})
	odpID := odpForRegistration(t, svc.db, 8)
	port := 5

	req := validZTERequest(olt.ID)
	req.ODPID, req.ODPPort = &odpID, &port
	_, err := svc.RegisterAndConfigure(context.Background(), req, uuid.New())
	require.NoError(t, err)

	var stored models.ONT
	require.NoError(t, svc.db.Where("serial_number = ?", "HWTCB403E8A0").First(&stored).Error)
	require.NotNil(t, stored.ODPID)
	assert.Equal(t, odpID, *stored.ODPID)
	require.NotNil(t, stored.ODPPort)
	assert.Equal(t, 5, *stored.ODPPort)
}

func TestRegisterRefusesAPortAnotherSubscriberHolds(t *testing.T) {
	commander := &zteIntegrationCommander{}
	svc, olt := newZTEGPONIntegrationService(t, commander,
		&zteIntegrationSnapshotFake{after: &ConfigSnapshot{ZTE: &ZTESnapshot{SerialNumber: "HWTCB403E8A0"}}},
		&zteIntegrationRollbackFake{})
	odpID := odpForRegistration(t, svc.db, 8)
	port := 5
	ontService := NewONTService(svc.db)
	sitting := createTestONT(t, ontService, olt.ID, "ZXCVEXISTING1", 9, 9)
	require.NoError(t, ontService.AssignONTToODP(sitting.ID, odpID, port))

	req := validZTERequest(olt.ID)
	req.ODPID, req.ODPPort = &odpID, &port
	_, err := svc.RegisterAndConfigure(context.Background(), req, uuid.New())

	require.ErrorIs(t, err, ErrValidation)
	// The refusal has to come before the OLT is touched: a half-registered ONU
	// on live hardware is far worse than a rejected form.
	assert.Empty(t, commander.commands)
}
