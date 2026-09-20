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
	// memory. Generous next to any plant this ISP runs today, but not
	// unbounded - a preview response has to stay renderable.
	maxKMLPlacemarks = 200_000
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

// walkKML reads every Placemark in a KML document via an explicit token
// loop, rather than unmarshalling the whole Folder tree in one call: KML
// nowhere bounds how many <Folder> elements a hostile file nests, and a Go
// struct type that mirrored that nesting would ask encoding/xml to recurse
// once per level on the goroutine's own call stack. depth is tracked here on
// the heap instead, so maxDepth rejects a pathological file long before that
// recursion would ever be attempted.
//
// A single Placemark's own contents are decoded through kmlPlacemark
// (DecodeElement), which has no recursive field anywhere in its shape - an
// attacker nesting garbage inside one placemark instead of in the folder
// tree hits encoding/xml's own Skip, which is an iterative loop for exactly
// this reason.
func walkKML(r io.Reader, maxDepth, maxPlacemarks int) ([]rawPlacemark, error) {
	dec := xml.NewDecoder(r)
	var placemarks []rawPlacemark
	var folders []string
	depth := 0

	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			return placemarks, nil
		}
		if err != nil {
			return nil, fmt.Errorf("baca token KML: %w", err)
		}

		if end, ok := tok.(xml.EndElement); ok {
			depth--
			if end.Name.Local == "Folder" && len(folders) > 0 {
				folders = folders[:len(folders)-1]
			}
			continue
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}

		depth++
		if depth > maxDepth {
			return nil, fmt.Errorf("struktur folder KML terlalu dalam (lebih dari %d level)", maxDepth)
		}

		placemarks, err = walkStartElement(dec, start, &folders, &depth, placemarks, maxPlacemarks)
		if err != nil {
			return nil, err
		}
	}
}

// walkStartElement handles the three element kinds walkKML's loop cares
// about, kept separate so walkKML itself stays inside the project's function
// length guideline.
func walkStartElement(
	dec *xml.Decoder, start xml.StartElement, folders *[]string, depth *int,
	placemarks []rawPlacemark, maxPlacemarks int,
) ([]rawPlacemark, error) {
	switch start.Name.Local {
	case "Folder":
		*folders = append(*folders, "")
	case "name":
		if len(*folders) == 0 {
			return placemarks, nil // Document's own <name>, not a folder's
		}
		var value string
		if err := dec.DecodeElement(&value, &start); err != nil {
			return nil, fmt.Errorf("baca nama folder: %w", err)
		}
		*depth--
		(*folders)[len(*folders)-1] = value
	case "Placemark":
		if len(placemarks) >= maxPlacemarks {
			return nil, fmt.Errorf("terlalu banyak placemark dalam satu berkas (lebih dari %d)", maxPlacemarks)
		}
		var pm kmlPlacemark
		if err := dec.DecodeElement(&pm, &start); err != nil {
			return nil, fmt.Errorf("baca placemark: %w", err)
		}
		*depth--
		placemarks = append(placemarks, rawPlacemark{Placemark: pm, Folder: currentFolder(*folders)})
	}
	return placemarks, nil
}

func currentFolder(folders []string) string {
	if len(folders) == 0 {
		return ""
	}
	return folders[len(folders)-1]
}
