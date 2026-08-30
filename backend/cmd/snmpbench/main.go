// Command snmpbench measures what an OLT's SNMP agent will actually serve.
//
// The scaling budget rests on GETBULK cutting round trips by roughly its
// repetition count, and on the agent agreeing to serve requests that large.
// Some agents answer a big GETBULK with tooBig, or truncate it. Which is true
// for this hardware decides the polling design, so it is measured against a
// real chassis before any polling code changes.
//
// It reads the OLT through the same config and database the worker uses, so no
// credential is handled outside the application.
package main

import (
	"flag"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gosnmp/gosnmp"
	"github.com/tikman/olt-provisioning/internal/config"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/database"
	"github.com/tikman/olt-provisioning/internal/models"
)

// defaultRepetitions brackets the useful range: small enough that almost any
// agent serves it, large enough to pass the point where bigger requests stop
// paying for themselves.
const defaultRepetitions = "5,10,15,25,50,100"

func main() {
	name := flag.String("olt", "", "name of the OLT to measure")
	oid := flag.String("oid", connectivity.OID_ZXAN_ONU_PHASE_STATE_TABLE, "table OID to walk")
	repsList := flag.String("reps", defaultRepetitions, "comma-separated GETBULK repetition counts to try")
	print := flag.Bool("print", false, "print the values instead of timing the walk")
	flag.Parse()

	if *name == "" {
		log.Fatal("-olt is required")
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}

	var olt models.OLT
	if err := db.First(&olt, "name = ?", *name).Error; err != nil {
		log.Fatalf("find OLT %q: %v", *name, err)
	}

	fmt.Printf("OLT %s (%s:%d), table %s\n\n", olt.Name, olt.IPAddress, olt.SNMPPort, *oid)

	// Reading a table is the other half of asking what an agent serves: a count
	// says the MIB is implemented, the values say what it was configured with.
	if *print {
		if err := dump(&olt, *oid); err != nil {
			log.Fatalf("walk %s: %v", *oid, err)
		}
		return
	}

	fmt.Printf("%-18s %8s %10s %12s\n", "mode", "values", "elapsed", "values/sec")

	baseline := run(&olt, *oid, 0)
	report("GETNEXT", baseline)

	for _, reps := range parseReps(*repsList) {
		result := run(&olt, *oid, reps)
		report(fmt.Sprintf("GETBULK@%d", reps), result)

		// A run that returns fewer values than GETNEXT did is the failure that
		// matters and the one that does not announce itself: the walk looks like
		// it worked and the missing ONUs simply never appear.
		if result.err == nil && baseline.err == nil && result.values != baseline.values {
			fmt.Printf("  ^ MISMATCH: %d values against GETNEXT's %d — do not use this setting\n",
				result.values, baseline.values)
		}
	}
}

// parseReps turns the flag into repetition counts, ignoring anything that is
// not a usable one so a typo costs a line of the table rather than the run.
func parseReps(list string) []uint8 {
	reps := make([]uint8, 0, 8)
	for _, field := range strings.Split(list, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(field))
		if err != nil || n < 1 || n > 255 {
			continue
		}
		reps = append(reps, uint8(n))
	}
	return reps
}

// dump prints every value under an OID.
func dump(olt *models.OLT, oid string) error {
	client := &gosnmp.GoSNMP{
		Target:         olt.IPAddress,
		Port:           uint16(olt.SNMPPort),
		Community:      olt.SNMPCommunity,
		Version:        gosnmp.Version2c,
		Timeout:        5 * time.Second,
		Retries:        1,
		MaxRepetitions: 10,
	}
	if err := client.Connect(); err != nil {
		return err
	}
	defer func() { _ = client.Conn.Close() }()

	return client.BulkWalk(oid, func(pdu gosnmp.SnmpPDU) error {
		switch value := pdu.Value.(type) {
		case []byte:
			// Addresses and community strings come back as octet strings, which
			// are printable here and hex elsewhere; show both so neither case
			// needs a second run to read.
			fmt.Printf("%s = %q (hex %x)\n", pdu.Name, string(value), value)
		default:
			fmt.Printf("%s = %v\n", pdu.Name, value)
		}
		return nil
	})
}

type outcome struct {
	values  int
	elapsed time.Duration
	err     error
}

func run(olt *models.OLT, oid string, reps uint8) outcome {
	client := &gosnmp.GoSNMP{
		Target:         olt.IPAddress,
		Port:           uint16(olt.SNMPPort),
		Community:      olt.SNMPCommunity,
		Version:        gosnmp.Version2c,
		Timeout:        5 * time.Second,
		Retries:        1,
		MaxRepetitions: uint32(reps),
	}
	if err := client.Connect(); err != nil {
		return outcome{err: err}
	}
	defer func() { _ = client.Conn.Close() }()

	values := 0
	count := func(gosnmp.SnmpPDU) error { values++; return nil }

	started := time.Now()
	var err error
	if reps == 0 {
		err = client.Walk(oid, count)
	} else {
		err = client.BulkWalk(oid, count)
	}
	return outcome{values: values, elapsed: time.Since(started), err: err}
}

func report(mode string, o outcome) {
	if o.err != nil {
		fmt.Printf("%-18s %8d %10s  ERROR: %v\n", mode, o.values, o.elapsed.Round(time.Millisecond), o.err)
		return
	}
	perSecond := float64(o.values) / o.elapsed.Seconds()
	fmt.Printf("%-18s %8d %10s %12.0f\n", mode, o.values, o.elapsed.Round(time.Millisecond), perSecond)
}
