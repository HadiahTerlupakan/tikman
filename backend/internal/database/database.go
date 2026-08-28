package database

import (
	"fmt"
	"time"

	"github.com/tikman/olt-provisioning/internal/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

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

func Connect(cfg *config.Config) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%d sslmode=disable TimeZone=UTC",
		cfg.DBHost, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBPort,
	)

	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	}

	db, err := gorm.Open(newDialector(dsn), gormConfig)
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
