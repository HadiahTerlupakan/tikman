package services

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

// Registration asks which box the drop lands in. After the plant model changed,
// that box is a mapping node — and a registration naming a node that is not a
// distribution box is a mistake worth refusing.
func TestRegisteringAgainstAMappingODPIsAccepted(t *testing.T) {
	db := setupTestDB(t)
	svc := NewMappingService(db)
	node, err := svc.CreateNode(models.MappingNode{
		NodeID: "ODP-01", Type: models.NodeODP, Name: "ODP Satu",
		Latitude: -6.2, Longitude: 106.8, Capacity: 8,
	})
	require.NoError(t, err)

	port := 3
	err = validateRegisterODP(db, models.ZTEGPONRegisterRequest{ODPID: &node.ID, ODPPort: &port})

	require.NoError(t, err)
}

func TestRegisteringAgainstSomethingThatIsNotAnODPIsRefused(t *testing.T) {
	db := setupTestDB(t)
	svc := NewMappingService(db)
	node, err := svc.CreateNode(models.MappingNode{
		NodeID: "ODC-01", Type: models.NodeODC, Name: "ODC Satu",
		Latitude: -6.2, Longitude: 106.8,
	})
	require.NoError(t, err)

	port := 1
	err = validateRegisterODP(db, models.ZTEGPONRegisterRequest{ODPID: &node.ID, ODPPort: &port})

	require.Error(t, err)
}

func TestRegisteringAgainstAnODPThatIsNotThereIsRefused(t *testing.T) {
	db := setupTestDB(t)
	missing := uuid.New()

	port := 1
	err := validateRegisterODP(db, models.ZTEGPONRegisterRequest{ODPID: &missing, ODPPort: &port})

	require.Error(t, err)
}
