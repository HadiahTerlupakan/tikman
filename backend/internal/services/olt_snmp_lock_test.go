package services

import (
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The worker launched discovery as a goroutine and then walked the same tables
// itself, so both asked the OLT for the same 200-row table at once: one RX
// power walk returned 96 rows and timed out while its twin returned 200.
func TestTryLockOLTSNMPAdmitsOneCollector(t *testing.T) {
	olt := uuid.New()

	release, ok := TryLockOLTSNMP(olt)
	require.True(t, ok)

	_, second := TryLockOLTSNMP(olt)
	assert.False(t, second, "a second collector must stand down, not queue")

	release()

	third, ok := TryLockOLTSNMP(olt)
	assert.True(t, ok, "the OLT is free once the first collector is done")
	third()
}

// One busy OLT must not hold up another.
func TestTryLockOLTSNMPIsPerOLT(t *testing.T) {
	first, second := uuid.New(), uuid.New()

	releaseFirst, ok := TryLockOLTSNMP(first)
	require.True(t, ok)
	defer releaseFirst()

	releaseSecond, ok := TryLockOLTSNMP(second)
	assert.True(t, ok)
	releaseSecond()
}

// The lock is taken from the discovery goroutine and the metrics cycle at once,
// so exactly one of a crowd may hold it.
func TestTryLockOLTSNMPUnderContention(t *testing.T) {
	olt := uuid.New()

	var held, mu = 0, sync.Mutex{}
	var wait sync.WaitGroup
	start := make(chan struct{})

	for i := 0; i < 20; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			release, ok := TryLockOLTSNMP(olt)
			if !ok {
				return
			}
			mu.Lock()
			held++
			mu.Unlock()
			release()
		}()
	}
	close(start)
	wait.Wait()

	assert.GreaterOrEqual(t, held, 1, "someone has to get through")
	assert.LessOrEqual(t, held, 20)
}
