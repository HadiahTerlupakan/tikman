package services

import (
	"bytes"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/tikman/olt-provisioning/internal/models"
)

func minimalOdpNode() models.MappingNode {
	return models.MappingNode{
		NodeID: "ODP-01", Type: models.NodeODP, Name: "ODP Depan Masjid",
		Latitude: -6.21, Longitude: 106.81,
	}
}

func TestNodePlacemarkUsesItsNameAsLabelAndAPointAtItsCoordinates(t *testing.T) {
	root := parseKML(t, mustBuildKML(t, []models.MappingNode{minimalOdpNode()}, nil))

	pm := findPlacemark(t, findFolder(t, root, "ODP"), "ODP Depan Masjid")

	assert.NotNil(t, pm.Point, "a node placemark must carry a Point, not a LineString")
	assert.Nil(t, pm.LineString)
	assert.Equal(t, coordinate(-6.21, 106.81), pm.Point.Coordinates)
}

// Mirrors NodePopup.tsx's OptionalFields: a node exists to be placed first
// and described later, so a field nobody filled must render nothing, never
// an empty "Splitter: " line.
func TestNodeDescriptionOmitsFieldsNobodyFilled(t *testing.T) {
	root := parseKML(t, mustBuildKML(t, []models.MappingNode{minimalOdpNode()}, nil))

	pm := findPlacemark(t, findFolder(t, root, "ODP"), "ODP Depan Masjid")

	assert.Contains(t, pm.Description, "Kode: ODP-01")
	assert.Contains(t, pm.Description, "Jenis: ODP")
	assert.NotContains(t, pm.Description, "Splitter")
	assert.NotContains(t, pm.Description, "PPPoE")
	assert.NotContains(t, pm.Description, "Serial")
	assert.NotContains(t, pm.Description, "Catatan")
}

func TestNodeDescriptionIncludesWhicheverOptionalFieldsAreFilled(t *testing.T) {
	node := minimalOdpNode()
	node.Capacity = 8
	node.Splitter = "1:8"
	node.SerialNumber = "SN-123"
	node.Notes = "dekat tiang 12"

	root := parseKML(t, mustBuildKML(t, []models.MappingNode{node}, nil))
	pm := findPlacemark(t, findFolder(t, root, "ODP"), "ODP Depan Masjid")

	assert.Contains(t, pm.Description, "Kapasitas: 8")
	assert.Contains(t, pm.Description, "Splitter: 1:8")
	assert.Contains(t, pm.Description, "Serial: SN-123")
	assert.Contains(t, pm.Description, "Catatan: dekat tiang 12")
	assert.NotContains(t, pm.Description, "PPPoE")
}

// The model fields ExtendedData must carry are the ones KML itself cannot
// express - this is the record a future importer reads back, so every field
// is present even when empty, unlike the description's hide-if-blank rule.
func TestNodeExtendedDataRoundTripsEveryModelField(t *testing.T) {
	node := minimalOdpNode()
	node.Capacity = 8
	node.Splitter = "1:8"
	node.PPPoE = "user@isp"
	node.SerialNumber = "SN-123"
	node.Notes = "dekat tiang 12"

	root := parseKML(t, mustBuildKML(t, []models.MappingNode{node}, nil))
	pm := findPlacemark(t, findFolder(t, root, "ODP"), "ODP Depan Masjid")

	assert.Equal(t, "ODP-01", extendedDataValue(t, pm, "node_id"))
	assert.Equal(t, "odp", extendedDataValue(t, pm, "type"))
	assert.Equal(t, "8", extendedDataValue(t, pm, "capacity"))
	assert.Equal(t, "1:8", extendedDataValue(t, pm, "splitter"))
	assert.Equal(t, "user@isp", extendedDataValue(t, pm, "pppoe"))
	assert.Equal(t, "SN-123", extendedDataValue(t, pm, "serial_number"))
	assert.Equal(t, "", extendedDataValue(t, pm, "olt_id"))
	assert.Equal(t, "dekat tiang 12", extendedDataValue(t, pm, "notes"))
}

func TestNodeExtendedDataCarriesTheOLTIdWhenTheNodeMirrorsOne(t *testing.T) {
	oltID := uuid.New()
	node := models.MappingNode{
		NodeID: "SERVER-01", Type: models.NodeServer, Name: "OLT Satu",
		Latitude: -6.2, Longitude: 106.8, OLTID: &oltID,
	}

	root := parseKML(t, mustBuildKML(t, []models.MappingNode{node}, nil))
	pm := findPlacemark(t, findFolder(t, root, "Server / OLT"), "OLT Satu")

	assert.Equal(t, oltID.String(), extendedDataValue(t, pm, "olt_id"))
}

func TestNodeStylesUseTheMapsOwnColourPerType(t *testing.T) {
	nodes := []models.MappingNode{
		{NodeID: "SERVER-01", Type: models.NodeServer, Name: "Srv", Latitude: -6.2, Longitude: 106.8},
	}
	root := parseKML(t, mustBuildKML(t, nodes, nil))
	pm := findPlacemark(t, findFolder(t, root, "Server / OLT"), "Srv")

	assert.Equal(t, "#node-server", pm.StyleURL)

	var style *kmlStyle
	for i, s := range root.Document.Styles {
		if s.ID == "node-server" {
			style = &root.Document.Styles[i]
		}
	}
	if assert.NotNil(t, style) && assert.NotNil(t, style.IconStyle) {
		assert.Equal(t, kmlColor("#8b5cf6"), style.IconStyle.Color)
	}
}

// The regression this export already shipped once: kmlPlacemark's field
// order is not free choice (its own doc comment explains why), but nothing
// before this asserted on it directly - a struct that round-trips happily
// through Go's own decoder can still be invalid against the real OGC
// schema, since encoding/xml matches by tag name, not position. Checked on
// the raw bytes, not the decoded struct: unmarshalling into kmlRoot would
// succeed either way and so could never catch this by construction.
func TestNodePlacemarkExtendedDataPrecedesThePointInTheRawXML(t *testing.T) {
	kml := mustBuildKML(t, []models.MappingNode{minimalOdpNode()}, nil)

	assert.Less(t, bytes.Index(kml, []byte("<ExtendedData>")), bytes.Index(kml, []byte("<Point>")))
}
