package services

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

// baseOLTInput is a valid OLT with no SNMP community, so Create skips the live
// SNMP probe and these tests exercise validation rather than the network.
func baseOLTInput(siteID uuid.UUID) CreateOLTInput {
	return CreateOLTInput{
		SiteID:    siteID,
		Name:      "Test OLT",
		IPAddress: "192.168.1.1",
		Username:  "admin",
		Password:  "password123",
		Model:     models.OLTModelZTEC300,

		SSHPort:           22,
		TelnetPort:        23,
		SNMPPort:          161,
		PreferredProtocol: models.OLTProtocolSSH,
	}
}

func TestOLTAcceptsAValidCoordinatePair(t *testing.T) {
	db := setupTestDB(t)
	site, err := NewSiteService(db).Create("Depok", "Depok", "")
	require.NoError(t, err)

	input := baseOLTInput(site.ID)
	input.Latitude = floatPtr(-6.4025)
	input.Longitude = floatPtr(106.7942)

	olt, err := NewOLTService(db, testEncryptionKey).Create(input)
	require.NoError(t, err)
	require.NotNil(t, olt.Latitude)
	require.InDelta(t, -6.4025, *olt.Latitude, 0.0001)
	require.InDelta(t, 106.7942, *olt.Longitude, 0.0001)
}

func TestOLTSavesWithoutCoordinates(t *testing.T) {
	// An OLT that cannot be placed on the map must still be a valid OLT, or a
	// location nobody has looked up yet would block registering the device.
	db := setupTestDB(t)
	site, err := NewSiteService(db).Create("Depok", "Depok", "")
	require.NoError(t, err)

	olt, err := NewOLTService(db, testEncryptionKey).Create(baseOLTInput(site.ID))
	require.NoError(t, err)
	require.Nil(t, olt.Latitude)
	require.Nil(t, olt.Longitude)
}

func TestOLTRefusesHalfACoordinate(t *testing.T) {
	// One value alone would place a pin on the equator or the prime meridian
	// and look like a deliberate answer.
	db := setupTestDB(t)
	site, err := NewSiteService(db).Create("Depok", "Depok", "")
	require.NoError(t, err)
	service := NewOLTService(db, testEncryptionKey)

	onlyLatitude := baseOLTInput(site.ID)
	onlyLatitude.Latitude = floatPtr(-6.4)
	_, err = service.Create(onlyLatitude)
	require.ErrorIs(t, err, ErrValidation)

	onlyLongitude := baseOLTInput(site.ID)
	onlyLongitude.Longitude = floatPtr(106.8)
	_, err = service.Create(onlyLongitude)
	require.ErrorIs(t, err, ErrValidation)
}

func TestOLTRefusesCoordinatesOutsideTheGlobe(t *testing.T) {
	db := setupTestDB(t)
	site, err := NewSiteService(db).Create("Depok", "Depok", "")
	require.NoError(t, err)
	service := NewOLTService(db, testEncryptionKey)

	badLatitude := baseOLTInput(site.ID)
	badLatitude.Latitude = floatPtr(91)
	badLatitude.Longitude = floatPtr(0)
	_, err = service.Create(badLatitude)
	require.ErrorIs(t, err, ErrValidation)

	badLongitude := baseOLTInput(site.ID)
	badLongitude.Latitude = floatPtr(0)
	badLongitude.Longitude = floatPtr(181)
	_, err = service.Create(badLongitude)
	require.ErrorIs(t, err, ErrValidation)
}

func TestOLTUpdateValidatesCoordinatesToo(t *testing.T) {
	db := setupTestDB(t)
	site, err := NewSiteService(db).Create("Depok", "Depok", "")
	require.NoError(t, err)
	service := NewOLTService(db, testEncryptionKey)

	olt, err := service.Create(baseOLTInput(site.ID))
	require.NoError(t, err)

	err = service.Update(olt.ID, map[string]interface{}{
		"latitude":  floatPtr(-6.4),
		"longitude": floatPtr(200.0),
	})
	require.ErrorIs(t, err, ErrValidation)
}

func TestOLTUpdateClearsACoordinatePair(t *testing.T) {
	// A wrongly placed pin has to be removable, not merely overwritable.
	db := setupTestDB(t)
	site, err := NewSiteService(db).Create("Depok", "Depok", "")
	require.NoError(t, err)
	service := NewOLTService(db, testEncryptionKey)

	input := baseOLTInput(site.ID)
	input.Latitude = floatPtr(-6.4025)
	input.Longitude = floatPtr(106.7942)
	olt, err := service.Create(input)
	require.NoError(t, err)

	require.NoError(t, service.Update(olt.ID, map[string]interface{}{
		"latitude":  (*float64)(nil),
		"longitude": (*float64)(nil),
	}))

	stored, err := service.GetByID(olt.ID)
	require.NoError(t, err)
	require.Nil(t, stored.Latitude)
	require.Nil(t, stored.Longitude)
}
