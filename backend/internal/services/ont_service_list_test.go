package services

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

func newTestOLT(t *testing.T, db *gorm.DB, siteID uuid.UUID, name, ip string) *models.OLT {
	t.Helper()

	olt := &models.OLT{
		ID:                uuid.New(),
		SiteID:            siteID,
		Name:              name,
		IPAddress:         ip,
		SNMPCommunity:     "public",
		Username:          "admin",
		Password:          "encrypted",
		SSHPort:           22,
		TelnetPort:        23,
		SNMPPort:          161,
		Model:             models.OLTModelZTEC300,
		PreferredProtocol: models.OLTProtocolSSH,
		Status:            models.OLTStatusOnline,
	}
	require.NoError(t, db.Create(olt).Error)

	return olt
}

// Ordering used to be created_at DESC, which let one OLT hide another: after
// registering a large OLT its ONTs filled the whole first page and the older
// OLT's ONTs fell outside the client's 200-row window, disappearing from the UI.
// Listing by position keeps every OLT represented and stays stable across polls.
func TestONTServiceListOrdersByPositionNotCreationTime(t *testing.T) {
	db := setupTestDB(t)
	ontService := NewONTService(db)
	site, _ := NewSiteService(db).Create("Site", "Loc", "Desc")

	older := newTestOLT(t, db, site.ID, "OlderOLT", "10.0.0.1")
	newer := newTestOLT(t, db, site.ID, "NewerOLT", "10.0.0.2")

	// The older OLT's ONTs are created first, so created_at DESC would list the
	// newer OLT's ONTs ahead of all of them.
	require.NoError(t, ontService.Create(&models.ONT{
		OLTID: older.ID, PortID: 1, ONTID: 2, SerialNumber: "OLD-1-2", Status: models.ONTStatusOnline,
	}))
	require.NoError(t, ontService.Create(&models.ONT{
		OLTID: older.ID, PortID: 1, ONTID: 1, SerialNumber: "OLD-1-1", Status: models.ONTStatusOnline,
	}))
	for i := 1; i <= 3; i++ {
		require.NoError(t, ontService.Create(&models.ONT{
			OLTID: newer.ID, PortID: 1, ONTID: i, SerialNumber: "NEW-1-" + string(rune(48+i)), Status: models.ONTStatusOnline,
		}))
	}

	onts, total, err := ontService.List(nil, nil, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(5), total)

	// Within an OLT, ONTs come out in port/ONU order regardless of insert order.
	byOLT := map[uuid.UUID][]int{}
	for _, ont := range onts {
		byOLT[ont.OLTID] = append(byOLT[ont.OLTID], ont.ONTID)
	}
	assert.Equal(t, []int{1, 2}, byOLT[older.ID], "older OLT's ONTs must be in position order")
	assert.Equal(t, []int{1, 2, 3}, byOLT[newer.ID])

	// Each OLT's ONTs must be contiguous, so a page cannot interleave them and a
	// small page size still shows a whole OLT at a time.
	seen := []uuid.UUID{}
	for _, ont := range onts {
		if len(seen) == 0 || seen[len(seen)-1] != ont.OLTID {
			seen = append(seen, ont.OLTID)
		}
	}
	assert.Len(t, seen, 2, "an OLT's ONTs were split across the ordering: %v", seen)
}

// Registration used to hardcode ONTStatusUnknown even though the discovery walk
// had already read each ONT's phase state. Because new ONTs are what an operator
// looks at first after adding an OLT, every freshly registered ONT showed as
// UNKNOWN until some later poll corrected it.
func TestBulkRegisterFromDiscoveryKeepsDiscoveredStatus(t *testing.T) {
	db := setupTestDB(t)
	ontService := NewONTService(db)
	site, _ := NewSiteService(db).Create("Site", "Loc", "Desc")
	olt := newTestOLT(t, db, site.ID, "OLT", "10.0.0.1")

	result := ontService.BulkRegisterFromDiscovery(olt.ID, []connectivity.DiscoveredONT{
		{PortID: 1, ONTID: 1, SerialNumber: "SN-ONLINE", RunState: connectivity.PhaseStateOnline},
		{PortID: 1, ONTID: 2, SerialNumber: "SN-OFFLINE", RunState: connectivity.PhaseStateOffline},
		{PortID: 1, ONTID: 3, SerialNumber: "SN-LOS", RunState: connectivity.PhaseStateLOS},
		// A driver that cannot map the vendor's raw value reports 0, and that must
		// still land as unknown rather than being coerced to a real state.
		{PortID: 1, ONTID: 4, SerialNumber: "SN-UNMAPPED", RunState: connectivity.PhaseStateUnknown},
	})

	require.Empty(t, result.Errors)
	assert.Equal(t, 4, result.Registered)

	want := map[string]models.ONTStatus{
		"SN-ONLINE":   models.ONTStatusOnline,
		"SN-OFFLINE":  models.ONTStatusOffline,
		"SN-LOS":      models.ONTStatusLOS,
		"SN-UNMAPPED": models.ONTStatusUnknown,
	}
	for serial, expected := range want {
		ont, err := ontService.GetBySerialNumber(serial)
		require.NoError(t, err, serial)
		assert.Equal(t, expected, ont.Status, "serial %s", serial)
	}
}
