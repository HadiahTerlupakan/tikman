package services

import (
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

func listONT(t *testing.T, db *gorm.DB, oltID uuid.UUID, slot *int, port, ontID int, serial string, status models.ONTStatus) {
	t.Helper()
	require.NoError(t, db.Create(&models.ONT{
		ID: uuid.New(), OLTID: oltID, Slot: slot, PortID: port, ONTID: ontID,
		SerialNumber: serial, Status: status,
	}).Error)
}

func TestListFilteredByCard(t *testing.T) {
	// Filtering by card in the browser only ever worked on the rows the browser
	// happened to have. On a chassis larger than one page it answered from a
	// slice of the network and said nothing about the rest.
	db := setupTestDB(t)
	ontService := NewONTService(db)
	oltID := oltForPositions(t, db, "Cariu", "172.30.30.3")

	eight, nine := 8, 9
	listONT(t, db, oltID, &eight, 1, 1, "SNCARD08A", models.ONTStatusOnline)
	listONT(t, db, oltID, &eight, 1, 2, "SNCARD08B", models.ONTStatusOnline)
	listONT(t, db, oltID, &nine, 1, 1, "SNCARD09A", models.ONTStatusOnline)

	onts, total, err := ontService.ListFiltered(ONTListFilter{Slot: &eight, Limit: 10})
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Len(t, onts, 2)
	for _, ont := range onts {
		require.Equal(t, 8, *ont.Slot)
	}
}

func TestListFilteredByPortStaysWithinTheCard(t *testing.T) {
	// Port 1 exists on every card. Filtering on port alone matched all of them
	// at once, which is how a port selection returned other subscribers' boxes.
	db := setupTestDB(t)
	ontService := NewONTService(db)
	oltID := oltForPositions(t, db, "Cariu", "172.30.30.3")

	eight, nine := 8, 9
	listONT(t, db, oltID, &eight, 1, 1, "SNCARD08A", models.ONTStatusOnline)
	listONT(t, db, oltID, &nine, 1, 1, "SNCARD09A", models.ONTStatusOnline)

	port := 1
	onts, total, err := ontService.ListFiltered(ONTListFilter{Slot: &nine, PortID: &port, Limit: 10})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Equal(t, "SNCARD09A", onts[0].SerialNumber)
}

func TestListFilteredBySerialSearch(t *testing.T) {
	db := setupTestDB(t)
	ontService := NewONTService(db)
	oltID := oltForPositions(t, db, "Cariu", "172.30.30.3")

	listONT(t, db, oltID, nil, 1, 1, "RTEGC6090CD5", models.ONTStatusOnline)
	listONT(t, db, oltID, nil, 1, 2, "ZTEGCACC2F40", models.ONTStatusOnline)

	onts, total, err := ontService.ListFiltered(ONTListFilter{Search: "acc2f", Limit: 10})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Equal(t, "ZTEGCACC2F40", onts[0].SerialNumber)
}

func TestListFilteredSearchIgnoresCase(t *testing.T) {
	// Technicians read serials off a label and type them in whatever case comes
	// out. A search that only matched upper case found nothing most of the time.
	db := setupTestDB(t)
	ontService := NewONTService(db)
	oltID := oltForPositions(t, db, "Cariu", "172.30.30.3")

	listONT(t, db, oltID, nil, 1, 1, "ZTEGCACC2F40", models.ONTStatusOnline)

	onts, _, err := ontService.ListFiltered(ONTListFilter{Search: "ZTEGCACC2F40", Limit: 10})
	require.NoError(t, err)
	require.Len(t, onts, 1)

	onts, _, err = ontService.ListFiltered(ONTListFilter{Search: "ztegcacc2f40", Limit: 10})
	require.NoError(t, err)
	require.Len(t, onts, 1)
}

func TestListFilteredSearchAlsoMatchesTheSubscriberName(t *testing.T) {
	// The name is what an operator is given on the phone; the serial is what is
	// printed on the box. Searching one field only made the other unfindable.
	db := setupTestDB(t)
	ontService := NewONTService(db)
	oltID := oltForPositions(t, db, "Cariu", "172.30.30.3")

	require.NoError(t, db.Create(&models.ONT{
		ID: uuid.New(), OLTID: oltID, PortID: 1, ONTID: 1,
		SerialNumber: "ZTEGC0DE0001", Name: "Heru Kurniawan", Status: models.ONTStatusOnline,
	}).Error)
	listONT(t, db, oltID, nil, 1, 2, "ZTEGC0DE0002", models.ONTStatusOnline)

	onts, total, err := ontService.ListFiltered(ONTListFilter{Search: "kurniawan", Limit: 10})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Equal(t, "ZTEGC0DE0001", onts[0].SerialNumber)
}

func TestListFilteredTotalCountsMatchesNotThePage(t *testing.T) {
	// The page shows one window; the total drives the pager. Reporting the
	// window's size as the total is what made a 930-ONT network claim to have
	// however many rows happened to arrive.
	db := setupTestDB(t)
	ontService := NewONTService(db)
	oltID := oltForPositions(t, db, "Cariu", "172.30.30.3")

	for i := 0; i < 25; i++ {
		listONT(t, db, oltID, nil, 1, i, fmt.Sprintf("SN%08d", i), models.ONTStatusOnline)
	}

	onts, total, err := ontService.ListFiltered(ONTListFilter{Limit: 10, Offset: 20})
	require.NoError(t, err)
	require.Equal(t, int64(25), total, "the total described the page rather than the network")
	require.Len(t, onts, 5)
}

func TestListFilteredCombinesEveryFilter(t *testing.T) {
	db := setupTestDB(t)
	ontService := NewONTService(db)
	cariu := oltForPositions(t, db, "Cariu", "172.30.30.3")
	bekasi := oltForPositions(t, db, "Bekasi", "172.30.30.2")

	nine := 9
	listONT(t, db, cariu, &nine, 6, 21, "ZTEGWANTED01", models.ONTStatusOnline)
	listONT(t, db, cariu, &nine, 6, 22, "ZTEGWANTED02", models.ONTStatusOffline)
	listONT(t, db, cariu, &nine, 7, 21, "ZTEGOTHER001", models.ONTStatusOnline)
	listONT(t, db, bekasi, &nine, 6, 21, "ZTEGOTHER002", models.ONTStatusOnline)

	port := 6
	online := models.ONTStatusOnline
	onts, total, err := ontService.ListFiltered(ONTListFilter{
		OLTID: &cariu, Slot: &nine, PortID: &port, Status: &online, Search: "wanted", Limit: 10,
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Equal(t, "ZTEGWANTED01", onts[0].SerialNumber)
}
