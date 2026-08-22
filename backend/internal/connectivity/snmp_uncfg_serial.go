package connectivity

import (
	"fmt"
	"sort"
	"strings"
)

// autofindSerialLen is the fixed ZTE autofind serial width: a 4-character
// vendor ID followed by a 4-byte ONU identifier.
const autofindSerialLen = 8

// formatAutofindSerial renders an autofind serial the way the OLT CLI does.
// The value arrives as 8 raw bytes where the first 4 are printable ASCII vendor
// characters and the last 4 are binary, e.g. 48 57 54 43 B4 03 E8 A0 becomes
// "HWTCB403E8A0". ExtractSerialNumber cannot be reused because it assumes the
// whole value is either text or hex, never this mixed layout.
func formatAutofindSerial(value any) string {
	raw, ok := value.([]byte)
	if !ok {
		if s, isString := value.(string); isString {
			raw = []byte(s)
		} else {
			return ""
		}
	}

	if len(raw) != autofindSerialLen {
		return ""
	}

	vendor := raw[:4]
	for _, b := range vendor {
		if b < 0x20 || b > 0x7E {
			return ""
		}
	}

	var sb strings.Builder
	sb.Write(vendor)
	for _, b := range raw[4:] {
		fmt.Fprintf(&sb, "%02X", b)
	}

	return sb.String()
}

// sortUnconfiguredONUs orders results by physical position so the list stays
// stable across polls despite SNMP map iteration.
func sortUnconfiguredONUs(onus []UnconfiguredONU) {
	sort.Slice(onus, func(i, j int) bool {
		if onus[i].Slot != onus[j].Slot {
			return onus[i].Slot < onus[j].Slot
		}
		if onus[i].Port != onus[j].Port {
			return onus[i].Port < onus[j].Port
		}
		return onus[i].SerialNumber < onus[j].SerialNumber
	})
}
