package database

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tikman/olt-provisioning/internal/config"
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
