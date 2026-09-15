package database

import (
	"errors"
	"fmt"
	"net"
	"syscall"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestConnect_InvalidConfig(t *testing.T) {
	cfg := &config.Config{
		DBHost:     "invalid-host",
		DBPort:     5432,
		DBUser:     "test",
		DBPassword: "test",
		DBName:     "test",
	}

	db, err := connect(cfg, 0)
	assert.Error(t, err)
	assert.Nil(t, db)
}

// At boot Docker starts every container at once, and Postgres answered about a
// second after api, worker, trapd and wa had each exited on their first
// attempt. A worker or trapd restarted while api was itself restarting could not
// join api's network namespace, and Docker never retried that: after the reboot
// of 2026-09-12, polling and traps stayed down for more than two days.
func TestOpenWithRetryWaitsForAPostgresThatIsNotAnsweringYet(t *testing.T) {
	notYet := map[string]error{
		"connection refused":  &net.OpError{Op: "dial", Net: "tcp", Err: syscall.ECONNREFUSED},
		"name not registered": &net.DNSError{Err: "no such host", Name: "postgres", IsNotFound: true},
		"still starting up":   fmt.Errorf("server error: %w", &pgconn.PgError{Code: "57P03"}),
	}
	for name, failure := range notYet {
		t.Run(name, func(t *testing.T) {
			attempts := 0
			db, err := openWithRetry(func() (*gorm.DB, error) {
				attempts++
				if attempts < 3 {
					return nil, failure
				}
				return &gorm.DB{}, nil
			}, time.Second, time.Millisecond)

			require.NoError(t, err)
			assert.NotNil(t, db)
			assert.Equal(t, 3, attempts)
		})
	}
}

// A server that answered and refused will not change its mind, so waiting on
// it would only hold back the real error for the whole retry window.
func TestOpenWithRetryReturnsARefusalAtOnce(t *testing.T) {
	rejected := fmt.Errorf("server error: %w", &pgconn.PgError{Code: "28P01", Message: "password authentication failed"})
	attempts := 0

	_, err := openWithRetry(func() (*gorm.DB, error) {
		attempts++
		return nil, rejected
	}, time.Minute, time.Millisecond)

	require.ErrorIs(t, err, rejected)
	assert.Equal(t, 1, attempts)
}

func TestOpenWithRetryGivesUpWhenTheWindowCloses(t *testing.T) {
	refused := &net.OpError{Op: "dial", Net: "tcp", Err: syscall.ECONNREFUSED}
	attempts := 0
	started := time.Now()

	_, err := openWithRetry(func() (*gorm.DB, error) {
		attempts++
		return nil, refused
	}, 30*time.Millisecond, 5*time.Millisecond)

	require.True(t, errors.Is(err, syscall.ECONNREFUSED))
	assert.Greater(t, attempts, 1)
	assert.Less(t, time.Since(started), time.Second)
}

// The fakes above only hold if the driver's real errors wrap the way they do.
func TestTheDriverReportsAnUnreachableServerAsNotAnsweringYet(t *testing.T) {
	_, err := open("host=127.0.0.1 port=1 user=x password=x dbname=x sslmode=disable connect_timeout=2")

	require.Error(t, err)
	assert.True(t, notAnsweringYet(err), "a refused dial must be retried: %v", err)
}

// A migration that adds a column changes the result type of a cached plan, and
// Postgres then fails every later query on that connection with "cached plan
// must not change result type". The pool keeps the connection, so the worker
// stopped polling entirely until restarted. The simple protocol is what
// prevents it, so reverting to postgres.Open would bring the outage back.
func TestNewDialectorDisablesPreparedStatements(t *testing.T) {
	dialector, ok := newDialector("host=localhost user=x dbname=y port=5432").(*postgres.Dialector)

	assert.True(t, ok, "expected the Postgres dialector")
	assert.True(t, dialector.PreferSimpleProtocol,
		"implicit prepared statements must stay off across migrations")
}
