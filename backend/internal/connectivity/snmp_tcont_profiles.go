package connectivity

import (
	"fmt"
	"sort"

	"github.com/gosnmp/gosnmp"
)

// ZTETcontProfile is one T-CONT profile with the bandwidths it grants. The OLT
// reports kbps; the speed shown to an operator is derived from these rather
// than from the profile's name, which is only a label and can lie.
type ZTETcontProfile struct {
	Name      string `json:"name"`
	Type      int    `json:"type"`
	FixedBW   int    `json:"fixed_bw_kbps"`
	AssuredBW int    `json:"assured_bw_kbps"`
	MaxBW     int    `json:"max_bw_kbps"`
}

// The ZTE T-CONT profile table, indexed by an opaque profile ID. Its columns
// carry the same figures as "show gpon profile tcont", so the profile list
// needs no CLI session.
const (
	zteTcontProfileName    = ".1.3.6.1.4.1.3902.1012.3.26.1.1.2"
	zteTcontProfileFixed   = ".1.3.6.1.4.1.3902.1012.3.26.1.1.3"
	zteTcontProfileAssured = ".1.3.6.1.4.1.3902.1012.3.26.1.1.4"
	zteTcontProfileMax     = ".1.3.6.1.4.1.3902.1012.3.26.1.1.5"
	zteTcontProfileType    = ".1.3.6.1.4.1.3902.1012.3.26.1.1.6"
)

// WalkTcontProfiles reads the T-CONT profiles and their bandwidths over SNMP.
// Verified against the same OLT's CLI listing: default is type 1 with 10000
// kbps fixed, 1G is type 3 with 1024000 kbps maximum.
func WalkTcontProfiles(ipAddress, community string, snmpPort int) ([]ZTETcontProfile, error) {
	client, err := newSNMPClient(ipAddress, community, snmpPort)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Conn.Close() }()

	byID := make(map[int]*ZTETcontProfile)
	order := make([]int, 0)

	if err := client.Walk(zteTcontProfileName, func(pdu gosnmp.SnmpPDU) error {
		id, ok := lastOIDSegment(pdu.Name)
		if !ok {
			return nil
		}
		name := printableText(pdu.Value)
		if name == "" {
			return nil
		}
		byID[id] = &ZTETcontProfile{Name: name}
		order = append(order, id)
		return nil
	}); err != nil {
		return nil, fmt.Errorf("T-CONT profile walk failed: %w", err)
	}

	// Only profiles the name column listed are filled in: a bandwidth with no
	// profile behind it has nothing to belong to.
	collect := func(oid string, apply func(profile *ZTETcontProfile, value int)) {
		_ = client.Walk(oid, func(pdu gosnmp.SnmpPDU) error {
			id, ok := lastOIDSegment(pdu.Name)
			if !ok {
				return nil
			}
			profile, named := byID[id]
			if !named {
				return nil
			}
			if value, ok := toInt64(pdu.Value); ok {
				apply(profile, int(value))
			}
			return nil
		})
	}

	collect(zteTcontProfileFixed, func(p *ZTETcontProfile, v int) { p.FixedBW = v })
	collect(zteTcontProfileAssured, func(p *ZTETcontProfile, v int) { p.AssuredBW = v })
	collect(zteTcontProfileMax, func(p *ZTETcontProfile, v int) { p.MaxBW = v })
	collect(zteTcontProfileType, func(p *ZTETcontProfile, v int) { p.Type = v })

	sort.Ints(order)
	profiles := make([]ZTETcontProfile, 0, len(order))
	for _, id := range order {
		profiles = append(profiles, *byID[id])
	}

	return profiles, nil
}
