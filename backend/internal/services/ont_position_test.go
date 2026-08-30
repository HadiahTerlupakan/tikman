package services

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

func discoveredAt(slot, portID, ontID int, serial string) connectivity.DiscoveredONT {
	return connectivity.DiscoveredONT{
		Slot:         slot,
		PortID:       portID,
		ONTID:        ontID,
		SerialNumber: serial,
		RunState:     3,
	}
}

func oltForPositions(t *testing.T, db *gorm.DB, name, ip string) uuid.UUID {
	t.Helper()
	olt := &models.OLT{
		ID:        uuid.New(),
		SiteID:    uuid.New(),
		Name:      name,
		IPAddress: ip,
		Model:     models.OLTModelZTEC320,
		Username:  "admin",
		Password:  "pass",
	}
	require.NoError(t, db.Create(olt).Error)
	return olt.ID
}

func TestONTsOnDifferentCardsOfOneOLTBothSurvive(t *testing.T) {
	// A ZTE chassis carries several cards. Port 1 / ONU 5 on card 8 and the same
	// position on card 9 are two different customers' boxes, and losing one of
	// them loses a real subscriber from the system.
	db := setupTestDB(t)
	ontService := NewONTService(db)
	oltID := oltForPositions(t, db, "Cariu", "172.30.30.3")

	result := ontService.BulkRegisterFromDiscovery(oltID, []connectivity.DiscoveredONT{
		discoveredAt(8, 1, 5, "ZTEGCARD0008"),
		discoveredAt(9, 1, 5, "ZTEGCARD0009"),
	})
	require.Empty(t, result.Errors)

	var stored []models.ONT
	require.NoError(t, db.Where("olt_id = ?", oltID).Find(&stored).Error)
	require.Len(t, stored, 2, "the second card's ONT overwrote the first instead of joining it")

	serials := []string{stored[0].SerialNumber, stored[1].SerialNumber}
	require.ElementsMatch(t, []string{"ZTEGCARD0008", "ZTEGCARD0009"}, serials)
}

func TestTwoOLTsMayUseTheSameCardAndPortPosition(t *testing.T) {
	// Position is only unique within one OLT. Two OLTs each having a card 3,
	// port 1, ONU 1 is the normal case, not a conflict.
	db := setupTestDB(t)
	ontService := NewONTService(db)
	first := oltForPositions(t, db, "Depok", "192.168.220.22")
	second := oltForPositions(t, db, "Bekasi", "172.30.30.2")

	firstResult := ontService.BulkRegisterFromDiscovery(first, []connectivity.DiscoveredONT{
		discoveredAt(3, 1, 1, "ZTEGDEPOK001"),
	})
	require.Empty(t, firstResult.Errors)

	secondResult := ontService.BulkRegisterFromDiscovery(second, []connectivity.DiscoveredONT{
		discoveredAt(3, 1, 1, "ZTEGBEKASI01"),
	})
	require.Empty(t, secondResult.Errors, "the unique index rejected a position another OLT already used")

	var count int64
	require.NoError(t, db.Model(&models.ONT{}).Count(&count).Error)
	require.Equal(t, int64(2), count)
}

func TestGetByOLTAndPositionDistinguishesCards(t *testing.T) {
	db := setupTestDB(t)
	ontService := NewONTService(db)
	oltID := oltForPositions(t, db, "Cariu", "172.30.30.3")

	ontService.BulkRegisterFromDiscovery(oltID, []connectivity.DiscoveredONT{
		discoveredAt(8, 1, 5, "ZTEGCARD0008"),
		discoveredAt(9, 1, 5, "ZTEGCARD0009"),
	})

	onCard8, err := ontService.GetByOLTAndPosition(oltID, 8, 1, 5)
	require.NoError(t, err)
	require.Equal(t, "ZTEGCARD0008", onCard8.SerialNumber)

	onCard9, err := ontService.GetByOLTAndPosition(oltID, 9, 1, 5)
	require.NoError(t, err)
	require.Equal(t, "ZTEGCARD0009", onCard9.SerialNumber)
}
