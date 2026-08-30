package connectivity

import (
	"errors"
	"log"

	"github.com/gosnmp/gosnmp"
)

// snmpWalker is the part of a GoSNMP client this package reads tables with.
// Narrow enough that a test can supply an agent which refuses GETBULK.
type snmpWalker interface {
	BulkWalk(rootOid string, fn gosnmp.WalkFunc) error
	Walk(rootOid string, fn gosnmp.WalkFunc) error
}

// callbackError marks a failure as ours rather than the agent's, so a decode
// bug is never mistaken for a refusal to serve GETBULK.
type callbackError struct{ err error }

func (c callbackError) Error() string { return c.err.Error() }
func (c callbackError) Unwrap() error { return c.err }

// bulkWalk reads a table with GETBULK, falling back to GETNEXT if the agent
// refuses.
//
// GETBULK returns many values per request where GETNEXT returns one. Measured
// against a ZTE C320 carrying 655 ONUs, the same table takes 3.9s by GETBULK
// against 11-20s by GETNEXT. Not every agent serves it, and a refusal costs one
// wasted request rather than the table.
//
// The fallback is logged. A silent downgrade would leave the poller quietly
// slow with nothing on record to explain why.
func bulkWalk(w snmpWalker, oid string, fn gosnmp.WalkFunc) error {
	wrapped := func(pdu gosnmp.SnmpPDU) error {
		if err := fn(pdu); err != nil {
			return callbackError{err}
		}
		return nil
	}

	err := w.BulkWalk(oid, wrapped)
	if err == nil {
		return nil
	}
	if unwrapped, ours := callbackFailure(err); ours {
		return unwrapped
	}

	log.Printf("[SNMP] GETBULK refused for %s (%v); falling back to GETNEXT for this walk", oid, err)

	if err := w.Walk(oid, wrapped); err != nil {
		if unwrapped, ours := callbackFailure(err); ours {
			return unwrapped
		}
		return err
	}
	return nil
}

// callbackFailure reports whether an error came from the caller's callback, and
// returns it unwrapped so the caller sees the error it raised rather than one
// of ours wrapped around it.
func callbackFailure(err error) (error, bool) {
	var cb callbackError
	if errors.As(err, &cb) {
		return cb.err, true
	}
	return err, false
}

// defaultMaxRepetitions is how many values one GETBULK asks for.
//
// Measured with cmd/snmpbench against a ZTE C320 carrying 655 ONUs: 10 read the
// table in 3.9s where GETNEXT took 11-20s, and larger counts were consistently
// slower — 25 took 4.9s, 50 took 5.4s. At 10 repetitions the network accounts
// for about half a second of that walk, so the rest is the agent assembling
// responses, and asking it for more per request costs more than the round trips
// it saves. Every count returned the same 655 values, so this is a speed
// setting, not a correctness one.
const defaultMaxRepetitions uint8 = 10

var maxRepetitions = defaultMaxRepetitions

// SetMaxRepetitions overrides how many values one GETBULK asks for. A zero is
// ignored: gosnmp would then return a single value per request, which is the
// GETNEXT cost this exists to avoid.
func SetMaxRepetitions(n uint8) {
	if n == 0 {
		n = defaultMaxRepetitions
	}
	maxRepetitions = n
}
