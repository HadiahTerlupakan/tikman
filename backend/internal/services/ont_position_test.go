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

func TestSeveralONUsWithNoSerialCanAllBeRegistered(t *testing.T) {
	// The inventory walk does not return a serial for every ONU it finds. An
	// absent serial is not a value, so two serial-less ONUs are not duplicates
	// of each other — before this, the first one registered locked out every
	// other serial-less ONU in the whole database, on any OLT.
	db := setupTestDB(t)
	ontService := NewONTService(db)
	oltID := oltForPositions(t, db, "Cariu", "172.30.30.3")

	result := ontService.BulkRegisterFromDiscovery(oltID, []connectivity.DiscoveredONT{
		discoveredAt(9, 1, 42, ""),
		discoveredAt(9, 1, 43, ""),
		discoveredAt(8, 2, 7, ""),
	})
	require.Empty(t, result.Errors)

	var count int64
	require.NoError(t, db.Model(&models.ONT{}).Where("olt_id = ?", oltID).Count(&count).Error)
	require.Equal(t, int64(3), count)
}

func TestASerialLessONUDoesNotBlockAnotherOLT(t *testing.T) {
	// The duplicate-serial check is global, so Cariu's one serial-less row was
	// what kept Bekasi's serial-less ONU out of the table.
	db := setupTestDB(t)
	ontService := NewONTService(db)
	cariu := oltForPositions(t, db, "Cariu", "172.30.30.3")
	bekasi := oltForPositions(t, db, "Bekasi", "172.30.30.2")

	ontService.BulkRegisterFromDiscovery(cariu, []connectivity.DiscoveredONT{
		discoveredAt(9, 1, 42, ""),
	})
	result := ontService.BulkRegisterFromDiscovery(bekasi, []connectivity.DiscoveredONT{
		discoveredAt(3, 6, 4, ""),
	})

	require.Empty(t, result.Errors)
	var count int64
	require.NoError(t, db.Model(&models.ONT{}).Where("olt_id = ?", bekasi).Count(&count).Error)
	require.Equal(t, int64(1), count)
}

func TestARealSerialIsStillUnique(t *testing.T) {
	db := setupTestDB(t)
	ontService := NewONTService(db)
	first := oltForPositions(t, db, "Cariu", "172.30.30.3")
	second := oltForPositions(t, db, "Bekasi", "172.30.30.2")

	ontService.BulkRegisterFromDiscovery(first, []connectivity.DiscoveredONT{
		discoveredAt(9, 1, 42, "ZTEGC0DE0001"),
	})
	result := ontService.BulkRegisterFromDiscovery(second, []connectivity.DiscoveredONT{
		discoveredAt(3, 6, 4, "ZTEGC0DE0001"),
	})

	require.Len(t, result.Errors, 1, "the same box cannot be on two OLTs at once")
	var count int64
	require.NoError(t, db.Model(&models.ONT{}).Count(&count).Error)
	require.Equal(t, int64(1), count)
}

func TestABoxThatMovedPositionIsFollowedNotRejected(t *testing.T) {
	// A serial names one physical box, so when the OLT reports it at a new
	// position the box moved and its row moves with it, keeping the metrics and
	// event history attached to the same subscriber. Creating a second row is
	// impossible — the serial is taken — and refusing outright is what left five
	// discovered ONUs unstored, each one's old row holding its serial hostage.
	db := setupTestDB(t)
	ontService := NewONTService(db)
	oltID := oltForPositions(t, db, "Cariu", "172.30.30.3")

	ontService.BulkRegisterFromDiscovery(oltID, []connectivity.DiscoveredONT{
		discoveredAt(8, 12, 22, "ZTEGCACC2F40"),
	})
	before, err := ontService.GetByOLTAndPosition(oltID, 8, 12, 22)
	require.NoError(t, err)

	result := ontService.BulkRegisterFromDiscovery(oltID, []connectivity.DiscoveredONT{
		discoveredAt(9, 6, 21, "ZTEGCACC2F40"),
	})
	require.Empty(t, result.Errors)

	var count int64
	require.NoError(t, db.Model(&models.ONT{}).Where("olt_id = ?", oltID).Count(&count).Error)
	require.Equal(t, int64(1), count, "the box was duplicated instead of moved")

	moved, err := ontService.GetByOLTAndPosition(oltID, 9, 6, 21)
	require.NoError(t, err)
	require.Equal(t, before.ID, moved.ID, "a new row was created, so the box lost its history")

	_, err = ontService.GetByOLTAndPosition(oltID, 8, 12, 22)
	require.Error(t, err, "the box was left behind at the position it vacated")
}

func TestAMovedBoxNeverDisplacesTheOccupantOfItsNewPosition(t *testing.T) {
	// Following the serial must never overwrite whoever the walk says is already
	// at the target position: that row is a different subscriber's box.
	db := setupTestDB(t)
	ontService := NewONTService(db)
	oltID := oltForPositions(t, db, "Cariu", "172.30.30.3")

	ontService.BulkRegisterFromDiscovery(oltID, []connectivity.DiscoveredONT{
		discoveredAt(8, 12, 22, "ZTEGCACC2F40"),
		discoveredAt(9, 6, 21, "ZTEGCAFFD0A9"),
	})

	ontService.BulkRegisterFromDiscovery(oltID, []connectivity.DiscoveredONT{
		discoveredAt(9, 6, 21, "ZTEGCACC2F40"),
	})

	occupant, err := ontService.GetByOLTAndPosition(oltID, 9, 6, 21)
	require.NoError(t, err)
	require.Equal(t, "ZTEGCAFFD0A9", occupant.SerialNumber, "the occupant was overwritten")

	var count int64
	require.NoError(t, db.Model(&models.ONT{}).Where("olt_id = ?", oltID).Count(&count).Error)
	require.Equal(t, int64(2), count)
}

func TestASerialOnAnotherOLTIsNotFollowed(t *testing.T) {
	// Moving a box between OLTs is a different event from moving it between
	// ports, and nothing here has evidence of it. It stays an error the log
	// names, rather than a row silently relocated to another site.
	db := setupTestDB(t)
	ontService := NewONTService(db)
	cariu := oltForPositions(t, db, "Cariu", "172.30.30.3")
	bekasi := oltForPositions(t, db, "Bekasi", "172.30.30.2")

	ontService.BulkRegisterFromDiscovery(cariu, []connectivity.DiscoveredONT{
		discoveredAt(8, 12, 22, "ZTEGCACC2F40"),
	})
	result := ontService.BulkRegisterFromDiscovery(bekasi, []connectivity.DiscoveredONT{
		discoveredAt(2, 6, 4, "ZTEGCACC2F40"),
	})

	require.Len(t, result.Errors, 1)
	stillOnCariu, err := ontService.GetByOLTAndPosition(cariu, 8, 12, 22)
	require.NoError(t, err)
	require.Equal(t, "ZTEGCACC2F40", stillOnCariu.SerialNumber)
}
