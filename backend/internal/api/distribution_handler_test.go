package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"gorm.io/gorm"
)

func setupDistributionHandlerTest(t *testing.T) (*DistributionHandler, *gorm.DB) {
	db := TestDB(t)
	return NewDistributionHandler(services.NewDistributionService(db)), db
}

func distributionHandlerFixture(t *testing.T, db *gorm.DB) (models.Site, models.OLT) {
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

func TestCreateODPHandlerRefusesTwoParents(t *testing.T) {
	handler, db := setupDistributionHandlerTest(t)
	site, olt := distributionHandlerFixture(t, db)
	odc := models.ODC{SiteID: site.ID, Name: "ODC"}
	require.NoError(t, db.Create(&odc).Error)
	slot, port := 1, 4

	w, c := SetupTestContext(http.MethodPost, "/api/v1/odps", CreateODPRequest{
		Name: "ODP", PortCount: 8,
		ODCID: odc.ID.String(), OLTID: olt.ID.String(), Slot: &slot, PortID: &port,
	})

	handler.CreateODP(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var response ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, "INVALID_ODP", response.Code)
	// The operator reads this, so it has to be the sentence, not a constraint
	// name leaking out of Postgres.
	assert.Contains(t, response.Error, "one parent")
}

func TestCreateODPHandlerAcceptsAPONPortParent(t *testing.T) {
	handler, db := setupDistributionHandlerTest(t)
	_, olt := distributionHandlerFixture(t, db)
	slot, port := 1, 4

	w, c := SetupTestContext(http.MethodPost, "/api/v1/odps", CreateODPRequest{
		Name: "ODP-02", PortCount: 16,
		OLTID: olt.ID.String(), Slot: &slot, PortID: &port,
	})

	handler.CreateODP(c)

	require.Equal(t, http.StatusCreated, w.Code)
	var response struct {
		Data models.ODP `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, olt.ID, *response.Data.OLTID)
	assert.Nil(t, response.Data.ODCID)
}

func TestAssignONTHandlerRefusesAPortBeyondTheSplitter(t *testing.T) {
	handler, db := setupDistributionHandlerTest(t)
	site, olt := distributionHandlerFixture(t, db)
	odc := models.ODC{SiteID: site.ID, Name: "ODC"}
	require.NoError(t, db.Create(&odc).Error)
	odp := models.ODP{Name: "ODP", PortCount: 8, ODCID: &odc.ID}
	require.NoError(t, db.Create(&odp).Error)
	slot := 1
	ont := models.ONT{
		OLTID: olt.ID, Slot: &slot, PortID: 1, ONTID: 1,
		SerialNumber: "ZTEGC0000001", Status: models.ONTStatusOnline,
	}
	require.NoError(t, db.Create(&ont).Error)

	w, c := SetupTestContext(http.MethodPut, "/api/v1/onts/"+ont.ID.String()+"/odp",
		AssignONTToODPRequest{ODPID: odp.ID.String(), Port: 9})
	c.Params = gin.Params{{Key: "id", Value: ont.ID.String()}}

	handler.AssignONT(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var response ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Contains(t, response.Error, "8 ports")
}

func TestListODPsHandlerReportsTheRoomLeft(t *testing.T) {
	handler, db := setupDistributionHandlerTest(t)
	site, olt := distributionHandlerFixture(t, db)
	odc := models.ODC{SiteID: site.ID, Name: "ODC"}
	require.NoError(t, db.Create(&odc).Error)
	odp := models.ODP{Name: "ODP", PortCount: 8, ODCID: &odc.ID}
	require.NoError(t, db.Create(&odp).Error)
	slot, port := 1, 2
	require.NoError(t, db.Create(&models.ONT{
		OLTID: olt.ID, Slot: &slot, PortID: 1, ONTID: 1,
		SerialNumber: uuid.NewString()[:12], Status: models.ONTStatusOnline,
		ODPID: &odp.ID, ODPPort: &port,
	}).Error)

	w, c := SetupTestContext(http.MethodGet, "/api/v1/odps", nil)

	handler.ListODPs(c)

	require.Equal(t, http.StatusOK, w.Code)
	var response struct {
		Data []services.ODPWithUsage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Len(t, response.Data, 1)
	assert.Equal(t, 1, response.Data[0].UsedPorts)
	assert.Equal(t, 8, response.Data[0].PortCount)
}
