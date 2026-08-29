package services

import (
	"sync"

	"github.com/google/uuid"
)

// oltSNMPLocks serialises SNMP access to one OLT within this process.
var oltSNMPLocks sync.Map

// TryLockOLTSNMP reserves an OLT for SNMP work, reporting false when another
// collector already holds it.
//
// The worker launches the discovery poll as a goroutine and then walks the same
// tables itself, so both asked the OLT for the same 200-row table at once and
// slowed each other into timeouts: one RX power walk returned 96 rows and gave
// up while its twin returned 200. Callers skip rather than queue, because the
// two collect the same metrics — waiting would only repeat work the other has
// just done.
func TryLockOLTSNMP(oltID uuid.UUID) (release func(), ok bool) {
	value, _ := oltSNMPLocks.LoadOrStore(oltID, &sync.Mutex{})
	mutex := value.(*sync.Mutex)
	if !mutex.TryLock() {
		return func() {}, false
	}
	return mutex.Unlock, true
}
