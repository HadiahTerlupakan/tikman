package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"gorm.io/gorm"
)

func setupUnconfiguredONUHandlerTest(t *testing.T) (*UnconfiguredONUHandler, *gorm.DB) {
	db := TestDB(t)
	return NewUnconfiguredONUHandler(services.NewUnconfiguredONUService(db)), db
}

func TestUnconfiguredONUHandler_ListByOLT_InvalidID(t *testing.T) {
	handler, _ := setupUnconfiguredONUHandlerTest(t)

	w, c := SetupTestContext(http.MethodGet, "/api/v1/olts/not-a-uuid/unconfigured-onus", nil)
	c.Params = gin.Params{{Key: "id", Value: "not-a-uuid"}}

	handler.ListByOLT(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, "INVALID_ID", response.Code)
}

func TestUnconfiguredONUHandler_ListByOLT_OLTNotFound(t *testing.T) {
	handler, _ := setupUnconfiguredONUHandlerTest(t)

	oltID := uuid.New()
	w, c := SetupTestContext(http.MethodGet, "/api/v1/olts/"+oltID.String()+"/unconfigured-onus", nil)
	c.Params = gin.Params{{Key: "id", Value: oltID.String()}}

	handler.ListByOLT(c)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var response ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, "NOT_FOUND", response.Code)
}

func TestUnconfiguredONUHandler_ListByOLT_ReturnsScanEnvelope(t *testing.T) {
	db := TestDB(t)

	site, err := services.NewSiteService(db).Create("Site", "Location", "Description")
	require.NoError(t, err)

	olt := CreateTestOLT(t, db, site.ID, "OLT", "192.168.1.1", "admin", "password",
		22, 23, 161, "public", models.OLTProtocolSSH)

	service := services.NewUnconfiguredONUServiceWithWalker(db,
		func(string, string, int) ([]connectivity.UnconfiguredONU, error) {
			return []connectivity.UnconfiguredONU{
				{Slot: 3, Port: 1, SerialNumber: "HWTCB403E8A0", DeviceType: "HG8245H5"},
				{Slot: 3, Port: 2, SerialNumber: "ZTEGCAFFC2FD"},
			}, nil
		})

	w, c := SetupTestContext(http.MethodGet, "/api/v1/olts/"+olt.ID.String()+"/unconfigured-onus", nil)
	c.Params = gin.Params{{Key: "id", Value: olt.ID.String()}}

	NewUnconfiguredONUHandler(service).ListByOLT(c)

	require.Equal(t, http.StatusOK, w.Code)

	var response struct {
		OLTID string                         `json:"olt_id"`
		Data  []connectivity.UnconfiguredONU `json:"data"`
		Total int                            `json:"total"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, olt.ID.String(), response.OLTID)
	assert.Equal(t, 2, response.Total)
	require.Len(t, response.Data, 2)
	assert.Equal(t, "HWTCB403E8A0", response.Data[0].SerialNumber)
	assert.Equal(t, 3, response.Data[0].Slot)
	assert.Equal(t, 1, response.Data[0].Port)
	assert.Equal(t, "HG8245H5", response.Data[0].DeviceType)
	assert.Empty(t, response.Data[1].DeviceType)
}

func TestUnconfiguredONUHandler_ListByOLT_MissingCommunity(t *testing.T) {
	handler, db := setupUnconfiguredONUHandlerTest(t)

	site, err := services.NewSiteService(db).Create("Site", "Location", "Description")
	require.NoError(t, err)

	olt := CreateTestOLT(t, db, site.ID, "OLT", "192.168.1.1", "admin", "password",
		22, 23, 161, "public", models.OLTProtocolSSH)
	require.NoError(t, db.Model(olt).Update("snmp_community", "").Error)

	w, c := SetupTestContext(http.MethodGet, "/api/v1/olts/"+olt.ID.String()+"/unconfigured-onus", nil)
	c.Params = gin.Params{{Key: "id", Value: olt.ID.String()}}

	handler.ListByOLT(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, "CONFIG_ERROR", response.Code)
}
