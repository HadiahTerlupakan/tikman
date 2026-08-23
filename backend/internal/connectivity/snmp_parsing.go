package connectivity

import (
	"log"
	"strconv"
	"strings"
)

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// isValidHex checks if a string contains only valid hexadecimal characters
func isValidHex(s string) bool {
	for _, r := range s {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') && (r < 'A' || r > 'F') {
			return false
		}
	}
	return true
}

// cleanString removes non-alphanumeric characters from a string
func cleanString(s string) string {
	if s == "" {
		return ""
	}
	result := make([]rune, 0, len(s))
	for _, r := range s {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == ' ' || r == '-' || r == '_' {
			result = append(result, r)
		}
	}
	return strings.TrimSpace(string(result))
}

// toInt64 extracts an integer from an SNMP value, reporting whether it was numeric.
func toInt64(value interface{}) (int64, bool) {
	switch v := value.(type) {
	case int:
		return int64(v), true
	case int64:
		return v, true
	case uint:
		return int64(v), true
	case uint32:
		return int64(v), true
	case uint64:
		return int64(v), true
	default:
		return 0, false
	}
}

// ExtractSerialNumber extracts serial number from SNMP value and converts to hex string
// Following exact approach from https://github.com/Cepat-Kilat-Teknologi/snmp-olt-zte:
// - ZTE returns serial as 8-byte binary/octet string (e.g., [82 84 69 71 ...])
// - We convert to lowercase hex: 52544547... = ASCII "ZTEG" + remaining bytes
// - Also handles "1," prefix removal if present (some firmware versions)
func ExtractSerialNumber(oidValue any) string {
	switch v := oidValue.(type) {
	case string:
		// Already a string, but may have "1," prefix from some firmwares
		s := v
		if strings.HasPrefix(s, "1,") {
			return s[2:]
		}
		return s

	case []byte:
		data := v

		// Handle ASN.1 encoded octet string (common for ZTE firmware V2.x)
		// Format: [tag][length][data...] where tag=0x31, length=data length
		if len(data) >= 2 && data[0] == 0x31 {
			// Remove ASN.1 header (first 2 bytes)
			if len(data) > 2 {
				data = data[2:]
				log.Printf("[Serial] Stripped ASN.1 header: %v -> %q", v, string(data))
			} else {
				return ""
			}
		}

		// Check if it starts with "1," prefix (some firmware versions add this)
		str := string(data)
		if strings.HasPrefix(str, "1,") {
			str = str[2:]
			log.Printf("[Serial] Removed '1,' prefix: %q -> %q", string(data), str)
		}

		// If remaining data looks like hex string (only hex chars), convert to ASCII
		if len(str) >= 8 && isValidHex(str[:min(8, len(str))]) {
			// Try to decode as ASCII first, fallback to hex
			asciiDecoded := ""
			for _, b := range str {
				if b >= 48 && b <= 57 { // '0'-'9'
					asciiDecoded += string(b)
				} else if b >= 65 && b <= 70 { // 'A'-'F'
					// Skip or handle differently based on your use case
				} else if b >= 97 && b <= 102 { // 'a'-'f'
					// Skip or handle differently based on your use case
				} else {
					break
				}
			}

			if len(asciiDecoded) >= 8 {
				// Return printable ASCII serial number
				log.Printf("[Serial] Returned ASCII serial: %q", asciiDecoded)
				return asciiDecoded
			}
		}

		// Default: return as-is string
		log.Printf("[Serial] Returning: %q (type=%T)", str, oidValue)
		return str

	default:
		// Data type is not recognized
		log.Printf("[Serial] Unknown type: %T, value=%v", oidValue, oidValue)
		return ""
	}
}

// ExtractName extracts name/description from SNMP value
// Handles both string and byte slice types
func ExtractName(oidValue any) string {
	switch v := oidValue.(type) {
	case string:
		v = strings.TrimPrefix(v, "1,")
		return cleanString(v)
	case []byte:
		s := string(v)
		s = strings.TrimPrefix(s, "1,")
		return cleanString(s)
	default:
		return ""
	}
}

// printableText reads a text value keeping every printable character, and is
// the counterpart to ExtractName for values whose punctuation carries meaning.
//
// ExtractName runs cleanString, which keeps only alphanumerics, space, "-" and
// "_". That is right for the ZTE description fields it was written for, but it
// destroys three things HSGQ reports: an interface name ("gpon-onu_1/1/1:1"
// becomes "gpon-onu_1111", losing the PON position), a firmware version
// ("V6.0.3P1T1" becomes "V603P1T1") and an ONU name ("ONU01/01" becomes
// "ONU0101"). cleanString is left alone because the ZTE paths depend on it.
//
// Control bytes are still dropped, so a malformed value cannot smuggle escape
// sequences into an operator's terminal.
func printableText(value any) string {
	var raw string
	switch v := value.(type) {
	case []byte:
		raw = string(v)
	case string:
		raw = v
	default:
		return ""
	}

	printable := make([]rune, 0, len(raw))
	for _, r := range raw {
		if r >= 0x20 && r <= 0x7E {
			printable = append(printable, r)
		}
	}

	return strings.TrimSpace(string(printable))
}

// parseZxGponSuffix decodes the OID suffix after a ZXGPON table base into an
// ONT location. Accepts both <ifIndex>.<onuIndex> and <ifIndex>.<onuIndex>.<sub>
// shapes, since the optical power tables carry a trailing sub-instance.
func parseZxGponSuffix(oid, base string) (ONTLocation, bool) {
	trimmed := strings.TrimPrefix(oid, ".")
	baseTrimmed := strings.TrimPrefix(base, ".")
	if !strings.HasPrefix(trimmed, baseTrimmed+".") {
		return ONTLocation{}, false
	}

	parts := strings.Split(strings.TrimPrefix(trimmed, baseTrimmed+"."), ".")
	if len(parts) < 2 {
		return ONTLocation{}, false
	}

	ifIndexStr, err := strconv.ParseUint(parts[0], 10, 32)
	if err != nil {
		return ONTLocation{}, false
	}
	onuIndex, err := strconv.Atoi(parts[1])
	if err != nil {
		return ONTLocation{}, false
	}

	slot, port, ok := decodeZxGponIfIndex(uint32(ifIndexStr))
	if !ok {
		return ONTLocation{}, false
	}

	return ONTLocation{Slot: slot, Port: port, ONTID: onuIndex}, true
}

// decodeZxGponIfIndex decodes ZXGPON ifIndex format (0xFFSSPP00)
// Example: 0x10030100 (268632320) -> frame=1, slot=3, port=1
func decodeZxGponIfIndex(ifIndex uint32) (slot, port int, ok bool) {
	frame := (ifIndex >> 24) & 0xFF
	slot = int((ifIndex >> 16) & 0xFF)
	port = int((ifIndex >> 8) & 0xFF)

	if frame != 0x10 {
		return 0, 0, false
	}
	if slot == 0 || port == 0 {
		return 0, 0, false
	}

	return slot, port, true
}
