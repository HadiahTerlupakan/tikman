package database

import (
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/tikman/olt-provisioning/internal/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// connectRetryWindow is how long Connect keeps waiting for a Postgres that does
// not answer yet. At boot Docker starts every container at once and Postgres
// answered about a second after the others had exited on their first attempt.
// api then restarted just as worker and trapd did, they could not join its
// network namespace, and Docker never retries that: after the reboot of
// 2026-09-12 polling and traps stayed down for more than two days. A process
// that waits instead of exiting never gives that race a chance to run.
const (
	connectRetryWindow   = 2 * time.Minute
	connectRetryInterval = time.Second
)

// pgCannotConnectNow is what Postgres answers while it is still starting up or
// recovering, which is exactly the moment a boot reaches it.
const pgCannotConnectNow = "57P03"

// newDialector builds the Postgres dialector with implicit prepared statements
// turned off.
//
// The driver otherwise caches a query plan per connection, and a migration that
// adds a column changes the result type of "SELECT * FROM olts" underneath it.
// Postgres then rejects every later use of that connection with "cached plan
// must not change result type", which does not heal: the pool keeps the
// connection, so the worker stopped listing OLTs entirely and no discovery ran
// until it was restarted by hand. The simple protocol costs a little
// per-statement performance and removes the failure mode.
func newDialector(dsn string) gorm.Dialector {
	return postgres.New(postgres.Config{DSN: dsn, PreferSimpleProtocol: true})
}

// Connect opens the Postgres pool, waiting up to connectRetryWindow for a
// server that is not answering yet.
func Connect(cfg *config.Config) (*gorm.DB, error) {
	return connect(cfg, connectRetryWindow)
}

func connect(cfg *config.Config, retryWindow time.Duration) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%d sslmode=disable TimeZone=UTC",
		cfg.DBHost, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBPort,
	)

	db, err := openWithRetry(func() (*gorm.DB, error) { return open(dsn) }, retryWindow, connectRetryInterval)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database instance: %w", err)
	}

	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

func open(dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(newDialector(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil && db != nil {
		// A failed attempt still holds a pool with its own opener goroutine, and
		// a boot can make a hundred attempts.
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	}
	return db, err
}

// openWithRetry repeats attempt until it succeeds, fails in a way waiting
// cannot fix, or the window closes.
func openWithRetry(attempt func() (*gorm.DB, error), window, interval time.Duration) (*gorm.DB, error) {
	deadline := time.Now().Add(window)
	for {
		db, err := attempt()
		if err == nil || !notAnsweringYet(err) || time.Now().Add(interval).After(deadline) {
			return db, err
		}
		time.Sleep(interval)
	}
}

// notAnsweringYet tells a Postgres that cannot be reached, or is still starting,
// from one that answered and refused. Bad credentials or a missing database
// will not fix themselves.
func notAnsweringYet(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == pgCannotConnectNow
	}
	return true
}
