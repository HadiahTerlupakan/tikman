package services

import (
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// setupPollJobPostgres connects to the Postgres the queue is actually written
// for. SQLite has no FOR UPDATE SKIP LOCKED and no concurrent writers, so the
// behaviour that matters most here — several workers claiming at once and each
// coming away with a different job — cannot be exercised anywhere else.
//
// A missing DSN fails rather than skips under CI. A test that quietly never
// runs is the failure mode this whole project keeps uncovering, and it would be
// a poor thing to introduce while fixing it.
func setupPollJobPostgres(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		if os.Getenv("CI") != "" {
			t.Fatal("TEST_POSTGRES_DSN is unset under CI; the queue's concurrency is then never tested")
		}
		t.Skip("set TEST_POSTGRES_DSN to run the queue's concurrency tests against Postgres")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)

	// Each test owns the table: these run against a throwaway database, and a
	// leftover row from a previous test would decide which job gets claimed.
	require.NoError(t, db.Exec("DROP TABLE IF EXISTS olt_poll_jobs").Error)
	require.NoError(t, db.AutoMigrate(&models.OLTPollJob{}))

	return db
}

func seedJobs(t *testing.T, db *gorm.DB, count int) {
	t.Helper()
	due := time.Now().Add(-time.Minute)
	for i := 0; i < count; i++ {
		require.NoError(t, db.Create(&models.OLTPollJob{
			OLTID: uuid.New(), Kind: models.PollKindStatus, DueAt: due,
		}).Error)
	}
}

func TestConcurrentClaimsNeverHandOutTheSameJob(t *testing.T) {
	db := setupPollJobPostgres(t)
	service := NewPollJobService(db)

	const workers = 8
	seedJobs(t, db, workers)

	var mu sync.Mutex
	claimedBy := map[uuid.UUID]string{}

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			worker := uuid.New().String()
			job, err := service.Claim(worker)
			if err != nil || job == nil {
				return
			}
			mu.Lock()
			defer mu.Unlock()
			previous, taken := claimedBy[job.OLTID]
			require.False(t, taken, "job %s went to both %s and %s", job.OLTID, previous, worker)
			claimedBy[job.OLTID] = worker
		}(i)
	}
	wg.Wait()

	require.Len(t, claimedBy, workers, "workers queued behind each other instead of taking a job each")
}

func TestConcurrentClaimsOnOneJobLetExactlyOneThrough(t *testing.T) {
	// The case the lease exists for: more workers than work. Every worker but
	// one must come away empty rather than a second one reading the same
	// chassis.
	db := setupPollJobPostgres(t)
	service := NewPollJobService(db)

	seedJobs(t, db, 1)

	var mu sync.Mutex
	claims := 0

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			job, err := service.Claim(uuid.New().String())
			if err != nil || job == nil {
				return
			}
			mu.Lock()
			claims++
			mu.Unlock()
		}()
	}
	wg.Wait()

	require.Equal(t, 1, claims, "one job was claimed %d times", claims)
}
