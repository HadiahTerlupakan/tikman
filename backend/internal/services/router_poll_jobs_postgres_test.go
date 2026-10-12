package services

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

// Same reason as the OLT queue's Postgres test: only a real Postgres has FOR
// UPDATE SKIP LOCKED and concurrent writers.
func TestConcurrentRouterClaimsNeverHandOutTheSameJob(t *testing.T) {
	db := setupPollJobPostgres(t)
	require.NoError(t, db.Exec("DROP TABLE IF EXISTS router_poll_jobs").Error)
	require.NoError(t, db.AutoMigrate(&models.RouterPollJob{}))
	service := NewRouterPollJobService(db)

	const jobs = 8
	for i := 0; i < jobs; i++ {
		seedRouterJob(t, service, models.RouterPollKindBGP, time.Now().Add(-time.Minute))
	}

	var mu sync.Mutex
	seen := map[string]bool{}
	var wg sync.WaitGroup
	for w := 0; w < jobs; w++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			job, err := service.Claim("worker")
			require.NoError(t, err)
			require.NotNil(t, job)
			mu.Lock()
			defer mu.Unlock()
			require.False(t, seen[job.RouterID.String()], "job handed out twice")
			seen[job.RouterID.String()] = true
		}(w)
	}
	wg.Wait()
	require.Len(t, seen, jobs)
}
