package connectivity

import (
	"context"

	"github.com/gosnmp/gosnmp"
)

// ifNameRepetitions is how many ifName values one GETBULK asks for. A BRAS
// lists an interface per PPP session, so the column runs to thousands of rows:
// at 50 a 2000-row walk is about 40 requests, where the default of 10 (tuned
// for a slow ZTE agent) would need 200.
const ifNameRepetitions = 50

// WalkIfNames reads ifName alone and returns each name's ifIndex. Only this
// client asks for more repetitions; every other walk keeps the package setting.
func WalkIfNames(ctx context.Context, host string, port int, community string) (map[string]int, error) {
	client, err := newSNMPClientWithContext(ctx, host, community, port)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Conn.Close() }()
	client.Timeout = ifCounterRequestTimeout
	client.MaxRepetitions = ifNameRepetitions

	names := map[string]int{}
	err = bulkWalk(client, oidIfName, func(pdu gosnmp.SnmpPDU) error {
		index, err := lastSubidentifier(pdu.Name)
		if err != nil {
			return err
		}
		names[pduText(pdu)] = index
		return nil
	})
	if err != nil {
		return nil, err
	}
	return names, nil
}
