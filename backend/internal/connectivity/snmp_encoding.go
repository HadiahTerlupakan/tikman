package connectivity

import (
	"fmt"
	"time"
)

// encodeZxGponIfIndex encodes the ZXGPON-MIB ifIndex according to ZTE spec
// Format: 0xFFSSPP00 where FF=frame(0x10), SS=slot, PP=port
// Example: frame=1, slot=3, port=1 -> 0x10030100 = 268632320
func encodeZxGponIfIndex(frame, slot, port int) uint32 {
	return (uint32(frame&0xff) << 24) | (uint32(slot&0xff) << 16) | (uint32(port&0xff) << 8)
}

// decodeOnuIDIfIndex reverses the ONU-ID space ifIndex encoding (BaseOID1)
// ZTE C300 format: 0xFFSSPP00 where:
//
//	FF = frame (0x10)
//	SS = slot number (byte 2)
//	PP = PON port number (byte 1)
//	00 = reserved (byte 0)
//
// Example: 268632320 = 0x10030100 -> slot=3, port=1
func decodeOnuIDIfIndex(ifIndex uint32) (slot, port int, ok bool) {
	slot = int((ifIndex >> 16) & 0xFF)
	port = int((ifIndex >> 8) & 0xFF)

	if slot == 0 || port == 0 {
		return 0, 0, false
	}

	return slot, port, true
}

// decodeOnuTypeIfIndex reverses the TYPE space ifIndex encoding
func decodeOnuTypeIfIndex(ifIndex uint32) (slot, port int, ok bool) {
	base := uint32(OnuTypeIfIndexBase)
	if ifIndex < base {
		return 0, 0, false
	}

	offset := ifIndex - base
	slot = int(offset / uint32(OnuTypeSlotStride))
	remainder := offset % uint32(OnuTypeSlotStride)
	port = int(remainder / uint32(OnuTypeIncrement))

	if slot == 0 || port == 0 {
		return 0, 0, false
	}

	return slot, port, true
}

// encodeOnuIDIfIndex encodes the ONU-ID space ifIndex (BaseOID1)
// ZTE C300 format: 0x1101SSPP where:
//
//	0x1101 = prefix for ONU-ID space
//	SS = slot number
//	PP = PON port number
func encodeOnuIDIfIndex(frame, slot, port int) uint32 {
	return (uint32(frame) << 28) | (0x1101 << 16) | (uint32(slot&0xff) << 8) | uint32(port&0xff)
}

// parseZteHexTimestamp converts ZTE hex timestamp to time.Time
// ZTE timestamps are in format: hex string representing seconds since epoch
// Example: "07 E6 0C 0F 0E 1E 28 00" represents 2022-12-15 14:30:40
func parseZteHexTimestamp(hexBytes []byte) (time.Time, error) {
	if len(hexBytes) < 8 {
		return time.Time{}, fmt.Errorf("invalid hex timestamp length: %d", len(hexBytes))
	}

	// ZTE format: YYYY MM DD HH MM SS MS MS (8 bytes)
	year := int(hexBytes[0])<<8 | int(hexBytes[1])
	month := int(hexBytes[2])
	day := int(hexBytes[3])
	hour := int(hexBytes[4])
	minute := int(hexBytes[5])
	second := int(hexBytes[6])

	// Validate components
	if year < 1970 || year > 2100 || month < 1 || month > 12 || day < 1 || day > 31 {
		return time.Time{}, fmt.Errorf("invalid timestamp components: %d-%d-%d", year, month, day)
	}

	return time.Date(year, time.Month(month), day, hour, minute, second, 0, time.UTC), nil
}

// decodeZxGponPower converts ZTE's raw optical power into dBm. The formula per
// NetManeger is: raw * 0.002 - 30 = dBm. Raw values >= 30000 indicate "no
// optical signal" and map to nil to distinguish missing readings from real 0 dBm.
func decodeZxGponPower(raw int64) *float64 {
	// 65535 (0xFFFF) is the standard "no signal" sentinel in ZTE devices
	if raw == 65535 {
		return nil
	}

	if raw >= 30000 && raw < 32768 {
		return nil // no signal sentinel (for values 30000-32767)
	}

	// ZTE C300 uses unsigned 16-bit integers
	// Values >= 32768 must be treated as signed (subtract 65536 first)
	var adjustedRaw int64
	if raw >= 32768 && raw < 65536 {
		adjustedRaw = raw - 65536 // Convert to signed 16-bit
	} else {
		adjustedRaw = raw
	}

	// Standard ZTE formula: (raw * 0.002) - 30
	dbm := (float64(adjustedRaw) * 0.002) - 30.0
	return &dbm
}
