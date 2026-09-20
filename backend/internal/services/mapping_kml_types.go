package services

import "encoding/xml"

// kmlNamespace is what Google Earth and every KML validator expect the root
// element to declare; a document missing it is still XML, but not KML.
const kmlNamespace = "http://www.opengis.net/kml/2.2"

type kmlRoot struct {
	XMLName  xml.Name    `xml:"kml"`
	Xmlns    string      `xml:"xmlns,attr"`
	Document kmlDocument `xml:"Document"`
}

type kmlDocument struct {
	Name    string      `xml:"name"`
	Styles  []kmlStyle  `xml:"Style"`
	Folders []kmlFolder `xml:"Folder"`
}

// kmlFolder is what keeps the sidebar navigable: one per node kind plus one
// for cables, instead of a flat list of every placemark on the map.
type kmlFolder struct {
	Name       string         `xml:"name"`
	Placemarks []kmlPlacemark `xml:"Placemark"`
}

// kmlPlacemark covers both shapes this export produces: a node (Point) or a
// cable (LineString). Exactly one of the two is ever set.
//
// Field order here is the element order Go's encoding/xml writes, and it is
// not free choice: the OGC KML 2.2 schema (ogckml22.xsd) defines Placemark as
// an extension of AbstractFeatureType, whose own sequence ends with
// ExtendedData - the geometry is appended only by the Placemark extension
// itself, after it. ExtendedData placed after Point/LineString validates
// against nothing; validating a sample against the real schema is what
// caught this.
type kmlPlacemark struct {
	Name         string           `xml:"name"`
	Description  string           `xml:"description,omitempty"`
	StyleURL     string           `xml:"styleUrl,omitempty"`
	ExtendedData *kmlExtendedData `xml:"ExtendedData,omitempty"`
	Point        *kmlPoint        `xml:"Point,omitempty"`
	LineString   *kmlLineString   `xml:"LineString,omitempty"`
}

type kmlPoint struct {
	Coordinates string `xml:"coordinates"`
}

type kmlLineString struct {
	Coordinates string `xml:"coordinates"`
}

// kmlExtendedData is KML's own mechanism for carrying fields it has no
// element for - what makes a re-imported file exact instead of a guess.
type kmlExtendedData struct {
	Data []kmlData `xml:"Data"`
}

type kmlData struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value"`
}

// kmlStyle covers exactly the two shapes this export needs: a coloured point
// icon for a node, or a coloured line for a cable. A given style never
// carries both.
type kmlStyle struct {
	ID        string        `xml:"id,attr"`
	IconStyle *kmlIconStyle `xml:"IconStyle,omitempty"`
	LineStyle *kmlLineStyle `xml:"LineStyle,omitempty"`
}

type kmlIconStyle struct {
	Color string  `xml:"color"`
	Icon  kmlIcon `xml:"Icon"`
}

type kmlIcon struct {
	Href string `xml:"href"`
}

type kmlLineStyle struct {
	Color string  `xml:"color"`
	Width float64 `xml:"width"`
}
