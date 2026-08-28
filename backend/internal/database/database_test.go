package database

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tikman/olt-provisioning/internal/config"
	"gorm.io/driver/postgres"
)

func TestConnect_InvalidConfig(t *testing.T) {
	cfg := &config.Config{
		DBHost:     "invalid-host",
		DBPort:     5432,
		DBUser:     "test",
		DBPassword: "test",
		DBName:     "test",
	}

	db, err := Connect(cfg)
	assert.Error(t, err)
	assert.Nil(t, db)
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
