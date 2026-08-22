package connectivity

import (
	"context"
	"testing"
	"time"

	"github.com/gosnmp/gosnmp"
)

// blackholeIP is in RFC 5737 TEST-NET-3. Unlike TEST-NET-1, which this host
// rejects immediately with "no route to host", packets here are dropped without
// a reply, so the walk can only end by hitting a deadline.
const blackholeIP = "203.0.113.1"

// ifIndex 268632320 is slot 3 port 1, 268632576 is slot 3 port 2 and 268698880
// is slot 4 port 5 under the ZTE frame/slot/port encoding.
const (
	ifIndexSlot3Port1 = 268632320
	ifIndexSlot3Port2 = 268632576
	ifIndexSlot4Port5 = 268698880
)

func TestWalkUnconfiguredONUs_ReadsAutofindTable(t *testing.T) {
	_, port := newUncfgAgent(t, []gosnmp.SnmpPDU{
		{
			Name:  uncfgOID(OnuUncfgSerialNumberPrefix, ifIndexSlot3Port1, 1),
			Type:  gosnmp.OctetString,
			Value: []byte{0x48, 0x57, 0x54, 0x43, 0xB4, 0x03, 0xE8, 0xA0},
		},
		{
			Name:  uncfgOID(OnuUncfgDeviceTypePrefix, ifIndexSlot3Port1, 1),
			Type:  gosnmp.OctetString,
			Value: []byte("HG8245H5"),
		},
		{
			Name:  uncfgOID(OnuUncfgSoftwareVerPrefix, ifIndexSlot3Port1, 1),
			Type:  gosnmp.OctetString,
			Value: []byte("V5R019C00S105"),
		},
		{
			Name:  uncfgOID(OnuUncfgSerialNumberPrefix, ifIndexSlot4Port5, 2),
			Type:  gosnmp.OctetString,
			Value: []byte{0x5A, 0x54, 0x45, 0x47, 0xCA, 0xFF, 0xC2, 0xFD},
		},
	})

	onus, err := walkUnconfiguredONUs(context.Background(), "127.0.0.1", "public", port)
	if err != nil {
		t.Fatalf("walk: %v", err)
	}

	if len(onus) != 2 {
		t.Fatalf("got %d ONUs, want 2: %+v", len(onus), onus)
	}

	// Slot 3 sorts before slot 4.
	if onus[0] != (UnconfiguredONU{
		Slot: 3, Port: 1, SerialNumber: "HWTCB403E8A0",
		DeviceType: "HG8245H5", SoftwareVersion: "V5R019C00S105",
	}) {
		t.Errorf("first ONU = %+v", onus[0])
	}

	// The OLT reported no model or firmware for this row, so both stay empty
	// rather than borrowing the previous row's values.
	if onus[1] != (UnconfiguredONU{Slot: 4, Port: 5, SerialNumber: "ZTEGCAFFC2FD"}) {
		t.Errorf("second ONU = %+v", onus[1])
	}
}

func TestWalkUnconfiguredONUs_EmptyTableYieldsNoONUs(t *testing.T) {
	_, port := newUncfgAgent(t, nil)

	onus, err := walkUnconfiguredONUs(context.Background(), "127.0.0.1", "public", port)
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if onus == nil {
		t.Error("got nil, want an empty slice so the handler serialises []")
	}
	if len(onus) != 0 {
		t.Errorf("got %d ONUs, want 0", len(onus))
	}
}

func TestWalkUnconfiguredONUs_SkipsRowsWithUnusableSerials(t *testing.T) {
	_, port := newUncfgAgent(t, []gosnmp.SnmpPDU{
		{
			// Truncated serial: the OLT occasionally reports a partial row while
			// the ONU is still being ranged.
			Name:  uncfgOID(OnuUncfgSerialNumberPrefix, ifIndexSlot3Port1, 1),
			Type:  gosnmp.OctetString,
			Value: []byte{0x48, 0x57, 0x54, 0x43},
		},
		{
			Name:  uncfgOID(OnuUncfgSerialNumberPrefix, ifIndexSlot3Port2, 1),
			Type:  gosnmp.OctetString,
			Value: []byte{0x5A, 0x54, 0x45, 0x47, 0xCA, 0xFF, 0xC2, 0xFD},
		},
	})

	onus, err := walkUnconfiguredONUs(context.Background(), "127.0.0.1", "public", port)
	if err != nil {
		t.Fatalf("walk: %v", err)
	}

	if len(onus) != 1 {
		t.Fatalf("got %d ONUs, want 1: %+v", len(onus), onus)
	}
	if onus[0].SerialNumber != "ZTEGCAFFC2FD" || onus[0].Port != 2 {
		t.Errorf("got %+v, want the slot 3 port 2 row", onus[0])
	}
}

func TestWalkUnconfiguredONUs_HonoursContextDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := walkUnconfiguredONUs(ctx, blackholeIP, "public", 161)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected an error walking an unroutable address")
	}

	// Each walk allows 5s with one retry, so an unbounded scan cannot finish
	// this quickly. The bound is what keeps a hung OLT from holding the HTTP
	// request open until the browser's own timeout fires.
	if elapsed > 3*time.Second {
		t.Errorf("walk ran for %v, expected the context deadline to cut it short", elapsed)
	}
}

func TestWalkUnconfiguredONUs_StopsOnCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	_, err := walkUnconfiguredONUs(ctx, blackholeIP, "public", 161)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected an error for an already-cancelled context")
	}
	if elapsed > time.Second {
		t.Errorf("walk ran for %v, expected an immediate return", elapsed)
	}
}
