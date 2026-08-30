package connectivity

import (
	"errors"
	"testing"

	"github.com/gosnmp/gosnmp"
	"github.com/stretchr/testify/require"
)

type fakeWalker struct {
	bulkErr   error
	bulkCalls int
	walkCalls int
	deliver   int
}

func (f *fakeWalker) BulkWalk(oid string, fn gosnmp.WalkFunc) error {
	f.bulkCalls++
	if f.bulkErr != nil {
		return f.bulkErr
	}
	return f.emit(oid, fn)
}

func (f *fakeWalker) Walk(oid string, fn gosnmp.WalkFunc) error {
	f.walkCalls++
	return f.emit(oid, fn)
}

func (f *fakeWalker) emit(oid string, fn gosnmp.WalkFunc) error {
	for i := 0; i < f.deliver; i++ {
		if err := fn(gosnmp.SnmpPDU{Name: oid + ".1"}); err != nil {
			return err
		}
	}
	return nil
}

func TestBulkWalkUsesGetBulkWhenTheAgentAllowsIt(t *testing.T) {
	// Measured against a ZTE C320: GETBULK reads the same table three to four
	// times faster than GETNEXT, which is the whole point of this path.
	w := &fakeWalker{deliver: 3}
	seen := 0

	require.NoError(t, bulkWalk(w, "1.2.3", func(gosnmp.SnmpPDU) error { seen++; return nil }))

	require.Equal(t, 1, w.bulkCalls)
	require.Equal(t, 0, w.walkCalls, "GETNEXT was used even though GETBULK worked")
	require.Equal(t, 3, seen)
}

func TestBulkWalkFallsBackWhenTheAgentRefusesGetBulk(t *testing.T) {
	// Not every agent serves GETBULK. Losing a whole table over that would be
	// worse than reading it the slow way.
	w := &fakeWalker{bulkErr: errors.New("request too big"), deliver: 2}
	seen := 0

	require.NoError(t, bulkWalk(w, "1.2.3", func(gosnmp.SnmpPDU) error { seen++; return nil }))

	require.Equal(t, 1, w.bulkCalls)
	require.Equal(t, 1, w.walkCalls)
	require.Equal(t, 2, seen)
}

func TestBulkWalkDoesNotRetryWhenTheCallbackItselfFailed(t *testing.T) {
	// A decode error in the callback is our bug, not the agent's. Retrying with
	// GETNEXT would read the whole table again and fail in exactly the same way.
	w := &fakeWalker{deliver: 1}
	boom := errors.New("decode failed")

	err := bulkWalk(w, "1.2.3", func(gosnmp.SnmpPDU) error { return boom })

	require.ErrorIs(t, err, boom)
	require.Equal(t, 0, w.walkCalls)
}

func TestBulkWalkReportsAFallbackThatAlsoFailed(t *testing.T) {
	// An agent that answers neither way is unreachable, and the caller has to
	// hear that rather than receive an empty table as though it were the truth.
	w := &fakeWalker{bulkErr: errors.New("too big")}
	w.deliver = 0
	failing := errors.New("timeout")

	err := bulkWalk(&refusingWalker{fake: w, walkErr: failing}, "1.2.3", func(gosnmp.SnmpPDU) error { return nil })

	require.ErrorIs(t, err, failing)
}

type refusingWalker struct {
	fake    *fakeWalker
	walkErr error
}

func (r *refusingWalker) BulkWalk(oid string, fn gosnmp.WalkFunc) error {
	return r.fake.BulkWalk(oid, fn)
}

func (r *refusingWalker) Walk(string, gosnmp.WalkFunc) error {
	return r.walkErr
}

func TestClientCarriesTheConfiguredRepetitionCount(t *testing.T) {
	// A GETBULK with a repetition count of zero returns one value per request,
	// which is the GETNEXT cost this whole path exists to remove.
	SetMaxRepetitions(25)
	t.Cleanup(func() { SetMaxRepetitions(defaultMaxRepetitions) })

	client, err := newSNMPClient("127.0.0.1", "public", 161)
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Conn.Close() })

	require.Equal(t, uint32(25), client.MaxRepetitions)
}

func TestRepetitionCountNeverFallsToZero(t *testing.T) {
	SetMaxRepetitions(0)
	t.Cleanup(func() { SetMaxRepetitions(defaultMaxRepetitions) })

	client, err := newSNMPClient("127.0.0.1", "public", 161)
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Conn.Close() })

	require.Equal(t, uint32(defaultMaxRepetitions), client.MaxRepetitions)
}
