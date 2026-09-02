package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListODPsCountsThePortsInUse(t *testing.T) {
	db := setupTestDB(t)
	service := NewDistributionService(db)
	site, olt := distributionFixture(t, db)
	odc, err := service.CreateODC(ODCInput{SiteID: site.ID, Code: "ODC"})
	require.NoError(t, err)
	busy, err := service.CreateODP(ODPInput{Code: "ODP penuh", PortCount: 8, ODCID: &odc.ID})
	require.NoError(t, err)
	empty, err := service.CreateODP(ODPInput{Code: "ODP kosong", PortCount: 16, ODCID: &odc.ID})
	require.NoError(t, err)

	for port := 1; port <= 3; port++ {
		ont := seedDistributionONT(t, db, olt.ID, uniqueSerial())
		require.NoError(t, service.AssignONT(ont.ID, busy.ID, port))
	}

	boxes, err := service.ListODPs()
	require.NoError(t, err)

	// The one question the map has to answer without being opened: how much
	// room is left in each box.
	byID := map[string]ODPWithUsage{}
	for _, box := range boxes {
		byID[box.ID.String()] = box
	}
	assert.Equal(t, 3, byID[busy.ID.String()].UsedPorts)
	assert.Equal(t, 8, byID[busy.ID.String()].PortCount)

	// A box nobody is on still belongs on the map — it is the one with room.
	require.Contains(t, byID, empty.ID.String())
	assert.Equal(t, 0, byID[empty.ID.String()].UsedPorts)
}

func TestListODPsLeavesOutSubscribersOnOtherBoxes(t *testing.T) {
	db := setupTestDB(t)
	service := NewDistributionService(db)
	site, olt := distributionFixture(t, db)
	odc, err := service.CreateODC(ODCInput{SiteID: site.ID, Code: "ODC"})
	require.NoError(t, err)
	first, err := service.CreateODP(ODPInput{Code: "ODP A", PortCount: 8, ODCID: &odc.ID})
	require.NoError(t, err)
	second, err := service.CreateODP(ODPInput{Code: "ODP B", PortCount: 8, ODCID: &odc.ID})
	require.NoError(t, err)

	one := seedDistributionONT(t, db, olt.ID, uniqueSerial())
	require.NoError(t, service.AssignONT(one.ID, first.ID, 1))
	two := seedDistributionONT(t, db, olt.ID, uniqueSerial())
	require.NoError(t, service.AssignONT(two.ID, second.ID, 1))

	boxes, err := service.ListODPs()
	require.NoError(t, err)

	// A join that counted every assigned subscriber against every box would
	// report both as full, and the map would send nobody anywhere.
	for _, box := range boxes {
		assert.Equal(t, 1, box.UsedPorts, box.Code)
	}
}
