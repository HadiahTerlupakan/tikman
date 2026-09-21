package services

import (
	"archive/zip"
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildTestKMZ zips one entry under the given name with the given raw bytes -
// deliberately not going through buildKML/BuildKMZ, since these tests exist
// to prove the importer survives a file it did not produce itself: a
// hand-written KML, one from a source with its own ideas about structure.
func buildTestKMZ(t *testing.T, entryName string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create(entryName)
	require.NoError(t, err)
	_, err = w.Write(content)
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

const minimalKML = `<?xml version="1.0" encoding="UTF-8"?>
<kml xmlns="http://www.opengis.net/kml/2.2"><Document>
<Folder><name>ODP</name>
<Placemark><name>ODP-01</name><Point><coordinates>106.810000,-6.210000,0</coordinates></Point></Placemark>
</Folder>
<Folder><name>Kabel</name>
<Placemark><name>E-1</name><LineString><coordinates>106.800000,-6.200000,0 106.810000,-6.210000,0</coordinates></LineString></Placemark>
</Folder>
</Document></kml>`

func TestWalkKMLFindsPlacemarksAcrossNestedFolders(t *testing.T) {
	raw, err := walkKML(strings.NewReader(minimalKML), maxKMLNestingDepth, maxKMLPlacemarks)
	require.NoError(t, err)
	require.Len(t, raw, 2)

	assert.Equal(t, "ODP-01", raw[0].Placemark.Name)
	assert.Equal(t, "ODP", raw[0].Folder)
	assert.NotNil(t, raw[0].Placemark.Point)

	assert.Equal(t, "E-1", raw[1].Placemark.Name)
	assert.Equal(t, "Kabel", raw[1].Folder)
	assert.NotNil(t, raw[1].Placemark.LineString)
}

// A placemark Google Earth lets you drop straight into the document, with no
// folder around it at all - a real, legal KML shape our own export never
// produces (it always folders), so nothing but a hand-written file exercises
// this.
func TestWalkKMLHandlesAPlacemarkWithNoFolderAtAll(t *testing.T) {
	kml := `<kml><Document>
<Placemark><name>Lepas</name><Point><coordinates>106.8,-6.2,0</coordinates></Point></Placemark>
</Document></kml>`

	raw, err := walkKML(strings.NewReader(kml), maxKMLNestingDepth, maxKMLPlacemarks)

	require.NoError(t, err)
	require.Len(t, raw, 1)
	assert.Equal(t, "", raw[0].Folder)
}

// A <GroundOverlay> or <NetworkLink> beside the placemarks is ordinary in a
// field survey - Google Earth uses both. Each carries its own <name>, and
// the walk must not mistake a name two levels below the folder (its own
// container's child) for the folder's own name one level below.
func TestWalkKMLDoesNotLetASiblingElementsNameStealTheFoldersAttribution(t *testing.T) {
	kml := `<kml><Document><Folder><name>ODP</name>
<GroundOverlay><name>Overlay Foto</name></GroundOverlay>
<Placemark><name>ODP-01</name><Point><coordinates>0,0,0</coordinates></Point></Placemark>
</Folder></Document></kml>`

	raw, err := walkKML(strings.NewReader(kml), maxKMLNestingDepth, maxKMLPlacemarks)

	require.NoError(t, err)
	require.Len(t, raw, 1)
	assert.Equal(t, "ODP", raw[0].Folder, "the overlay's own name must not overwrite the enclosing folder's")
}

// A small, cheap stand-in for "a KML nested a thousand folders deep": the
// production ceiling is generous (maxKMLNestingDepth), but the guard itself
// has to fire well before that many bytes are worth allocating in a test.
func TestWalkKMLRefusesNestingPastMaxDepth(t *testing.T) {
	var b strings.Builder
	b.WriteString("<kml><Document>")
	for i := 0; i < 50; i++ {
		b.WriteString("<Folder><name>f</name>")
	}
	b.WriteString(`<Placemark><name>x</name><Point><coordinates>0,0,0</coordinates></Point></Placemark>`)
	for i := 0; i < 50; i++ {
		b.WriteString("</Folder>")
	}
	b.WriteString("</Document></kml>")

	_, err := walkKML(strings.NewReader(b.String()), 10, maxKMLPlacemarks)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "dalam")
}

// walkKML's own loop only ever sees the tokens between one Placemark's start
// and end tags as a single pair - everything inside is handed to
// dec.DecodeElement, whose unmarshalling (and Skip, for whatever kmlPlacemark
// does not recognise) recurses on Go's own call stack, invisible to a depth
// counter that only increments on the outer loop's own Token calls. The
// nesting guard has to bind there too, not only on <Folder>.
func TestWalkKMLRefusesNestingInsideAPlacemarkToo(t *testing.T) {
	var b strings.Builder
	b.WriteString("<kml><Document><Folder><name>ODP</name>")
	b.WriteString("<Placemark><name>x</name><description>")
	for i := 0; i < 50; i++ {
		b.WriteString("<a>")
	}
	for i := 0; i < 50; i++ {
		b.WriteString("</a>")
	}
	b.WriteString("</description><Point><coordinates>0,0,0</coordinates></Point></Placemark>")
	b.WriteString("</Folder></Document></kml>")

	_, err := walkKML(strings.NewReader(b.String()), 10, maxKMLPlacemarks)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "dalam")
}

func TestWalkKMLRefusesTooManyPlacemarks(t *testing.T) {
	var b strings.Builder
	b.WriteString("<kml><Document><Folder><name>ODP</name>")
	for i := 0; i < 5; i++ {
		fmt.Fprintf(&b, `<Placemark><name>ODP-%d</name><Point><coordinates>0,0,0</coordinates></Point></Placemark>`, i)
	}
	b.WriteString("</Folder></Document></kml>")

	_, err := walkKML(strings.NewReader(b.String()), maxKMLNestingDepth, 3)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "placemark")
}

// The production value itself, exercised through parseKMZForImport (not
// walkKML with an injected small cap): both preview tables render every row
// with no pagination, and classifyEdge's own nearest-node search is O(nodes
// x edges without ExtendedData) - a cap high enough to describe "a plant far
// larger than anything this system runs today" was also high enough to be
// impractical to even test at full scale, which was itself the tell.
func TestParseKMZForImportRefusesMoreThanMaxKMLPlacemarks(t *testing.T) {
	var b strings.Builder
	b.WriteString("<kml><Document><Folder><name>ODP</name>")
	for i := 0; i <= maxKMLPlacemarks; i++ {
		fmt.Fprintf(&b, `<Placemark><name>ODP-%d</name><Point><coordinates>0,0,0</coordinates></Point></Placemark>`, i)
	}
	b.WriteString("</Folder></Document></kml>")
	kmz := buildTestKMZ(t, kmzKMLEntry, []byte(b.String()))

	_, err := parseKMZForImport(kmz)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "placemark")
}

// Go's xml.Decoder has no DTD support: a DOCTYPE's internal <!ENTITY> subset
// is never parsed, so a reference to one is always "undefined" from the
// decoder's own point of view, whether or not this file declares it. This is
// what makes XXE structurally unreachable here rather than merely
// undemonstrated - the test proves the claim instead of only asserting it in
// a comment.
func TestWalkKMLRejectsAnUndefinedEntityReference(t *testing.T) {
	kml := `<?xml version="1.0"?>
<!DOCTYPE kml [<!ENTITY xxe SYSTEM "file:///etc/passwd">]>
<kml><Document><Folder><name>ODP</name>
<Placemark><name>&xxe;</name><Point><coordinates>0,0,0</coordinates></Point></Placemark>
</Folder></Document></kml>`

	_, err := walkKML(strings.NewReader(kml), maxKMLNestingDepth, maxKMLPlacemarks)

	require.Error(t, err, "an undefined entity reference must fail to parse, not resolve to a file")
}

func TestExtractKMLPrefersDocKMLButFallsBackToAnyKmlEntry(t *testing.T) {
	kmz := buildTestKMZ(t, "network-export.kml", []byte(minimalKML))

	got, err := extractKML(kmz, maxKMLDecompressedBytes)

	require.NoError(t, err)
	assert.Equal(t, []byte(minimalKML), got)
}

func TestExtractKMLRefusesAZipWithNoKMLEntry(t *testing.T) {
	kmz := buildTestKMZ(t, "readme.txt", []byte("bukan KML"))

	_, err := extractKML(kmz, maxKMLDecompressedBytes)

	require.Error(t, err)
}

func TestParseKMZForImportRefusesANonZipFile(t *testing.T) {
	_, err := parseKMZForImport([]byte("bukan zip sama sekali"))

	require.Error(t, err)
}

// A stand-in zip bomb: real DEFLATE data compresses repetitive bytes by
// several hundred times, so a small, fast-to-build fixture already proves the
// guard fires on the declared/actual size rather than needing a genuine
// gigabyte-scale payload in a unit test.
func TestExtractKMLRefusesAnEntryPastTheDecompressedSizeCap(t *testing.T) {
	big := bytes.Repeat([]byte("a"), 2<<20) // 2 MiB of one byte, compresses to well under 1 KiB
	kmz := buildTestKMZ(t, kmzKMLEntry, big)

	_, err := extractKML(kmz, 1024)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "besar")
}
