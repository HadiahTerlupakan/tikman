package connectivity

import (
	"fmt"
	"strings"
)

// queryZTEInventoryFor reads the inventory of specific ONUs by scoping every
// walk to their own subtree, so each returns a varbind or two instead of the
// whole table.
//
// The subtree is walked rather than fetched with a GET because these tables do
// not share one index shape: hardware version is indexed by ifIndex.onuID while
// IP and MAC carry a further element after it. Scoping to ifIndex.onuID is
// exact for all of them without depending on what follows.
func queryZTEInventoryFor(ipAddress, community string, snmpPort int, locations []ONTLocation) (map[ONTLocation]ONTInventory, error) {
	client, err := newSNMPClient(ipAddress, community, snmpPort)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Conn.Close() }()

	return fetchZTEInventory(client, locations), nil
}

// readZTEIPAddress reads the management address as the OLT reports it. The
// name decoder cannot be used here: it strips punctuation, which turns
// 10.0.0.9 into 10009. An unset address reads as 0.0.0.0 and counts as absent.
func readZTEIPAddress(value any) string {
	var text string
	switch typed := value.(type) {
	case string:
		text = typed
	case []byte:
		text = string(typed)
	default:
		return ""
	}
	text = strings.TrimSpace(text)
	if text == "0.0.0.0" {
		return ""
	}
	return text
}

// formatZTEMACAddress renders the six raw bytes the OLT reports. Anything of
// another length is not a MAC and is reported as absent rather than as a
// mangled one.
func formatZTEMACAddress(value any) string {
	raw, ok := value.([]byte)
	if !ok || len(raw) != 6 {
		return ""
	}
	return fmt.Sprintf("%02X:%02X:%02X:%02X:%02X:%02X",
		raw[0], raw[1], raw[2], raw[3], raw[4], raw[5])
}
