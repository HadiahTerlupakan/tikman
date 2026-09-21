package services

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
)

const (
	// maxKMLDecompressedBytes bounds doc.kml itself once unzipped, checked
	// against the zip's own declared size before anything is read and again
	// against what is actually read - the standard defence against a small
	// upload whose one entry decompresses to something enormous. 100 MiB of
	// KML text already describes a plant far larger than anything this
	// system runs today.
	maxKMLDecompressedBytes = 100 << 20

	// maxKMLNestingDepth bounds how many elements deep the walk in walkKML
	// will follow, independent of file size: a handful of bytes per level
	// ("<Folder>") is enough to reach millions of levels within
	// maxKMLDecompressedBytes alone, and Go's own goroutine stack, while it
	// grows, is not unlimited. Google Earth's own folder trees are rarely
	// more than a few levels deep, so this leaves generous headroom while
	// still refusing a file nested "a thousand folders deep".
	maxKMLNestingDepth = 64

	// maxKMLPlacemarks caps how many placemarks one import will carry into
	// memory and render in the preview - both tables show every row with no
	// pagination, so this is a DOM row count as much as a memory bound. It
	// also bounds classifyEdge's own nearest-node search, which is O(nodes x
	// edges without ExtendedData): measured at roughly 613s of CPU from a
	// file that zips to 65 KiB when this was 200,000. 2,000 is still far
	// more than a single technician draws in one field survey (a real route
	// is a handful of corners, not a plant's whole placemark count), while
	// keeping that search trivially fast and the table sizes the preview
	// screen was actually built to show.
	maxKMLPlacemarks = 2000
)

// rawPlacemark is one Placemark element as found in the file, together with
// the name of the Folder immediately enclosing it (empty when the placemark
// sits directly under Document). It carries no opinion yet about whether it
// is a node or a cable - see classifyPlacemarks.
type rawPlacemark struct {
	Placemark kmlPlacemark
	Folder    string
}

// parseKMZForImport extracts doc.kml from an uploaded .kmz and walks it for
// placemarks, under the fixed production limits. See walkKML and extractKML
// for what each limit defends against.
func parseKMZForImport(data []byte) ([]rawPlacemark, error) {
	kml, err := extractKML(data, maxKMLDecompressedBytes)
	if err != nil {
		return nil, err
	}
	return walkKML(bytes.NewReader(kml), maxKMLNestingDepth, maxKMLPlacemarks)
}

// extractKML reads the one KML document out of a KMZ archive. It prefers the
// exact entry name this export writes (kmzKMLEntry), but falls back to the
// first ".kml" entry found for a file this system did not produce - a vendor
// tool or Google Earth's own save is under no obligation to call it
// "doc.kml".
func extractKML(data []byte, maxDecompressed int64) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("berkas bukan KMZ (zip) yang valid: %w", err)
	}
	entry, err := findKMLEntry(zr)
	if err != nil {
		return nil, err
	}
	if int64(entry.UncompressedSize64) > maxDecompressed {
		return nil, fmt.Errorf("berkas %s di dalam KMZ terlalu besar (lebih dari %d MB setelah diekstrak)",
			entry.Name, maxDecompressed>>20)
	}

	rc, err := entry.Open()
	if err != nil {
		return nil, fmt.Errorf("buka %s dari KMZ: %w", entry.Name, err)
	}
	defer rc.Close()

	// Belt and suspenders: UncompressedSize64 above comes from the zip's own
	// central directory, which a crafted archive can misstate. Capping the
	// actual read the same way means a lie in that field costs nothing more
	// than the cap itself, never the memory to hold what it lied about.
	content, err := io.ReadAll(io.LimitReader(rc, maxDecompressed+1))
	if err != nil {
		return nil, fmt.Errorf("baca %s dari KMZ: %w", entry.Name, err)
	}
	if int64(len(content)) > maxDecompressed {
		return nil, fmt.Errorf("berkas %s di dalam KMZ terlalu besar (lebih dari %d MB setelah diekstrak)",
			entry.Name, maxDecompressed>>20)
	}
	return content, nil
}

func findKMLEntry(zr *zip.Reader) (*zip.File, error) {
	var fallback *zip.File
	for _, f := range zr.File {
		if f.Name == kmzKMLEntry {
			return f, nil
		}
		if fallback == nil && strings.HasSuffix(strings.ToLower(f.Name), ".kml") {
			fallback = f
		}
	}
	if fallback != nil {
		return fallback, nil
	}
	return nil, errors.New("KMZ tidak berisi berkas .kml")
}

// depthLimitedTokens wraps a plain *xml.Decoder as an xml.TokenReader,
// refusing once nesting passes maxDepth. Handed to xml.NewTokenDecoder so
// that *every* token the resulting Decoder ever reads passes through here -
// not only the ones walkKML's own loop consumes, but also every one
// DecodeElement and its own internal Skip read while working through a
// single Placemark's contents. Go's unmarshal recurses on the goroutine's
// call stack once per nesting level for exactly that content (measured: a
// few hundred KB of nesting inside one placemark, with the outer loop's own
// depth never above single digits, drove live heap into the hundreds of
// megabytes with the previous version of this guard, which only counted
// what its own loop saw). Binding the limit at the token source itself
// closes that gap regardless of which internal call path does the
// recursing.
type depthLimitedTokens struct {
	dec      *xml.Decoder
	maxDepth int
	depth    int
}

func (d *depthLimitedTokens) Token() (xml.Token, error) {
	tok, err := d.dec.Token()
	if err != nil {
		return tok, err
	}
	switch tok.(type) {
	case xml.StartElement:
		d.depth++
		if d.depth > d.maxDepth {
			return nil, fmt.Errorf("struktur KML terlalu dalam (lebih dari %d level)", d.maxDepth)
		}
	case xml.EndElement:
		d.depth--
	}
	return tok, nil
}

// folderFrame is one open <Folder> on walkKML's own stack, together with the
// depth it was itself opened at - what tells "this folder's own <name>"
// apart from a sibling container's, one level further down.
type folderFrame struct {
	name  string
	depth int
}

// walkKML reads every Placemark in a KML document via an explicit token
// loop, rather than unmarshalling the whole Folder tree in one call: KML
// nowhere bounds how many <Folder> elements a hostile file nests, and a Go
// struct type that mirrored that nesting would ask encoding/xml to recurse
// once per level on the goroutine's own call stack. The depth limit lives in
// depthLimitedTokens below the Decoder itself, so it applies uniformly
// whether the nesting is in the folder tree this loop walks directly or
// inside a single Placemark's own contents, decoded through kmlPlacemark
// via DecodeElement.
func walkKML(r io.Reader, maxDepth, maxPlacemarks int) ([]rawPlacemark, error) {
	tokens := &depthLimitedTokens{dec: xml.NewDecoder(r), maxDepth: maxDepth}
	dec := xml.NewTokenDecoder(tokens)
	var placemarks []rawPlacemark
	var folders []folderFrame

	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			return placemarks, nil
		}
		if err != nil {
			return nil, fmt.Errorf("baca token KML: %w", err)
		}

		if end, ok := tok.(xml.EndElement); ok {
			if end.Name.Local == "Folder" && len(folders) > 0 {
				folders = folders[:len(folders)-1]
			}
			continue
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}

		placemarks, err = walkStartElement(dec, start, &folders, placemarks, maxPlacemarks, tokens.depth)
		if err != nil {
			return nil, err
		}
	}
}

// walkStartElement handles the three element kinds walkKML's loop cares
// about, kept separate so walkKML itself stays inside the project's function
// length guideline. depth is this start element's own depth (depthLimitedTokens
// increments before handing the token back), used to tell a folder's own
// <name> apart from one belonging to a sibling container such as
// <GroundOverlay> or <NetworkLink> - both ordinary in a field survey, and
// both carry a <name> of their own one level further down than the
// folder's, which must not overwrite it.
func walkStartElement(
	dec *xml.Decoder, start xml.StartElement, folders *[]folderFrame,
	placemarks []rawPlacemark, maxPlacemarks, depth int,
) ([]rawPlacemark, error) {
	switch start.Name.Local {
	case "Folder":
		*folders = append(*folders, folderFrame{depth: depth})
	case "name":
		if len(*folders) == 0 || depth != (*folders)[len(*folders)-1].depth+1 {
			return placemarks, nil // Document's own <name>, or a sibling container's
		}
		var value string
		if err := dec.DecodeElement(&value, &start); err != nil {
			return nil, fmt.Errorf("baca nama folder: %w", err)
		}
		(*folders)[len(*folders)-1].name = value
	case "Placemark":
		if len(placemarks) >= maxPlacemarks {
			return nil, fmt.Errorf("terlalu banyak placemark dalam satu berkas (lebih dari %d)", maxPlacemarks)
		}
		var pm kmlPlacemark
		if err := dec.DecodeElement(&pm, &start); err != nil {
			return nil, fmt.Errorf("baca placemark: %w", err)
		}
		placemarks = append(placemarks, rawPlacemark{Placemark: pm, Folder: currentFolder(*folders)})
	}
	return placemarks, nil
}

func currentFolder(folders []folderFrame) string {
	if len(folders) == 0 {
		return ""
	}
	return folders[len(folders)-1].name
}
