package connectivity

import (
	"net"
	"sort"
	"strings"
	"testing"

	"github.com/gosnmp/gosnmp"
)

// uncfgAgent is a minimal SNMP v2c agent serving a fixed table over UDP. The
// autofind walk is the part of this module that talks to a real OLT, so it is
// exercised against real GetNext traffic rather than a stubbed client.
type uncfgAgent struct {
	t      *testing.T
	conn   *net.UDPConn
	oids   []string
	values map[string]gosnmp.SnmpPDU
}

// newUncfgAgent starts an agent on a loopback port and returns it with that
// port. The agent stops when the test finishes.
func newUncfgAgent(t *testing.T, pdus []gosnmp.SnmpPDU) (*uncfgAgent, int) {
	t.Helper()

	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	agent := &uncfgAgent{t: t, conn: conn, values: make(map[string]gosnmp.SnmpPDU, len(pdus))}
	for _, pdu := range pdus {
		name := normaliseOID(pdu.Name)
		agent.oids = append(agent.oids, name)
		pdu.Name = name
		agent.values[name] = pdu
	}
	// GetNext requires lexicographic order by numeric arc, which string sorting
	// does not give (".10" would precede ".2").
	sort.Slice(agent.oids, func(i, j int) bool {
		return compareOIDs(agent.oids[i], agent.oids[j]) < 0
	})

	go agent.serve()
	t.Cleanup(func() { _ = conn.Close() })

	return agent, conn.LocalAddr().(*net.UDPAddr).Port
}

func (a *uncfgAgent) serve() {
	client := &gosnmp.GoSNMP{Version: gosnmp.Version2c, Community: "public"}
	buf := make([]byte, 4096)

	for {
		n, addr, err := a.conn.ReadFromUDP(buf)
		if err != nil {
			return // closed by cleanup
		}

		request, err := client.SnmpDecodePacket(buf[:n])
		if err != nil {
			continue
		}

		response := &gosnmp.SnmpPacket{
			Version:   gosnmp.Version2c,
			Community: request.Community,
			PDUType:   gosnmp.GetResponse,
			RequestID: request.RequestID,
		}

		// A GET asks for one exact instance and a GetNext asks for the one
		// after it. Answering both as GetNext made the double lie: code that
		// fetches an instance directly got its neighbour's value back.
		for _, requested := range request.Variables {
			if request.PDUType == gosnmp.GetRequest {
				response.Variables = append(response.Variables, a.exact(requested.Name))
				continue
			}
			response.Variables = append(response.Variables, a.next(requested.Name))
		}

		out, err := response.MarshalMsg()
		if err != nil {
			continue
		}
		_, _ = a.conn.WriteToUDP(out, addr)
	}
}

// exact answers a Get, reporting noSuchInstance when the OID is not one the
// agent holds.
func (a *uncfgAgent) exact(requested string) gosnmp.SnmpPDU {
	target := normaliseOID(requested)
	for _, oid := range a.oids {
		if compareOIDs(oid, target) == 0 {
			return a.values[oid]
		}
	}
	return gosnmp.SnmpPDU{Name: requested, Type: gosnmp.NoSuchInstance}
}

// next answers a GetNext by returning the first OID strictly greater than the
// requested one, or endOfMibView once the table is exhausted.
func (a *uncfgAgent) next(requested string) gosnmp.SnmpPDU {
	target := normaliseOID(requested)
	for _, oid := range a.oids {
		if compareOIDs(oid, target) > 0 {
			return a.values[oid]
		}
	}
	return gosnmp.SnmpPDU{Name: target, Type: gosnmp.EndOfMibView}
}

func normaliseOID(oid string) string {
	return "." + strings.TrimPrefix(oid, ".")
}

func compareOIDs(a, b string) int {
	left, right := strings.Split(strings.TrimPrefix(a, "."), "."), strings.Split(strings.TrimPrefix(b, "."), ".")

	for i := 0; i < len(left) && i < len(right); i++ {
		if left[i] == right[i] {
			continue
		}
		if len(left[i]) != len(right[i]) {
			return len(left[i]) - len(right[i])
		}
		return strings.Compare(left[i], right[i])
	}

	return len(left) - len(right)
}

// uncfgOID builds a full autofind column OID for the given ifIndex and sequence.
func uncfgOID(prefix string, ifIndex uint32, seq int) string {
	return normaliseOID(BaseOID2) + prefix + "." + itoa(int(ifIndex)) + "." + itoa(seq)
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	var digits []byte
	for v > 0 {
		digits = append([]byte{byte('0' + v%10)}, digits...)
		v /= 10
	}
	return string(digits)
}
