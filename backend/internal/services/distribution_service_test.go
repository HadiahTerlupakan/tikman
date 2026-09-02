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

func distributionFixture(t *testing.T, db *gorm.DB) (models.Site, models.OLT) {
	t.Helper()
	site := models.Site{Name: "Cariu"}
	require.NoError(t, db.Create(&site).Error)
	olt := models.OLT{
		SiteID: site.ID, Name: "Cariu", IPAddress: "10.0.0.1",
		Username: "admin", Password: "enc", Model: models.OLTModelZTEC300,
	}
	require.NoError(t, db.Create(&olt).Error)
	return site, olt
}

func TestCreateODPAcceptsACabinetParent(t *testing.T) {
	db := setupTestDB(t)
	service := NewDistributionService(db)
	site, _ := distributionFixture(t, db)

	odc, err := service.CreateODC(ODCInput{SiteID: site.ID, Name: "ODC Cariu 1"})
	require.NoError(t, err)

	odp, err := service.CreateODP(ODPInput{Name: "ODP-01", PortCount: 8, ODCID: &odc.ID})
	require.NoError(t, err)
	assert.Equal(t, odc.ID, *odp.ODCID)
	assert.Nil(t, odp.OLTID)
}

func TestCreateODPAcceptsAPONPortParent(t *testing.T) {
	db := setupTestDB(t)
	service := NewDistributionService(db)
	_, olt := distributionFixture(t, db)
	slot, port := 1, 4

	// Some ports reach a distribution box with no cabinet in between, which is
	// why the parent cannot simply be a cabinet.
	odp, err := service.CreateODP(ODPInput{
		Name: "ODP-02", PortCount: 16, OLTID: &olt.ID, Slot: &slot, PortID: &port,
	})
	require.NoError(t, err)
	assert.Nil(t, odp.ODCID)
	assert.Equal(t, olt.ID, *odp.OLTID)
}

func TestCreateODPRejectsTwoParents(t *testing.T) {
	db := setupTestDB(t)
	service := NewDistributionService(db)
	site, olt := distributionFixture(t, db)
	odc, err := service.CreateODC(ODCInput{SiteID: site.ID, Name: "ODC"})
	require.NoError(t, err)
	slot, port := 1, 4

	_, err = service.CreateODP(ODPInput{
		Name: "ODP", PortCount: 8, ODCID: &odc.ID,
		OLTID: &olt.ID, Slot: &slot, PortID: &port,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "one parent")
}

func TestCreateODPRejectsNoParent(t *testing.T) {
	db := setupTestDB(t)
	service := NewDistributionService(db)
	distributionFixture(t, db)

	// A box connected to nothing is not a box anyone can find light in.
	_, err := service.CreateODP(ODPInput{Name: "ODP", PortCount: 8})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "one parent")
}

func TestAddODCFeedRejectsAPONPortAlreadyFeedingAnotherCabinet(t *testing.T) {
	db := setupTestDB(t)
	service := NewDistributionService(db)
	site, olt := distributionFixture(t, db)
	first, err := service.CreateODC(ODCInput{SiteID: site.ID, Name: "ODC A"})
	require.NoError(t, err)
	second, err := service.CreateODC(ODCInput{SiteID: site.ID, Name: "ODC B"})
	require.NoError(t, err)

	_, err = service.AddODCFeed(ODCFeedInput{
		ODCID: first.ID, OLTID: olt.ID, Slot: 1, PortID: 1, SplitterOutputs: 8,
	})
	require.NoError(t, err)

	// The light from a port is split once at this stage. A second cabinet
	// claiming it describes a network that cannot exist.
	_, err = service.AddODCFeed(ODCFeedInput{
		ODCID: second.ID, OLTID: olt.ID, Slot: 1, PortID: 1, SplitterOutputs: 8,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "already feeds")
}

func TestAssignONTRejectsAPortBeyondTheSplitter(t *testing.T) {
	db := setupTestDB(t)
	service := NewDistributionService(db)
	site, olt := distributionFixture(t, db)
	odc, err := service.CreateODC(ODCInput{SiteID: site.ID, Name: "ODC"})
	require.NoError(t, err)
	odp, err := service.CreateODP(ODPInput{Name: "ODP", PortCount: 8, ODCID: &odc.ID})
	require.NoError(t, err)
	ont := seedDistributionONT(t, db, olt.ID, "ZTEGC0000001")

	// A 1:8 splitter has no port 9, however confidently it is typed.
	err = service.AssignONT(ont.ID, odp.ID, 9)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "8 ports")
}

func TestAssignONTRejectsAPortAnotherSubscriberHolds(t *testing.T) {
	db := setupTestDB(t)
	service := NewDistributionService(db)
	site, olt := distributionFixture(t, db)
	odc, err := service.CreateODC(ODCInput{SiteID: site.ID, Name: "ODC"})
	require.NoError(t, err)
	odp, err := service.CreateODP(ODPInput{Name: "ODP", PortCount: 8, ODCID: &odc.ID})
	require.NoError(t, err)
	first := seedDistributionONT(t, db, olt.ID, "ZTEGC0000001")
	second := seedDistributionONT(t, db, olt.ID, "ZTEGC0000002")

	require.NoError(t, service.AssignONT(first.ID, odp.ID, 3))

	// Two drop cables cannot land on one port, and a silent overwrite would
	// send a technician to a port that is not free.
	err = service.AssignONT(second.ID, odp.ID, 3)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "taken")
}

func TestAssignONTMovesASubscriberBetweenPorts(t *testing.T) {
	db := setupTestDB(t)
	service := NewDistributionService(db)
	site, olt := distributionFixture(t, db)
	odc, err := service.CreateODC(ODCInput{SiteID: site.ID, Name: "ODC"})
	require.NoError(t, err)
	odp, err := service.CreateODP(ODPInput{Name: "ODP", PortCount: 8, ODCID: &odc.ID})
	require.NoError(t, err)
	ont := seedDistributionONT(t, db, olt.ID, "ZTEGC0000001")
	require.NoError(t, service.AssignONT(ont.ID, odp.ID, 3))

	// Re-patching to a different port is ordinary field work, not a conflict
	// with the subscriber's own previous assignment.
	require.NoError(t, service.AssignONT(ont.ID, odp.ID, 5))

	var stored models.ONT
	require.NoError(t, db.First(&stored, "id = ?", ont.ID).Error)
	assert.Equal(t, 5, *stored.ODPPort)
}

func seedDistributionONT(t *testing.T, db *gorm.DB, oltID uuid.UUID, serial string) models.ONT {
	t.Helper()
	slot := 1
	ont := models.ONT{
		OLTID: oltID, Slot: &slot, PortID: 1, ONTID: nextDistributionONTID(),
		SerialNumber: serial, Status: models.ONTStatusOnline,
	}
	require.NoError(t, db.Create(&ont).Error)
	return ont
}

var distributionONTSeq int

func nextDistributionONTID() int {
	distributionONTSeq++
	return distributionONTSeq
}

func TestODPAssignmentSurvivesADiscoveryCycle(t *testing.T) {
	db := setupTestDB(t)
	service := NewDistributionService(db)
	site, olt := distributionFixture(t, db)
	odc, err := service.CreateODC(ODCInput{SiteID: site.ID, Name: "ODC"})
	require.NoError(t, err)
	odp, err := service.CreateODP(ODPInput{Name: "ODP", PortCount: 8, ODCID: &odc.ID})
	require.NoError(t, err)
	ont := seedDistributionONT(t, db, olt.ID, "ZTEGC0000001")
	require.NoError(t, service.AssignONT(ont.ID, odp.ID, 3))

	// Which ODP a drop lands in is operator knowledge; the OLT has never heard
	// of it. A scan that overwrote the row wholesale would erase the field work
	// every minute, so this holds discovery to writing only what it knows.
	NewONTService(db).BulkRegisterFromDiscovery(olt.ID, []connectivity.DiscoveredONT{{
		Slot: 1, PortID: 1, ONTID: ont.ONTID, SerialNumber: "ZTEGC0000001",
		Name: "nama dari OLT", RunState: 1,
	}})

	var stored models.ONT
	require.NoError(t, db.First(&stored, "id = ?", ont.ID).Error)
	require.NotNil(t, stored.ODPID, "discovery erased the ODP assignment")
	assert.Equal(t, odp.ID, *stored.ODPID)
	assert.Equal(t, 3, *stored.ODPPort)
	assert.Equal(t, "nama dari OLT", stored.Name, "the OLT's own label should still land")
}
