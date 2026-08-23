package connectivity

import (
	"testing"
	"time"

	"github.com/gosnmp/gosnmp"
)

// A zero timeout used to make SNMPTest give up before the device could answer,
// so creating an OLT failed with "request timeout" even when SNMP was perfectly
// reachable - and the caller in olt_service.go passes exactly zero. Served by a
// live local agent, this proves the clamp lets a reachable device respond.
func TestSNMPTestClampsNonPositiveTimeout(t *testing.T) {
	_, port := newUncfgAgent(t, []gosnmp.SnmpPDU{
		{Name: ".1.3.6.1.2.1.1.1.0", Type: gosnmp.OctetString, Value: []byte("HSGQ-XE08ID")},
	})

	for _, timeout := range []time.Duration{0, -1 * time.Second} {
		if err := SNMPTest("127.0.0.1", port, "public", timeout); err != nil {
			t.Errorf("SNMPTest(timeout=%v) = %v, want success against a live agent", timeout, err)
		}
	}
}
