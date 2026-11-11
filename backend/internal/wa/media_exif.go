package wa

import "encoding/binary"

const (
	jpegMarkerSOS     = 0xDA
	jpegMarkerAPP1    = 0xE1
	exifOrientationID = 0x0112
	exifIFDEntrySize  = 12
)

// jpegOrientation answers the EXIF Orientation of a JPEG, or 0 when it has
// none. Go's decoder ignores the tag, so a portrait phone photo measures as
// landscape; the caller needs to know to leave its layout to the client.
func jpegOrientation(data []byte) int {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return 0
	}
	for pos := 2; pos+4 <= len(data) && data[pos] == 0xFF; {
		marker := data[pos+1]
		size := int(binary.BigEndian.Uint16(data[pos+2:]))
		if marker == jpegMarkerSOS || size < 2 || pos+2+size > len(data) {
			return 0
		}
		if marker == jpegMarkerAPP1 {
			if o := exifOrientation(data[pos+4 : pos+2+size]); o > 0 {
				return o
			}
		}
		pos += 2 + size
	}
	return 0
}

// exifOrientation reads the Orientation entry of IFD0 in an APP1 payload.
func exifOrientation(app1 []byte) int {
	const header = "Exif\x00\x00"
	if len(app1) < len(header)+8 || string(app1[:len(header)]) != header {
		return 0
	}
	tiff := app1[len(header):]
	var order binary.ByteOrder = binary.LittleEndian
	if string(tiff[:2]) == "MM" {
		order = binary.BigEndian
	}
	ifd := int(order.Uint32(tiff[4:]))
	if ifd < 8 || ifd+2 > len(tiff) {
		return 0
	}
	count := int(order.Uint16(tiff[ifd:]))
	for i := 0; i < count; i++ {
		entry := ifd + 2 + i*exifIFDEntrySize
		if entry+exifIFDEntrySize > len(tiff) {
			return 0
		}
		if order.Uint16(tiff[entry:]) == exifOrientationID {
			return int(order.Uint16(tiff[entry+8:]))
		}
	}
	return 0
}
