package services

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// setupPlantPostgres builds the plant tables on real Postgres.
//
// The route columns are jsonb, and jsonb is where SQLite stops being a stand-in:
// it accepts anything in a text column, so a value Postgres rejects passes there
// unnoticed.
func setupPlantPostgres(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		if os.Getenv("CI") != "" {
			t.Fatal("TEST_POSTGRES_DSN is unset under CI; the jsonb columns are then never written anywhere")
		}
		t.Skip("set TEST_POSTGRES_DSN to store cable routes against Postgres")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	for _, table := range []string{"odps", "odc_feeds", "odcs", "onts", "olts", "sites"} {
		require.NoError(t, db.Exec("DROP TABLE IF EXISTS "+table+" CASCADE").Error)
	}
	require.NoError(t, db.AutoMigrate(&models.Site{}, &models.OLT{}, &models.ONT{},
		&models.ODC{}, &models.ODCFeed{}, &models.ODP{}))
	return db
}

func TestSetODPRouteStoresJSONPostgresAccepts(t *testing.T) {
	db := setupPlantPostgres(t)
	service := NewDistributionService(db)
	site := models.Site{Name: "Cariu"}
	require.NoError(t, db.Create(&site).Error)
	odc, err := service.CreateODC(ODCInput{SiteID: site.ID, Code: "ODC-01"})
	require.NoError(t, err)
	odp, err := service.CreateODP(ODPInput{Code: "ODP-01", PortCount: 8, ODCID: &odc.ID})
	require.NoError(t, err)

	err = service.SetODPRoute(odp.ID, []models.RoutePoint{
		{Lat: -6.4000, Lng: 107.0000},
		{Lat: -6.4100, Lng: 107.0000},
	})

	require.NoError(t, err)
	stored, err := service.ODPByID(odp.ID)
	require.NoError(t, err)
	points, err := stored.RoutePath()
	require.NoError(t, err)
	assert.Len(t, points, 2)
	assert.InDelta(t, 1112, stored.RouteMeters, 20)
}
