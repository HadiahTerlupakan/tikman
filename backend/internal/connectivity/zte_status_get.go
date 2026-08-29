package connectivity

import (
	"context"
	"fmt"
)

// StatusQuerier is implemented by drivers that can read the phase state of
// named ONUs directly, rather than walking the table for every ONU on the OLT.
// A caller that already knows which ONU it wants has no reason to sweep.
type StatusQuerier interface {
	QueryStatusFor(ipAddress, community string, snmpPort int, locations []ONTLocation) (map[ONTLocation]int, error)
}

// QueryStatusFor reads the phase state of known ONUs with batched GETs.
//
// The phase state table is indexed the same way as the optical tables:
// ifIndex.onuID in the TYPE space. A location is absent from the result when
// the OLT holds no state for it.
func (zteDriver) QueryStatusFor(ipAddress, community string, snmpPort int, locations []ONTLocation) (map[ONTLocation]int, error) {
	statuses := make(map[ONTLocation]int, len(locations))
	if len(locations) == 0 {
		return statuses, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), zteReadDeadline)
	defer cancel()

	client, err := newSNMPClientWithContext(ctx, ipAddress, community, snmpPort)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Conn.Close() }()

	for start := 0; start < len(locations); start += zteGetBatchSize {
		end := start + zteGetBatchSize
		if end > len(locations) {
			end = len(locations)
		}
		batch := locations[start:end]

		oids := make([]string, len(batch))
		for i, loc := range batch {
			typeSpace := OnuTypeIfIndexBase + loc.Slot*OnuTypeSlotStride + loc.Port*OnuTypeIncrement
			oids[i] = fmt.Sprintf("%s.%d.%d", OID_ZXAN_ONU_PHASE_STATE_TABLE, typeSpace, loc.ONTID)
		}

		result, err := client.Get(oids)
		if err != nil || result == nil {
			continue
		}
		for i, pdu := range result.Variables {
			if i >= len(batch) {
				break
			}
			// The walk this mirrors discards 30000 and above the same way: the
			// OLT reports those for an ONU it has no signal reading for.
			if value, ok := toInt64(pdu.Value); ok && value < 30000 {
				statuses[batch[i]] = int(value)
			}
		}
	}

	return statuses, nil
}
