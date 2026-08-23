// Command probe_hsgq resolves the four HSGQ unknowns that phase 1 of the
// multi-vendor spec refused to guess. It is read-only: it walks SNMP tables and
// prints what it found, never writing to the OLT.
//
// Usage:
//
//	go run ./cmd/probe_hsgq -host 10.0.0.5 -community public [-port 161]
//
// Hand the output back to whoever is closing out phase 2. Each section names the
// constant in internal/connectivity/driver_hsgq.go that its finding unblocks.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/tikman/olt-provisioning/internal/connectivity"
)

func main() {
	host := flag.String("host", "", "OLT IP address (required)")
	community := flag.String("community", "public", "SNMP v2c community")
	port := flag.Int("port", 161, "SNMP port")
	flag.Parse()

	if *host == "" {
		fmt.Fprintln(os.Stderr, "-host is required")
		flag.Usage()
		os.Exit(2)
	}

	report, err := connectivity.ProbeHSGQ(context.Background(), *host, *community, *port)
	if err != nil {
		fmt.Fprintf(os.Stderr, "probe failed: %v\n", err)
		if errors.Is(err, connectivity.ErrUnsupported) {
			fmt.Fprintln(os.Stderr, "\nThe status table was empty. That is a result, not a bug: the OIDs taken")
			fmt.Fprintln(os.Stderr, "from the third-party reference do not match this firmware, which is the")
			fmt.Fprintln(os.Stderr, "risk the spec flagged. Capture the vendor's own MIB before going further.")
		}
		os.Exit(1)
	}

	printInventory(report)
	printIfIndexFindings(report)
	printPolarityFinding(report)
	printSentinelFinding(report)
	printSerialFinding(report)
}

func printInventory(report connectivity.HSGQProbeReport) {
	fmt.Printf("Found %d ONUs in the HSGQ status table.\n\n", len(report.Rows))

	out := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(out, "IFINDEX\tIF LABEL\tSTATUS\tRX RAW\tNAME")
	for _, row := range report.Rows {
		status := "-"
		if row.HasStatus {
			status = fmt.Sprintf("%d", row.StatusRaw)
		}
		rx := "-"
		if row.HasRx {
			rx = fmt.Sprintf("%d", row.RxRaw)
		}
		_, _ = fmt.Fprintf(out, "%d\t%s\t%s\t%s\t%s\n", row.IfIndex, dash(row.IfLabel), status, rx, dash(row.Name))
	}
	_ = out.Flush()
	fmt.Println()
}

// Unknown #1: how the ifIndex encodes PON port and ONU id.
func printIfIndexFindings(report connectivity.HSGQProbeReport) {
	fmt.Println("== Unknown 1: ifIndex layout (hsgqLocation) ==")

	labelled := 0
	for _, row := range report.Rows {
		if row.IfLabel != "" {
			labelled++
		}
	}

	if labelled == 0 {
		fmt.Println("No IF-MIB label came back, so the PON position cannot be read off the")
		fmt.Println("device's own naming. Compare these against `show onu` on the OLT CLI:")
	} else {
		fmt.Println("Match each label against its ifIndex to derive the packing, then implement")
		fmt.Println("it in hsgqLocation and set Slot/Port instead of leaving them zero:")
	}

	out := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, row := range report.Rows {
		_, _ = fmt.Fprintf(out, "  %s\t%s\n", row.IfIndexBreakdown(), dash(row.IfLabel))
	}
	_ = out.Flush()
	fmt.Println()
}

// Unknown #2: which raw status value means online.
func printPolarityFinding(report connectivity.HSGQProbeReport) {
	fmt.Println("== Unknown 2: status polarity (hsgqDecodeStatus) ==")

	if !report.PolarityConfident {
		fmt.Println("INCONCLUSIVE. Status did not split cleanly against optical presence, so")
		fmt.Println("hsgqDecodeStatus must keep returning PhaseStateUnknown.")
		fmt.Println("Re-run when at least one ONU is up and one is genuinely down; if a group")
		fmt.Println("mixes ONUs with and without signal, this OID is not a simple up/down flag.")
		fmt.Println()
		return
	}

	fmt.Printf("Status %d means ONLINE: every ONU reporting it has a plausible optical\n", report.OnlineStatusValue)
	fmt.Println("level, and every ONU with another value has none.")
	fmt.Printf("Change hsgqDecodeStatus to map %d to PhaseStateOnline and the rest to\n", report.OnlineStatusValue)
	fmt.Println("PhaseStateOffline, and update TestHSGQDecodeStatusNeverGuessesPolarity.")
	fmt.Println()
}

// Unknown #4: the no-signal sentinel.
func printSentinelFinding(report connectivity.HSGQProbeReport) {
	fmt.Println("== Unknown 4: no-signal sentinel (hsgqDecodePower) ==")

	if len(report.SentinelCandidates) == 0 {
		fmt.Println("Every optical reading was a plausible level, so no sentinel was observed.")
		fmt.Println("Re-run with a dark or unplugged ONU present to capture it.")
		fmt.Println()
		return
	}

	fmt.Println("Raw rx values that cannot be real GPON levels, most frequent first. The one")
	fmt.Println("repeated across dark ONUs is the sentinel; name it in hsgqDecodePower rather")
	fmt.Println("than relying on the current plausibility window:")
	for _, candidate := range report.SentinelCandidates {
		fmt.Printf("  %d\treported by %d ONU(s)\n", candidate.Raw, candidate.Count)
	}
	fmt.Println()
}

// ONU identity: on EPON this is the MAC, and there is no byte-order ambiguity.
func printSerialFinding(report connectivity.HSGQProbeReport) {
	fmt.Println("== ONU identity (MAC) ==")
	fmt.Println("A wrong length here means the column being read is not a MAC on this")
	fmt.Println("firmware, which is the same class of mismatch that invalidated the")
	fmt.Println("original third-party OID set.")

	out := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(out, "  IFINDEX\tMAC\tRAW BYTES")

	shown := 0
	for _, row := range report.Rows {
		mac, hex := row.MACRendering()
		if mac == "" && hex == "" {
			continue
		}
		_, _ = fmt.Fprintf(out, "  %d\t%s\t%s\n", row.IfIndex, dash(mac), hex)
		shown++
	}
	_ = out.Flush()

	if shown == 0 {
		fmt.Println("  No MAC decoded; check the raw dump above.")
	}
	fmt.Println()
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
